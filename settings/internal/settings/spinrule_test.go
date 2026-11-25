package settings

import (
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/shell/apptheme"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

// Spins filled the page's control column (Max Attempts 270px for "0")
// and differed from card to card (Length 68px, Words 174px), their arrows
// moving with their unit. On every page a row's spin ends on the control
// edge, the spins of a page share one width, and so do their units, so
// their arrows stand on one x.
func TestEveryPagesSpinsEndOnTheControlEdgeAtOneWidth(t *testing.T) {
	k := testKit(t, geometryYAML)
	cfg := k.store.svc.Config()
	w := newWindow(k, cfg, func(_ time.Duration, fn func()) { fn() })
	w.stack.SetTransition(widget.StackNone, 0)
	apptheme.New(cfg).Attach(w.root)
	pages := 0
	for _, sec := range w.sections {
		for _, p := range sec.pages {
			w.show(p.id)
			w.root.Arrange(render.Rect{W: sweepW, H: sweepH})
			var edge int
			for _, r := range w.page.rows {
				if r.ControlFit() == widget.ItemControlColumn && shown(r.ctl) {
					b := r.ctl.(widget.Boundser).Bounds()
					edge = b.X + b.W
				}
			}
			widths := map[bool]map[int]bool{false: {}, true: {}}
			for _, r := range w.page.rows {
				if _, ok := r.ctl.(interface{ spin() }); !ok || !shown(r.ctl) {
					continue
				}
				b := r.ctl.(widget.Boundser).Bounds()
				if edge > 0 && b.X+b.W != edge {
					t.Errorf("%s: %s's spin ends at %d, the control edge at %d", p.id, r.path, b.X+b.W, edge)
				}
				_, withUnit := r.ctl.(*unitNumber)
				widths[withUnit][b.W] = true
			}
			for withUnit, ws := range widths {
				if len(ws) > 1 {
					t.Errorf("%s: its spins (with a unit %v) are %v wide, want one width", p.id, withUnit, ws)
				}
				if len(ws) > 0 {
					pages++
				}
			}
		}
	}
	if pages == 0 {
		t.Fatal("no page shows a spin")
	}
}
