package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"
)

// The bar page's Layout preview drew its mock bar wider than the column
// and cut it after the system tray, its right modules never showing.
// A bar wider than its preview is drawn whole, scaled down to fit.
func TestTheBarLayoutPreviewFitsAWideBar(t *testing.T) {
	win := laidOutPage(t, "", barPageID, windowWidth)
	fits := treetest.All[*fitWidth](win.page)
	if len(fits) == 0 {
		t.Fatal("the bar page shows no fitted preview")
	}
	f := fits[0]
	slot := f.Parent().(widget.Boundser).Bounds().W
	nat := f.child.Measure(widget.Row, -1).Nat
	if nat <= slot {
		t.Fatalf("the test needs a bar wider than its %dpx preview, the bar is %dpx", slot, nat)
	}
	if drawn := int(float64(f.child.(widget.Boundser).Bounds().W) * f.scaleFor(slot)); drawn > slot {
		t.Errorf("the bar draws %dpx wide in a %dpx preview", drawn, slot)
	}
	if b := f.child.(widget.Boundser).Bounds(); b.W < nat {
		t.Errorf("the bar is laid out %dpx wide, under its natural %dpx", b.W, nat)
	}
}
