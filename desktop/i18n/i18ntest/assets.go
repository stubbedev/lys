package i18ntest

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
)

// CheckAssets fails t where assets, an app's catalogues, fall short:
// they ship other languages than i18n.Shipped, a language lacks a
// message English has, a language does not negotiate to itself or
// lacks probe, an absent message is found, or a message takes an
// error's own text. sentences are the messages whose $reason, $error
// or $err is always a localized sentence; each must be one the scan
// finds in English, so the list cannot go stale.
func CheckAssets(t testing.TB, assets i18n.Assets, probe string, sentences ...string) {
	t.Helper()
	available, err := assets.Available()
	if err != nil {
		t.Fatalf("listing the catalogues: %v", err)
	}
	if !slices.EqualFunc(available, i18n.Shipped(), i18n.LangID.Equal) {
		t.Errorf("the catalogues are %v, want %v", available, i18n.Shipped())
	}
	gaps, err := assets.Gaps()
	if err != nil {
		t.Fatalf("comparing the catalogues with English: %v", err)
	}
	for _, gap := range gaps {
		t.Error(gap)
	}
	for _, lang := range available {
		checkLanguage(t, assets, lang, probe, sentences)
	}
	english, err := assets.Source(i18n.Fallback.String())
	if err != nil {
		t.Fatalf("reading the %s catalogue: %v", i18n.Fallback, err)
	}
	reasons := RawReasons(english)
	for _, id := range sentences {
		if !slices.Contains(reasons, id) {
			t.Errorf("%s is listed as a sentence reason but takes no $reason, $error or $err", id)
		}
	}
}

func checkLanguage(t testing.TB, assets i18n.Assets, lang i18n.LangID, probe string, sentences []string) {
	t.Helper()
	l, err := i18n.Select(assets, []i18n.LangID{lang})
	if err != nil {
		t.Fatalf("%s: loading: %v", lang, err)
	}
	if got := l.Languages(); !got[0].Equal(lang) {
		t.Errorf("%s: negotiated %v", lang, got)
	}
	if !l.Has(probe) {
		t.Errorf("%s: no %s", lang, probe)
	}
	if l.Has("no-such-message") {
		t.Errorf("%s: a message no catalogue has is found", lang)
	}
	src, err := assets.Source(lang.String())
	if err != nil {
		t.Fatalf("%s: reading the catalogue: %v", lang, err)
	}
	for _, id := range RawReasons(src, sentences...) {
		t.Errorf("%s: %s takes an error's text; say what failed and show the error as a state's detail", lang, id)
	}
}
