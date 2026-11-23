package greeter

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n/i18ntest"
)

// greeterRoots are the trees whose code puts text on the login screen:
// the greeter, and the screen parts it shares with the lock screen,
// whose refusals it shows.
var greeterRoots = []string{".", "../screen"}

// No error's own text reaches the greeter's message line, a label or a
// tooltip: the greeter says what failed in its words and logs the
// error.
func TestNoErrorTextReachesTheGreeter(t *testing.T) {
	files, err := i18ntest.GoSources(greeterRoots)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"screen.go", "../screen/refusal.go"} {
		if !slices.Contains(files, filepath.Clean(want)) {
			t.Fatalf("the scan reads %d files without %s", len(files), want)
		}
	}
	found, err := i18ntest.RawErrors(files)
	if err != nil {
		t.Fatal(err)
	}
	for _, at := range found {
		t.Errorf("%s: an error's text reaches the login screen; show a message of lys-greeter.ftl and log the error", at)
	}
}

func TestTheScanCatchesTheGreetersWaysOfShowingAnError(t *testing.T) {
	const bad = `package greeter

func (s *screen) a(err error) { s.prompt.SetMessage(t("greeter-greetd-unavailable") + ": " + err.Error()) }

func (s *screen) b(ev auth.Event) { s.prompt.SetMessage(fmt.Sprintf("%v", ev.Err)) }

func (s *screen) c(err error) { s.moduleError = err.Error() }
`
	found, err := i18ntest.RawErrorsIn("screen.go", bad)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 3 {
		t.Errorf("the scan finds %q, want the message line, the formatted event and the module error", found)
	}

	const good = `package greeter

func (s *screen) a(err error) {
	log.Printf("greeter: cannot connect to greetd: %v", err)
	s.prompt.SetMessage(t("greeter-greetd-unavailable"))
}

func (e UsageError) Error() string { return e.msg }

func (s *screen) b(ev auth.Event) { s.prompt.SetMessage(loginscreen.RefusalText(ev.Refusal)) }
`
	if found, err := i18ntest.RawErrorsIn("screen.go", good); err != nil || len(found) != 0 {
		t.Errorf("the scan flags %q (%v) in code keeping errors off the login screen", found, err)
	}
}
