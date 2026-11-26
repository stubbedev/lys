package main

import (
	"fmt"
	"io/fs"
	"regexp"
	"strings"
)

var (
	execKey        = regexp.MustCompile(`^Exec[A-Za-z]*$`)
	getExeNamed    = regexp.MustCompile(`getExe'\s+[^\s()]+\s+"([^"]+)"`)
	pathOption     = regexp.MustCompile(`--(?:suffix|prefix)\s+PATH\s+:\s+(.*?)\s*\\?$`)
	unitExecLine   = regexp.MustCompile(`^\s*(Exec[A-Za-z]*)=(.*)$`)
	placeholder    = regexp.MustCompile(`^@([A-Za-z0-9_]+)@(?:\s|$)`)
	unitFileSuffix = []string{".service", ".service.in"}
)

func packagingSites(fsys fs.FS, nix []*nixFile, lys map[string]bool) ([]site[packagingKey], error) {
	var sites []site[packagingKey]
	add := func(file string, line int, what, program string) {
		if lys[program] {
			return
		}
		sites = append(sites, site[packagingKey]{at: position{file, line}, what: what, key: packagingKey{file, program}})
	}
	for _, f := range nix {
		if err := nixSites(f, add); err != nil {
			return nil, err
		}
	}
	err := fs.WalkDir(fsys, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name != "." && skippedDir(d.Name()) {
				return fs.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || !isUnitFile(name) {
			return nil
		}
		src, err := fs.ReadFile(fsys, name)
		if err != nil {
			return fmt.Errorf("read %s: %w", name, err)
		}
		unitSites(name, src, add)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk the unit files: %w", err)
	}
	return sites, nil
}

func skippedDir(name string) bool {
	return name == "testdata" || name == "vendor" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")
}

func isUnitFile(name string) bool {
	for _, suffix := range unitFileSuffix {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

type addSite func(file string, line int, what, program string)

func nixSites(f *nixFile, add addSite) error {
	bindings, err := f.bindings()
	if err != nil {
		return err
	}
	for _, b := range bindings {
		last := b.key[strings.LastIndexByte(b.key, '.')+1:]
		value := f.text(b.valueStart, b.valueEnd)
		switch {
		case last == "systemPackages" || b.key == "home.packages" || strings.HasSuffix(b.key, ".home.packages"):
			add(f.path, f.line(b.at), b.key+" installs", value)
		case last == "path" && strings.HasPrefix(value, "["):
			add(f.path, f.line(b.at), "a unit's path adds", value)
		case execKey.MatchString(last):
			add(f.path, f.line(b.at), last+" runs", f.execProgram(b))
		}
	}
	for i := range len(f.src) {
		if !f.code(i) || !f.wordStart(i) || f.word(i) != "makeBinPath" {
			continue
		}
		open := f.skipSpace(i + len("makeBinPath"))
		if open >= len(f.src) || f.src[open] != '[' {
			return fmt.Errorf("%s:%d: makeBinPath takes no literal list", f.path, f.line(i))
		}
		end, err := f.expressionEnd(open + 1)
		if err != nil {
			return err
		}
		add(f.path, f.line(i), "makeBinPath puts on PATH", f.text(open+1, end))
	}
	for _, s := range f.outerStrings() {
		f.stringSites(s, add)
	}
	return nil
}

func (f *nixFile) execProgram(b nixBinding) string {
	raw := f.src[b.valueStart:b.valueEnd]
	if m := getExeNamed.FindSubmatch(raw); m != nil {
		return string(m[1])
	}
	for _, s := range f.outerStrings() {
		if s.start < b.valueStart || s.end > b.valueEnd {
			continue
		}
		if head := commandHead(string(f.src[s.start:s.end])); head != "" {
			return head
		}
	}
	return f.text(b.valueStart, b.valueEnd)
}

func (f *nixFile) outerStrings() []span {
	var out []span
	for _, s := range f.strings {
		if len(out) > 0 && s.start < out[len(out)-1].end {
			continue
		}
		out = append(out, s)
	}
	return out
}

func (f *nixFile) stringSites(s span, add addSite) {
	offset := s.start
	for line := range strings.Lines(string(f.src[s.start:s.end])) {
		text := strings.TrimRight(line, "\r\n")
		at := f.line(offset)
		offset += len(line)
		if m := unitExecLine.FindStringSubmatch(text); m != nil {
			if head := commandHead(m[2]); head != "" {
				add(f.path, at, m[1]+" runs", head)
			}
			continue
		}
		if m := pathOption.FindStringSubmatch(text); m != nil && !strings.Contains(m[1], "makeBinPath") {
			add(f.path, at, "a wrapper's PATH adds", m[1])
		}
	}
}

func unitSites(file string, src []byte, add addSite) {
	line := 0
	for text := range strings.Lines(string(src)) {
		line++
		m := unitExecLine.FindStringSubmatch(strings.TrimRight(text, "\r\n"))
		if m == nil {
			continue
		}
		if head := commandHead(m[2]); head != "" {
			add(file, line, m[1]+" runs", head)
		}
	}
}

func commandHead(command string) string {
	command = strings.TrimLeft(strings.TrimSpace(command), "-:+!|")
	if m := placeholder.FindStringSubmatch(command); m != nil {
		return strings.ReplaceAll(m[1], "_", "-")
	}
	command = strings.TrimPrefix(command, "@")
	depth := 0
	end := 0
	for end < len(command) {
		switch {
		case strings.HasPrefix(command[end:], "${"):
			depth++
			end += 2
			continue
		case command[end] == '}' && depth > 0:
			depth--
		case depth == 0 && strings.IndexByte(" \t\n\"", command[end]) >= 0:
			return baseProgram(command[:end])
		}
		end++
	}
	return baseProgram(command[:end])
}

func baseProgram(word string) string {
	slash := strings.LastIndexByte(word, '/')
	if slash < 0 || slash == len(word)-1 || strings.ContainsAny(word[slash:], "{}") {
		return word
	}
	return word[slash+1:]
}
