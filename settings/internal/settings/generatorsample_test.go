package settings

import (
	"errors"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func presetSample(t *testing.T, c *cardList) *generatorSample {
	t.Helper()
	samples := treetest.All[*generatorSample](c)
	if len(samples) != 1 {
		t.Fatalf("%d samples for one preset", len(samples))
	}
	return samples[0]
}

func TestAPasswordPresetShowsASampleDrawnFromItsRules(t *testing.T) {
	const path = "modules.bitwarden.generator-passwords"
	k := testKit(t, "modules:\n  bitwarden:\n    generator-passwords:\n      - {name: PIN, length: 12, charsets: [digits]}\n")
	c := fieldControl(k, k.store, field(path, passwordPresetList)).(*cardList)
	g := presetSample(t, c)
	if len(g.sample) != 12 || strings.Trim(g.sample, "0123456789") != "" || g.text.Text() != g.sample {
		t.Fatalf("a 12-digit preset samples %q, shown as %q", g.sample, g.text.Text())
	}
	if err := k.store.set(path, []any{map[string]any{"name": "PIN", "length": int64(8), "charsets": []any{"uppercase"}}}); err != nil {
		t.Fatal(err)
	}
	c.refresh()
	g = presetSample(t, c)
	if len(g.sample) != 8 || strings.ToUpper(g.sample) != g.sample || strings.Trim(g.sample, "0123456789") != g.sample {
		t.Errorf("an 8-letter uppercase preset samples %q", g.sample)
	}
	var copied string
	k.copySecret = func(s string) error { copied = s; return nil }
	g.copySample()
	if copied != g.sample || g.problem.Visible() {
		t.Errorf("copying put %q on the clipboard, want the sample %q; problem %q", copied, g.sample, g.problem.Text())
	}
	k.copySecret = func(string) error { return errors.New("no clipboard") }
	g.copySample()
	if g.text.Text() != g.sample || !g.problem.Visible() || g.problem.Text() != locale.T().Get("settings-generator-sample-copy-failed") || g.problem.detail.Text() != "no clipboard" || g.problem.detail.Visible() {
		t.Errorf("a failed copy shows %q, says %q over %q", g.text.Text(), g.problem.Text(), g.problem.detail.Text())
	}
	k.copySecret = func(string) error { return nil }
	g.copySample()
	if g.problem.Visible() {
		t.Errorf("a copy that went through still says %q", g.problem.Text())
	}
}

func TestAPassphrasePresetShowsASampleOfItsWords(t *testing.T) {
	const path = "modules.bitwarden.generator-passphrases"
	k := testKit(t, "modules:\n  bitwarden:\n    generator-passphrases:\n      - {name: Short, words: 3, separator: \"+\", capitalize: false, number: false}\n")
	c := fieldControl(k, k.store, field(path, passphrasePresetList)).(*cardList)
	words := strings.Split(presetSample(t, c).sample, "+")
	if len(words) != 3 || strings.ToLower(words[0]) != words[0] {
		t.Errorf("a three-word lowercase preset samples %q", words)
	}
}

func TestAPresetWhoseRulesDoNotHoldShowsWhyInsteadOfASample(t *testing.T) {
	rules := map[string]any{"length": int64(2)}
	field := func(key string) slot { return slot{get: func() any { return rules[key] }} }
	if _, err := passwordRules(field); err == nil || !strings.Contains(err.Error(), "length") {
		t.Fatalf("a 2-character password reads as rules: %v", err)
	}
	k := testKit(t, "")
	g := newGeneratorSample(k, field, passwordRules)
	if g.sample != "" || g.text.Text() != "" || widget.IsEnabled(g.copy) || g.problem.Text() != locale.T().Get("settings-generator-sample-failed") || !strings.Contains(g.problem.detail.Text(), "length") || g.problem.detail.Visible() {
		t.Errorf("rules that do not hold sample %q, show %q, say %q over %q, copy enabled %v", g.sample, g.text.Text(), g.problem.Text(), g.problem.detail.Text(), widget.IsEnabled(g.copy))
	}
}

// The sample's copy button was a bare glyph beside the framed draw-again
// button. Both are framed alike.
func TestASamplesCopyIsFramedAsItsRedraw(t *testing.T) {
	k := testKit(t, "")
	g := newGeneratorSample(k, func(string) slot { var v any; return memSlot(&v) }, passwordRules)
	for _, b := range []*widget.Button{g.again, g.copy} {
		if r, ok := roleOf(b); !ok || r.classes()[0] != css.Outline {
			t.Errorf("a sample button wears %v, want the outline frame", b.Classes())
		}
	}
}
