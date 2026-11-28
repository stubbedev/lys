package filebrowser

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stubbedev/lys/lib/xkb"
	"github.com/stubbedev/lys/ui/widget"
)

// A standalone window browses with no request behind it: it maps
// titled, its file view focused, lists the folder, enters a folder
// activated and walks back, opens Quick Look on Space, and Escape,
// which the browse hands to its host, hides it.
func TestStandaloneBrowsesAFolderAndOpensQuickLook(t *testing.T) {
	hs := newHarness(t, "")
	hs.open(t, Options{Title: "Files", Selection: widget.SelectionMultiple, Gathered: true, NewFolder: true, OnEscape: hs.s.Hide})
	if hs.cfg.Title != "Files" || hs.windows[0].focus != hs.c.FileView() {
		t.Fatalf("mapped %q focused on %T, want Files on its file view", hs.cfg.Title, hs.windows[0].focus)
	}
	if got := hs.shown(); !slices.Equal(got, []string{"docs", "pics", "a.png", "b.txt"}) {
		t.Errorf("listing = %v", got)
	}
	hs.pick(t, "pics")
	hs.do(func() { hs.c.activeView().KeyAction(widget.KeyEnter, 0) })
	if dir := hs.dir(); dir != filepath.Join(hs.root, "pics") {
		t.Fatalf("Enter on pics shows %s", dir)
	}
	hs.do(func() { hs.cfg.OnPress(btnBack, nil) })
	if dir := hs.dir(); dir != hs.root {
		t.Fatalf("back shows %s, want the folder it started in", dir)
	}
	hs.do(func() { hs.cfg.OnPress(btnForward, nil) })
	hs.pick(t, "a.png")
	hs.key(xkb.KeySpace, 0, nil)
	var shown bool
	var path string
	hs.do(func() { shown, path = hs.c.ui.quicklook.shown, hs.c.ui.ql.Path() })
	if !shown || path != filepath.Join(hs.root, "pics", "a.png") {
		t.Errorf("Space showed Quick Look %v on %q, want it on pics/a.png", shown, path)
	}
	hs.key(xkb.KeyEscape, 0, nil)
	hs.do(func() { shown = hs.c.ui.quicklook.shown })
	if shown || hs.windows[0].closed {
		t.Errorf("the first Escape left Quick Look %v and closed the window %v, want Quick Look closed alone", shown, hs.windows[0].closed)
	}
	hs.key(xkb.KeyEscape, 0, nil)
	if !hs.windows[0].closed {
		t.Error("Escape with Quick Look closed left the window mapped")
	}
	hs.key(xkb.KeySpace, 0, nil)
	hs.do(func() { shown = hs.c.ui.quicklook.shown })
	if shown {
		t.Error("a hidden browse took a key")
	}
}

// A window that cannot map, or cannot take the focus, is an error and
// leaves nothing browsing; showing again replaces the window showing.
func TestStandaloneThatCannotMapShowsNothing(t *testing.T) {
	hs := newHarness(t, "")
	hs.openErr = errors.New("no compositor")
	var err error
	hs.run(func() { err = hs.s.Show(Options{Folder: hs.root}) })
	if !errors.Is(err, hs.openErr) || hs.c.active {
		t.Errorf("an unmapped window: %v, browsing %v; want the open error and no browse", err, hs.c.active)
	}
	hs.openErr, hs.focusErr = nil, errors.New("surface gone")
	hs.run(func() { err = hs.s.Show(Options{Folder: hs.root}) })
	if !errors.Is(err, hs.focusErr) || hs.c.active || !hs.windows[0].closed {
		t.Errorf("an unfocusable window: %v, browsing %v, closed %v; want the focus error, no browse, the window closed", err, hs.c.active, hs.windows[0].closed)
	}
	hs.focusErr = nil
	hs.open(t, Options{})
	hs.open(t, Options{})
	if !hs.windows[1].closed || hs.windows[2].closed {
		t.Error("showing again did not replace the window showing")
	}
}
