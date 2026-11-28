package locale

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/i18n/i18ntest"
)

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
		if !l.Has("wm-gaps-inner") {
			t.Errorf("%s: no lys-wm label", lang)
		}
		if !l.Has("app-name") {
			t.Errorf("%s: no message of lys-shell's config domain", lang)
		}
	}
}

// sentenceReasons are the messages whose $reason, $error or $err is
// always a localized sentence, never an error's own text.
var sentenceReasons = []string{"settings-sleep-detail-auto-suspends"}

func TestTheCataloguesAreComplete(t *testing.T) {
	i18ntest.CheckAssets(t, Assets(), "settings-title", sentenceReasons...)
}

// Placeholders read as an identifier ("id", "label", "value") rather than
// as words saying what goes in the field. Every placeholder of every
// language opens with a capital, as the field's words do.
func TestEveryPlaceholderReadsAsWords(t *testing.T) {
	available, err := Assets().Available()
	if err != nil {
		t.Fatal(err)
	}
	for _, lang := range available {
		src, err := Assets().Source(lang.String())
		if err != nil {
			t.Fatal(err)
		}
		for line := range strings.Lines(src) {
			id, text, ok := strings.Cut(strings.TrimSpace(line), " = ")
			if !ok || !strings.HasSuffix(id, "-placeholder") {
				continue
			}
			if first, _ := utf8.DecodeRuneInString(text); unicode.IsLower(first) {
				t.Errorf("%s: %s reads %q", lang, id, text)
			}
		}
	}
}
