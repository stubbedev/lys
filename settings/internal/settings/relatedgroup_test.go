package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
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
