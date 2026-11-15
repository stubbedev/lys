package locale

import (
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
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

func TestEveryLanguageIsShipped(t *testing.T) {
	available, err := Assets().Available()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, lang := range available {
		got = append(got, lang.String())
	}
	if want := "de en-US es fr it nl pt"; strings.Join(got, " ") != want {
		t.Fatalf("languages %q, want %q", strings.Join(got, " "), want)
	}
}

func TestOwnMessagesAndTheConfigsLabelsResolveTogether(t *testing.T) {
	available, err := Assets().Available()
	if err != nil {
		t.Fatal(err)
	}
	for _, lang := range available {
		l, err := i18n.Select(Assets(), []i18n.LangID{lang})
		if err != nil {
			t.Fatal(err)
		}
		if got := l.Languages(); !got[0].Equal(lang) {
			t.Errorf("%s: negotiated %v", lang, got)
		}
		if !l.Has("settings-title") {
			t.Errorf("%s: no message of this app", lang)
		}
		if !l.Has("wm-gaps-inner") {
			t.Errorf("%s: no lys-wm label", lang)
		}
		if !l.Has("app-name") {
			t.Errorf("%s: no message of lys-shell's config domain", lang)
		}
		if got := l.Get("settings-title"); strings.Contains(got, "No localization") {
			t.Errorf("%s: %q", lang, got)
		}
	}
}

func TestAMessageNoneHasIsNotFound(t *testing.T) {
	if T().Has("no-such-message") {
		t.Fatal("found")
	}
}
