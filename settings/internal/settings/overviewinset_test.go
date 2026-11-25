package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"
)

// Overview's tiles inset their content half as far as every row of a
// card does: a tile's title starts as far in from the tile's edge as a
// row's title does from the row's.
func TestOverviewTilesInsetTheirContentAsRowsDo(t *testing.T) {
	k := testKit(t, "")
	row := k.row("Row", "")
	themed(k, sectionGroup(widget.NewItemGroup(row)))
	rowIn := treetest.LabelReading(t, row, "Row").Bounds().X - row.Bounds().X
	win := laidOutPage(t, "", overviewID, windowWidth)
	tiles := treetest.WithClass(win.page, css.Card)
	if len(tiles) == 0 {
		t.Fatal("the overview shows no tile")
	}
	for _, tile := range tiles {
		b := tile.(*widget.Button)
		title := treetest.WithClass(b, "settings-overview-title")[0].(*widget.Label)
		if in := title.Bounds().X - b.Bounds().X; in != rowIn {
			t.Errorf("the %s tile's title starts %dpx in, a row's %dpx", b.AccessibleName(), in, rowIn)
		}
	}
}
