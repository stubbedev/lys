package widgets

import (
	"testing"

	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/desktop/config"
)

func TestPlacementPutsASurfaceWhereItsScreenPositionSays(t *testing.T) {
	want := map[config.ScreenPosition][2]widget.Align{
		config.ScreenTopLeft:      {widget.AlignStart, widget.AlignStart},
		config.ScreenTopCenter:    {widget.AlignCenter, widget.AlignStart},
		config.ScreenTopRight:     {widget.AlignEnd, widget.AlignStart},
		config.ScreenCenterLeft:   {widget.AlignStart, widget.AlignCenter},
		config.ScreenCenterRight:  {widget.AlignEnd, widget.AlignCenter},
		config.ScreenBottomLeft:   {widget.AlignStart, widget.AlignEnd},
		config.ScreenBottomCenter: {widget.AlignCenter, widget.AlignEnd},
		config.ScreenBottomRight:  {widget.AlignEnd, widget.AlignEnd},
	}
	meta, ok := config.Field("osd.position")
	if !ok || len(meta.Variants) != len(want) {
		t.Fatalf("osd.position offers %v, want the %d screen positions", meta.Variants, len(want))
	}
	for _, v := range meta.Variants {
		pos := config.ScreenPosition(v)
		if h, v := Placement(pos); h != want[pos][0] || v != want[pos][1] {
			t.Errorf("%s places at %v/%v, want %v/%v", pos, h, v, want[pos][0], want[pos][1])
		}
	}
}

func TestPlacementRefusesAPositionThatIsNone(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("an unknown position was placed")
		}
	}()
	Placement("middle")
}
