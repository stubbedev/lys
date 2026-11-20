package settings

import (
	"slices"
	"testing"

	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
)

func TestALocaleReadsAsItsLanguageAndCountry(t *testing.T) {
	en := display.Tags(language.English)
	for name, want := range map[string]string{
		"de_DE.UTF-8":      "German (Germany)",
		"pt_PT.utf8":       "European Portuguese",
		"de_DE.ISO-8859-1": "German (Germany) (de_DE.ISO-8859-1)",
		"not a locale":     "not a locale",
	} {
		if got := localeLabel(en, name); got != want {
			t.Errorf("%q reads %q, want %q", name, got, want)
		}
	}
}

func TestLocalesAreListedByTheirNames(t *testing.T) {
	names, labels := localeLabels([]string{"sv_SE.UTF-8", "de_DE.UTF-8", "en_GB.UTF-8"})
	if len(names) != 3 || len(labels) != 3 {
		t.Fatalf("%q %q", names, labels)
	}
	if !slices.IsSorted(labels) {
		t.Errorf("labels are not in order: %q", labels)
	}
	for i, n := range names {
		if localeLabel(display.Tags(interfaceLanguage()), n) != labels[i] {
			t.Errorf("%s lost its label", n)
		}
	}
}
