package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"
)

// A list read from the system set its reload button, a bare icon, on a
// line of its own above its rows. The button is an action of the list's
// row, on its title line beside its other actions, framed like the add
// buttons of the lists around it.
func TestALiveListsReloadIsAnActionOfItsRow(t *testing.T) {
	win := laidOutPage(t, "", "systray", sweepW)
	var list *trayItemsList
	var row *fieldRow
	for _, r := range win.page.rows {
		if l, ok := r.ctl.(*trayItemsList); ok {
			list, row = l, r
		}
	}
	if list == nil {
		t.Fatal("the System Tray page shows no tray item list")
	}
	if treetest.Contains(list.Box, list.reload) {
		t.Error("the reload button still stands in the list, above its rows")
	}
	if !treetest.Contains(row, list.reload) {
		t.Fatal("the list's row does not carry its reload button")
	}
	title, reload := row.Bounds(), list.reload.Bounds()
	for _, l := range treetest.All[*widget.Label](row) {
		if l.Text() == row.Title() {
			title = l.Bounds()
		}
	}
	if reload.Y > title.Y+title.H || reload.Y+reload.H < title.Y {
		t.Errorf("the reload button at y=%d..%d is off the title's line %d..%d", reload.Y, reload.Y+reload.H, title.Y, title.Y+title.H)
	}
	if !list.reload.HasClass(css.Outline) {
		t.Error("the reload button is not framed like an add button")
	}
}
