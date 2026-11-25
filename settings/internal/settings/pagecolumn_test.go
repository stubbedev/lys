package settings

import (
	"testing"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

// tallWindowH is a window tall enough to hold the Fonts page whole.
const tallWindowH = 1080

// A page that scrolls laid its column out 4px left of one that does not,
// the scrollbar's gutter taken only while the page overflowed, so the
// page jumped sideways between them. A page that scrolls and one that
// does not set their column at one x.
func TestAPagesColumnStandsStillWhetherItScrollsOrNot(t *testing.T) {
	columnX := func(id string) (x int, scrolls bool) {
		win := laidOutPage(t, "", id, sweepW)
		for range 2 {
			win.root.Arrange(render.Rect{W: sweepW, H: tallWindowH})
		}
		col := win.page.Children()[0].(*widget.Clamp).Children()[0].(widget.Boundser).Bounds()
		return col.X, win.page.AxisRange(widget.Column).Overflows()
	}
	still, stillScrolls := columnX("fonts")
	long, longScrolls := columnX("notifications")
	if stillScrolls || !longScrolls {
		t.Fatalf("fonts scrolls %v, notifications %v; the test needs one page of each", stillScrolls, longScrolls)
	}
	if still != long {
		t.Errorf("the column stands at x=%d on a page that fits, at %d on one that scrolls", still, long)
	}
}
