package settings

import (
	"testing"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

// After a page opened from within another (Battery, CPU from a link) the
// focus fell from the page dropped to the window's close button, which
// showed its ring as the page loaded. The page itself takes the keys,
// scrolling under them; a sidebar pick leaves the keys in the sidebar.
func TestAPageOpenedFromAnotherTakesTheKeys(t *testing.T) {
	win := laidOutPage(t, "", "notifications", sweepW)
	var focused []widget.Widget
	win.focus = func(w widget.Widget) { focused = append(focused, w) }
	win.navigate("fonts")
	if len(focused) != 0 {
		t.Errorf("a sidebar pick moved the keys to %T", focused[0])
	}
	win.openPage("cpu")
	if len(focused) != 0 {
		t.Fatalf("the keys moved to %T before the page was laid out", focused[0])
	}
	layOut(win, sweepW)
	if len(focused) != 1 || focused[0] != widget.Widget(win.page) {
		t.Fatalf("opening a page gave the keys to %v, want the page", focused)
	}
	if widget.FirstFocusable(win.page) != widget.Widget(win.page) {
		t.Error("the page itself is no focus stop")
	}
	win.root.Arrange(render.Rect{W: sweepW, H: 300})
	win.page.KeyAction(widget.KeyEnd, 0)
	r := win.page.AxisRange(widget.Column)
	if !r.Overflows() || r.Offset != r.Max() {
		t.Errorf("End scrolled the page to %d of %d", r.Offset, r.Max())
	}
	win.page.KeyAction(widget.KeyHome, 0)
	if win.page.AxisRange(widget.Column).Offset != 0 {
		t.Error("Home left the page scrolled")
	}
}
