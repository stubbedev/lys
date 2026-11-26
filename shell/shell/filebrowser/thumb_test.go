package filebrowser

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/desktop/thumbnail"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/ui/widget"
)

// photoFolder is a folder of n small PNG photos in the harness's tree,
// and their paths.
func photoFolder(t *testing.T, hs *harness, n int) (string, []string) {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	dir := filepath.Join(hs.root, "photos")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 40, 30))); err != nil {
		t.Fatal(err)
	}
	paths := make([]string, n)
	for i := range paths {
		paths[i] = filepath.Join(dir, string(rune('a'+i))+".png")
		if err := os.WriteFile(paths[i], buf.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir, paths
}

// scrollGrid builds every grid cell, as scrolling through them does.
func (hs *harness) scrollGrid() {
	hs.do(func() {
		grid := rows{hs.c, true}
		for i := range grid.Len() {
			grid.Row(i)
		}
	})
}

// thumbIDs are the identities of the request's thumbnails found.
func (hs *harness) thumbIDs() []render.ImageID {
	var ids []render.ImageID
	hs.run(func() {
		for _, r := range hs.c.thumbs.found {
			if r != nil {
				ids = append(ids, r.Image.ID)
			}
		}
	})
	slices.Sort(ids)
	return ids
}

// Closing the dialog frees every thumbnail it found: the renderer
// forgets each one and its pixels go back; while it is open none is.
// Opening it again makes them anew.
func TestClosingFreesEveryThumbnailAndReopeningMakesThemAgain(t *testing.T) {
	hs := newHarness(t, "")
	dir, paths := photoFolder(t, hs, 3)
	open := func() []render.ImageID {
		hs.open(t, in(openFile, dir))
		hs.scrollGrid()
		for _, p := range paths {
			hs.waitThumb(t, p, gridIconPx)
		}
		ids := hs.thumbIDs()
		if len(ids) != len(paths) {
			t.Fatalf("%d thumbnails found, want %d", len(ids), len(paths))
		}
		if live := hs.c.thumbnailer.Live(); live != len(paths) {
			t.Errorf("%d thumbnails allocated while the dialog is open, want %d", live, len(paths))
		}
		if len(hs.forgotten) != 0 {
			t.Errorf("the open dialog's thumbnails %v were forgotten", hs.forgotten)
		}
		return ids
	}
	closeDialog := func(ids []render.ImageID) {
		hs.hide()
		hs.run(func() {
			if live := hs.c.thumbnailer.Live(); live != 0 {
				t.Errorf("%d thumbnails still allocated after the dialog closed", live)
			}
			if len(hs.c.thumbs.found) != 0 || len(hs.c.thumbs.waiting) != 0 {
				t.Errorf("the closed dialog still holds %d thumbnails and %d lookups", len(hs.c.thumbs.found), len(hs.c.thumbs.waiting))
			}
			got := slices.Sorted(slices.Values(hs.forgotten))
			if !slices.Equal(got, ids) {
				t.Errorf("forgot %v, want every thumbnail %v", got, ids)
			}
			hs.forgotten = nil
		})
	}
	first := open()
	closeDialog(first)
	again := open()
	if slices.ContainsFunc(again, func(id render.ImageID) bool { return slices.Contains(first, id) }) {
		t.Errorf("the reopened dialog draws the freed thumbnails %v again", first)
	}
	cells := 0
	hs.run(func() {
		grid := rows{hs.c, true}
		for i := range grid.Len() {
			for _, img := range treetest.All[*widget.Image](grid.Row(i)) {
				if img.Loaded() {
					cells++
				}
			}
		}
	})
	if cells != len(paths) {
		t.Errorf("%d cells draw a thumbnail after reopening, want %d", cells, len(paths))
	}
	closeDialog(again)
}

// A lookup still waiting when the dialog closes gives up and lands
// nothing; one that finished as it closed is freed instead of shown.
func TestClosingCancelsThumbnailsInFlight(t *testing.T) {
	hs := newHarness(t, "")
	dir, paths := photoFolder(t, hs, 3)
	budget, release := holdBudget(t, hs)
	th := hs.c.thumbnailer
	hs.open(t, in(openFile, dir))
	hs.scrollGrid()
	waittest.Until(t, func() bool { return budget.Waiting() == len(paths) }, "every lookup to wait for the budget")
	hs.hide()
	waittest.Until(t, func() bool { return budget.Waiting() == 0 }, "the lookups to stop waiting")
	release()
	waittest.Until(t, func() bool { return budget.Waiting() == 0 && budget.Held() == 0 }, "the lookups to end")
	hs.flush()
	hs.run(func() {
		if n := len(hs.c.thumbs.found); n != 0 {
			t.Errorf("%d cancelled lookups landed", n)
		}
		if live := th.Live(); live != 0 {
			t.Errorf("%d thumbnails allocated after the cancelled lookups", live)
		}
	})

	late, err := th.Square(context.Background(), paths[0], "", 2*gridIconPx)
	if err != nil {
		t.Fatal(err)
	}
	hs.run(func() {
		hs.c.landThumb(hs.c.thumbs, thumbKey{paths[0], gridIconPx}, late, nil)
		if len(hs.c.thumbs.found) != 0 {
			t.Error("a thumbnail that finished after its dialog closed landed")
		}
		if live := th.Live(); live != 0 {
			t.Errorf("the late thumbnail is still allocated (%d)", live)
		}
	})
}

// Thumbnails made after the loop ended land nowhere: Close frees them
// with the ones the dialog found, leaving no allocation behind.
func TestCloseFreesThumbnailsMadeAfterTheLoopEnded(t *testing.T) {
	hs := newHarness(t, "")
	dir, paths := photoFolder(t, hs, 3)
	budget, release := holdBudget(t, hs)
	th := hs.c.thumbnailer
	hs.open(t, in(openFile, dir))
	hs.scrollGrid()
	waittest.Until(t, func() bool { return budget.Waiting() == len(paths) }, "every lookup to wait for the budget")
	hs.endLoop()
	release()
	waittest.Until(t, func() bool { return th.Live() == len(paths) }, "every lookup to make its thumbnail after the loop ended")
	hs.c.Close()
	if live := th.Live(); live != 0 {
		t.Errorf("%d thumbnails still allocated after Close", live)
	}
}

// holdBudget gives hs a thumbnailer whose decode budget the test holds
// whole until it calls release.
func holdBudget(t *testing.T, hs *harness) (budget *thumbnail.Budget, release func()) {
	t.Helper()
	budget = thumbnail.NewBudget(thumbnail.DecodeBudget)
	hs.c.thumbnailer = thumbnail.New(thumbnail.Config{Budget: budget})
	release, err := budget.Acquire(context.Background(), budget.Total())
	if err != nil {
		t.Fatal(err)
	}
	return budget, release
}
