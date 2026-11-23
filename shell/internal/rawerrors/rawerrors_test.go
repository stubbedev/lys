// Package rawerrors_test holds the check that no error's own text
// reaches the shell's screens outside an alert's description.
package rawerrors_test

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n/i18ntest"
)

// uiRoots are the trees whose code puts text on the shell's screens:
// the surfaces, the launcher's providers, whose outcomes the launcher
// shows as they come, and the recorder, whose failures it toasts and
// notifies and the bar's recorder module shows.
var uiRoots = []string{"../../shell", "../../service/launcher", "../../service/recorder"}

// detailFile is the one file whose code may turn an error into text:
// the bar's failure alert and page, which show it as a description
// under a summary in the shell's words.
const detailFile = "../../shell/bar/failurealert.go"

// No error's own text reaches a label, a title, a toast, a notification
// or a message outside the bar's failure alert, where it is the
// description under a summary in the shell's words.
func TestNoErrorTextReachesTheShellsScreensOutsideAFailureAlert(t *testing.T) {
	files, err := i18ntest.GoSources(uiRoots, detailFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"../../shell/bar/dock.go", "../../service/launcher/search/env.go", "../../service/recorder/recorder.go", "../../shell/filechooser/host.go"} {
		if !slices.Contains(files, filepath.Clean(want)) {
			t.Fatalf("the scan reads %d files without %s", len(files), want)
		}
	}
	found, err := i18ntest.RawErrors(files)
	if err != nil {
		t.Fatal(err)
	}
	for _, at := range found {
		t.Errorf("%s: an error's text reaches the screen; show a summary, with the error as a failure alert's description or in the log", at)
	}
}

func TestTheScanCatchesTheRecordersWayOfShowingAFailure(t *testing.T) {
	const bad = `package recorder

func (s *State) fail(err error) {
	s.lastErr = err
	s.showError(i18n.T("recorder-toast-failed") + ": " + err.Error())
}

func (s *State) snapshotLocked() Change { return Change{Error: s.lastErr.Error()} }

func (s *State) replayFailed(gen uint64, err error) { s.toast(fmt.Sprint(err), errorIcon, stopToastMS) }
`
	found, err := i18ntest.RawErrorsIn("recorder.go", bad)
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 3 {
		t.Errorf("the scan finds %q, want the toast, the snapshot and the replay toast", found)
	}
}
