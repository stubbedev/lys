package settings

import (
	"math"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"
)

func dockIconSize(k *kit) *iconScaleControl {
	return fieldControl(k, k.store, field("dock.icon-size", iconScale(config.TaskIconBaseRem, 1, 4))).(*iconScaleControl)
}

func TestTheDockIconSizeSlidesBetweenASmallAndALargeIconInPixels(t *testing.T) {
	k := testKit(t, "")
	c := dockIconSize(k)
	if icons := treetest.All[*widget.Icon](c); len(icons) != 2 {
		t.Fatalf("the size has %d end icons, want a small and a large one", len(icons))
	}
	if c.slider.Value() != 2 || c.slider.ValueLabel().Text() != unitPixels.format(0)(42) {
		t.Fatalf("the default scale of 2 shows %v as %q", c.slider.Value(), c.slider.ValueLabel().Text())
	}
	c.slider.OnCommit(3)
	if got := k.store.value("dock.icon-size"); got != 3.0 {
		t.Fatalf("sliding to 3 stored %v (%T), want the scale", got, got)
	}
}

func TestADockIconSizeInPixelsReadsAsItsScale(t *testing.T) {
	k := testKit(t, "dock:\n  icon-size: 52px\n")
	c := dockIconSize(k)
	if v := c.slider.Value(); math.Abs(v-2.5) > 1e-9 || c.slider.ValueLabel().Text() != unitPixels.format(0)(52) {
		t.Fatalf("52px shows %v as %q, want the scale 2.5", v, c.slider.ValueLabel().Text())
	}
	if got := k.store.value("dock.icon-size"); got != "52px" {
		t.Errorf("reading the size rewrote it to %v", got)
	}
}
