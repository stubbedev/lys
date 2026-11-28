package files

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

// The file views click as Nautilus's and Finder's do: a plain click
// selects the row alone, Ctrl+click adds or takes away one row,
// Shift+click selects the range from the last row clicked.
func TestClicksSelectAsAFileManagersDo(t *testing.T) {
	hs := newHarness(t)
	hs.open(hs.root)
	hs.pick(t, "docs", "pics", "a.png", "b.txt")
	hs.pressRow(t, widget.BTNLeft, 0, "a.png")
	if got := hs.selected(); !slices.Equal(got, []string{"a.png"}) {
		t.Errorf("a plain click left %v selected, want [a.png]", got)
	}
	hs.pressRow(t, widget.BTNLeft, widget.ModCtrl, "docs")
	if got := hs.selected(); !slices.Equal(got, []string{"docs", "a.png"}) {
		t.Errorf("ctrl+click on docs left %v selected, want [docs a.png]", got)
	}
	hs.pressRow(t, widget.BTNLeft, widget.ModCtrl, "a.png")
	if got := hs.selected(); !slices.Equal(got, []string{"docs"}) {
		t.Errorf("ctrl+click on a.png again left %v selected, want [docs]", got)
	}
	hs.pressRow(t, widget.BTNLeft, 0, "docs")
	hs.pressRow(t, widget.BTNLeft, widget.ModShift, "b.txt")
	if got := hs.selected(); !slices.Equal(got, []string{"docs", "pics", "a.png", "b.txt"}) {
		t.Errorf("shift+click on b.txt left %v selected, want the range [docs pics a.png b.txt]", got)
	}
}

// A click on the file view where no file is clears the selection; with
// Ctrl held it keeps it, so a Ctrl+click that misses takes nothing away.
func TestAClickOnEmptySpaceClearsTheSelection(t *testing.T) {
	hs := newHarness(t)
	hs.open(hs.root)
	below := func(_ *tab, list render.Rect) widget.Point {
		return widget.Point{X: list.X + list.W/2, Y: list.Y + list.H - 4}
	}
	for _, grid := range []bool{false, true} {
		hs.do(func() { hs.f.active().b.SetGrid(grid) })
		hs.pick(t, "docs", "b.txt")
		hs.clickList(t, widget.BTNLeft, widget.ModCtrl, below)
		if got := hs.selected(); !slices.Equal(got, []string{"docs", "b.txt"}) {
			t.Errorf("grid %v: a ctrl+click on empty space left %v selected, want [docs b.txt]", grid, got)
		}
		hs.clickList(t, widget.BTNLeft, 0, below)
		if got := hs.selected(); len(got) != 0 {
			t.Errorf("grid %v: a click on empty space left %v selected", grid, got)
		}
	}
}
