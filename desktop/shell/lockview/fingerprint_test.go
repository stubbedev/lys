package lockview

import (
	"testing"

	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/desktop/shell/treetest"
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
