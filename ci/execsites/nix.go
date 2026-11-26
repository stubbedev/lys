package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

const flakeFile = "flake.nix"

type byteKind uint8

const (
	codeByte byteKind = iota
	stringByte
	commentByte
)

type span struct {
	start, end int
}

type nixFile struct {
	path    string
	src     []byte
	kind    []byteKind
	strings []span
}

func readNix(fsys fs.FS, file string) (*nixFile, error) {
	src, err := fs.ReadFile(fsys, file)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", file, err)
	}
	kind, spans, err := classify(src)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	return &nixFile{path: file, src: src, kind: kind, strings: spans}, nil
}

type frame uint8

const (
	codeFrame frame = iota
	doubleQuoted
	indented
)

func classify(src []byte) ([]byteKind, []span, error) {
	kind := make([]byteKind, len(src))
	stack := []frame{codeFrame}
	depth := []int{0}
	var spans []span
	var open []int
	push := func(f frame, start int) {
		stack, depth = append(stack, f), append(depth, 0)
		if f != codeFrame {
			open = append(open, start)
		}
	}
	pop := func(end int) {
		if stack[len(stack)-1] != codeFrame {
			spans = append(spans, span{open[len(open)-1], end})
			open = open[:len(open)-1]
		}
		stack, depth = stack[:len(stack)-1], depth[:len(depth)-1]
	}
	mark := func(i, n int, k byteKind) {
		for j := i; j < i+n && j < len(src); j++ {
			kind[j] = k
		}
	}
	for i := 0; i < len(src); {
		top := stack[len(stack)-1]
		rest := src[i:]
		switch top {
		case codeFrame:
			switch {
			case rest[0] == '#':
				end := bytes.IndexByte(rest, '\n')
				if end < 0 {
					end = len(rest)
				}
				mark(i, end, commentByte)
				i += end
			case bytes.HasPrefix(rest, []byte("/*")):
				end := bytes.Index(rest[2:], []byte("*/"))
				if end < 0 {
					return nil, nil, errors.New("an unterminated /* comment")
				}
				mark(i, end+4, commentByte)
				i += end + 4
			case rest[0] == '"':
				push(doubleQuoted, i+1)
				i++
			case bytes.HasPrefix(rest, []byte("''")):
				push(indented, i+2)
				i += 2
			case rest[0] == '{':
				depth[len(depth)-1]++
				i++
			case rest[0] == '}':
				if depth[len(depth)-1] == 0 && len(stack) > 1 {
					pop(i)
				} else {
					depth[len(depth)-1]--
				}
				i++
			default:
				i++
			}
		case doubleQuoted:
			switch {
			case rest[0] == '\\':
				mark(i, 2, stringByte)
				i += 2
			case rest[0] == '"':
				pop(i)
				i++
			case bytes.HasPrefix(rest, []byte("${")):
				push(codeFrame, i)
				i += 2
			default:
				kind[i] = stringByte
				i++
			}
		case indented:
			switch {
			case bytes.HasPrefix(rest, []byte("'''")), bytes.HasPrefix(rest, []byte("''$")), bytes.HasPrefix(rest, []byte("''\\")):
				mark(i, 3, stringByte)
				i += 3
			case bytes.HasPrefix(rest, []byte("''")):
				pop(i)
				i += 2
			case bytes.HasPrefix(rest, []byte("${")):
				push(codeFrame, i)
				i += 2
			default:
				kind[i] = stringByte
				i++
			}
		}
	}
	if len(stack) != 1 {
		return nil, nil, errors.New("an unterminated string or interpolation")
	}
	slices.SortFunc(spans, func(a, b span) int { return a.start - b.start })
	return kind, spans, nil
}

func (f *nixFile) line(offset int) int {
	return 1 + bytes.Count(f.src[:offset], []byte("\n"))
}

func (f *nixFile) code(offset int) bool {
	return f.kind[offset] == codeByte
}

func (f *nixFile) text(start, end int) string {
	var b strings.Builder
	space := false
	for i := start; i < end; i++ {
		c := f.src[i]
		if f.kind[i] == commentByte {
			continue
		}
		if f.kind[i] == codeByte && (c == ' ' || c == '\t' || c == '\n' || c == '\r') {
			space = true
			continue
		}
		if space && b.Len() > 0 {
			b.WriteByte(' ')
		}
		space = false
		b.WriteByte(c)
	}
	return b.String()
}

