package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"
)

// A Related card's group title ("Sleep & Lock") wore the page's section
// headings' bold caption style inside the card, so it read as a section
// heading. It is a plain caption of the card; the section's own title
// keeps the heading style.
func TestARelatedGroupTitleIsACaptionNotASectionHeading(t *testing.T) {
	win := laidOutPage(t, "", powerPageID, sweepW)
	heads := treetest.WithClass(win.page, "settings-link-group")
	if len(heads) != 1 {
		t.Fatalf("the Battery & Power Mode page shows %d related group titles", len(heads))
	}
	head := heads[0].(*widget.Label)
	if head.HasClass(css.CaptionHeading) || !head.HasClass(css.Caption) {
		t.Errorf("the group title %q is styled as a section heading", head.Text())
	}
	titles := treetest.WithClass(win.page, "settings-section-title")
	if len(titles) == 0 || !titles[0].(*widget.Label).HasClass(css.CaptionHeading) {
		t.Fatal("the page's section titles lost their heading style")
	}
}

// Power's and Lock's Related cards opened with a dim "Sleep & Lock" or
// "Locking and Unlocking" row inside the card, read as one more link.
// A titled group's title is a caption over its own card, outside every
// card, the card under it holding the group's links. The caption stood
// as far from the card above as from its own, attached to neither; it
// is held to its own card.
func TestARelatedGroupTitleIsACaptionOverItsCard(t *testing.T) {
	for _, id := range []string{powerPageID, "lock"} {
		win := laidOutPage(t, "", id, sweepW)
		heads := treetest.WithClass(win.page, "settings-link-group")
		if len(heads) != 1 {
			t.Fatalf("the %s page shows %d related group titles", id, len(heads))
		}
		head := heads[0]
		for w := head.(interface{ Parent() widget.Widget }).Parent(); w != nil; w = w.(interface{ Parent() widget.Widget }).Parent() {
			if c, ok := w.(interface{ HasClass(string) bool }); ok && c.HasClass(css.Card) {
				t.Errorf("the %s page's group title sits inside a card", id)
				break
			}
		}
		section := head.(interface{ Parent() widget.Widget }).Parent().(*widget.Box)
		kids := section.Children()
		at := slices.Index(kids, head)
		if at < 0 || at+1 >= len(kids) {
			t.Fatalf("the %s page's group title ends its section", id)
		}
		card, ok := kids[at+1].(*widget.Box)
		if !ok || !card.HasClass(css.Card) || len(treetest.All[*pageLinkRow](card)) == 0 {
			t.Errorf("the %s page's group title is not over a card of links", id)
		}
		b, c := head.(widget.Boundser).Bounds(), kids[at+1].(widget.Boundser).Bounds()
		if b.Y+b.H > c.Y {
			t.Errorf("the %s page's group title at %+v runs into its card at %+v", id, b, c)
		}
		cards := treetest.WithClass(win.page, css.Card)
		var above render.Rect
		for _, w := range cards {
			if r := w.(widget.Boundser).Bounds(); r.Y+r.H <= b.Y && r.Y+r.H > above.Y+above.H {
				above = r
			}
		}
		if under, over := c.Y-(b.Y+b.H), b.Y-(above.Y+above.H); over < 2*under {
			t.Errorf("the %s page's group title sits %dpx under the card above and %dpx over its own; want it held to its own", id, over, under)
		}
	}
}
