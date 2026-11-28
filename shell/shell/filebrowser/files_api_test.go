package filebrowser

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/shell/filebrowser/browsertest"
)

// SetGrid switches the layout as the header's toggle does, and setting
// the layout showing changes nothing.
func TestSetGridSwitchesTheLayout(t *testing.T) {
	hs := newHarness(t, "")
	hs.open(t, openFile)
	var grid, listShown bool
	hs.do(func() { hs.c.SetGrid(true) })
	hs.run(func() { grid, listShown = hs.c.Grid(), hs.c.ui.list.Visible() })
	if !grid || listShown {
		t.Fatalf("SetGrid(true): grid %v, the list showing %v", grid, listShown)
	}
	hs.do(func() { hs.c.SetGrid(true) })
	hs.run(func() { grid = hs.c.Grid() })
	if !grid {
		t.Error("SetGrid(true) on a grid toggled it back to a list")
	}
	hs.do(func() { hs.c.SetGrid(false) })
	hs.run(func() { grid, listShown = hs.c.Grid(), hs.c.ui.list.Visible() })
	if grid || !listShown {
		t.Errorf("SetGrid(false): grid %v, the list showing %v", grid, listShown)
	}
}

// Reload reads a listing no watch follows again, keeping the file
// selected; without it the listing stays as first read.
func TestReloadReadsTheListingAgain(t *testing.T) {
	hs := newHarness(t, "")
	trash := filepath.Join(t.TempDir(), "Trash")
	browsertest.Trashed(t, trash, "notes.txt.2", browsertest.TrashInfo("/home/u/notes.txt"))
	hs.c.d.TrashDirs = func() []string { return []string{trash} }
	hs.open(t, openFile)
	hs.do(func() { hs.c.goTo(TrashLocation) })
	hs.pick(t, "notes.txt")
	browsertest.Trashed(t, trash, "plan.txt", browsertest.TrashInfo("/home/u/plan.txt"))
	if shown := hs.shown(); !slices.Equal(shown, []string{"notes.txt"}) {
		t.Fatalf("the trash shows %v before a reload", shown)
	}
	hs.do(hs.c.Reload)
	if shown := hs.shown(); !slices.Equal(shown, []string{"notes.txt", "plan.txt"}) {
		t.Errorf("the trash shows %v after a reload", shown)
	}
	if got := hs.selectedNames(); !slices.Equal(got, []string{"notes.txt"}) {
		t.Errorf("a reload left %v selected", got)
	}
}

// SelectAll, SelectNone and InvertSelection act on the view showing, in
// the list and the grid; SelectAll with every file selected tells the
// host nothing new, and a single-selection browse selects no more than
// it can.
func TestTheSelectionCommands(t *testing.T) {
	hs := newHarness(t, "")
	var heard int
	hs.open(t, Options{Selection: widget.SelectionMultiple, OnSelection: func([]Entry) { heard++ }})
	all := []string{"docs", "pics", "a.png", "b.txt"}
	for _, grid := range []bool{false, true} {
		hs.do(func() { hs.c.SetGrid(grid) })
		hs.do(hs.c.SelectAll)
		if got := hs.selectedNames(); !slices.Equal(got, all) {
			t.Errorf("grid %v: SelectAll selected %v", grid, got)
		}
		heard = 0
		hs.do(hs.c.SelectAll)
		if heard != 0 {
			t.Errorf("grid %v: SelectAll with all selected told the host %d times", grid, heard)
		}
		hs.pick(t, "pics")
		hs.do(hs.c.InvertSelection)
		if got := hs.selectedNames(); !slices.Equal(got, []string{"docs", "a.png", "b.txt"}) {
			t.Errorf("grid %v: InvertSelection selected %v", grid, got)
		}
		hs.do(hs.c.SelectNone)
		if got := hs.selectedNames(); len(got) != 0 {
			t.Errorf("grid %v: SelectNone left %v selected", grid, got)
		}
	}

	single := newHarness(t, "")
	single.open(t, openFile)
	single.pick(t, "a.png")
	single.do(single.c.SelectAll)
	single.do(single.c.InvertSelection)
	if got := single.selectedNames(); !slices.Equal(got, []string{"a.png"}) {
		t.Errorf("a single-selection browse selected %v after SelectAll and InvertSelection", got)
	}
	single.do(single.c.SelectNone)
	if got := single.selectedNames(); len(got) != 0 {
		t.Errorf("SelectNone left %v selected in a single-selection browse", got)
	}
}
