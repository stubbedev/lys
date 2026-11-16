package i18n

import (
	"fmt"
	"io/fs"
	"maps"
	"slices"
	"strings"
)

// Gaps lists every way a catalogue falls short of the en-US fallback,
// sorted: a message, term or attribute en-US has and it lacks, one it
// has and en-US does not, a placeable set that differs from en-US's,
// a variant key naming a plural category its language does not have,
// a count shown without a plural select on it, and a catalogue without
// a plural rule set. Each names the locale, the file and the id.
func (a Assets) Gaps() ([]string, error) {
	available, err := a.Available()
	if err != nil {
		return nil, fmt.Errorf("i18n: listing the catalogues: %w", err)
	}
	reference, err := a.catalogueOf(Fallback)
	if err != nil {
		return nil, err
	}
	var problems []string
	for _, locale := range available {
		entries := reference
		if !locale.Equal(Fallback) {
			if entries, err = a.catalogueOf(locale); err != nil {
				return nil, err
			}
			problems = append(problems, compareCatalogues(locale, reference, entries)...)
		}
		problems = append(problems, countGaps(locale, entries)...)
		rules, err := cardinalRulesFor(locale)
		if err != nil {
			problems = append(problems, err.Error())
			continue
		}
		problems = append(problems, pluralGaps(locale, rules, entries)...)
	}
	slices.Sort(problems)
	return problems, nil
}

// catalogued is one message, term or attribute of a catalogue: the file
// defining it, the sorted placeables it references, the variant keys
// naming a plural category and the counts it shows without selecting
// on them.
type catalogued struct {
	file             string
	placeables       []string
	pluralKeys       []pluralCategory
	unselectedCounts []string
}

var countVariables = []string{"count", "length", "max", "new", "total", "words"}

func compareCatalogues(locale LangID, reference, translated map[string]catalogued) []string {
	var problems []string
	for _, id := range slices.Sorted(maps.Keys(reference)) {
		want := reference[id]
		have, ok := translated[id]
		if !ok {
			problems = append(problems, fmt.Sprintf("%s: %s: %s: missing", locale, want.file, id))
			continue
		}
		if !slices.Equal(have.placeables, want.placeables) {
			problems = append(problems, fmt.Sprintf("%s: %s: %s: placeables [%s], want [%s]",
				locale, have.file, id, strings.Join(have.placeables, " "), strings.Join(want.placeables, " ")))
		}
	}
	for _, id := range slices.Sorted(maps.Keys(translated)) {
		if _, ok := reference[id]; !ok {
			problems = append(problems, fmt.Sprintf("%s: %s: %s: not in %s", locale, translated[id].file, id, Fallback))
		}
	}
	return problems
}

func countGaps(locale LangID, entries map[string]catalogued) []string {
	var problems []string
	for _, id := range slices.Sorted(maps.Keys(entries)) {
		for _, count := range entries[id].unselectedCounts {
			problems = append(problems, fmt.Sprintf("%s: %s: %s: count $%s is shown without a plural select on it",
				locale, entries[id].file, id, count))
		}
	}
	return problems
}

func pluralGaps(locale LangID, rules pluralRules, entries map[string]catalogued) []string {
	var problems []string
	for _, id := range slices.Sorted(maps.Keys(entries)) {
		for _, key := range entries[id].pluralKeys {
			if !rules.has(key) {
				problems = append(problems, fmt.Sprintf("%s: %s: %s: variant [%s] is no plural category of %s",
					locale, entries[id].file, id, key, locale))
			}
		}
	}
	return problems
}

