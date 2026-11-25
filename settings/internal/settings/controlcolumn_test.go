package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"
)

// A row nested in another row's control - the dock's All displays, a
// monitor's own wallpaper - took a row's inset twice, so its switch
// ended 17px short of every other switch on the page. Every shown
// switch of a page ends on the one control column.
func TestEverySwitchOfAPageEndsOnOneColumn(t *testing.T) {
	for _, id := range []string{"dock", "notifications", "osd", "wallpaper", "theme"} {
		t.Run(id, func(t *testing.T) {
			win := laidOutPage(t, "", id, windowWidth)
			edge := -1
			for _, sw := range treetest.All[*widget.Switch](win.page) {
				b := sw.Bounds()
				if b.W == 0 || !shownIn(win.page, sw) {
					continue
				}
				if right := b.X + b.W; edge < 0 {
					edge = right
				} else if right != edge {
					t.Errorf("a switch ends at %d, the first at %d", right, edge)
				}
			}
			if edge < 0 {
				t.Fatal("the page shows no switch")
			}
		})
	}
}

// shownIn reports whether w and every ancestor up to root shows.
func shownIn(root widget.Widget, w widget.Widget) bool {
	shown := true
	treetest.Walk(root, func(k widget.Widget) {
		if v, ok := k.(interface{ Visible() bool }); ok && !v.Visible() && treetest.Contains(k, w) {
			shown = false
		}
	})
	return shown
}
