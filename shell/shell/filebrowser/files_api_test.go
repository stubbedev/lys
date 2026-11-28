package filebrowser

import (
	"path/filepath"
	"slices"
	"testing"

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
