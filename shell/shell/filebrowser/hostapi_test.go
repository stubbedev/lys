package filebrowser

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/lib/xkb"
	"github.com/stubbedev/lys/ui/transfer"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/shell/filebrowser/browsertest"
)

// surfaceHost records the menus the tree opens.
type surfaceHost struct{ menus [][]widget.MenuItem }

func (h *surfaceHost) OpenMenu(_ widget.Boundser, _ render.Font, _ float64, items []widget.MenuItem, _ widget.MenuOptions) {
	h.menus = append(h.menus, items)
}

func (h *surfaceHost) OpenPopover(widget.Boundser, widget.Widget, widget.PopoverOptions) func() {
	return func() {}
}

func (h *surfaceHost) Focus(widget.Widget) error { return nil }

// rightClick lays the window out and right-clicks the row reading
// name, or below the rows for "".
func (hs *harness) rightClick(t *testing.T, name string) {
	t.Helper()
	hs.clickRow(t, widget.BTNRight, name)
}

// clickRow lays the window out and clicks button on the list row
// reading name, or below the rows for "".
func (hs *harness) clickRow(t *testing.T, button uint32, name string) {
	t.Helper()
	hs.do(func() {
		lay(hs.cfg.Root, defaultW, defaultH)
		r := &widget.Router{Root: hs.cfg.Root}
		b := hs.c.ui.list.Bounds()
		at := widget.Point{X: b.X + b.W/2, Y: b.Y + b.H - 4}
		if name != "" {
			i := slices.Index(names(hs.c.s.entries), name)
			if i < 0 {
				t.Fatalf("no %s in %v", name, names(hs.c.s.entries))
			}
			at.Y = b.Y + i*ListRowH + ListRowH/2
		}
		r.Move(at)
		r.Press(button, at, 0)
		r.Release(button, at)
	})
}

// within reports whether w sits below ancestor.
func within(w widget.Widget, ancestor *widget.List) bool {
	for p := w; p != nil; {
		if l, ok := p.(*typeAhead); ok && l.List == ancestor || p == widget.Widget(ancestor) {
			return true
		}
		parent, ok := p.(interface{ Parent() widget.Widget })
		if !ok {
			return false
		}
		p = parent.Parent()
	}
	return false
}

func (hs *harness) selectedNames() []string {
	var got []string
	hs.run(func() { got = names(hs.c.Selected()) })
	return got
}

// A right click on a row outside the selection selects it alone and
// opens the host's menu for it; one on a row of a selection keeps the
// selection; one below the rows clears it, and the menu hears none.
func TestTheContextMenuOpensForWhatTheRightClickSelects(t *testing.T) {
	hs := newHarness(t, "")
	host := &surfaceHost{}
	var heard [][]string
	o := openFiles
	o.ContextMenu = func(sel []Entry) []widget.MenuItem {
		heard = append(heard, names(sel))
		return []widget.MenuItem{{Label: "Open", OnClick: func() {}}}
	}
	hs.open(t, o)
	hs.run(func() { widget.SetSurfaceHost(hs.cfg.Root, host) })
	hs.rightClick(t, "b.txt")
	if got := hs.selectedNames(); !slices.Equal(got, []string{"b.txt"}) {
		t.Errorf("a right click on b.txt selected %v", got)
	}
	hs.do(func() {
		hs.c.activeView().SelectRows(slices.Index(names(hs.c.s.entries), "a.png"), slices.Index(names(hs.c.s.entries), "b.txt"))
	})
	hs.rightClick(t, "a.png")
	if got := hs.selectedNames(); !slices.Equal(got, []string{"a.png", "b.txt"}) {
		t.Errorf("a right click inside the selection left %v selected", got)
	}
	hs.rightClick(t, "")
	if got := hs.selectedNames(); len(got) != 0 {
		t.Errorf("a right click below the rows left %v selected", got)
	}
	want := [][]string{{"b.txt"}, {"a.png", "b.txt"}, {}}
	if !slices.EqualFunc(heard, want, slices.Equal) || len(host.menus) != 3 || host.menus[0][0].Label != "Open" {
		t.Errorf("the menu was built for %v and opened %d times, want built for %v and opened each time", heard, len(host.menus), want)
	}
}

