package settings

import (
	"cmp"
	"slices"
	"strings"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func interfaceLanguage() language.Tag {
	if langs := locale.T().Languages(); len(langs) > 0 {
		if tag, err := language.Parse(langs[0].String()); err == nil {
			return tag
		}
	}
	return language.English
}

func localeTag(name string) (language.Tag, bool) {
	base, _, _ := strings.Cut(name, ".")
	base, _, _ = strings.Cut(base, "@")
	tag, err := language.Parse(strings.ReplaceAll(base, "_", "-"))
	return tag, err == nil && base != ""
}

func localeLabel(namer display.Namer, name string) string {
	tag, ok := localeTag(name)
	if !ok {
		return name
	}
	label := namer.Name(tag)
	if label == "" {
		return name
	}
	if strings.Contains(name, ".") && !strings.HasSuffix(strings.ToLower(name), ".utf-8") && !strings.HasSuffix(strings.ToLower(name), ".utf8") {
		return label + " (" + name + ")"
	}
	return label
}

func localeLabels(names []string) (sorted, labels []string) {
	namer := display.Tags(interfaceLanguage())
	type entry struct{ name, label string }
	entries := make([]entry, len(names))
	for i, n := range names {
		entries[i] = entry{n, localeLabel(namer, n)}
	}
	slices.SortStableFunc(entries, func(a, b entry) int {
		return cmp.Or(cmp.Compare(strings.ToLower(a.label), strings.ToLower(b.label)), cmp.Compare(a.name, b.name))
	})
	sorted, labels = make([]string, len(entries)), make([]string, len(entries))
	for i, e := range entries {
		sorted[i], labels[i] = e.name, e.label
	}
	return sorted, labels
}
