package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"
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

// A frameless icon button ending a line (a list entry's trash, a
// threshold card's) drew its glyph centred in its 36px box, 10px inside
// the edge every field and switch of its card ends on. Its glyph ends on
// that edge.
func TestATrailingGlyphButtonEndsOnTheControlEdge(t *testing.T) {
	glyphEnd := func(b *widget.Button) int {
		ic := treetest.All[*widget.Icon](b)[0].Bounds()
		return ic.X + ic.W/2 + appIconPx/2
	}
	trailing := func(root widget.Widget, icon string) []*widget.Button {
		var out []*widget.Button
		for _, b := range treetest.All[*widget.Button](root) {
			ics := treetest.All[*widget.Icon](b)
			if !shown(b) || !b.HasClass(css.Ghost) || len(ics) != 1 || ics[0].Name() != icon {
				continue
			}
			if p, ok := b.Parent().(*widget.Box); ok && p.Children()[len(p.Children())-1] == widget.Widget(b) {
				out = append(out, b)
			}
		}
		return out
	}
	win := laidOutPage(t, "", "thresholds", sweepW)
	checked := 0
	for _, e := range treetest.All[*thresholdEditor](win.page) {
		edge := 0
		for _, n := range treetest.All[*optionalNumber](e) {
			if shown(n) {
				edge = n.Bounds().X + n.Bounds().W
			}
		}
		for _, b := range trailing(e, "ld-trash-2-symbolic") {
			checked++
			if got := glyphEnd(b); got != edge {
				t.Errorf("a threshold's trash glyph ends at %d, its fields at %d", got, edge)
			}
		}
	}
	if checked == 0 {
		t.Fatal("the Thresholds page shows no trailing trash button")
	}
}
