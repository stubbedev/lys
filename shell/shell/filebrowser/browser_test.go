package filebrowser

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/lib/xkb"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/transfer"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/shell/shell/filebrowser/browsertest"
)

// A breadcrumb longer than crumbMaxChars characters keeps what fits and
// ends in an ellipsis, cut between characters, not inside one; a short
// one is shown whole.
func TestALongBreadcrumbIsShortenedBetweenCharacters(t *testing.T) {
	hs := newHarness(t, "")
	long := filepath.Join(hs.root, "re\u0301sume\u0301s-and-many-more-letters")
	if err := os.Mkdir(long, 0o755); err != nil {
		t.Fatalf("make the folder %s: %v", long, err)
	}
	hs.open(t, openFile)
	hs.do(func() { hs.c.goTo(long) })
	hs.run(func() {
		var got []string
		for _, crumb := range treetest.WithClass(hs.cfg.Root, "file-chooser-crumb") {
			got = append(got, widget.Describe(crumb).Name)
		}
		if want := []string{"Home", "re\u0301sume\u0301s-and-man…"}; !slices.Equal(got, want) {
			t.Errorf("crumbs = %q, want %q", got, want)
		}
	})
}

func TestHistoryButtonsAndUp(t *testing.T) {
	hs := newHarness(t, "")
	hs.open(t, openFile)
	hs.do(func() { hs.c.goTo(filepath.Join(hs.root, "docs")) })
	hs.do(func() { hs.cfg.OnPress(btnBack, nil) })
	if dir := hs.c.s.dir; dir != hs.root {
		t.Errorf("back = %s", dir)
	}
	hs.do(func() { hs.cfg.OnPress(btnForward, nil) })
	if dir := hs.c.s.dir; dir != filepath.Join(hs.root, "docs") {
		t.Errorf("forward = %s", dir)
	}
	hs.do(func() { hs.cfg.OnPress(widget.BTNRight, nil) })
	if dir := hs.c.s.dir; dir != filepath.Join(hs.root, "docs") {
		t.Errorf("another button moved to %s", dir)
	}
	hs.do(hs.c.goUp)
	if dir := hs.c.s.dir; dir != hs.root {
		t.Errorf("up = %s", dir)
	}
}

func TestSpaceInATextFieldTypes(t *testing.T) {
	hs := newHarness(t, "")
	hs.open(t, openFile)
	hs.pick(t, "a.png")
	r := &widget.Router{Root: hs.cfg.Root}
	hs.do(func() {
		r.SetFocus(hs.c.ui.search)
		hs.cfg.OnKey(r, treetest.Code(xkb.KeySpace), 0)
	})
	if hs.c.ui.quicklook.Visible() {
		t.Error("space in the search box opened quick look")
	}
}

func TestRecursiveSearchStreamsMatchesForTheLiveQuery(t *testing.T) {
	hs := newHarness(t, "")
	browsertest.Touch(t, hs.root, ".hidden/a.png")
	hs.open(t, openFile)
	hs.do(func() { hs.c.ui.search.SetText("a.p") })
	if got := hs.shown(); !slices.Equal(got, []string{"a.png"}) {
		t.Errorf("in-folder search = %v", got)
	}
	hs.click(t, "Search subfolders")
	want := []string{"a.png", filepath.Join("pics", "a.png")}
	waittest.Until(t, func() bool { return slices.Equal(hs.shown(), want) }, "the recursive search to show the match in pics")
	// The walk skipped the dot folder; showing hidden files walks again.
	hs.click(t, "Show hidden files")
	want = []string{"a.png", filepath.Join(".hidden", "a.png"), filepath.Join("pics", "a.png")}
	waittest.Until(t, func() bool { return slices.Equal(hs.shown(), want) }, "the recursive search to show the match in the hidden folder")
	// Navigating drops the query and its walk.
	hs.do(func() { hs.c.goTo(filepath.Join(hs.root, "docs")) })
	if hs.c.ui.search.Text() != "" || hs.c.stopWalk != nil || len(hs.shown()) != 0 {
		t.Errorf("after navigating: search %q, walk %v, rows %v", hs.c.ui.search.Text(), hs.c.stopWalk != nil, hs.shown())
	}
	if !hs.c.ui.empty.Visible() {
		t.Error("an empty folder hides the empty label")
	}
}

// A file dropped on the browser's body, over the files or a host's row
// under them, opens its folder; a drag without files, a remote file,
// or a drop on the header does not.
func TestDropNavigates(t *testing.T) {
	hs := newHarness(t, "")
	row := Section(widget.Row, "host-row")
	row.Append(widget.NewSpacer(0, 0), true)
	o := openFile
	o.Below = []widget.Widget{row}
	hs.open(t, o)
	var r *widget.Router
	var body, footer, outside widget.Point
	hs.run(func() {
		r, body = treetest.Pointer(hs.cfg.Root, hs.c.ui.list, 1280, 720)
		footer = center(row)
		outside = widget.Point{X: 2, Y: 2}
	})
	drop := func(p widget.Point, rel string) {
		t.Helper()
		hs.do(func() {
			if got := r.DragEnter([]string{"text/plain"}, p); got != "" {
				t.Errorf("a text drag at %v was accepted as %q", p, got)
			}
			if got := r.DragEnter([]string{"text/plain", uriListMime}, p); got != uriListMime {
				t.Fatalf("a file drag at %v = %q", p, got)
			}
			r.Drop(uriListMime, []byte("# comment\r\n"+hs.uri(rel)+"\r\n"), p, widget.Dropped{Action: transfer.ActionCopy})
		})
	}
	drop(body, "pics/a.png")
	if hs.dir() != filepath.Join(hs.root, "pics") {
		t.Errorf("a drop on the body navigated to %s", hs.dir())
	}
	drop(footer, "b.txt")
	if hs.dir() != hs.root {
		t.Errorf("a drop on the host's row navigated to %s", hs.dir())
	}
	hs.do(func() {
		r.DragEnter([]string{uriListMime}, body)
		r.Drop(uriListMime, []byte("https://example.com/x\n"), body, widget.Dropped{Action: transfer.ActionCopy})
	})
	if hs.dir() != hs.root {
		t.Errorf("a remote drop navigated to %s", hs.dir())
	}
	hs.run(func() {
		if got := r.DragEnter([]string{uriListMime}, outside); got != "" {
			t.Errorf("a file drag on the header was accepted as %q", got)
		}
	})
}

