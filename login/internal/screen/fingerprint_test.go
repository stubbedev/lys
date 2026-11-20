package screen

import (
	"testing"

	"github.com/stubbedev/lys/login/internal/fprint"
)

func TestTheReaderStandsInForTheEntryWhileItListensUntilThePasswordIsChosen(t *testing.T) {
	shows := 0
	f := NewFingerprintReader(func() { shows++ })
	if f.Shown() {
		t.Fatal("a reader not yet listening is shown")
	}
	if f.Follow(fprint.Listening) || !f.Shown() || shows != 1 {
		t.Fatalf("listening: shown %v after %d shows", f.Shown(), shows)
	}
	if f.Follow(fprint.Listening) || shows != 1 {
		t.Errorf("listening again showed it %d times", shows)
	}
	for _, r := range []fprint.Result{fprint.Match, fprint.NoMatch, fprint.Retry} {
		if !f.Follow(r) {
			t.Errorf("result %d is no scan", r)
		}
	}
	f.ChoosePassword()
	if f.Shown() || shows != 2 {
		t.Fatalf("the password chosen: shown %v after %d shows", f.Shown(), shows)
	}
	f.Stop()
	f.Follow(fprint.Listening)
	if f.Shown() {
		t.Error("a reader listening again took back the chosen password")
	}
	f.Reset()
	f.Follow(fprint.Listening)
	if !f.Shown() {
		t.Error("after a reset the listening reader is not shown")
	}
	if f.Follow(fprint.Stopped) || f.Shown() {
		t.Error("a stopped reader is still shown")
	}
}

func TestOnlyAFailedScanHasAMessage(t *testing.T) {
	for _, r := range []fprint.Result{fprint.NoMatch, fprint.Retry} {
		if FingerprintMessage(r) == "" {
			t.Errorf("result %d has no message", r)
		}
	}
	for _, r := range []fprint.Result{fprint.Match, fprint.Listening, fprint.Stopped} {
		if got := FingerprintMessage(r); got != "" {
			t.Errorf("result %d says %q", r, got)
		}
	}
}