// A browse without the host's menu opens none, and a right click
// selects nothing; a menu built empty opens none either.
func TestNoContextMenuOpensWithoutItems(t *testing.T) {
	hs := newHarness(t, "")
	host := &surfaceHost{}
	hs.open(t, openFiles)
	hs.run(func() { widget.SetSurfaceHost(hs.cfg.Root, host) })
	hs.rightClick(t, "b.txt")
	if got := hs.selectedNames(); len(host.menus) != 0 || len(got) != 0 {
		t.Errorf("without a menu a right click opened %d menus and selected %v", len(host.menus), got)
	}
	hs.hide()
	o := openFiles
	o.ContextMenu = func([]Entry) []widget.MenuItem { return nil }
	hs.open(t, o)
	hs.run(func() { widget.SetSurfaceHost(hs.cfg.Root, host) })
	hs.rightClick(t, "b.txt")
	if len(host.menus) != 0 {
		t.Errorf("a menu built empty opened %d times", len(host.menus))
	}
}

// drop is what a host heard of a drop.
type drop struct {
	paths []string
	into  string
	how   widget.Dropped
}

func (d drop) equal(o drop) bool {
	return slices.Equal(d.paths, o.paths) && d.into == o.into && d.how == o.how
}

// rowPoint is the middle of the row of the entry name in the list.
func (hs *harness) rowPoint(t *testing.T, name string) widget.Point {
	t.Helper()
	var at widget.Point
	hs.run(func() {
		lay(hs.cfg.Root, defaultW, defaultH)
		i := slices.Index(names(hs.c.s.entries), name)
		if i < 0 {
			t.Fatalf("no %s in %v", name, names(hs.c.s.entries))
		}
		b := hs.c.ui.list.Bounds()
		at = widget.Point{X: b.X + b.W/2, Y: b.Y + i*ListRowH + ListRowH/2}
	})
	return at
}

// Files dropped on a browse whose host takes drops go to the host,
// every local one, into the folder shown and with how the drop
// finished, and the browse stays where it is.
func TestADropGoesToTheHostWithHowItFinished(t *testing.T) {
	hs := newHarness(t, "")
	var drops []drop
	o := openFiles
	o.OnDrop = func(paths []string, into string, how widget.Dropped) { drops = append(drops, drop{paths, into, how}) }
	hs.open(t, o)
	payload := []byte(hs.uri("pics/a.png") + "\r\nhttps://example.com/x\r\n" + hs.uri("b.txt") + "\r\n")
	var r *widget.Router
	var body widget.Point
	hs.run(func() { r, body = treetest.Pointer(hs.cfg.Root, hs.c.ui.list, 1280, 720) })
	moved, copied := widget.Dropped{Action: transfer.ActionMove}, widget.Dropped{Action: transfer.ActionCopy, Own: true}
	hs.do(func() {
		r.DragEnter([]string{uriListMime}, body)
		r.Drop(uriListMime, payload, body, moved)
		r.DragEnter([]string{uriListMime}, body)
		r.Drop(uriListMime, payload, body, copied)
		r.DragEnter([]string{uriListMime}, body)
		r.Drop(uriListMime, []byte("https://example.com/x\n"), body, moved)
	})
	files := []string{filepath.Join(hs.root, "pics", "a.png"), filepath.Join(hs.root, "b.txt")}
	want := []drop{{files, hs.root, moved}, {files, hs.root, copied}}
	if !slices.EqualFunc(drops, want, drop.equal) {
		t.Errorf("the host heard %v, want %v", drops, want)
	}
	if dir := hs.dir(); dir != hs.root {
		t.Errorf("a drop the host took navigated to %s", dir)
	}
}