// catalogueOf reads a locale's entries, the first definition of an id
// winning as in a bundle.
func (a Assets) catalogueOf(locale LangID) (map[string]catalogued, error) {
	files, err := a.partials(locale.String())
	if err != nil {
		return nil, fmt.Errorf("i18n: listing the files of %s: %w", locale, err)
	}
	entries := map[string]catalogued{}
	for _, f := range files {
		data, err := fs.ReadFile(f.tree, f.path)
		if err != nil {
			return nil, fmt.Errorf("i18n: reading %s: %w", f.path, err)
		}
		file := strings.TrimPrefix(f.path, locale.String()+"/")
		res := parseResource(strings.ReplaceAll(string(data), "\r\n", "\n"))
		for _, e := range res.entries {
			id := e.id
			if e.term {
				id = "-" + id
			}
			fields := map[string]*pattern{id: e.value}
			for _, attr := range e.attrs {
				fields[id+"."+attr.id] = attr.value
			}
			for _, field := range slices.Sorted(maps.Keys(fields)) {
				if _, dup := entries[field]; dup {
					continue
				}
				entries[field] = catalogued{
					file:             file,
					placeables:       placeablesOf(fields[field]),
					pluralKeys:       pluralKeysOf(fields[field]),
					unselectedCounts: unselectedCountsOf(fields[field]),
				}
			}
		}
	}
	return entries, nil
}

// walkPattern visits every expression in a pattern, nested ones and
// select variants included.
func walkPattern(p *pattern, visit func(expression)) {
	if p == nil {
		return
	}
	for _, el := range p.elements {
		if el.expr != nil {
			walkExpression(el.expr, visit)
		}
	}
}

func walkExpression(e expression, visit func(expression)) {
	visit(e)
	walkArgs := func(args callArgs) {
		for _, arg := range args.positional {
			walkExpression(arg, visit)
		}
		for _, arg := range args.named {
			walkExpression(arg.value, visit)
		}
	}
	switch x := e.(type) {
	case nestedPlaceable:
		walkExpression(x.expr, visit)
	case functionRef:
		walkArgs(x.args)
	case termRef:
		if x.args != nil {
			walkArgs(*x.args)
		}
	case selectExpr:
		walkExpression(x.selector, visit)
		for _, v := range x.variants {
			walkPattern(v.value, visit)
		}
	}
}

// placeablesOf is the sorted, distinct references and non-blank string
// literals a pattern holds: what a translation must carry over.
func placeablesOf(p *pattern) []string {
	var found []string
	walkPattern(p, func(e expression) {
		switch x := e.(type) {
		case variableRef:
			found = append(found, "$"+x.id)
		case messageRef:
			found = append(found, withAttr(x.id, x.attr))
		case termRef:
			found = append(found, withAttr("-"+x.id, x.attr))
		case functionRef:
			found = append(found, x.id+"()")
		case stringLiteral:
			if strings.TrimSpace(unescape(x.raw)) != "" {
				found = append(found, `"`+x.raw+`"`)
			}
		}
	})
	slices.Sort(found)
	return slices.Compact(found)
}

func withAttr(id, attr string) string {
	if attr == "" {
		return id
	}
	return id + "." + attr
}

// pluralKeysOf is every select variant key in a pattern that names a
// CLDR plural category.
func pluralKeysOf(p *pattern) []pluralCategory {
	var keys []pluralCategory
	walkPattern(p, func(e expression) {
		sel, ok := e.(selectExpr)
		if !ok {
			return
		}
		for _, v := range sel.variants {
			if !v.numberKey && isPluralCategory(v.key) {
				keys = append(keys, pluralCategory(v.key))
			}
		}
	})
	return keys
}

func unselectedCountsOf(p *pattern) []string {
	var shown, selected []string
	walkPattern(p, func(e expression) {
		switch x := e.(type) {
		case variableRef:
			if slices.Contains(countVariables, x.id) {
				shown = append(shown, x.id)
			}
		case selectExpr:
			if id, ok := selectorVariable(x.selector); ok {
				selected = append(selected, id)
			}
		}
	})
	slices.Sort(shown)
	return slices.DeleteFunc(slices.Compact(shown), func(id string) bool { return slices.Contains(selected, id) })
}

func selectorVariable(e inlineExpr) (string, bool) {
	switch x := e.(type) {
	case variableRef:
		return x.id, true
	case functionRef:
		if x.id == "NUMBER" && len(x.args.positional) > 0 {
			return selectorVariable(x.args.positional[0])
		}
	}
	return "", false
}