func identByte(c byte) bool {
	return c == '_' || c == '\'' || c == '-' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func (f *nixFile) word(at int) string {
	end := at
	for end < len(f.src) && f.code(end) && identByte(f.src[end]) {
		end++
	}
	return string(f.src[at:end])
}

func (f *nixFile) wordStart(at int) bool {
	return at == 0 || !f.code(at-1) || !identByte(f.src[at-1])
}

func (f *nixFile) skipSpace(at int) int {
	for at < len(f.src) && (f.kind[at] == commentByte || f.code(at) && strings.IndexByte(" \t\r\n", f.src[at]) >= 0) {
		at++
	}
	return at
}

func (f *nixFile) expressionEnd(start int) (int, error) {
	depth := 0
	pending := map[int]int{}
	for i := start; i < len(f.src); i++ {
		if !f.code(i) {
			continue
		}
		c := f.src[i]
		switch {
		case c == '(' || c == '[' || c == '{':
			depth++
		case c == ')' || c == ']' || c == '}':
			if depth == 0 {
				return i, nil
			}
			depth--
		case c == ';':
			if pending[depth] > 0 {
				pending[depth]--
				continue
			}
			if depth == 0 {
				return i, nil
			}
		case f.wordStart(i) && identByte(c):
			switch f.word(i) {
			case "let":
				depth++
			case "in":
				depth--
			case "with", "assert":
				pending[depth]++
			}
		}
	}
	return 0, fmt.Errorf("%s:%d: the expression has no end", f.path, f.line(start))
}

func (f *nixFile) attrPathBefore(eq int) (string, int) {
	end := eq
	for end > 0 && f.code(end-1) && (f.src[end-1] == ' ' || f.src[end-1] == '\t') {
		end--
	}
	start := end
	for start > 0 && f.code(start-1) && (identByte(f.src[start-1]) || f.src[start-1] == '.') {
		start--
	}
	return string(f.src[start:end]), start
}

type nixBinding struct {
	key        string
	at         int
	valueStart int
	valueEnd   int
}

func (f *nixFile) bindings() ([]nixBinding, error) {
	var out []nixBinding
	for i := 0; i < len(f.src); i++ {
		if !f.code(i) || f.src[i] != '=' {
			continue
		}
		if i+1 < len(f.src) && f.src[i+1] == '=' || i > 0 && strings.IndexByte("=!<>", f.src[i-1]) >= 0 {
			i++
			continue
		}
		key, at := f.attrPathBefore(i)
		if key == "" {
			continue
		}
		start := f.skipSpace(i + 1)
		end, err := f.expressionEnd(start)
		if err != nil {
			return nil, err
		}
		out = append(out, nixBinding{key: key, at: at, valueStart: start, valueEnd: end})
	}
	return out, nil
}

var relativeNix = regexp.MustCompile(`(?:^|[^A-Za-z0-9_./'-])(\.\.?/[A-Za-z0-9_./-]+\.nix)\b`)

var checkImport = regexp.MustCompile(`\(\s*checks\s+"[^"]*"\s+(\./[A-Za-z0-9_./-]+\.nix)\s*\)`)

func (f *nixFile) references() []string {
	checks := map[int]bool{}
	for _, m := range checkImport.FindAllSubmatchIndex(f.src, -1) {
		checks[m[2]] = true
	}
	var refs []string
	for _, m := range relativeNix.FindAllSubmatchIndex(f.src, -1) {
		if !f.code(m[2]) || checks[m[2]] {
			continue
		}
		refs = append(refs, path.Join(path.Dir(f.path), string(f.src[m[2]:m[3]])))
	}
	return refs
}

var devShellsAttr = regexp.MustCompile(`\bdevShells\b`)

func (f *nixFile) definesDevShells() bool {
	for _, m := range devShellsAttr.FindAllIndex(f.src, -1) {
		if f.code(m[0]) {
			return true
		}
	}
	return false
}

func runtimeNix(fsys fs.FS) ([]*nixFile, error) {
	var files []*nixFile
	seen := map[string]bool{flakeFile: true}
	queue := []string{flakeFile}
	for len(queue) > 0 {
		file := queue[0]
		queue = queue[1:]
		f, err := readNix(fsys, file)
		if err != nil {
			return nil, err
		}
		if file != flakeFile && f.definesDevShells() {
			continue
		}
		files = append(files, f)
		for _, ref := range f.references() {
			if !seen[ref] {
				seen[ref] = true
				queue = append(queue, ref)
			}
		}
	}
	slices.SortFunc(files, func(a, b *nixFile) int { return strings.Compare(a.path, b.path) })
	return files, nil
}

var subPackagesList = regexp.MustCompile(`^\[([^\]]*)\]$`)

func subPackages(files []*nixFile) ([]string, error) {
	var out []string
	var errs []error
	for _, f := range files {
		bindings, err := f.bindings()
		if err != nil {
			return nil, err
		}
		for _, b := range bindings {
			if b.key != "subPackages" {
				continue
			}
			m := subPackagesList.FindStringSubmatch(string(f.src[b.valueStart:b.valueEnd]))
			if m == nil {
				errs = append(errs, fmt.Errorf("%s:%d: subPackages is not a list of package directories", f.path, f.line(b.at)))
				continue
			}
			for field := range strings.FieldsSeq(m[1]) {
				dir, err := strconv.Unquote(field)
				if err != nil {
					errs = append(errs, fmt.Errorf("%s:%d: subPackages element %s: %w", f.path, f.line(b.at), field, err))
					continue
				}
				out = append(out, dir)
			}
		}
	}
	return out, errors.Join(errs...)
}