// A file drag over a folder row rings it and drops into that folder;
// over a file row, or off the rows, nothing is ringed and the drop
// goes into the folder shown. Leaving takes the ring down; a browse
// whose host takes no drops rings nothing.
func TestADropOnAFolderRowGoesIntoIt(t *testing.T) {
	hs := newHarness(t, "")
	var drops []drop
	o := openFiles
	o.OnDrop = func(paths []string, into string, how widget.Dropped) { drops = append(drops, drop{paths, into, how}) }
	hs.open(t, o)
	pics, file := hs.rowPoint(t, "pics"), hs.rowPoint(t, "b.txt")
	payload := []byte(hs.uri("a.png") + "\r\n")
	how := widget.Dropped{Action: transfer.ActionMove}
	var r *widget.Router
	var body widget.Point
	ringed := func() int {
		var row int
		hs.run(func() { row = hs.c.ui.list.DropRow() })
		return row
	}
	var picsRow int
	hs.run(func() {
		r, body = treetest.Pointer(hs.cfg.Root, hs.c.ui.list, 1280, 720)
		picsRow = slices.Index(names(hs.c.s.entries), "pics")
	})
	hs.run(func() { r.DragEnter([]string{uriListMime}, body) })
	if got := ringed(); got != -1 {
		t.Errorf("a drag off the rows rings row %d", got)
	}
	hs.run(func() { r.DragHover(pics) })
	if got := ringed(); got != picsRow {
		t.Errorf("a drag over pics rings row %d, want %d", got, picsRow)
	}
	hs.run(func() { r.DragHover(file) })
	if got := ringed(); got != -1 {
		t.Errorf("a drag over a file rings row %d", got)
	}
	hs.run(func() { r.DragHover(pics) })
	hs.do(func() { r.Drop(uriListMime, payload, pics, how) })
	if got := ringed(); got != -1 {
		t.Errorf("the drop left row %d ringed", got)
	}
	hs.run(func() { r.DragEnter([]string{uriListMime}, file) })
	hs.do(func() { r.Drop(uriListMime, payload, file, how) })
	hs.run(func() {
		r.DragEnter([]string{uriListMime}, pics)
		r.DragLeave()
	})
	if got := ringed(); got != -1 {
		t.Errorf("a drag that left keeps row %d ringed", got)
	}
	a := []string{filepath.Join(hs.root, "a.png")}
	want := []drop{{a, filepath.Join(hs.root, "pics"), how}, {a, hs.root, how}}
	if !slices.EqualFunc(drops, want, drop.equal) {
		t.Errorf("the host heard %v, want %v", drops, want)
	}

	hs.hide()
	hs.open(t, openFiles)
	pics = hs.rowPoint(t, "pics")
	hs.run(func() {
		r.DragEnter([]string{uriListMime}, pics)
		r.DragHover(pics)
	})
	if got := ringed(); got != -1 {
		t.Errorf("a browse whose host takes no drops rings row %d", got)
	}
}

// Rows dragged out offer what the host lets them go with; a browse
// that names nothing offers copy alone.
func TestDraggedRowsOfferTheHostsActions(t *testing.T) {
	hs := newHarness(t, "")
	o := openFiles
	o.DragActions = transfer.ActionCopy | transfer.ActionMove
	hs.open(t, o)
	var drag *widget.DragContent
	hs.do(func() { drag = hs.c.ui.list.DragContent([]int{slices.Index(names(hs.c.s.entries), "b.txt")}) })
	if drag == nil || drag.Actions != transfer.ActionCopy|transfer.ActionMove {
		t.Fatalf("the drag offers %+v, want copy and move", drag)
	}
}

// What is cut says so to the apps that paste it, what is copied too.
func TestFilesContentSaysCutOrCopy(t *testing.T) {
	for cut, op := range map[bool]string{true: "cut", false: "copy"} {
		c := FilesContent([]string{"/a b", "/c"}, cut)
		got, err := c.Bytes(GnomeCopiedFiles)
		if err != nil || string(got) != op+"\nfile:///a%20b\nfile:///c" {
			t.Errorf("cut %v: %q %v", cut, got, err)
		}
		if list, _ := c.Bytes(transfer.MimeURIList); string(list) != "file:///a%20b\r\nfile:///c\r\n" || c.Mimes[0] != transfer.MimeURIList {
			t.Errorf("cut %v: the uri list %q, mimes %v", cut, list, c.Mimes)
		}
	}
}

