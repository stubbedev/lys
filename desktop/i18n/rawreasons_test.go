package i18n_test

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/i18n/i18ntest"
	greeterlocales "github.com/stubbedev/lys/desktop/i18n/locales/greeter"
	shelllocales "github.com/stubbedev/lys/desktop/i18n/locales/shell"
)

// sentenceReasons are the shell's and the greeter's messages whose
// $reason, $error or $err is always a localized sentence, never an
// error's own text.
var sentenceReasons = []string{"weather-stale"}

// A shell or greeter message tells what failed in the shell's words: an
// error's own text waits in an alert's description or the journal,
// never in a message's placeable.
func TestNoShellOrGreeterMessageTakesAnErrorsOwnText(t *testing.T) {
	for name, assets := range map[string]i18n.Assets{
		"shell":   i18n.NewAssets(shelllocales.FS),
		"greeter": i18n.NewFileAssets(greeterlocales.FS, "lys-greeter.ftl"),
	} {
		available, err := assets.Available()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, lang := range available {
			src, err := assets.Source(lang.String())
			if err != nil {
				t.Fatalf("%s %s: %v", name, lang, err)
			}
			for _, id := range i18ntest.RawReasons(src, sentenceReasons...) {
				t.Errorf("%s %s: %s takes an error's text; say what failed and show the error as an alert's description", name, lang, id)
			}
		}
	}
}

func TestTheShellsSentenceReasonsAreItsOwn(t *testing.T) {
	src, err := i18n.NewAssets(shelllocales.FS).Source(i18n.Fallback.String())
	if err != nil {
		t.Fatal(err)
	}
	if got := i18ntest.RawReasons(src); !slices.Equal(got, sentenceReasons) {
		t.Errorf("without the allow-list the scan finds %q, want exactly %q", got, sentenceReasons)
	}
}
