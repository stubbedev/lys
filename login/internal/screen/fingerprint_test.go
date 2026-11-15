package screen

import (
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/login/internal/fprint"
)

func fingerprintPrompt(t *testing.T, onPassword func(p *Prompt)) *Prompt {
	t.Helper()
	var p *Prompt
	p = newPrompt(Options{Font: treetest.Font(t), Fingerprint: &Fingerprint{
		Text:       "Touch the reader or type your password",
		OnPassword: func() { onPassword(p) },
	}}, func(string) {})
	return p
}

func TestTheFingerprintPromptStandsInForTheEntry(t *testing.T) {
	p := fingerprintPrompt(t, func(*Prompt) {})
	if p.Fingerprint.Visible() || !p.Entry.Visible() || p.FocusTarget() != p.Entry {
		t.Fatal("before SetFingerprint the entry is the prompt")
	}
	p.SetFingerprint(true)
	if !p.Fingerprint.Visible() || p.Entry.Visible() || p.FocusTarget() != p.Fingerprint {
		t.Fatal("SetFingerprint(true) does not put the fingerprint prompt in the entry's place")
	}
	p.SetFingerprint(false)
	if p.Fingerprint.Visible() || !p.Entry.Visible() || p.FocusTarget() != p.Entry {
		t.Fatal("SetFingerprint(false) does not bring the entry back")
	}
}

func TestTypingOverTheFingerprintPromptKeepsTheFirstCharacter(t *testing.T) {
	switched := 0
	p := fingerprintPrompt(t, func(p *Prompt) {
		switched++
		p.SetFingerprint(false)
	})
	p.SetFingerprint(true)
	p.Reveal.SetDuration(0)
	p.Reveal.SetRevealed(true)
	r := &widget.Router{Root: p.Reveal}
	if err := r.SetFocus(p.FocusTarget()); err != nil {
		t.Fatal(err)
	}
	r.Type('h')
	if switched != 1 || p.Fingerprint.Visible() || p.Entry.Text() != "h" {
		t.Fatalf("switched %d times, fingerprint shown %v, entry %q; want one switch and the entry holding h", switched, p.Fingerprint.Visible(), p.Entry.Text())
	}
}

func TestClickingTheFingerprintPromptAsksForThePassword(t *testing.T) {
	switched := 0
	p := fingerprintPrompt(t, func(*Prompt) { switched++ })
	p.SetFingerprint(true)
	p.Fingerprint.OnClick()
	if switched != 1 || p.Entry.Text() != "" {
		t.Fatalf("switched %d times, entry %q", switched, p.Entry.Text())
	}
}

func TestWithoutAFingerprintTheEntryStays(t *testing.T) {
	p := newPrompt(Options{Font: treetest.Font(t)}, func(string) {})
	p.SetFingerprint(true)
	if p.Fingerprint != nil || !p.Entry.Visible() || p.FocusTarget() != p.Entry {
		t.Fatal("a prompt without a fingerprint lost its entry")
	}
}

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
