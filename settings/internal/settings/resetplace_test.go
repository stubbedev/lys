package settings

import (
	"testing"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

// Reset buttons sat beside a slider on one row, beside a switch on
// another and on the control column's edge on a third. resetPlacement
// holds on every page: showing every setting row's reset button leaves
// every control where it was with them hidden, and a row's reset stands
// immediately before its control, the theme's spacing from it, whatever
// the control; a control with a line of its own keeps its reset on the
// title's line.
func resetPlacement() pageCheck {
	checked := 0
	return pageCheck{
		visit: func(t *testing.T, w *window) {
			at := func(on bool) map[*fieldRow]render.Rect {
				out := map[*fieldRow]render.Rect{}
				for _, r := range w.page.rows {
					r.reset.show(on)
				}
				w.root.Arrange(render.Rect{W: sweepW, H: sweepH})
				for _, r := range w.page.rows {
					if shown(r.ctl) {
						out[r] = r.ctl.(widget.Boundser).Bounds()
					}
				}
				return out
			}
			hidden, showing := at(false), at(true)
			for r, b := range hidden {
				if got := showing[r]; got != b {
					t.Errorf("%s: showing %s's reset moved its control from %v to %v", w.pageID, r.path, b, got)
				}
			}
			gap := widget.Current().Spacing
			for _, r := range w.page.rows {
				if !shown(r.ctl) || r.ControlFit() == widget.ItemControlWide {
					continue
				}
				checked++
				b, c := r.reset.Bounds(), showing[r]
				if got := c.X - (b.X + b.W); got != gap {
					t.Errorf("%s: %s's reset ends %dpx before its control, want %d", w.pageID, r.path, got, gap)
				}
			}
			for _, r := range w.page.rows {
				r.refreshSource()
			}
			w.root.Arrange(render.Rect{W: sweepW, H: sweepH})
		},
		done: func(t *testing.T) {
			if checked == 0 {
				t.Fatal("no row with a reset was checked")
			}
		},
	}
}