// waitThumb waits until the thumbnail lookup for path at px ended.
func (hs *harness) waitThumb(t *testing.T, path string, px int) {
	t.Helper()
	waittest.Until(t, hs.thumbEnded(path, px), "the thumbnail lookup for "+path+" to end")
}

// thumbEnded reports whether the thumbnail lookup for path at px ended.
func (hs *harness) thumbEnded(path string, px int) func() bool {
	return func() bool {
		var ok bool
		hs.do(func() { _, ok = hs.c.thumbs.found[thumbKey{path, px}] })
		return ok
	}
}

// A row shows its type's icon until its thumbnail is found, then the
// thumbnail; a rebuilt row shows it at once, and a file with none keeps
// the icon.
func TestImageRowsGetThumbnails(t *testing.T) {
	hs := newHarness(t, "")
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	real := filepath.Join(hs.root, "pics", "real.png")
	f, err := os.Create(real)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	hs.open(t, in(openFile, filepath.Join(hs.root, "pics")))
	var row widget.Widget
	hs.run(func() { row = rows{hs.c, false}.Row(slices.Index(names(hs.c.s.entries), "real.png")) })
	hs.waitThumb(t, real, rowIconPx)
	if thumbs := treetest.All[*widget.Image](row); len(thumbs) != 1 || !thumbs[0].Loaded() {
		t.Errorf("the waiting row holds %d images, want its filled thumbnail", len(thumbs))
	}
	hs.do(func() {
		again := treetest.All[*widget.Image](rows{hs.c, false}.Row(slices.Index(names(hs.c.s.entries), "real.png")))
		if len(again) != 1 || !again[0].Loaded() {
			t.Error("a rebuilt row looked the thumbnail up again")
		}
	})
	// The empty a.png cannot decode: its row keeps the icon.
	broken := filepath.Join(hs.root, "pics", "a.png")
	hs.run(func() { row = rows{hs.c, false}.Row(slices.Index(names(hs.c.s.entries), "a.png")) })
	hs.waitThumb(t, broken, rowIconPx)
	if len(treetest.All[*widget.Image](row)) != 0 || len(treetest.All[*widget.Icon](row)) == 0 {
		t.Error("an undecodable image lost its icon")
	}
}

// A column's grip resizes it by its left edge through the pointer,
// within the column bounds; hovering does not, and the rows re-lay on
// release, not on every motion.
func TestColumnGripResizesAndRelaysRows(t *testing.T) {
	hs := newHarness(t, "")
	hs.open(t, openFile)
	const W, H = 1800, 1200
	var grips []*columnGrip
	hs.run(func() { grips = treetest.All[*columnGrip](hs.cfg.Root) })
	if len(grips) != 3 {
		t.Fatalf("%d column grips, want one before each of size, modified and kind", len(grips))
	}
	col := grips[2]
	// paint renders the list's rows as they are cached, after laying
	// out the whole surface as its window does, so the rows inherit
	// the stylesheet's ink through their ancestors.
	paint := func() []uint8 {
		var px []uint8
		hs.do(func() {
			lay(hs.cfg.Root, W, H)
			l := hs.c.ui.list
			widget.SizeWithin(l, widget.Size{W: 600, H: 200})
			l.Arrange(render.Rect{W: 600, H: 200})
			px = make([]uint8, 600*200*4)
			l.Paint(render.New(px, 600*4, 600, 200))
		})
		return px
	}
	before := paint()
	var r *widget.Router
	var at widget.Point
	hs.run(func() { r, at = treetest.Pointer(hs.cfg.Root, col, W, H) })
	if got := r.Hovered(); got != widget.Widget(col) {
		t.Fatalf("the pointer rests on %T, want the kind column's grip", got)
	}
	hs.do(func() { r.Move(widget.Point{X: at.X - 30, Y: at.Y}) })
	if hs.c.columns[2] != 90 {
		t.Errorf("kind column = %d after hovering across its grip, want the 90 it had", hs.c.columns[2])
	}
	hs.do(func() {
		r.Move(at)
		r.Press(widget.BTNLeft, at, 0)
		r.Move(widget.Point{X: at.X - 30, Y: at.Y + 9})
	})
	if hs.c.columns[2] != 120 {
		t.Errorf("kind column = %d after dragging its edge 30px left, want 120", hs.c.columns[2])
	}
	paint()
	if w := hs.c.ui.columns[2].(widget.Boundser).Bounds().W; w != 120 {
		t.Errorf("kind header = %dpx wide mid-drag, want the column's 120", w)
	}
	hs.do(func() { r.Move(widget.Point{X: at.X - 5000, Y: at.Y}) })
	if hs.c.columns[2] != maxColumnW {
		t.Errorf("kind column = %d, want clamped to %d", hs.c.columns[2], maxColumnW)
	}
	if !slices.Equal(paint(), before) {
		t.Error("the rows re-laid mid-drag")
	}
	hs.do(func() { r.Release(widget.BTNLeft, widget.Point{X: at.X - 5000, Y: at.Y}) })
	if slices.Equal(paint(), before) {
		t.Error("releasing the grip left the rows at the old widths")
	}
	hs.run(func() { r, at = treetest.Pointer(hs.cfg.Root, col, W, H) })
	hs.do(func() { drag(r, at, widget.Point{X: at.X + 5000, Y: at.Y}) })
	if hs.c.columns[2] != minColumnW {
		t.Errorf("kind column = %d, want clamped to %d", hs.c.columns[2], minColumnW)
	}
}

