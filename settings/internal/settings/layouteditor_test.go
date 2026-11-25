package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// The bar layout's zones were as wide as their chips, so CENTER, holding
// only the clock, ran its heading into its + with 6px between, while
// LEFT and RIGHT kept wide gaps; and the display tabs stood 30px tall
// beside their 36px +. The zones share one width, so each heading has
// the same room before its +, and the tabs are as tall as the + beside
// them.
func TestTheBarLayoutsZonesShareOneWidthAndItsTabsTheirPlusHeight(t *testing.T) {
	win := laidOutPage(t, "", "bar", sweepW)
	c := treetest.First[*layoutEditor](t, win.page)
	zones := treetest.WithClass(c, "layout-zone")
	if len(zones) != zoneCount {
		t.Fatalf("the layout shows %d zones", len(zones))
	}
	var widths []int
	for _, z := range zones {
		widths = append(widths, z.(widget.Boundser).Bounds().W)
	}
	for i := 1; i < len(zones); i++ {
		if widths[i] != widths[0] {
			t.Errorf("the zones are %v wide", widths)
			break
		}
	}
	add := treetest.Button(t, c, locale.T().Get("settings-layout-add")).Bounds()
	if tabs := c.tabs.Bounds(); tabs.H != add.H {
		t.Errorf("the display tabs are %dpx tall beside a %dpx +", tabs.H, add.H)
	}
}
