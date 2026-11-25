package settings

import (
	"slices"
	"testing"

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

// The dropdowns' faces reserved an empty swatch slot for Auto, None and
// Custom…, so their text started a slot's width in and staggered against
// the fields around them. A row leads with a swatch only for a colour.
func TestOnlyAColourRowLeadsWithASwatch(t *testing.T) {
	k := testKit(t, "")
	c := fieldControl(k, k.store, field("modules.clock.icon-color")).(*swatchPicker)
	for i, row := range dropdownRows(c.shown) {
		if has := row.Leading != nil; has != c.shown[i].swatch {
			t.Errorf("the %q row leads with a swatch %v, has a colour %v", row.Label, has, c.shown[i].swatch)
		}
	}
}
