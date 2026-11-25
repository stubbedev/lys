package settings

import (
	"testing"

	"github.com/stubbedev/lys/ui/widget"
)

// A colour list put its swatch inside the dropdown on a named colour's
// row and beside it, before the dropdown, on a custom colour's, so the
// dropdowns of one card started at two x. Every colour's dropdown starts
// at one x with the colour's swatch in its face, a custom one's colour
// button following it on its right.
func TestEveryColourDropdownStartsAtOneXWithItsSwatchInside(t *testing.T) {
	k := testKit(t, "modules:\n  clock:\n    icon-color: \"#123456\"\n    label-color: red\n")
	custom := fieldControl(k, k.store, field("modules.clock.icon-color")).(*swatchPicker)
	named := fieldControl(k, k.store, field("modules.clock.label-color")).(*swatchPicker)
	col := gapBox(widget.Column)
	col.Append(custom, false)
	col.Append(named, false)
	themed(k, col)
	if c, n := custom.drop.Bounds().X, named.drop.Bounds().X; c != n {
		t.Errorf("a custom colour's dropdown starts at x=%d, a named one's at %d", c, n)
	}
	if b, d := custom.swatch.Bounds(), custom.drop.Bounds(); b.X < d.X+d.W {
		t.Errorf("the colour button at x=%d stands before the dropdown ending at %d", b.X, d.X+d.W)
	}
	for _, p := range []*swatchPicker{custom, named} {
		if !p.shown[p.drop.Selected()].swatch {
			t.Errorf("the dropdown reading %q shows no swatch in its face", p.drop.Selection())
		}
	}
}
