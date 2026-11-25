package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"
)

// A passphrase preset's switch rows (Capitalize, Include a Number) stood
// 26px apart while its field rows stood 35px and more apart, a switch's
// row only as tall as the switch. Every row of a form whose control sits
// at its end stands as tall as the others.
func TestAFormsSwitchRowsStandAsTallAsItsFieldRows(t *testing.T) {
	k := testKit(t, "")
	grid := k.formGrid()
	field := grid.row(grid.label("Separator"), k.entry(), false)
	toggle := grid.row(grid.label("Capitalize"), widget.NewSwitch(true), false)
	form := gapBox(widget.Column)
	form.Append(field, false)
	form.Append(toggle, false)
	themed(k, form)
	if f, s := field.Bounds().H, toggle.Bounds().H; f != s || f == 0 {
		t.Errorf("a field's row is %dpx tall, a switch's %dpx", f, s)
	}
	sw := treetest.First[*widget.Switch](t, toggle).Bounds()
	if row := toggle.Bounds(); sw.Y+sw.H/2 < row.Y+row.H/2-1 || sw.Y+sw.H/2 > row.Y+row.H/2+1 {
		t.Errorf("the switch at y=%d..%d is off its %d..%d row's centre", sw.Y, sw.Y+sw.H, row.Y, row.Y+row.H)
	}
}
