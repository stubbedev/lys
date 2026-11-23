package locale

import (
	"regexp"
	"slices"
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

// sentenceReasons are the messages whose $reason, $error or $err is
// always a localized sentence, never an error's own text.
var sentenceReasons = []string{"settings-sleep-detail-auto-suspends"}

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

// A message tells what failed in the app's words: an error's own text
// waits in a state's detail, never in a message's placeable.
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
		for _, id := range rawReasons(src) {
			t.Errorf("%s: %s takes an error's text; say what failed and show the error as a state's detail", lang, id)
		}
	}
}

func TestTheReasonScanFindsEachMessageTakingAnError(t *testing.T) {
	const src = `# { $reason } in a comment is no placeable
settings-plain = Saved
settings-bad-reason = Cannot read it: { $reason }
settings-bad-error = Failed ({ $error })
settings-bad-err =
    Two lines,
    the second { $err }
settings-attr = Fine
    .tooltip = Why: { $reason }
settings-sleep-detail-auto-suspends = Suspends only: { $reason }
settings-reasoning = { $reasons } and { $errors } are other names
`
	if got, want := rawReasons(src), []string{"settings-bad-reason", "settings-bad-error", "settings-bad-err", "settings-attr"}; !slices.Equal(got, want) {
		t.Errorf("the scan finds %q, want %q", got, want)
	}
}
