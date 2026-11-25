package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"
)

// The category tiles lay against the card's border: they sit inset in
// it as a row's content does.
func TestTheCategoryTilesAreInsetInTheirCard(t *testing.T) {
	defaultAppsFixture(t)
	w := laidOutPage(t, "", defaultAppsID, 1000)
	tiles := treetest.All[*appTile](w.page)
	if len(tiles) == 0 {
		t.Fatal("no category tile")
	}
	for _, tile := range tiles {
		insetInItsCard(t, w.page, tile, "the "+tileName(tile)+" tile")
	}
}

// A tile's title was a dim caption under its status line's body text,
// which ran to the tile's edges: the title reads larger than the status
// and undimmed, and the status sits inside the tile's padding.
func TestACategoryTileTitlesItsStatus(t *testing.T) {
	defaultAppsFixture(t)
	w := laidOutPage(t, "", defaultAppsID, 1000)
	pad := widget.Current().Padding
	face := treetest.Font(t)
	for _, name := range []string{"Web browser", "Email", "PDF documents"} {
		tile := tileNamed(t, w.page, name)
		if title, status := treetest.RenderedTextPx(tile.title, face), treetest.RenderedTextPx(tile.app, face); title <= status {
			t.Errorf("%s: the title is %.1fpx, its status %.1fpx", name, title, status)
		}
		if widget.HasClass(tile.title, css.DimLabel) || !widget.HasClass(tile.app, css.DimLabel) {
			t.Errorf("%s: the title is dimmed %v, its status %v", name, widget.HasClass(tile.title, css.DimLabel), widget.HasClass(tile.app, css.DimLabel))
		}
		b, s := tile.button.Bounds(), tile.app.Bounds()
		if left, right := s.X-b.X, b.X+b.W-(s.X+s.W); left < pad || right < pad {
			t.Errorf("%s: the status %q sits %dpx and %dpx inside the tile, want at least %d", name, tile.app.Text(), left, right, pad)
		}
	}
}

// With no file type to list, the File Types card ended in a separator
// along its bottom edge: a separator shows only between two rows.
func TestTheFileTypesCardEndsInNoSeparator(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", t.TempDir())
	t.Setenv("XDG_DATA_DIRS", t.TempDir())
	w := laidOutPage(t, "", defaultAppsID, 1000)
	d := treetest.First[*defaultApps](t, w.page)
	for _, sep := range treetest.All[*widget.Separator](d.types) {
		if shown(sep) {
			t.Errorf("an empty File Types card shows a separator at %+v", sep.Bounds())
		}
	}
	defaultAppsFixture(t)
	w = laidOutPage(t, "", defaultAppsID, 1000)
	d = treetest.First[*defaultApps](t, w.page)
	seps := 0
	for _, sep := range treetest.All[*widget.Separator](d.types) {
		if shown(sep) {
			seps++
		}
	}
	if want := len(d.shown); seps != want {
		t.Errorf("%d separators between the filter and %d types, want %d", seps, len(d.shown), want)
	}
}

// A tile's status line sat 3px from its bottom border, its icon 5px from
// the top. Both stand a gap's width inside the tile, the same at the top
// and the bottom.
func TestACategoryTileHoldsItsContentOffItsEdges(t *testing.T) {
	defaultAppsFixture(t)
	w := laidOutPage(t, "", defaultAppsID, 1000)
	gap := widget.Current().Spacing
	for _, tile := range treetest.All[*appTile](w.page) {
		b := tile.button.Bounds()
		icon := treetest.All[*widget.Icon](tile.button)[0].Bounds()
		s := tile.app.Bounds()
		top, bottom := icon.Y-b.Y, b.Y+b.H-(s.Y+s.H)
		if top < gap || bottom < gap || top-bottom > 2 || bottom-top > 2 {
			t.Errorf("%s: the content stands %dpx from the top and %dpx from the bottom, want %d or more alike", tileName(tile), top, bottom, gap)
		}
	}
}

// Three category tiles a line left 156px of the card empty after them.
// The tiles share the line's width: one width each, a full line ending
// where the tiles' box ends, at every width.
func TestTheCategoryTilesShareTheLinesWidth(t *testing.T) {
	defaultAppsFixture(t)
	for _, width := range []int{1000, sweepW} {
		w := laidOutPage(t, "", defaultAppsID, width)
		tiles := treetest.All[*appTile](w.page)
		if len(tiles) < 2 {
			t.Fatalf("%d category tiles", len(tiles))
		}
		box := treetest.First[*widget.FlowBox](t, w.page).Bounds()
		size, end := tiles[0].Bounds().W, 0
		for _, tile := range tiles {
			b := tile.Bounds()
			if b.W < size-1 || b.W > size+1 {
				t.Errorf("at %dpx the %s tile is %dpx wide, the first %dpx", width, tileName(tile), b.W, size)
			}
			end = max(end, b.X+b.W)
		}
		if end != box.X+box.W {
			t.Errorf("at %dpx the tiles end at %d, their box at %d", width, end, box.X+box.W)
		}
	}
}
