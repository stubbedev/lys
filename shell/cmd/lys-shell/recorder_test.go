package main

import (
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	shelllocales "github.com/stubbedev/lys/desktop/i18n/locales/shell"
)

func TestTheReplayStatusSpeaksTheLocalesLanguageAndByteSize(t *testing.T) {
	for _, c := range []struct {
		locale, sentence, size, english string
	}{
		{"en-US", "Instant replay armed", "1.5 GiB", ""},
		{"de", "Sofortwiederholung bereit", "1,5 GiB", "1.5 GiB"},
		{"fr", "Relecture instantanée armée", "1,5\u202fGio", "GiB"},
	} {
		l, err := i18n.Select(i18n.NewAssets(shelllocales.FS), []i18n.LangID{i18n.MustLangID(c.locale)})
		if err != nil {
			t.Fatal(err)
		}
		got := strings.NewReplacer("\u2068", "", "\u2069", "").Replace(replayArmedStatus(l, 1536<<20))
		if want := c.sentence + " (" + c.size + ")"; got != want {
			t.Errorf("%s replay status = %q, want %q", c.locale, got, want)
		}
		if c.english != "" && strings.Contains(got, c.english) {
			t.Errorf("%s replay status = %q, carries the English %q", c.locale, got, c.english)
		}
	}
}
