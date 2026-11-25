package launcherview

import (
	"testing"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/desktop/config"
)

// A still caps its panel to the room below the panel's top, but painted
// the panel's list past it, the next rows showing under the panel's
// bottom edge. A list longer than the room ends at the panel's edge.
func TestAStillsListStaysInsideItsPanel(t *testing.T) {
	out := render.Rect{W: 600, H: 900}
	cfg := config.Defaults()
	s := NewSession(Spec{
		Font: rowStyle(t).Font, Config: cfg, Look: Look{Width: 400, Lines: 30, Location: config.LauncherCenter},
		Row: func(int) widget.Widget {
			b := widget.NewBox(widget.Row, 0, 0)
			b.SetInlineStyle("background-color: #ff0000; min-height: 24px;")
			return b
		},
	})
	s.SetRows(40)
	still := NewStill(s)
	still.Arrange(out)
	panel := still.Panel().Bounds()
	if s.Content.Bounds().H <= panel.H || panel.Y+panel.H+2 >= out.H {
		t.Fatalf("the list (%v) does not run past the panel %v inside %v", s.Content.Bounds(), panel, out)
	}
	stride := render.Stride(out.W)
	data := make([]byte, stride*out.H)
	still.Paint(render.New(data, stride, out.W, out.H))
	for y := panel.Y + panel.H + 1; y < out.H; y++ {
		x := panel.X + panel.W/2
		if r := data[y*stride+4*x+2]; r > 0x80 {
			t.Fatalf("the list paints red at (%d, %d), under its panel %v", x, y, panel)
		}
	}
}
