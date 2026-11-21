package main

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path"
	"slices"
	"strconv"
	"strings"
)

type Package struct {
	Imports      []string `json:"imports,omitempty"`
	External     []string `json:"external,omitempty"`
	Embeds       []string `json:"embeds,omitempty"`
	TestImports  []string `json:"testImports,omitempty"`
	TestExternal []string `json:"testExternal,omitempty"`
	TestEmbeds   []string `json:"testEmbeds,omitempty"`
}

type fileFacts struct {
	imports []string
	embeds  []string
}

func scan(fsys fs.FS, module string, tracked []string) (map[string]Package, error) {
	packages := map[string]*Package{}
	var errs []error
	for _, file := range tracked {
		dir, name := path.Split(file)
		dir = strings.TrimSuffix(dir, "/")
		if !isGoSource(name) || ignoredDir(dir) {
			continue
		}
		facts, err := parseFile(fsys, file)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		embeds, err := embedEntries(dir, facts.embeds, tracked)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", file, err))
		}
		p := packages[dir]
		if p == nil {
			p = &Package{}
			packages[dir] = p
		}
		imports, external := splitImports(module, dir, facts.imports)
		if strings.HasSuffix(name, "_test.go") {
			p.TestImports = append(p.TestImports, imports...)
			p.TestExternal = append(p.TestExternal, external...)
			p.TestEmbeds = append(p.TestEmbeds, embeds...)
		} else {
			p.Imports = append(p.Imports, imports...)
			p.External = append(p.External, external...)
			p.Embeds = append(p.Embeds, embeds...)
		}
	}
	result := make(map[string]Package, len(packages))
	for dir, p := range packages {
		for _, imported := range slices.Concat(p.Imports, p.TestImports) {
			if _, ok := packages[imported]; !ok {
				errs = append(errs, fmt.Errorf("%s imports %s/%s, which holds no Go files", dir, module, imported))
			}
		}
		imports, external, embeds := sortedSet(p.Imports), sortedSet(p.External), sortedSet(p.Embeds)
		result[dir] = Package{
			Imports:      imports,
			External:     external,
			Embeds:       embeds,
			TestImports:  without(sortedSet(p.TestImports), imports),
			TestExternal: without(sortedSet(p.TestExternal), external),
			TestEmbeds:   without(sortedSet(p.TestEmbeds), embeds),
		}
	}
	if err := errors.Join(errs...); err != nil {
		return nil, err
	}
	return result, nil
}

func isGoSource(name string) bool {
	return strings.HasSuffix(name, ".go") && !strings.HasPrefix(name, ".") && !strings.HasPrefix(name, "_")
}

func ignoredDir(dir string) bool {
	if dir == "" {
		return false
	}
	for elem := range strings.SplitSeq(dir, "/") {
		if elem == "testdata" || elem == "vendor" || strings.HasPrefix(elem, ".") || strings.HasPrefix(elem, "_") {
			return true
		}
	}
	return false
}

func parseFile(fsys fs.FS, file string) (fileFacts, error) {
	src, err := fs.ReadFile(fsys, file)
	if err != nil {
		return fileFacts{}, fmt.Errorf("read %s: %w", file, err)
	}
	f, err := parser.ParseFile(token.NewFileSet(), file, src, parser.ParseComments|parser.SkipObjectResolution)
	if err != nil {
		return fileFacts{}, fmt.Errorf("parse %s: %w", file, err)
	}
	var facts fileFacts
	for _, spec := range f.Imports {
		imported, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return fileFacts{}, fmt.Errorf("%s: import %s: %w", file, spec.Path.Value, err)
		}
		facts.imports = append(facts.imports, imported)
	}
	for _, group := range f.Comments {
		patterns, err := embedPatterns(group)
		if err != nil {
			return fileFacts{}, fmt.Errorf("%s: %w", file, err)
		}
		facts.embeds = append(facts.embeds, patterns...)
	}
	return facts, nil
}

func embedPatterns(group *ast.CommentGroup) ([]string, error) {
	var patterns []string
	for _, c := range group.List {
		args, ok := strings.CutPrefix(c.Text, "//go:embed")
		if !ok || (args != "" && args[0] != ' ' && args[0] != '\t') {
			continue
		}
		fields, err := splitEmbedArgs(args)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", c.Text, err)
		}
		if len(fields) == 0 {
			return nil, fmt.Errorf("%s names no pattern", c.Text)
		}
		patterns = append(patterns, fields...)
	}
	return patterns, nil
}

func splitEmbedArgs(args string) ([]string, error) {
	var fields []string
	for {
		args = strings.TrimLeft(args, " \t")
		if args == "" {
			return fields, nil
		}
		if args[0] != '"' && args[0] != '`' {
			end := strings.IndexAny(args, " \t")
			if end < 0 {
				end = len(args)
			}
			fields = append(fields, args[:end])
			args = args[end:]
			continue
		}
		quoted, err := strconv.QuotedPrefix(args)
		if err != nil {
			return nil, fmt.Errorf("unterminated quoted pattern %s: %w", args, err)
		}
		field, err := strconv.Unquote(quoted)
		if err != nil {
			return nil, fmt.Errorf("quoted pattern %s: %w", quoted, err)
		}
		fields = append(fields, field)
		args = args[len(quoted):]
	}
}

func embedEntries(dir string, patterns, tracked []string) ([]string, error) {
	entries := make([]string, 0, len(patterns))
	var errs []error
	for _, pattern := range patterns {
		entry := path.Join(dir, literalPrefix(strings.TrimPrefix(pattern, "all:")))
		if !slices.ContainsFunc(tracked, func(f string) bool { return f == entry || strings.HasPrefix(f, entry+"/") }) {
			errs = append(errs, fmt.Errorf("the embed pattern %q matches no tracked file under %s", pattern, entry))
			continue
		}
		entries = append(entries, entry)
	}
	return entries, errors.Join(errs...)
}

func literalPrefix(pattern string) string {
	elems := strings.Split(pattern, "/")
	for i, elem := range elems {
		if strings.ContainsAny(elem, `*?[\`) {
			return path.Join(elems[:i]...)
		}
	}
	return pattern
}

func splitImports(module, dir string, imports []string) (internal, external []string) {
	for _, imported := range imports {
		rel, inModule := strings.CutPrefix(imported, module+"/")
		switch {
		case inModule && rel != dir:
			internal = append(internal, rel)
		case !inModule && imported != module && imported != "C":
			external = append(external, imported)
		}
	}
	return internal, external
}

func sortedSet(items []string) []string {
	slices.Sort(items)
	return slices.Compact(items)
}

func without(items, excluded []string) []string {
	kept := slices.DeleteFunc(items, func(item string) bool {
		_, found := slices.BinarySearch(excluded, item)
		return found
	})
	if len(kept) == 0 {
		return nil
	}
	return kept
}
