package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// A lys-wm colour without a default (a blur's tint) showed Custom… while
// unset, with no colour to edit. It lists None ahead of Custom…, shows it
// while the colour is unset, and choosing None unsets the colour again.
func TestAnUnsetWMColourWithoutADefaultReadsNone(t *testing.T) {
	k, _, _, _ := wmKit(t, "")
	var stored any
	s := slot{get: func() any { return stored }, set: func(v any) error { stored = v; return nil }, unset: func() { stored = nil }}
	c := wmColorEditor(k, s, true)
	if got := c.ids[c.drop.Selected()]; got != colorUnset || c.swatch.Visible() {
		t.Fatalf("an unset tint shows row %q, the colour button %v", got, c.swatch.Visible())
	}
	if got := c.shown[c.drop.Selected()].label; got != locale.T().Get("settings-color-none") {
		t.Errorf("the unset row reads %q", got)
	}
	c.drop.SetSelected(slices.Index(c.ids, "blue"))
	if stored != "blue" {
		t.Fatalf("picking blue stored %v", stored)
	}
	c.drop.SetSelected(slices.Index(c.ids, colorUnset))
	if stored != nil || c.swatch.Visible() {
		t.Errorf("picking None left %v, the colour button %v", stored, c.swatch.Visible())
	}
	if withDefault := newWMColorEditor(k, s); slices.Contains(withDefault.ids, colorUnset) {
		t.Error("a colour with a default lists None")
	}
}

// Auto, None and Custom… led no swatch where every colour row does, so
// their labels started left of the colours'. Every row but a header
// leads with a swatch or an empty slot of the swatch's place.
func TestEveryColourRowLeadsWithASwatchOrItsSlot(t *testing.T) {
	k := testKit(t, "")
	c := fieldControl(k, k.store, field("modules.clock.icon-color")).(*swatchPicker)
	root, host := hosted(c)
	click(root, c.drop)
	if len(host.opened) != 1 {
		t.Fatalf("a click opened %d popovers, want the list", len(host.opened))
	}
	list := host.opened[0]
	c.drop.Close()
	rows := 0
	for _, n := range c.shown {
		if !n.header {
			rows++
		}
	}
	led := len(treetest.All[*widget.ColorSwatch](list)) + len(treetest.All[swatchSlot](list))
	if led != rows || len(treetest.All[swatchSlot](list)) == 0 {
		t.Errorf("%d of %d rows lead with a swatch or its slot", led, rows)
	}
}
