package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/barview"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"
)

func steppedOf(t *testing.T, k *kit, page string) *steppedPreview {
	t.Helper()
	spec, ok := pageByID(layout(config.Defaults(), bothParts), page)
	if !ok {
		t.Fatalf("no page %q", page)
	}
	for _, s := range spec.sections {
		if s.preview == nil {
			continue
		}
		if p, ok := s.preview(k).(*steppedPreview); ok {
			return p
		}
	}
	t.Fatalf("the %s page has no preview stepping through states", page)
	return nil
}

func chipNames(p *steppedPreview) []string {
	out := make([]string, len(p.states))
	for i, s := range p.states {
		out[i] = s.name
	}
	return out
}

func previewLabel(t *testing.T, p *steppedPreview) string {
	t.Helper()
	buttons := treetest.All[*barview.Button](p.shown)
	if len(buttons) != 1 {
		t.Fatalf("the preview holds %d buttons, want the module's one", len(buttons))
	}
	return buttons[0].Label().Text()
}

func TestTheKeybindModePreviewNamesLysWMsSubmaps(t *testing.T) {
	k, _, _, _ := wmKit(t, "submaps:\n  resize:\n    - {chord: Escape, action: kill}\n  move:\n    - {chord: Escape, action: kill}\n")
	if err := k.store.set("modules.keybind-mode.format", "[{{ mode }}]"); err != nil {
		t.Fatal(err)
	}
	p := steppedOf(t, k, "keybind-mode")
	if got, want := chipNames(p), []string{"Default", "move", "resize"}; !slices.Equal(got, want) {
		t.Fatalf("chips %v, want %v", got, want)
	}
	p.toggles[2].OnToggled(true)
	if got := previewLabel(t, p); got != "[resize]" {
		t.Errorf("the resize chip previews %q, want [resize]", got)
	}
	if p.toggles[1].Active() || !p.toggles[2].Active() {
		t.Error("the picked chip is not the one marked")
	}
	if p.problem.Visible() {
		t.Errorf("reading the submaps reports %q", p.problem.Text())
	}
}

func TestTheKeyboardInputChipCyclesThroughTheConfiguredLayouts(t *testing.T) {
	k, _, _, _ := wmKit(t, "input:\n  keyboard:\n    layout: us,de\n    variant: \",nodeadkeys\"\n")
	if err := k.store.set("modules.keyboard-input.format", "{{ alias }}"); err != nil {
		t.Fatal(err)
	}
	if err := k.store.set("modules.keyboard-input.layout-alias-map", map[string]any{"English (US)": "EN"}); err != nil {
		t.Fatal(err)
	}
	p := steppedOf(t, k, "keyboard-input")
	if got, want := chipNames(p), []string{"English (US)", "German (no dead keys)"}; !slices.Equal(got, want) {
		t.Fatalf("chips %v, want %v", got, want)
	}
	var seen []string
	for range 3 {
		seen = append(seen, previewLabel(t, p))
		treetest.All[*barview.Button](p.shown)[0].ClickAt(widget.Point{})
	}
	if want := []string{"EN", "German (no dead keys)", "EN"}; !slices.Equal(seen, want) {
		t.Errorf("pressing the chip showed %v, want %v", seen, want)
	}
}

func TestALayoutOutsideTheXKBRegistryIsSaidNotGuessed(t *testing.T) {
	k, _, _, _ := wmKit(t, "input:\n  keyboard:\n    layout: us,zz\n")
	p := steppedOf(t, k, "keyboard-input")
	if !p.problem.Visible() || len(p.states) != 1 {
		t.Errorf("an unknown layout gave states %v and problem shown %v, want the one known layout and the problem said", chipNames(p), p.problem.Visible())
	}
	if p.chips.Visible() {
		t.Error("a single state offers chips to pick it")
	}
}

func TestWithoutLysWMThePreviewKeepsItsSamples(t *testing.T) {
	k := testKit(t, "")
	k.parts.wm = false
	p := steppedOf(t, k, "keybind-mode")
	if len(p.states) != 0 || p.chips.Visible() || p.problem.Visible() {
		t.Errorf("without lys-wm the preview offers %v, chips shown %v, problem shown %v", chipNames(p), p.chips.Visible(), p.problem.Visible())
	}
	if previewLabel(t, p) == "" {
		t.Error("without lys-wm the preview shows no sample label")
	}
}
