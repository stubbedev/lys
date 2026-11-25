package settings

import (
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/shell/apptheme"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

// Reset buttons sat beside a slider on one row, beside a switch on
// another and on the control column's edge on a third. On every page a
// row's reset stands immediately before its control, the theme's spacing
// from it, whatever the control; a control with a line of its own keeps
// its reset on the title's line.
func TestEveryResetStandsImmediatelyBeforeItsControl(t *testing.T) {
	k := testKit(t, geometryYAML)
	cfg := k.store.svc.Config()
	w := newWindow(k, cfg, func(_ time.Duration, fn func()) { fn() })
	w.stack.SetTransition(widget.StackNone, 0)
	apptheme.New(cfg).Attach(w.root)
	gap := widget.Current().Spacing
	checked := 0
	for _, sec := range w.sections {
		for _, p := range sec.pages {
			w.show(p.id)
			for _, r := range w.page.rows {
				r.reset.show(true)
			}
			w.root.Arrange(render.Rect{W: sweepW, H: sweepH})
			for _, r := range w.page.rows {
				if !shown(r.ctl) || r.ControlFit() == widget.ItemControlWide {
					continue
				}
				checked++
				b, c := r.reset.Bounds(), r.ctl.(widget.Boundser).Bounds()
				if got := c.X - (b.X + b.W); got != gap {
					t.Errorf("%s: %s's reset ends %dpx before its control, want %d", p.id, r.path, got, gap)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no row with a reset was checked")
	}
}