// Reveal opens a folder with the files named selected, those of them
// it lists; naming none it lists selects nothing.
func TestRevealSelectsTheFilesInTheirFolder(t *testing.T) {
	hs := newHarness(t, "")
	hs.open(t, in(openFiles, filepath.Join(hs.root, "docs")))
	hs.do(func() {
		hs.c.Reveal(hs.root, []string{filepath.Join(hs.root, "a.png"), filepath.Join(hs.root, "b.txt"), filepath.Join(hs.root, "gone")})
	})
	if got := hs.selectedNames(); hs.dir() != hs.root || !slices.Equal(got, []string{"a.png", "b.txt"}) {
		t.Errorf("Reveal showed %s with %v selected", hs.dir(), got)
	}
	made := filepath.Join(hs.root, "made.txt")
	if err := os.WriteFile(made, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	hs.do(func() { hs.c.Reveal(hs.root, []string{made}) })
	if got := hs.selectedNames(); !slices.Equal(got, []string{"made.txt"}) {
		t.Errorf("revealing a file just made in the folder shown selected %v", got)
	}
	hs.do(func() { hs.c.Reveal(filepath.Join(hs.root, "pics"), []string{filepath.Join(hs.root, "b.txt")}) })
	if got := hs.selectedNames(); len(got) != 0 {
		t.Errorf("revealing a file the folder does not hold selected %v", got)
	}
}

// OnDir hears the folder a browse starts in and each it moves to.
func TestTheHostHearsEachFolderShown(t *testing.T) {
	hs := newHarness(t, "")
	var dirs []string
	o := openFiles
	o.OnDir = func(dir string) { dirs = append(dirs, dir) }
	hs.open(t, o)
	hs.do(func() { hs.c.GoTo(filepath.Join(hs.root, "docs")) })
	hs.key(xkb.KeyBackSpace, 0, nil)
	if want := []string{hs.root, filepath.Join(hs.root, "docs"), hs.root}; !slices.Equal(dirs, want) {
		t.Errorf("the host heard %v, want %v", dirs, want)
	}
}

// Rename edits a name in its row: the entry holds the name with its
// stem selected, Enter hands a new name on, an unchanged or blank one
// and Escape end with nothing, and a path not listed is refused.
func TestRenameEditsTheNameInItsRow(t *testing.T) {
	hs := newHarness(t, "")
	hs.open(t, openFiles)
	target := filepath.Join(hs.root, "b.txt")
	var typed []string
	done := func(name string) { typed = append(typed, name) }
	var entry *widget.Entry
	start := func() {
		t.Helper()
		var ok bool
		hs.do(func() {
			lay(hs.cfg.Root, defaultW, defaultH)
			ok = hs.c.Rename(target, done)
			entry = hs.c.renaming.entry
		})
		if !ok || hs.windows[0].focus != entry || !within(entry, hs.c.ui.list) {
			t.Fatalf("Rename of b.txt: %v, focused %T, entry in the list %v", ok, hs.windows[0].focus, within(entry, hs.c.ui.list))
		}
	}
	start()
	hs.run(func() {
		if sel, _ := entry.SelectedText(); entry.Text() != "b.txt" || sel != "b" {
			t.Errorf("the entry holds %q with %q selected, want b.txt with b", entry.Text(), sel)
		}
	})
	hs.do(func() { entry.OnActivate("  c.txt ") })
	if !slices.Equal(typed, []string{"c.txt"}) || hs.c.Renaming() != "" || hs.windows[0].focus != hs.c.FileView() {
		t.Errorf("Enter handed on %v, left %q renaming, focused %T", typed, hs.c.Renaming(), hs.windows[0].focus)
	}
	for _, end := range []func(){func() { entry.OnActivate("b.txt") }, func() { entry.OnActivate(" ") }, func() { entry.OnEscape() }} {
		start()
		hs.do(end)
		if len(typed) != 1 || hs.c.Renaming() != "" {
			t.Errorf("an end with nothing handed on %v and left %q renaming", typed, hs.c.Renaming())
		}
	}
	start()
	hs.do(func() { hs.c.GoTo(filepath.Join(hs.root, "docs")) })
	if hs.c.Renaming() != "" {
		t.Error("another folder left the rename showing")
	}
	var ok bool
	hs.do(func() { ok = hs.c.Rename(target, done) })
	if ok || hs.c.Renaming() != "" {
		t.Error("a path the view does not list was taken for a rename")
	}
}

// The text fields take keys as text; the file views do not.
func TestTypingIsTheTextFields(t *testing.T) {
	hs := newHarness(t, "")
	hs.open(t, openFiles)
	hs.run(func() {
		r := &widget.Router{Root: hs.cfg.Root}
		if hs.c.Typing(r) {
			t.Error("nothing focused reads as typing")
		}
		if err := r.SetFocus(hs.c.ui.search); err != nil {
			t.Fatal(err)
		}
		if !hs.c.Typing(r) {
			t.Error("the search box does not read as typing")
		}
	})
}

// stem keeps a folder's and a dotfile's whole name and drops a file's
// extension.
func TestStem(t *testing.T) {
	for _, tc := range []struct {
		name string
		dir  bool
		want string
	}{{"a.tar.gz", false, "a.tar"}, {"notes", false, "notes"}, {".bashrc", false, ".bashrc"}, {"v1.2", true, "v1.2"}} {
		if got := stem(tc.name, tc.dir); got != tc.want {
			t.Errorf("stem(%q, %v) = %q, want %q", tc.name, tc.dir, got, tc.want)
		}
	}
}

// The host hears the selection each time it changes, and again when
// the rows change under it; a browse without the hook hears nothing
// and selects as before.
func TestTheHostHearsTheSelection(t *testing.T) {
	hs := newHarness(t, "")
	var heard [][]string
	o := openFiles
	o.OnSelection = func(sel []Entry) { heard = append(heard, names(sel)) }
	hs.open(t, o)
	if len(heard) == 0 || len(heard[len(heard)-1]) != 0 {
		t.Fatalf("the first listing told the host %v, want an empty selection last", heard)
	}
	heard = nil
	hs.clickRow(t, widget.BTNLeft, "b.txt")
	if !slices.EqualFunc(heard, [][]string{{"b.txt"}}, slices.Equal) {
		t.Errorf("a click on b.txt told the host %v", heard)
	}
	heard = nil
	browsertest.Touch(t, hs.root, "c.txt")
	waittest.Until(t, func() bool {
		var n int
		hs.run(func() { n = len(heard) })
		return n > 0
	}, "the host to hear the listing change")
	hs.run(func() {
		if got := heard[len(heard)-1]; !slices.Equal(got, []string{"b.txt"}) {
			t.Errorf("after c.txt appeared the host heard %v, want b.txt still selected", got)
		}
	})
	heard = nil
	hs.do(func() { hs.c.activate(slices.Index(names(hs.c.s.entries), "docs")) })
	if len(heard) == 0 || len(heard[len(heard)-1]) != 0 {
		t.Errorf("entering docs told the host %v, want an empty selection last", heard)
	}

	hs.hide()
	heard = nil
	hs.open(t, openFiles)
	hs.clickRow(t, widget.BTNLeft, "b.txt")
	if got := hs.selectedNames(); len(heard) != 0 || !slices.Equal(got, []string{"b.txt"}) {
		t.Errorf("without the hook the host heard %v and a click on b.txt selected %v", heard, got)
	}
}

// A middle click on a folder row hands it to the host's new tab and
// neither selects nor enters it; one on a file does nothing. Without
// the hook a middle click opens no tab and leaves the folder shown.
func TestMiddleClickOpensAFolderInANewTab(t *testing.T) {
	hs := newHarness(t, "")
	var opened []string
	o := openFiles
	o.OnOpenInNewTab = func(e Entry) { opened = append(opened, e.Name()) }
	hs.open(t, o)
	hs.clickRow(t, widget.BTNMiddle, "docs")
	if !slices.Equal(opened, []string{"docs"}) || hs.dir() != hs.root || len(hs.selectedNames()) != 0 {
		t.Errorf("a middle click on docs opened %v, showed %s and selected %v; want docs in a new tab, the root shown, nothing selected", opened, hs.dir(), hs.selectedNames())
	}
	opened = nil
	hs.clickRow(t, widget.BTNMiddle, "b.txt")
	if len(opened) != 0 || len(hs.selectedNames()) != 0 {
		t.Errorf("a middle click on b.txt opened %v and selected %v", opened, hs.selectedNames())
	}
	hs.do(hs.c.toggleView)
	hs.do(func() {
		lay(hs.cfg.Root, defaultW, defaultH)
		g := hs.c.ui.grid
		i := slices.Index(names(hs.c.s.entries), "docs")
		var at widget.Point
		for y := g.Bounds().Y; y < g.Bounds().Y+g.Bounds().H && at == (widget.Point{}); y += 4 {
			for x := g.Bounds().X; x < g.Bounds().X+g.Bounds().W; x += 4 {
				if g.IndexAt(widget.Point{X: x, Y: y}) == i {
					at = widget.Point{X: x + 8, Y: y + 8}
					break
				}
			}
		}
		r := &widget.Router{Root: hs.cfg.Root}
		r.Move(at)
		r.Press(widget.BTNMiddle, at, 0)
		r.Release(widget.BTNMiddle, at)
	})
	if !slices.Equal(opened, []string{"docs"}) {
		t.Errorf("a middle click on docs in the grid opened %v", opened)
	}
	hs.do(hs.c.toggleView)

	hs.hide()
	opened = nil
	hs.open(t, openFiles)
	hs.clickRow(t, widget.BTNMiddle, "docs")
	if len(opened) != 0 || hs.dir() != hs.root {
		t.Errorf("without the hook a middle click on docs opened %v and moved to %s", opened, hs.dir())
	}
}
