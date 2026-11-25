package i18n

import (
	"slices"
	"strings"

	libi18n "github.com/stubbedev/lys/lib/i18n"
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

// requestedLanguages is the desktop locale's languages, libi18n's
// Languages with the encoding and modifier cut and '_' turned into '-',
// dropping a repeat and any tag that is no language identifier.
func requestedLanguages(getenv func(string) string) []LangID {
	var ids []LangID
	for _, locale := range libi18n.Languages(getenv) {
		id, err := ParseLangID(posixToBCP47(locale))
		if err != nil || slices.ContainsFunc(ids, id.Equal) {
			continue
		}
		ids = append(ids, id)
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