// A column grows only as far as the name column keeps its floor: the
// drag stops there instead of squeezing the name or hiding the column.
func TestColumnGripKeepsTheNameFloor(t *testing.T) {
	hs := newHarness(t, "")
	hs.open(t, openFile)
	const W, H = defaultW, defaultH
	var grips []*columnGrip
	hs.run(func() { grips = treetest.All[*columnGrip](hs.cfg.Root) })
	var r *widget.Router
	var at widget.Point
	hs.do(func() { lay(hs.cfg.Root, W, H) })
	hs.run(func() { r, at = treetest.Pointer(hs.cfg.Root, grips[2], W, H) })
	hs.do(func() { drag(r, at, widget.Point{X: at.X - 300, Y: at.Y}) })
	if hs.c.columns[2] >= maxColumnW {
		t.Fatalf("kind column = %d, want held under %d by the name's floor", hs.c.columns[2], maxColumnW)
	}
	var name, kind render.Rect
	hs.do(func() {
		lay(hs.cfg.Root, W, H)
		name = hs.c.ui.sortButtons[SortName].Bounds()
		kind = hs.c.ui.sortButtons[SortKind].Bounds()
	})
	if !hs.c.ui.sortButtons[SortKind].Visible() || kind.W != hs.c.columns[2] {
		t.Errorf("kind header shown=%v %dpx wide, want shown at the column's %d", hs.c.ui.sortButtons[SortKind].Visible(), kind.W, hs.c.columns[2])
	}
	if name.W != nameMinW {
		t.Errorf("name header = %dpx wide, want held at its floor %d", name.W, nameMinW)
	}
}

func TestDroppedPaths(t *testing.T) {
	for in, want := range map[string][]string{
		"file:///a/b%20c\r\nfile:///d":              {"/a/b c", "/d"},
		"#c\nfile:///x\nhttps://e.com/y\nfile:///z": {"/x", "/z"},
		"https://e.com/x":                           nil,
		"":                                          nil,
	} {
		if got := droppedPaths([]byte(in)); !slices.Equal(got, want) {
			t.Errorf("droppedPaths(%q) = %q, want %q", in, got, want)
		}
	}
}

// Enter on several selected folders activates each; the first one
// entered replaces the listing, so the rest must not descend again
// from indices into the new one.
func TestEnterOnSeveralFoldersEntersOnlyTheFirst(t *testing.T) {
	hs := newHarness(t, "")
	browsertest.Touch(t, hs.root, "docs/d1/", "docs/d2/")
	hs.open(t, openFiles)
	hs.do(func() {
		l := hs.c.activeView()
		l.SelectAll()
		l.KeyAction(widget.KeyEnter, 0)
	})
	var dir string
	hs.do(func() { dir = hs.c.s.dir })
	if dir != filepath.Join(hs.root, "docs") {
		t.Errorf("Enter on docs and pics went to %s, want docs", dir)
	}
	// The guard lifts once the gesture is over.
	var open bool
	hs.flush()
	hs.do(func() {
		hs.c.activate(0)
		open = hs.c.s.dir == filepath.Join(hs.root, "docs", "d1")
	})
	if !open {
		t.Error("a later activation was still ignored")
	}
}

// The sidebar's sections pack at the top of its scroll, not mid-view.
func TestSidebarPacksAtTheTop(t *testing.T) {
	hs := newHarness(t, "")
	hs.open(t, openFile)
	var labelY, scrollY int
	hs.do(func() {
		widget.SizeWithin(hs.cfg.Root, widget.Size{W: 1280, H: 720})
		hs.cfg.Root.Arrange(render.Rect{W: 1280, H: 720})
		for _, l := range treetest.All[*widget.Label](hs.cfg.Root) {
			if l.Text() == "Favorites" {
				labelY = l.Bounds().Y
			}
		}
		scrollY = treetest.First[*widget.Scroll](t, hs.cfg.Root).Bounds().Y
	})
	if labelY-scrollY > 16 {
		t.Errorf("Favorites sits %dpx into the sidebar, want it at the top", labelY-scrollY)
	}
}

// A long name ellipsizes instead of pushing its row's cells out of
// line with the column headers.
func TestLongNamesKeepTheColumnsAligned(t *testing.T) {
	hs := newHarness(t, "")
	browsertest.Touch(t, hs.root, "a-file-name-long-enough-to-overflow-any-sane-name-column-width.txt")
	hs.open(t, openFile)
	xs := map[int]bool{}
	hs.do(func() {
		for i := range hs.c.s.entries {
			row := rows{hs.c, false}.Row(i)
			widget.SizeWithin(row, widget.Size{W: 600, H: ListRowH})
			row.Arrange(render.Rect{W: 600, H: ListRowH})
			cells := treetest.WithClass(row, "file-chooser-cell")
			xs[cells[0].(interface{ Bounds() render.Rect }).Bounds().X] = true
		}
	})
	if len(xs) != 1 {
		t.Errorf("the size cells start at %v, want one column", xs)
	}
}

// GTK's chooser keys: Backspace and Alt+Up go up (not while typing),
// Alt+Left and Alt+Right walk the history, Ctrl+H shows hidden files,
// Ctrl+L the location entry, Ctrl+F the search; Space opens quick look
// only without modifiers.
func TestChooserKeys(t *testing.T) {
	hs := newHarness(t, "")
	hs.open(t, in(openFile, filepath.Join(hs.root, "docs")))
	hs.key(xkb.KeyBackSpace, 0, hs.c.ui.search)
	if hs.dir() != filepath.Join(hs.root, "docs") {
		t.Error("Backspace in the search box went up")
	}
	hs.key(xkb.KeyBackSpace, 0, nil)
	if hs.dir() != hs.root {
		t.Errorf("Backspace left %s, want the parent", hs.dir())
	}
	hs.key(xkb.KeyLeft, app.ModAlt, nil)
	if hs.dir() != filepath.Join(hs.root, "docs") {
		t.Errorf("Alt+Left left %s, want back in docs", hs.dir())
	}
	hs.key(xkb.KeyRight, app.ModAlt, nil)
	hs.key(xkb.KeyUp, app.ModAlt, nil)
	if hs.dir() != filepath.Dir(hs.root) {
		t.Errorf("Alt+Right then Alt+Up left %s", hs.dir())
	}
	hs.do(func() { hs.c.goTo(hs.root) })
	hs.key('h', app.ModCtrl, nil)
	if shown := hs.shown(); !slices.Contains(shown, ".dot.txt") {
		t.Errorf("Ctrl+H left %v", shown)
	}
	hs.key('l', app.ModCtrl, nil)
	var locating bool
	hs.do(func() { locating = hs.c.ui.location.Visible() && !hs.c.ui.crumbs.Visible() })
	if !locating || hs.windows[0].focus != hs.c.ui.location {
		t.Error("Ctrl+L did not open the focused location entry")
	}
	hs.key('f', app.ModCtrl, nil)
	if hs.windows[0].focus != hs.c.ui.search {
		t.Error("Ctrl+F did not focus the search")
	}
	hs.pick(t, "a.png")
	hs.key(xkb.KeySpace, app.ModCtrl, nil)
	var ql bool
	hs.do(func() { ql = hs.c.ui.quicklook.Visible() })
	if ql {
		t.Error("Ctrl+Space opened quick look")
	}
	hs.key(xkb.KeySpace, 0, nil)
	hs.do(func() { ql = hs.c.ui.quicklook.Visible() })
	if !ql {
		t.Error("Space did not open quick look")
	}
	hs.hide()
}

