package i18n

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	greeterlocales "github.com/stubbedev/lys/desktop/i18n/locales/greeter"
	shelllocales "github.com/stubbedev/lys/desktop/i18n/locales/shell"
)

// sentenceReasons are the shell's and the greeter's messages whose
// $reason, $error or $err is always a localized sentence, never an
// error's own text.
var sentenceReasons = []string{"weather-stale"}

var (
	ftlMessage    = regexp.MustCompile(`^(-?[a-zA-Z][a-zA-Z0-9_-]*)\s*=`)
	rawReasonPart = regexp.MustCompile(`\{[^}]*\$(reason|error|err)\b`)
)

// rawReasons are the messages of src, an FTL source, taking a $reason,
// $error or $err placeable that sentenceReasons does not name, in order.
func rawReasons(src string) []string {
	var found []string
	id := ""
	for line := range strings.SplitSeq(src, "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		if m := ftlMessage.FindStringSubmatch(line); m != nil {
			id = m[1]
		}
		if rawReasonPart.MatchString(line) && !slices.Contains(sentenceReasons, id) && !slices.Contains(found, id) {
			found = append(found, id)
		}
	}
	return found
}

// A shell or greeter message tells what failed in the shell's words: an
// error's own text waits in an alert's description or the journal,
// never in a message's placeable.
func TestNoShellOrGreeterMessageTakesAnErrorsOwnText(t *testing.T) {
	for name, assets := range map[string]Assets{
		"shell":   NewAssets(shelllocales.FS),
		"greeter": NewFileAssets(greeterlocales.FS, "lys-greeter.ftl"),
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
			for _, id := range rawReasons(src) {
				t.Errorf("%s %s: %s takes an error's text; say what failed and show the error as an alert's description", name, lang, id)
			}
		}
	}
}

func TestTheReasonScanFindsEachShellMessageTakingAnError(t *testing.T) {
	const src = `# { $reason } in a comment is no placeable
dock-plain = Pinned
dock-bad-reason = Cannot pin it: { $reason }
dock-bad-error = Failed ({ $error })
dock-bad-err =
    Two lines,
    the second { $err }
dock-attr = Fine
    .tooltip = Why: { $reason }
weather-stale = { $reading } (not refreshed: { $reason })
dock-reasoning = { $reasons } and { $errors } are other names
`
	if got, want := rawReasons(src), []string{"dock-bad-reason", "dock-bad-error", "dock-bad-err", "dock-attr"}; !slices.Equal(got, want) {
		t.Errorf("the scan finds %q, want %q", got, want)
	}
}
