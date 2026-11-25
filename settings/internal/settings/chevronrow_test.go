package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"
)

// linksTitled are the rows under root that open what tooltip says.
func linksTitled(root widget.Widget, tooltip string) []*settingRow {
	var out []*settingRow
	for _, r := range treetest.All[*settingRow](root) {
		if r.TooltipText() == tooltip {
			out = append(out, r)
		}
	}
	return out
}

// A link row's chevron was a 36px icon button, its glyph ending about
// 14px inside the control edge every switch of the page ends on. The
// whole row is the link, a click on it opening its page, and its chevron
// ends on that edge.
func TestALinkRowIsTheLinkWithItsChevronOnTheControlEdge(t *testing.T) {
	win := laidOutPage(t, "", "notifications", sweepW)
	var opened []string
	win.k.openPage = func(id string) { opened = append(opened, id) }
	links := treetest.All[*pageLinkRow](win.page)
	if len(links) == 0 {
		t.Fatal("the Notifications page shows no link row")
	}
	edge := -1
	for _, sw := range treetest.All[*widget.Switch](win.page) {
		if b := sw.Bounds(); b.W > 0 && shownIn(win.page, sw) {
			edge = b.X + b.W
		}
	}
	if edge < 0 {
		t.Fatal("the page shows no switch")
	}
	for _, l := range links {
		if len(treetest.All[*widget.Button](l)) != 0 {
			t.Errorf("the %s link holds a button of its own", l.page)
		}
		chevrons := treetest.WithClass(l, "row-chevron")
		if len(chevrons) != 1 {
			t.Fatalf("the %s link shows %d chevrons", l.page, len(chevrons))
		}
		if b := chevrons[0].(widget.Boundser).Bounds(); b.X+b.W != edge {
			t.Errorf("the %s link's chevron ends at %d, the switches at %d", l.page, b.X+b.W, edge)
		}
		l.ClickAt(widget.Point{})
	}
	if len(opened) != len(links) || opened[0] != links[0].page {
		t.Errorf("clicking the links opened %q", opened)
	}
}
