package i18n

import (
	"strings"
	"testing"
	"testing/fstest"
)

func TestCardinalCategoriesPerCatalogue(t *testing.T) {
	samples := []struct {
		name string
		n    number
	}{
		{"1", codeNumber(1)},
		{"0", codeNumber(0)},
		{"2", codeNumber(2)},
		{"1000000", codeNumber(1_000_000)},
		{"1.5", codeNumber(1.5)},
		{"1.0", number{v: 1, minFD: 1}},
		{"2000000", codeNumber(2_000_000)},
		{"1000001", codeNumber(1_000_001)},
		{"1000000.0", number{v: 1_000_000, minFD: 1}},
	}
	want := map[string]string{
		"en-US": "one other other other other other other other other",
		"de":    "one other other other other other other other other",
		"nl":    "one other other other other other other other other",
		"fr":    "one one other many one one many other other",
		"es":    "one other other many other one many other other",
		"it":    "one other other many other other many other other",
		"pt":    "one other other many other other many other other",
	}
	for locale, categories := range want {
		rules, err := cardinalRulesFor(MustLangID(locale))
		if err != nil {
			t.Fatal(err)
		}
		got := make([]string, len(samples))
		for i, s := range samples {
			ops, ok := s.n.pluralOperands()
			if !ok {
				t.Fatalf("%s: no operands", s.name)
			}
			got[i] = string(rules.category(ops))
			if !rules.has(rules.category(ops)) {
				t.Errorf("%s: %s falls in %s, which the rules do not list", locale, s.name, got[i])
			}
		}
		if strings.Join(got, " ") != categories {
			t.Errorf("%s: categories %q, want %q", locale, strings.Join(got, " "), categories)
		}
	}
}

func TestEveryCatalogueHasItsPluralRules(t *testing.T) {
	for _, locale := range parseLangs(t, shipped) {
		if _, err := cardinalRulesFor(locale); err != nil {
			t.Error(err)
		}
	}
	for _, unknown := range []string{"pt-BR", "de-AT", "en", "ja"} {
		_, err := cardinalRulesFor(MustLangID(unknown))
		if err == nil || !strings.Contains(err.Error(), unknown) {
			t.Errorf("%s: err = %v, want one naming it", unknown, err)
		}
	}
}

func TestACatalogueWithoutPluralRulesFailsToLoad(t *testing.T) {
	m := fstest.MapFS{
		"en-US/_a.ftl": {Data: []byte("x = en\n")},
		"pt-BR/_a.ftl": {Data: []byte("x = br\n")},
	}
	_, err := Select(NewAssets(m), []LangID{MustLangID("pt-BR")})
	if err == nil || !strings.Contains(err.Error(), "catalogue pt-BR has no plural rule set") {
		t.Fatalf("err = %v", err)
	}
}

func TestPluralVariantsSelectThroughTheLoader(t *testing.T) {
	src := "n = { $n ->\n    [one] one\n    [many] many\n   *[other] other\n}\n"
	for _, c := range []struct {
		locale string
		n      int64
		want   string
	}{
		{"es", 1, "one"},
		{"es", 1_000_000, "many"},
		{"es", 0, "other"},
		{"pt", 0, "other"},
		{"pt", 1_000_000, "many"},
		{"fr", 0, "one"},
		{"de", 1_000_000, "other"},
	} {
		l := loaderFor(t, map[string]string{"en-US/_a.ftl": "n = en\n", c.locale + "/_a.ftl": src}, c.locale)
		if got := l.Get("n", Int("n", c.n)); got != c.want {
			t.Errorf("%s %d -> %q, want %q", c.locale, c.n, got, c.want)
		}
	}
}
