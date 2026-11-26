package locale

import (
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/i18n/i18ntest"
)

func TestEveryLanguageHasEveryMessageEnglishHas(t *testing.T) {
	gaps, err := Assets().Gaps()
	if err != nil {
		t.Fatal(err)
	}
	for _, gap := range gaps {
		t.Error(gap)
	}
}

func TestEveryLanguageIsShippedAndResolves(t *testing.T) {
	available, err := Assets().Available()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, lang := range available {
		got = append(got, lang.String())
		l, err := i18n.Select(Assets(), []i18n.LangID{lang})
		if err != nil {
			t.Fatal(err)
		}
		if !l.Languages()[0].Equal(lang) {
			t.Errorf("%s: negotiated %v", lang, l.Languages())
		}
		if msg := l.Get("viewer-trashed", i18n.Str("name", "a.png")); !strings.Contains(msg, "a.png") {
			t.Errorf("%s: viewer-trashed reads %q", lang, msg)
		}
	}
	if want := "de en-US es fr it nl pt"; strings.Join(got, " ") != want {
		t.Fatalf("languages %q, want %q", strings.Join(got, " "), want)
	}
}

func TestAMessageNoneHasIsNotFound(t *testing.T) {
	if T().Has("no-such-message") {
		t.Fatal("found")
	}
}

func TestNoMessageTakesAnErrorsOwnText(t *testing.T) {
	available, err := Assets().Available()
	if err != nil {
		t.Fatal(err)
	}
	for _, lang := range available {
		src, err := Assets().Source(lang.String())
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range i18ntest.RawReasons(src) {
			t.Errorf("%s: %s takes an error's text", lang, id)
		}
	}
}
