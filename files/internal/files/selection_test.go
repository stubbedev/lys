package files

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/xkb"
	"github.com/stubbedev/lys/ui/app"
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

// Ctrl+A selects every file, Ctrl+Shift+A none, Ctrl+Shift+I the rest
// and Escape none, in the list and the grid; none of them acts while a
// text field has the key.
func TestTheSelectionKeys(t *testing.T) {
	hs := newHarness(t)
	hs.open(hs.root)
	all := []string{"docs", "pics", "a.png", "b.txt"}
	for _, grid := range []bool{false, true} {
		hs.do(func() { hs.f.active().b.SetGrid(grid) })
		hs.pick(t, "pics")
		hs.key('a', app.ModCtrl, nil)
		if got := hs.selected(); !slices.Equal(got, all) {
			t.Errorf("grid %v: Ctrl+A selected %v, want %v", grid, got, all)
		}
		hs.key('a', app.ModCtrl|app.ModShift, nil)
		if got := hs.selected(); len(got) != 0 {
			t.Errorf("grid %v: Ctrl+Shift+A left %v selected", grid, got)
		}
		hs.pick(t, "pics", "b.txt")
		hs.key('i', app.ModCtrl|app.ModShift, nil)
		if got := hs.selected(); !slices.Equal(got, []string{"docs", "a.png"}) {
			t.Errorf("grid %v: Ctrl+Shift+I selected %v, want [docs a.png]", grid, got)
		}
		hs.key(xkb.KeyEscape, 0, nil)
		if got := hs.selected(); len(got) != 0 {
			t.Errorf("grid %v: Escape left %v selected", grid, got)
		}
	}

	var entry widget.Widget
	hs.run(func() { entry = treetest.First[*widget.SearchEntry](t, hs.f.Root()) })
	hs.pick(t, "pics")
	for _, k := range []struct {
		sym  xkb.Keysym
		mods app.Mods
	}{{'a', app.ModCtrl | app.ModShift}, {'i', app.ModCtrl | app.ModShift}, {xkb.KeyEscape, 0}} {
		hs.key(k.sym, k.mods, entry)
		if got := hs.selected(); !slices.Equal(got, []string{"pics"}) {
			t.Errorf("%v with mods %v in the search field changed the selection to %v", k.sym, k.mods, got)
		}
	}
}

// The background menu offers Select All, and the selection's menu the
// selection commands under Selection; Select All is disabled while
// every file is selected.
func TestTheMenusOfferTheSelectionCommands(t *testing.T) {
	hs := newHarness(t)
	hs.open(hs.root)
	all := []string{"docs", "pics", "a.png", "b.txt"}
	hs.backgroundMenu(t, msg("files-select-all"))
	if got := hs.selected(); !slices.Equal(got, all) {
		t.Errorf("Select All selected %v, want %v", got, all)
	}
	var items []widget.MenuItem
	hs.run(func() { items = hs.f.menu(hs.f.active(), hs.f.active().b.Selected()) })
	sub := items[len(items)-1]
	if sub.Label != msg("files-selection") || len(sub.Items) != 3 || !sub.Items[0].Disabled {
		t.Errorf("the selection's menu ends in %q %v, want Selection with Select All disabled first", sub.Label, labels(sub.Items))
	}
	hs.pick(t, "docs", "b.txt")
	hs.menu(t, msg("files-selection"), msg("files-invert-selection"))
	if got := hs.selected(); !slices.Equal(got, []string{"pics", "a.png"}) {
		t.Errorf("Invert Selection selected %v, want [pics a.png]", got)
	}
	hs.menu(t, msg("files-selection"), msg("files-select-none"))
	if got := hs.selected(); len(got) != 0 {
		t.Errorf("Select None left %v selected", got)
	}
	hs.do(func() { hs.f.active().b.GoTo(hs.path("docs")) })
	hs.run(func() { items = hs.f.menu(hs.f.active(), nil) })
	if last := items[len(items)-1]; last.Label != msg("files-select-all") || !last.Disabled {
		t.Errorf("an empty folder's menu ends in %q (disabled %v), want Select All disabled", last.Label, last.Disabled)
	}
}

// The status line offers Select All while some file is not selected and
// Select None while some is, and each does what it says; an offer that
// does not apply stays in place, disabled, so the line keeps its
// height.
func TestTheStatusLineOffersSelectAllAndNone(t *testing.T) {
	hs := newHarness(t)
	hs.open(hs.root)
	offers := func() (all, none bool) {
		hs.run(func() {
			tb := hs.f.active()
			all, none = widget.IsEnabled(tb.selectAll), widget.IsEnabled(tb.none)
		})
		return all, none
	}
	if all, none := offers(); !all || none {
		t.Errorf("nothing selected: offers Select All %v, Select None %v; want only Select All", all, none)
	}
	hs.pick(t, "pics")
	if all, none := offers(); !all || !none {
		t.Errorf("one of four selected: offers Select All %v, Select None %v; want both", all, none)
	}
	hs.do(func() { hs.f.active().selectAll.OnClick() })
	if got := hs.selected(); len(got) != 4 {
		t.Errorf("the status line's Select All selected %v", got)
	}
	if all, none := offers(); all || !none {
		t.Errorf("all selected: offers Select All %v, Select None %v; want only Select None", all, none)
	}
	hs.do(func() { hs.f.active().none.OnClick() })
	if got := hs.selected(); len(got) != 0 {
		t.Errorf("the status line's Select None left %v selected", got)
	}
	statusH := func() int {
		hs.shot(nil)
		var h int
		hs.run(func() { h = treetest.WithClass(hs.f.Root(), "files-status")[0].(widget.Boundser).Bounds().H })
		return h
	}
	full := statusH()
	hs.do(func() { hs.f.active().b.GoTo(hs.path("docs")) })
	if all, none := offers(); all || none {
		t.Errorf("an empty folder: offers Select All %v, Select None %v; want neither", all, none)
	}
	if got := statusH(); got != full {
		t.Errorf("the status line is %dpx tall in an empty folder, %dpx with files", got, full)
	}
}
