package settings

import (
	"maps"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
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

// Each tile took its own text's width, so the tiles of every line split
// at a different place. Every line of two or more tiles splits evenly, at
// the same places as every other line.
func TestOverviewTilesSplitEveryLineAlike(t *testing.T) {
	win := laidOutPage(t, "", overviewID, 1664)
	lines := map[int][]render.Rect{}
	for _, b := range treetest.All[*widget.FlowBoxChild](win.page) {
		r := b.Bounds()
		lines[r.Y] = append(lines[r.Y], r)
	}
	var split []render.Rect
	for _, y := range slices.Sorted(maps.Keys(lines)) {
		line := lines[y]
		if len(line) < 2 {
			continue
		}
		if split == nil {
			split = line
			continue
		}
		if len(line) != len(split) {
			t.Errorf("a line holds %d tiles, another %d", len(line), len(split))
			continue
		}
		for i := range line {
			if line[i].X != split[i].X || line[i].W != split[i].W {
				t.Errorf("a tile spans %d..%d, the one above it %d..%d", line[i].X, line[i].X+line[i].W, split[i].X, split[i].X+split[i].W)
			}
		}
	}
	if split == nil {
		t.Fatal("no line holds two tiles")
	}
}