// A dropped file opens its folder with it selected.
func TestDropSelectsTheFile(t *testing.T) {
	hs := newHarness(t, "")
	hs.open(t, in(openFile, filepath.Join(hs.root, "docs")))
	hs.do(func() { hs.c.dropped(filepath.Join(hs.root, "pics", "a.png")) })
	var sel Entry
	var ok bool
	hs.do(func() { sel, ok = hs.c.selected() })
	if hs.dir() != filepath.Join(hs.root, "pics") || !ok || sel.Name() != "a.png" {
		t.Errorf("drop left %s selecting %v %v", hs.dir(), sel.Name(), ok)
	}
}

// A file appearing in, then leaving, the folder shown changes its row
// without navigating, the selection kept; a change after the dialog moved on lists nothing.
func TestListingFollowsTheFolder(t *testing.T) {
	hs := newHarness(t, "")
	hs.open(t, openFile)
	hs.pick(t, "b.txt")
	if err := os.WriteFile(filepath.Join(hs.root, "c.txt"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	waittest.Until(t, func() bool { return slices.Contains(hs.shown(), "c.txt") }, "the new file to list")
	var sel Entry
	var ok bool
	hs.do(func() { sel, ok = hs.c.selected() })
	if !ok || sel.Name() != "b.txt" {
		t.Errorf("after the reload %v %v is selected, want b.txt", sel.Name(), ok)
	}
	before := hs.shown()
	if err := os.Remove(filepath.Join(hs.root, "c.txt")); err != nil {
		t.Fatal(err)
	}
	waittest.Until(t, func() bool { return !slices.Contains(hs.shown(), "c.txt") }, "the removed file to leave the list")
	if got, want := hs.shown(), slices.DeleteFunc(before, func(n string) bool { return n == "c.txt" }); !slices.Equal(got, want) {
		t.Errorf("after the removal %v, want %v", got, want)
	}
	hs.do(func() { sel, ok = hs.c.selected() })
	if !ok || sel.Name() != "b.txt" {
		t.Errorf("after the removal %v %v is selected, want b.txt", sel.Name(), ok)
	}
	hs.hide()
}

// A mount coming or going rebuilds Locations; the watch stops with the
// dialog.
func TestLocationsFollowTheMounts(t *testing.T) {
	hs := newHarness(t, "")
	var changed func()
	stopped := 0
	locations := []Place{{Label: "Computer", Path: "/", Icon: "drive-harddisk-symbolic"}}
	hs.c.d.Locations = func() []Place { return locations }
	hs.c.d.WatchLocations = func(c func()) func() {
		changed = c
		return func() { stopped++ }
	}
	hs.open(t, openFile)
	if hs.hasButton("USB stick") {
		t.Fatal("a mount listed before it came")
	}
	locations = append(locations, Place{Label: "USB stick", Path: "/run/media/u/stick", Icon: "drive-removable-media-symbolic"})
	changed()
	hs.settle()
	if !hs.hasButton("USB stick") {
		t.Error("the new mount was not listed")
	}
	hs.hide()
	if stopped != 1 {
		t.Errorf("the mount watch stopped %d times", stopped)
	}
}

// The real mount watch starts and stops without leaking its reader.
func TestWatchMountsStops(t *testing.T) {
	stop := WatchMounts(func() {})
	done := make(chan struct{})
	go func() { stop(); close(done) }()
	waittest.For(t, done, "the mount watch to stop")
}

// A folder still being read says so, not "No items".
func TestListingShowsLoading(t *testing.T) {
	hs := newHarness(t, "")
	hs.open(t, openFile)
	var empty string
	var visible bool
	hs.run(func() {
		hs.c.s.request = func(string) {}
		hs.c.goTo(filepath.Join(hs.root, "docs"))
		empty, visible = hs.c.ui.empty.Text(), hs.c.ui.empty.Visible()
		hs.c.s.loading = false
	})
	if empty != "Loading…" || !visible {
		t.Errorf("a folder being read shows %q (%v)", empty, visible)
	}
	hs.hide()
}

// dragRead is what a drag's content writes for mime.
func dragRead(t *testing.T, d *widget.DragContent, mime string) string {
	t.Helper()
	var b strings.Builder
	if err := d.Write(mime, &b); err != nil {
		t.Fatalf("write %s: %v", mime, err)
	}
	return b.String()
}

// Rows dragged out offer their files to copy, as a uri list and in
// the GNOME file managers' form, from the list and the grid alike; a
// folder pick drags folders and a trashed file its real path; nothing
// is offered while a listing loads or for rows that are gone.
func TestDragFilesOut(t *testing.T) {
	hs := newHarness(t, "")
	hs.open(t, openFiles)
	var drag *widget.DragContent
	hs.do(func() {
		rows := []int{slices.Index(names(hs.c.s.entries), "a.png"), slices.Index(names(hs.c.s.entries), "b.txt")}
		drag = hs.c.ui.list.DragContent(rows)
		if g := hs.c.ui.grid.DragContent(rows); g == nil {
			t.Error("the grid offers no drag")
		}
	})
	if drag == nil {
		t.Fatal("dragging two files offered nothing")
	}
	if drag.Actions != transfer.ActionCopy {
		t.Errorf("actions = %v, want copy only", drag.Actions)
	}
	if list := dragRead(t, drag, transfer.MimeURIList); list != hs.uri("a.png")+"\r\n"+hs.uri("b.txt")+"\r\n" {
		t.Errorf("uri list = %q", list)
	}
	if gnome := dragRead(t, drag, GnomeCopiedFiles); gnome != "copy\n"+hs.uri("a.png")+"\n"+hs.uri("b.txt") {
		t.Errorf("gnome copied files = %q", gnome)
	}
	hs.do(func() {
		if d := hs.c.ui.list.DragContent([]int{99}); d != nil {
			t.Error("a row that is gone offered a drag")
		}
		hs.c.s.loading = true
		if d := hs.c.ui.list.DragContent([]int{0}); d != nil {
			t.Error("a loading listing offered a drag")
		}
		hs.c.s.loading = false
	})
	hs.hide()

	hs.open(t, pickFolder)
	hs.do(func() {
		d := hs.c.ui.list.DragContent([]int{slices.Index(names(hs.c.s.entries), "docs")})
		if d == nil || dragRead(t, d, transfer.MimeURIList) != hs.uri("docs")+"\r\n" {
			t.Errorf("a folder pick drags %v", d)
		}
	})
	hs.hide()

	trash := filepath.Join(t.TempDir(), "Trash")
	trashed := browsertest.Trashed(t, trash, "notes.txt.2", browsertest.TrashInfo("/home/u/notes.txt"))
	hs.c.d.TrashDirs = func() []string { return []string{trash} }
	hs.open(t, openFile)
	hs.do(func() { hs.c.goTo(TrashLocation) })
	hs.do(func() {
		d := hs.c.ui.list.DragContent([]int{0})
		if d == nil || dragRead(t, d, transfer.MimeURIList) != transfer.FileURI(trashed)+"\r\n" {
			t.Errorf("a trashed file drags %v", d)
		}
	})
	hs.hide()
}

// A place's icon resolves through GTK's dash fallbacks, as the GTK
// sidebar's GThemedIcon does: folder-documents finds folder.
func TestPlaceIconFallsBackAlongTheName(t *testing.T) {
	treetest.IconTheme(t, "folder")
	if treetest.Ink(placeIcon(Place{Label: "Documents", Path: "/d", Icon: "folder-documents"})) == 0 {
		t.Error("a place icon with a present dash-shortened form drew nothing")
	}
	if treetest.Ink(placeIcon(Place{Label: "Odd", Path: "/o", Icon: "unrelated"})) != 0 {
		t.Error("an absent place icon drew something")
	}
}

// The list's rows paint the stylesheet's fills behind their cells: the
// accent behind the selected row, the hover tint under the pointer,
// and the zebra tint on every other row by model index, never on the
// rows between, however far the list scrolled.
func TestListRowsPaintTheirFills(t *testing.T) {
	hs := newHarness(t, "")
	browsertest.Touch(t, hs.root, "c.txt", "d.txt")
	hs.open(t, openFile)
	hs.pick(t, "b.txt")
	const W, H = 1200, 800
	at := hs.frame(W, H)
	var lb render.Rect
	var sel int
	hs.run(func() {
		lb = hs.c.ui.list.Bounds()
		sel = slices.Index(names(hs.c.s.entries), "b.txt")
	})
	x := lb.X + lb.W - 3
	rowY := func(i int) int { return lb.Y + i*ListRowH + ListRowH/2 }
	n := len(hs.shown())
	blank := at(x, lb.Y+lb.H-2)
	if got, want := at(x, rowY(sel)), hs.token(t, config.TokenAccent); got != want {
		t.Errorf("selected row paints %v, want the accent %v", got, want)
	}
	for i := range n {
		if i == sel {
			continue
		}
		got := at(x, rowY(i))
		if i%2 == 0 && got != blank {
			t.Errorf("row %d paints %v, want the view's own %v", i, got, blank)
		}
		if i%2 == 1 && (got == blank || got != at(x, rowY(1))) {
			t.Errorf("alt row %d paints %v, want the zebra tint row 1 has, over the view's %v", i, got, blank)
		}
	}
	var r *widget.Router
	hs.run(func() {
		r = &widget.Router{Root: hs.cfg.Root}
		r.Move(widget.Point{X: lb.X + lb.W/2, Y: rowY(0)})
	})
	at = hs.frame(W, H)
	if got, want := at(x, rowY(0)), hs.token(t, config.TokenBgHover); got != want {
		t.Errorf("hovered row paints %v, want the hover tint %v", got, want)
	}
	if got := at(x, rowY(2)); got != blank {
		t.Errorf("row 2 paints %v with the pointer elsewhere, want the view's own %v", got, blank)
	}

	for i := range 40 {
		browsertest.Touch(t, hs.root, "docs/f"+strconv.Itoa(100+i)+".txt")
	}
	hs.run(func() { r.Move(widget.Point{}) })
	hs.do(func() { hs.c.goTo(filepath.Join(hs.root, "docs")) })
	at = hs.frame(W, H)
	plain, tint := at(x, rowY(0)), at(x, rowY(1))
	if plain != blank || tint == blank {
		t.Fatalf("rows 0 and 1 of the long listing paint %v and %v, want the view's %v and the zebra tint", plain, tint, blank)
	}
	for top := 1; top <= 3; top++ {
		hs.run(func() { hs.c.ui.list.ScrollPixels(0, ListRowH) })
		at = hs.frame(W, H)
		for k := range lb.H / ListRowH {
			want := plain
			if (top+k)%2 == 1 {
				want = tint
			}
			if got := at(x, rowY(k)); got != want {
				t.Errorf("scrolled %d rows down, row %d (slot %d) paints %v, want %v", top, top+k, k, got, want)
			}
		}
	}
}

// listRowCells lays the surface out and row i in its band of the list,
// under the list as the list places it, and returns the row's icon and
// its size, modified and kind cells.
func (hs *harness) listRowCells(t *testing.T, i, w, h int) (widget.Widget, []widget.Widget) {
	t.Helper()
	hs.frame(w, h)
	var icon widget.Widget
	var cells []widget.Widget
	hs.run(func() {
		lb := hs.c.ui.list.Bounds()
		row := rows{hs.c, false}.Row(i)
		widget.SetParents(hs.c.ui.list, row)
		band := render.Rect{X: lb.X, Y: lb.Y + i*ListRowH, W: lb.W, H: ListRowH}
		row.Arrange(band)
		cells = treetest.WithClass(row, "file-chooser-cell")
		icon = treetest.All[*columnRow](row)[0].lead.(*widget.Box).Children()[0]
	})
	return icon, cells
}

// A column that gave way paints nothing: a row laid out wide and then
// at the name's floor paints its cells at the wide layout and nothing
// past the floor once they hid, though they keep their wide bounds.
// A column hidden at the name's floor takes no hits: a point where it
// sat while shown lands on none of its widgets.
func TestHiddenColumnsTakeNoHits(t *testing.T) {
	hs := newHarness(t, "")
	hs.open(t, openFile)
	const wide, narrow, h = 1200, nameMinW, ListRowH
	hs.run(func() {
		row := rows{hs.c, false}.Row(0)
		widget.SetParents(hs.c.ui.list, row)
		cells := treetest.WithClass(row, "file-chooser-cell")
		layAt := func(w int) {
			row.Arrange(render.Rect{W: w, H: h})
		}
		layAt(wide)
		centers := make([]widget.Point, len(cells))
		for i, c := range cells {
			b := c.(widget.Boundser).Bounds()
			centers[i] = widget.Point{X: b.X + b.W/2, Y: b.Y + b.H/2}
			if hit := row.HitTest(centers[i]); !treetest.Contains(c, hit) {
				t.Fatalf("shown, cell %d at %v is not hit", i, centers[i])
			}
		}
		layAt(narrow)
		for i, c := range cells {
			if hit := row.HitTest(centers[i]); hit != nil && treetest.Contains(c, hit) {
				t.Errorf("hidden, cell %d still takes the hit at %v", i, centers[i])
			}
		}
	})
}

func TestHiddenColumnsPaintNothing(t *testing.T) {
	hs := newHarness(t, "")
	hs.open(t, openFile)
	const wide, narrow, h = 1200, nameMinW, ListRowH
	paintsPast := func(px []uint8) bool {
		for y := range h {
			for x := narrow; x < wide; x++ {
				if px[(y*wide+x)*4+3] != 0 {
					return true
				}
			}
		}
		return false
	}
	var atWide, atNarrow []uint8
	var cells []widget.Widget
	hs.run(func() {
		row := rows{hs.c, false}.Row(0)
		widget.SetParents(hs.c.ui.list, row)
		cells = treetest.WithClass(row, "file-chooser-cell")
		paintAt := func(w int) []uint8 {
			row.Arrange(render.Rect{W: w, H: h})
			px := make([]uint8, wide*h*4)
			widget.PaintChild(render.New(px, wide*4, wide, h), row)
			return px
		}
		atWide = paintAt(wide)
		atNarrow = paintAt(narrow)
	})
	if !paintsPast(atWide) {
		t.Fatalf("laid out %dpx wide, the row paints nothing past %dpx", wide, narrow)
	}
	for i, c := range cells {
		if widget.IsVisible(c) {
			t.Errorf("cell %d still shows at the name's floor", i)
		}
	}
	if paintsPast(atNarrow) {
		t.Errorf("laid out at the name's floor, the row paints past it: a hidden column painted")
	}
}

// Every column header sits over its values: the same span, the same
// text alignment, the numbers flush right and the text flush left, and
// the name header starts where the row's icon does.
func TestColumnHeadersSitOverTheirValues(t *testing.T) {
	hs := newHarness(t, "")
	hs.open(t, openFile)
	icon, cells := hs.listRowCells(t, 0, 1200, 800)
	if len(cells) != 3 {
		t.Fatalf("%d cells in a row, want size, modified and kind", len(cells))
	}
	hs.run(func() {
		if hx, ix := hs.c.ui.sortLabels[SortName].Bounds().X, icon.(widget.Boundser).Bounds().X; hx != ix {
			t.Errorf("name header text at x=%d, want the icon's x=%d", hx, ix)
		}
		for i, cell := range cells {
			col := SortColumn(i + 1)
			head := hs.c.ui.sortLabels[col]
			hb, cb := head.Bounds(), cell.(widget.Boundser).Bounds()
			if hb.X != cb.X || hb.W != cb.W {
				t.Errorf("column %d header text spans x=%d+%d, want its cells' x=%d+%d", col, hb.X, hb.W, cb.X, cb.W)
			}
			if ha, ca := head.Alignment(), cell.(*widget.Label).Alignment(); ha != ca {
				t.Errorf("column %d header aligns %v, want its cells' %v", col, ha, ca)
			}
		}
		if a := hs.c.ui.sortLabels[SortSize].Alignment(); a != render.AlignEnd {
			t.Errorf("size header aligns %v, want flush right over the numbers", a)
		}
		if a := hs.c.ui.sortLabels[SortKind].Alignment(); a != render.AlignStart {
			t.Errorf("kind header aligns %v, want flush left over the text", a)
		}
	})
}

// With the preview open at the default size the name keeps its floor:
// the columns give way from the kind on, header and rows alike, and
// without the preview they all show again.
func TestColumnsGiveWayToTheName(t *testing.T) {
	hs := newHarness(t, "")
	browsertest.Touch(t, hs.root, "a-file-name-long-enough-to-overflow-any-sane-name-column-width.txt")
	hs.open(t, openFile)
	shown := func() (head, cells [3]bool) {
		_, cs := hs.listRowCells(t, 0, defaultW, defaultH)
		hs.run(func() {
			for i := range head {
				head[i] = hs.c.ui.sortButtons[i+1].Visible()
				cells[i] = cs[i].(*widget.Label).Visible()
			}
		})
		return head, cells
	}
	if head, cells := shown(); head != [3]bool{true, true, true} || cells != head {
		t.Errorf("without the preview: header %v, cells %v; want every column", head, cells)
	}
	hs.do(func() { hs.c.togglePreview(true) })
	head, cells := shown()
	if want := [3]bool{false, true, false}; head != want || cells != want {
		t.Errorf("size, modified, kind shown: header %v, cells %v; want %v", head, cells, want)
	}
	var name render.Rect
	hs.run(func() { name = hs.c.ui.sortButtons[SortName].Bounds() })
	if name.W < nameMinW {
		t.Errorf("name column = %dpx, want at least %d", name.W, nameMinW)
	}
}

// Activating a folder enters it; activating a file hands it to the
// host, which a browse without one ignores; a row that is gone
// activates nothing. One gesture's later activations index the listing
// that is gone and do nothing.
func TestActivatingAFolderEntersItAndAFileGoesToTheHost(t *testing.T) {
	hs := newHarness(t, "")
	var files []string
	o := openFile
	o.OnFile = func(e Entry) { files = append(files, e.Path) }
	hs.open(t, o)
	hs.do(func() { hs.c.activate(slices.Index(names(hs.c.s.entries), "pics")) })
	if got := hs.shown(); !slices.Equal(got, []string{"a.png"}) {
		t.Fatalf("after entering pics = %v", got)
	}
	hs.run(func() {
		crumbs := treetest.WithClass(hs.cfg.Root, "file-chooser-crumb")
		if last := crumbs[len(crumbs)-1]; widget.Describe(last).Name != "pics" || !widget.HasClass(last, "current") {
			t.Errorf("last crumb = %q, want the current pics", widget.Describe(last).Name)
		}
	})
	hs.do(func() {
		hs.c.activate(0)
		hs.c.navigating = true
		hs.c.activate(0)
		hs.c.navigating = false
		hs.c.activate(42)
	})
	if len(files) != 1 || files[0] != filepath.Join(hs.root, "pics", "a.png") {
		t.Errorf("the host heard %v, want the one file activated once", files)
	}
	hs.hide()
	hs.open(t, openFile)
	hs.do(func() { hs.c.activate(slices.Index(names(hs.c.s.entries), "b.txt")) })
	if hs.dir() != hs.root {
		t.Errorf("a file activated without a host moved to %s", hs.dir())
	}
}

// The hidden toggle shows dot files and sticks to the next browse; the
// header toggles read as toggle buttons; a sort column flips its
// direction; the sort and layout persist across restarts.
func TestHiddenAndSort(t *testing.T) {
	viewFile := filepath.Join(t.TempDir(), "state", "file-chooser-view")
	hs := newHarness(t, viewFile)
	hs.open(t, openFile)
	hs.click(t, "Show hidden files")
	if got := hs.shown(); !slices.Contains(got, ".dot.txt") {
		t.Errorf("hidden shown = %v", got)
	}
	if st := widget.Describe(hs.c.ui.hiddenTg); st.Role != widget.RoleToggleButton || !st.Pressed {
		t.Errorf("hidden toggle reads %s pressed %v, want a pressed toggle button", st.Role, st.Pressed)
	}
	if widget.Describe(hs.c.ui.recursiveToggle).Pressed {
		t.Error("the untouched subfolder toggle reads pressed")
	}
	hs.click(t, "Name ↑")
	if got := hs.shown(); !slices.Equal(got[:2], []string{"pics", "docs"}) {
		t.Errorf("name descending = %v", got)
	}
	hs.c.viewFile.Wait()
	if b, _ := os.ReadFile(viewFile); string(b) != "name desc list\n" {
		t.Errorf("persisted view = %q", b)
	}
	hs.click(t, "Toggle grid view")
	if !widget.IsVisible(hs.c.ui.grid) || widget.IsVisible(hs.c.ui.list) || widget.IsVisible(hs.c.ui.colHeader) {
		t.Error("grid view did not swap the views")
	}
	hs.c.viewFile.Wait()
	again := newHarness(t, viewFile)
	if again.c.view != (view{SortName, false, true}) {
		t.Errorf("reloaded view = %+v", again.c.view)
	}
	hs.hide()
	hs.open(t, openFile)
	if !slices.Contains(hs.shown(), ".dot.txt") || !hs.c.ui.hiddenTg.Active() {
		t.Error("the hidden toggle did not stick")
	}
}

// Typing into a file view searches; / or ~ starts a path in the
// location entry, whose Enter opens a folder, hands the host a path
// naming none, or warns when the host takes nothing.
func TestTypeAheadAndLocation(t *testing.T) {
	hs := newHarness(t, "")
	type typed struct {
		path   string
		exists bool
	}
	var heard []typed
	take := false
	o := openFile
	o.OnTypedPath = func(path string, exists bool) bool {
		heard = append(heard, typed{path, exists})
		return take
	}
	hs.open(t, o)
	var hit widget.Widget
	hs.do(func() {
		widget.SizeWithin(hs.cfg.Root, widget.Size{W: 1280, H: 800})
		hs.cfg.Root.Arrange(render.Rect{W: 1280, H: 800})
		b := hs.c.ui.list.Bounds()
		hit = hs.c.ui.listView.HitTest(widget.Point{X: b.X + b.W/2, Y: b.Y + b.H/2})
	})
	if hit != widget.Widget(hs.c.ui.listView) {
		t.Errorf("a click on the list hits %T, want the type-ahead view", hit)
	}
	hs.do(func() { hs.c.ui.listView.InsertRune('b') })
	if shown := hs.shown(); !slices.Equal(shown, []string{"b.txt"}) || hs.windows[0].focus != hs.c.ui.search {
		t.Errorf("typing b showed %v, focus %v", shown, hs.windows[0].focus)
	}
	hs.do(func() { hs.c.ui.listView.InsertRune('~') })
	var text string
	hs.do(func() { text = hs.c.ui.location.Text() })
	if text != "~" {
		t.Errorf("typing ~ seeded the location with %q", text)
	}
	hs.do(func() { hs.c.goToLocation("~/pics") })
	if hs.dir() != filepath.Join(hs.root, "pics") || len(heard) != 0 {
		t.Errorf("~/pics opened %s, the host heard %v", hs.dir(), heard)
	}
	hs.do(func() { hs.c.goToLocation("../nope") })
	var notice string
	hs.do(func() { notice = hs.c.ui.noticeLabel.Text() })
	missing := filepath.Join(hs.root, "nope")
	if notice != i18n.T("file-chooser-location-missing", i18n.Str("path", missing)) {
		t.Errorf("a missing path the host did not take noticed %q", notice)
	}
	take = true
	hs.do(func() { hs.c.ClearNotice(); hs.c.goToLocation("a.png") })
	var shown bool
	hs.do(func() { shown = hs.c.ui.notice.Visible() })
	if want := []typed{{missing, false}, {filepath.Join(hs.root, "pics", "a.png"), true}}; !slices.Equal(heard, want) || shown {
		t.Errorf("the host heard %v (notice shown %v), want %v and no warning for the path it took", heard, shown, want)
	}
}

// Create Folder, offered, makes the folder in the one shown and opens
// it; a name taken or not a name warns. A browse not offering it has
// no Create Folder.
func TestCreateFolder(t *testing.T) {
	hs := newHarness(t, "")
	hs.open(t, saving)
	if !hs.hasButton("Create Folder") {
		t.Error("a browse offering Create Folder has none")
	}
	hs.do(func() { hs.c.showNewFolder(); hs.c.ui.newFolderName.SetText("Invoices") })
	hs.do(func() { hs.c.createFolder() })
	if hs.dir() != filepath.Join(hs.root, "Invoices") {
		t.Errorf("creating left %s", hs.dir())
	}
	if info, err := os.Stat(filepath.Join(hs.root, "Invoices")); err != nil || !info.IsDir() {
		t.Errorf("no folder created: %v", err)
	}
	var notice string
	for name, want := range map[string]string{"..": i18n.T("file-chooser-folder-name-invalid", i18n.Str("name", "..")), "a/b": i18n.T("file-chooser-folder-name-invalid", i18n.Str("name", "a/b"))} {
		hs.do(func() { hs.c.showNewFolder(); hs.c.ui.newFolderName.SetText(name); hs.c.createFolder() })
		hs.do(func() { notice = hs.c.ui.noticeLabel.Text() })
		if notice != want {
			t.Errorf("%q noticed %q", name, notice)
		}
	}
	hs.do(func() {
		hs.c.goTo(hs.root)
		hs.c.showNewFolder()
		hs.c.ui.newFolderName.SetText("docs")
		hs.c.createFolder()
	})
	hs.do(func() { notice = hs.c.ui.noticeLabel.Text() })
	if notice != i18n.T("file-chooser-create-failed", i18n.Str("name", "docs")) {
		t.Errorf("a taken name noticed %q", notice)
	}
	hs.hide()
	hs.open(t, openFile)
	if hs.hasButton("Create Folder") {
		t.Error("a browse not offering Create Folder has one")
	}
}

// Recent lists the recently used files, offered with the gathered
// places, and has no parent; a folder that cannot be read says so
// instead of "No items". A browse without the gathered places offers
// no Recent.
func TestRecentAndListingErrors(t *testing.T) {
	hs := newHarness(t, "")
	recent := filepath.Join(t.TempDir(), "recently-used.xbel")
	body := `<xbel><bookmark href="` + hs.uri("b.txt") + `" modified="2026-10-01T00:00:00Z"/></xbel>`
	if err := os.WriteFile(recent, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	hs.c.d.RecentFile = recent
	hs.open(t, openFile)
	if !hs.hasButton("Recent") {
		t.Fatal("no Recent place")
	}
	hs.do(func() { hs.c.goTo(recentLocation) })
	if shown := hs.shown(); !slices.Equal(shown, []string{"b.txt"}) {
		t.Errorf("Recent shows %v", shown)
	}
	var up bool
	hs.do(func() { up = hs.c.s.up() })
	if up {
		t.Error("Recent has a parent")
	}
	locked := filepath.Join(hs.root, "locked")
	if err := os.Mkdir(locked, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })
	hs.do(func() { hs.c.goTo(locked) })
	var empty string
	hs.do(func() { empty = hs.c.ui.empty.Text() })
	if os.Geteuid() != 0 && empty != "Cannot read this folder." {
		t.Errorf("an unreadable folder shows %q", empty)
	}
	hs.hide()
	hs.open(t, saving)
	if hs.hasButton("Recent") {
		t.Error("a browse without the gathered places offers Recent")
	}
}

// Trash lists the trashed files by their original names, offered with
// the gathered places, and has no parent; a browse without them offers
// no Trash.
func TestTrashPlace(t *testing.T) {
	hs := newHarness(t, "")
	trash := filepath.Join(t.TempDir(), "Trash")
	browsertest.Trashed(t, trash, "notes.txt.2", browsertest.TrashInfo("/home/u/notes.txt"))
	hs.c.d.TrashDirs = func() []string { return []string{trash} }
	hs.open(t, openFile)
	if !hs.hasButton("Trash") {
		t.Fatal("no Trash place")
	}
	hs.do(func() { hs.c.goTo(TrashLocation) })
	if shown := hs.shown(); !slices.Equal(shown, []string{"notes.txt"}) {
		t.Errorf("Trash shows %v", shown)
	}
	var up bool
	hs.do(func() { up = hs.c.s.up() })
	if up {
		t.Error("Trash has a parent")
	}
	hs.hide()
	hs.open(t, pickFolder)
	if hs.hasButton("Trash") {
		t.Error("a browse without the gathered places offers Trash")
	}
}

// The browser stands on lys-ui's parts: bands cut by separators, ghost
// icon buttons in the header (never the old flat ones), a heading
// title a step over the text size, ghost crumbs and places, caption
// headings over the sidebar sections, dim cells.
func TestBrowserTakesLysUiParts(t *testing.T) {
	hs := newHarness(t, "")
	hs.open(t, openFile)
	hs.run(func() {
		root := hs.cfg.Root
		all := func(class string, want ...string) {
			t.Helper()
			ws := treetest.WithClass(root, class)
			if len(ws) == 0 {
				t.Fatalf("no .%s", class)
			}
			for _, w := range ws {
				for _, cl := range want {
					if !widget.HasClass(w, cl) {
						t.Errorf(".%s lacks .%s", class, cl)
					}
				}
				if widget.HasClass(w, "flat") {
					t.Errorf(".%s is still a flat button", class)
				}
			}
		}
		all("file-chooser-nav", css.Ghost, css.Icon)
		all("file-chooser-title", css.Heading)
		all("file-chooser-crumb", css.Ghost)
		all("file-chooser-place", css.Ghost)
		all("file-chooser-sidebar-header", css.CaptionHeading, css.DimLabel)
		for _, cell := range treetest.WithClass(hs.c.fileRow(hs.c.s.entries[0]), "file-chooser-cell") {
			if !widget.HasClass(cell, css.DimLabel) {
				t.Error("a row's cell is not dim")
			}
		}
		if n := len(treetest.All[*widget.Separator](root)); n != 3 {
			t.Errorf("%d separators, want the 3 between the bands", n)
		}
		title := treetest.WithClass(root, "file-chooser-title")[0].(*widget.Label)
		if want := widgets.TextPx(styling.Subtitle, 1); title.SizePx() != want {
			t.Errorf("title %vpx, want %vpx", title.SizePx(), want)
		}
	})
}
