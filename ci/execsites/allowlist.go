package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

type class string

const (
	classIntegration class = "integration"
	classOwn         class = "own"
	classReplace     class = "replace"
)

func (c *class) UnmarshalText(text []byte) error {
	switch v := class(text); v {
	case classIntegration, classOwn, classReplace:
		*c = v
		return nil
	default:
		return fmt.Errorf("class %q is none of %s, %s, %s", text, classIntegration, classOwn, classReplace)
	}
}

type goKey struct {
	pkg, function, program string
}

func (k goKey) String() string {
	return fmt.Sprintf("package %s, function %s, program %q", k.pkg, k.function, k.program)
}

type packagingKey struct {
	file, program string
}

func (k packagingKey) String() string {
	return fmt.Sprintf("file %s, program %q", k.file, k.program)
}

type entry[K comparable] struct {
	key  K
	line int
}

type allowlist struct {
	goEntries        []entry[goKey]
	packagingEntries []entry[packagingKey]
}

type verdict struct {
	Class  class  `json:"class"`
	Reason string `json:"reason"`
	Issue  int    `json:"issue,omitempty"`
}

type goEntryJSON struct {
	Package  string `json:"package"`
	Function string `json:"function"`
	Program  string `json:"program"`
	verdict
}

type packagingEntryJSON struct {
	File    string `json:"file"`
	Program string `json:"program"`
	verdict
}

func parseAllowlist(name string, data []byte) (allowlist, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	lineAt := lineIndex(data)
	var list allowlist
	var errs []error
	seen := map[string]bool{}
	if err := expectDelim(dec, '{'); err != nil {
		return allowlist{}, fmt.Errorf("%s: %w", name, err)
	}
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return allowlist{}, fmt.Errorf("%s: %w", name, err)
		}
		section, _ := tok.(string)
		if seen[section] {
			return allowlist{}, fmt.Errorf("%s: section %q appears twice", name, section)
		}
		seen[section] = true
		switch section {
		case "go":
			list.goEntries, err = decodeSection(dec, name, lineAt, &errs, func(e goEntryJSON) (goKey, error) {
				if e.Package == "" || e.Function == "" || e.Program == "" {
					return goKey{}, errors.New("a go entry names its package, function and program")
				}
				return goKey{e.Package, e.Function, e.Program}, e.check()
			})
		case "packaging":
			list.packagingEntries, err = decodeSection(dec, name, lineAt, &errs, func(e packagingEntryJSON) (packagingKey, error) {
				if e.File == "" || e.Program == "" {
					return packagingKey{}, errors.New("a packaging entry names its file and program")
				}
				return packagingKey{e.File, e.Program}, e.check()
			})
		default:
			return allowlist{}, fmt.Errorf("%s: unknown section %v, want go or packaging", name, tok)
		}
		if err != nil {
			return allowlist{}, fmt.Errorf("%s: section %s: %w", name, section, err)
		}
	}
	if err := expectDelim(dec, '}'); err != nil {
		return allowlist{}, fmt.Errorf("%s: %w", name, err)
	}
	if _, err := dec.Token(); err == nil {
		return allowlist{}, fmt.Errorf("%s: data after the top-level object", name)
	}
	return list, errors.Join(errs...)
}

func (v verdict) check() error {
	var errs []error
	if strings.TrimSpace(v.Reason) == "" {
		errs = append(errs, errors.New("the entry gives no reason"))
	}
	switch {
	case v.Class == "":
		errs = append(errs, errors.New("the entry gives no class"))
	case v.Class == classReplace && v.Issue <= 0:
		errs = append(errs, errors.New("a replace entry names the issue that replaces it"))
	case v.Class != classReplace && v.Issue != 0:
		errs = append(errs, fmt.Errorf("the entry is class %s and names issue %d, which only a replace entry does", v.Class, v.Issue))
	}
	return errors.Join(errs...)
}

func decodeSection[J any, K comparable](dec *json.Decoder, name string, lineAt func(int64) int, errs *[]error, parse func(J) (K, error)) ([]entry[K], error) {
	if err := expectDelim(dec, '['); err != nil {
		return nil, err
	}
	var entries []entry[K]
	first := map[K]int{}
	for dec.More() {
		line := lineAt(dec.InputOffset())
		var raw J
		if err := dec.Decode(&raw); err != nil {
			return nil, fmt.Errorf("%s:%d: %w", name, line, err)
		}
		key, err := parse(raw)
		if err != nil {
			*errs = append(*errs, fmt.Errorf("%s:%d: %w", name, line, err))
			continue
		}
		if prev, ok := first[key]; ok {
			*errs = append(*errs, fmt.Errorf("%s:%d: %v repeats the entry on line %d", name, line, key, prev))
			continue
		}
		first[key] = line
		entries = append(entries, entry[K]{key: key, line: line})
	}
	return entries, expectDelim(dec, ']')
}

func expectDelim(dec *json.Decoder, want json.Delim) error {
	tok, err := dec.Token()
	if err != nil {
		return fmt.Errorf("want %v: %w", want, err)
	}
	if tok != want {
		return fmt.Errorf("want %v, have %v", want, tok)
	}
	return nil
}

func lineIndex(data []byte) func(int64) int {
	return func(offset int64) int {
		start := int(offset)
		for start < len(data) && strings.IndexByte(" \t\r\n,", data[start]) >= 0 {
			start++
		}
		return 1 + bytes.Count(data[:start], []byte("\n"))
	}
}
