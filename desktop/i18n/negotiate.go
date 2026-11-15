package i18n

import (
	"slices"
	"strings"
)

// negotiate orders the available catalogues by the requested languages:
// for each requested language in turn, every catalogue not yet chosen
// that serves it, closest first (the same identifier, then a catalogue
// whose every subtag the request carries, then one of the same language
// with any script, region or variants), and the fallback last. A
// language no catalogue serves contributes nothing, so a desktop in an
// unsupported language reads en-US. pt-BR is served by pt, European
// Portuguese, the closest catalogue lys has.
func negotiate(requested, available []LangID) []LangID {
	var chosen []LangID
	remaining := slices.Clone(available)
	take := func(serves func(LangID) bool) {
		remaining = slices.DeleteFunc(remaining, func(catalogue LangID) bool {
			if !serves(catalogue) {
				return false
			}
			chosen = append(chosen, catalogue)
			return true
		})
	}
	for _, want := range requested {
		take(want.Equal)
		take(func(catalogue LangID) bool { return catalogue.covers(want) })
		take(func(catalogue LangID) bool { return catalogue.language == want.language })
	}
	if !slices.ContainsFunc(chosen, Fallback.Equal) {
		chosen = append(chosen, Fallback)
	}
	return chosen
}

// requestedLanguages is the desktop locale's languages: every LANGUAGE
// entry, then LC_ALL, LC_MESSAGES and LANG, each deduplicated as a
// string, with the encoding and modifier cut and '_' turned into '-',
// dropping any tag that is no language identifier ("C", empty entries).
func requestedLanguages(getenv func(string) string) []LangID {
	var tags []string
	add := func(raw string) {
		tag := posixToBCP47(raw)
		if !slices.Contains(tags, tag) {
			tags = append(tags, tag)
		}
	}
	if v := getenv("LANGUAGE"); v != "" {
		for part := range strings.SplitSeq(v, ":") {
			add(part)
		}
	}
	for _, name := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := getenv(name); v != "" {
			add(v)
		}
	}
	var ids []LangID
	for _, tag := range tags {
		if id, err := ParseLangID(tag); err == nil {
			ids = append(ids, id)
		}
	}
	return ids
}

// posixToBCP47 cuts a POSIX locale's encoding and modifier and turns
// '_' into '-'.
func posixToBCP47(locale string) string {
	if i := strings.IndexAny(locale, ".@"); i >= 0 {
		locale = locale[:i]
	}
	return strings.ReplaceAll(locale, "_", "-")
}
