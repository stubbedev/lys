package filebrowser

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/preview"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/desktop/thumbnail/thumbnailtest"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/lib/xkb"
	"github.com/stubbedev/lys/ui/widget"
)

// loadWatch counts the preview loads started, those that ran on the
// loop, and the context of each file's last load, and can hold every
// load until released.
type loadWatch struct {
	mu      sync.Mutex
	started int
	onLoop  int
	loop    map[int64]bool
	hold    chan struct{}
	ctxs    map[string]context.Context
}

func watchLoads(t *testing.T, hs *harness) *loadWatch {
	t.Helper()
	w := &loadWatch{loop: map[int64]bool{goroutineID(): true}, ctxs: map[string]context.Context{}}
	queue := make(chan int64, 1)
	hs.invoke(func() { queue <- goroutineID() })
	w.loop[<-queue] = true
	load := hs.c.previewLoader
	hs.c.previewLoader = func(ctx context.Context, req preview.Request) preview.Content {
		w.mu.Lock()
		w.started++
		w.ctxs[filepath.Base(req.Path)] = ctx
		if w.loop[goroutineID()] {
			w.onLoop++
		}
		hold := w.hold
		w.mu.Unlock()
		if hold != nil {
			select {
			case <-hold:
			case <-ctx.Done():
			}
		}
		return load(ctx, req)
	}
	return w
}

func (w *loadWatch) count() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.started
}

// waitPreview waits until a preview showing has loaded the entry named
// name.
func (hs *harness) waitPreview(t *testing.T, name string) {
	t.Helper()
	waittest.Until(t, func() bool {
		var done bool
		hs.do(func() {
			for _, v := range hs.c.openPreviews() {
				done = done || filepath.Base(v.Path()) == name && v.Renderer() != preview.Loading
			}
		})
		return done
	}, "the preview of "+name+" to land")
}

// quicklookOn opens folder, selects name and presses Space.
func (hs *harness) quicklookOn(t *testing.T, folder, name string) {
	t.Helper()
	hs.open(t, in(openFile, folder))
	hs.pick(t, name)
	hs.key(xkb.KeySpace, 0, nil)
	hs.waitPreview(t, name)
}

// detailOf is the label of root reading like a detail line: one with a
// separator in it.
func detailOf(root widget.Widget) string {
	for _, l := range treetest.Labels(root) {
		if strings.Contains(l, " · ") {
			return l
		}
	}
	return ""
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

// Space on source shows its first lines highlighted; on a binary file
// it shows the card alone, with the file's size.
func TestQuicklookShowsTextHighlighted(t *testing.T) {
	hs := newHarness(t, "")
	dir := filepath.Join(hs.root, "docs")
	writeFile(t, filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"))
	writeFile(t, filepath.Join(dir, "blob.txt"), []byte("ELF\x00\x01\x02binary"))
	hs.quicklookOn(t, dir, "main.go")
	var code []*widgets.CodeView
	var markup string
	hs.run(func() {
		lay(hs.cfg.Root, 1400, 1000)
		code = treetest.All[*widgets.CodeView](hs.c.ui.ql.Root())
		if lines := treetest.All[*widget.RichLabel](hs.c.ui.ql.Root()); len(lines) > 2 {
			markup = lines[2].Markup()
		}
	})
	if len(code) != 1 || !strings.HasPrefix(code[0].Text(), "package main\n") || code[0].Name() != "main.go" {
		t.Fatalf("Quick Look on main.go shows %d code views", len(code))
	}
	if !strings.Contains(markup, "<span") || !strings.Contains(markup, "func") {
		t.Errorf("the third line is not highlighted: %q", markup)
	}
	hs.key(xkb.KeyEscape, 0, nil)
	hs.pick(t, "blob.txt")
	hs.key(xkb.KeySpace, 0, nil)
	hs.waitPreview(t, "blob.txt")
	var info string
	var shown preview.Renderer
	hs.run(func() {
		code = treetest.All[*widgets.CodeView](hs.c.ui.ql.Root())
		info, shown = detailOf(hs.c.ui.ql.Root()), hs.c.ui.ql.Renderer()
	})
	if len(code) != 0 || shown != preview.Icon || !strings.Contains(info, "B ·") {
		t.Errorf("a binary file shows as %v with %d code views, detail %q", shown, len(code), info)
	}
	hs.hide()
}

// A wide picture previews whole at its aspect ratio with its pixel size
// in the detail line, never cropped square, and closing the dialog
// lets its pixels go.
func TestQuicklookShowsTheWholeImage(t *testing.T) {
	hs := newHarness(t, "")
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	dir := filepath.Join(hs.root, "pics")
	f, err := os.Create(filepath.Join(dir, "pano.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, image.NewGray(image.Rect(0, 0, 4000, 1000))); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	hs.quicklookOn(t, dir, "pano.png")
	var views []*widget.ImageView
	var info string
	hs.run(func() {
		lay(hs.cfg.Root, 1400, 1000)
		views = treetest.All[*widget.ImageView](hs.c.ui.ql.Root())
		info = detailOf(hs.c.ui.ql.Root())
	})
	if len(views) != 1 {
		t.Fatalf("Quick Look on an image shows %d image views", len(views))
	}
	if b := views[0].Bounds(); b.W != 4*b.H || b.W == 0 {
		t.Errorf("the 4000x1000 image shows at %dx%d, want 4:1", b.W, b.H)
	}
	if !strings.Contains(info, "4000 × 1000") {
		t.Errorf("the detail line %q has no pixel size", info)
	}
	hs.hide()
	waittest.Until(t, func() bool {
		var held int64
		var path string
		hs.do(func() { held, path = hs.c.pictures.Held(), hs.c.ui.ql.Path() })
		if path != "" {
			t.Fatal("the closed dialog still shows its preview")
		}
		return held == 0
	}, "every picture to be freed after the dialog closed")
}

// A 2 GB text file previews from its first 64 KiB, read off the loop.
func TestQuicklookReadsAtMost64KiB(t *testing.T) {
	hs := newHarness(t, "")
	path := filepath.Join(hs.root, "docs", "huge.txt")
	writeFile(t, path, []byte(strings.Repeat("line of text\n", preview.TextBytes/13+1)))
	if err := os.Truncate(path, 2<<30); err != nil {
		t.Fatal(err)
	}
	w := watchLoads(t, hs)
	hs.quicklookOn(t, filepath.Dir(path), "huge.txt")
	var text int
	hs.run(func() { text = len(treetest.All[*widgets.CodeView](hs.c.ui.ql.Root())[0].Text()) })
	w.mu.Lock()
	started, onLoop := w.started, w.onLoop
	w.mu.Unlock()
	if started != 1 || text > preview.TextBytes || text == 0 || onLoop != 0 {
		t.Errorf("previewing 2 GB showed %d bytes in %d loads, %d on the loop; want one load of at most %d off it", text, started, onLoop, preview.TextBytes)
	}
	hs.hide()
}

// Moving the selection 50 times before the loop rests starts one load;
// a load superseded by a later move is cancelled.
func TestPreviewLoadsOncePerRest(t *testing.T) {
	hs := newHarness(t, "")
	dir := filepath.Join(hs.root, "docs")
	for i := range 10 {
		writeFile(t, filepath.Join(dir, "note"+strconv.Itoa(i)+".txt"), []byte("note "+strconv.Itoa(i)))
	}
	w := watchLoads(t, hs)
	hs.open(t, in(openFile, dir))
	hs.pick(t, "note0.txt")
	hs.do(func() { hs.c.ui.previewToggle.SetActive(true) })
	hs.waitPreview(t, "note0.txt")
	before := w.count()
	hs.do(func() {
		for i := range 50 {
			hs.c.activeView().Select(i % 10)
		}
	})
	hs.waitPreview(t, "note9.txt")
	if loads := w.count() - before; loads != 1 {
		t.Errorf("50 moves in one rest started %d loads, want 1", loads)
	}
	var text string
	hs.run(func() { text = treetest.All[*widgets.CodeView](hs.c.ui.pv.Root())[0].Text() })
	if text != "note 9" {
		t.Errorf("the pane shows %q, want the last selection's text", text)
	}

	w.mu.Lock()
	w.hold = make(chan struct{})
	w.mu.Unlock()
	hs.do(func() { hs.c.activeView().Select(3) })
	held := func() context.Context {
		w.mu.Lock()
		defer w.mu.Unlock()
		return w.ctxs["note3.txt"]
	}
	waittest.Until(t, func() bool { return held() != nil }, "the held load to start")
	hs.do(func() { hs.c.activeView().Select(4) })
	superseded := held()
	w.mu.Lock()
	close(w.hold)
	w.hold = nil
	w.mu.Unlock()
	if superseded.Err() == nil {
		t.Error("the superseded load was not cancelled")
	}
	hs.waitPreview(t, "note4.txt")
	hs.hide()
}

// The arrows step Quick Look through the files; Space closes it, and
// with no preview open they are left to the views.
func TestQuicklookStepsThroughTheFiles(t *testing.T) {
	hs := newHarness(t, "")
	dir := filepath.Join(hs.root, "docs")
	for i := range 3 {
		writeFile(t, filepath.Join(dir, "note"+strconv.Itoa(i)+".txt"), []byte("note "+strconv.Itoa(i)))
	}
	hs.quicklookOn(t, dir, "note0.txt")
	hs.key(xkb.KeyRight, 0, nil)
	hs.waitPreview(t, "note1.txt")
	hs.key(xkb.KeyLeft, 0, nil)
	hs.waitPreview(t, "note0.txt")
	hs.key(xkb.KeySpace, 0, nil)
	var open bool
	var path string
	hs.do(func() { open, path = hs.c.ui.quicklook.Visible(), hs.c.ui.ql.Path() })
	if open || path != "" {
		t.Errorf("Space left Quick Look open %v, showing %q", open, path)
	}
	hs.hide()
}

// A folder previews its item count.
func TestFolderPreviewCountsItems(t *testing.T) {
	hs := newHarness(t, "")
	hs.quicklookOn(t, hs.root, "docs")
	var info string
	hs.run(func() { info = detailOf(hs.c.ui.ql.Root()) })
	if !strings.HasPrefix(info, i18n.T("preview-kind-folder")+" · "+i18n.T("preview-folder-items", i18n.Int("count", 0), i18n.Str("shown", "0"))) {
		t.Errorf("an empty folder's detail line is %q", info)
	}
	hs.hide()
}

// A PDF previews its large thumbnail, made by lys-docthumb.
func TestQuicklookShowsAThumbnailerPage(t *testing.T) {
	hs := newHarness(t, "")
	doc := pdfFile(t, hs)
	installThumbnailer(t, hs, thumbnailtest.Page)
	hs.quicklookOn(t, filepath.Dir(doc), "paper.pdf")
	var views []*widget.ImageView
	hs.run(func() {
		lay(hs.cfg.Root, 1400, 1000)
		views = treetest.All[*widget.ImageView](hs.c.ui.ql.Root())
	})
	if len(views) != 1 {
		t.Fatalf("Quick Look on a PDF shows %d image views", len(views))
	}
	if b := views[0].Bounds(); 3*b.W != 2*b.H {
		t.Errorf("the 200x300 page shows at %dx%d, want 2:3", b.W, b.H)
	}
	hs.hide()
}

// press routes a plain key as the window does: to the focused file
// view first, then to the browser's key hook.
func (hs *harness) press(sym xkb.Keysym) {
	hs.do(func() {
		r := &widget.Router{Root: hs.cfg.Root}
		if err := r.SetFocus(hs.c.FileView()); err != nil {
			hs.t.Fatal(err)
		}
		if a, ok := widget.KeyActionForSym(sym); ok {
			r.KeyAction(a, 0)
		}
		hs.cfg.OnKey(r, treetest.Code(sym), 0)
	})
}

// quicklookIndex is the entry Quick Look shows once its load lands.
func (hs *harness) quicklookIndex(t *testing.T) int {
	t.Helper()
	var i int
	waittest.Until(t, func() bool {
		var ready bool
		hs.do(func() {
			ready = hs.c.ui.ql.Renderer() != preview.Loading
			i = slices.Index(names(hs.c.s.entries), filepath.Base(hs.c.ui.ql.Path()))
		})
		return ready
	}, "Quick Look to show its file")
	return i
}

// In grid view Quick Look's Up and Down step a whole row of the grid
// and Left and Right one item, as the grid's own arrows do; in list
// view every arrow steps one item. Each press moves once, whether the
// view has the focus and navigates itself or not.
func TestQuicklookArrowsFollowTheView(t *testing.T) {
	hs := newHarness(t, "")
	dir := filepath.Join(hs.root, "docs")
	for i := range 30 {
		writeFile(t, filepath.Join(dir, fmt.Sprintf("note%02d.txt", i)), []byte("note"))
	}
	hs.open(t, in(openFile, dir))
	hs.pick(t, "note05.txt")
	hs.key(xkb.KeySpace, 0, nil)
	steps := []struct {
		sym  xkb.Keysym
		name string
	}{{xkb.KeyDown, "Down"}, {xkb.KeyRight, "Right"}, {xkb.KeyUp, "Up"}, {xkb.KeyLeft, "Left"}}
	walk := func(line, item int) {
		t.Helper()
		at := hs.quicklookIndex(t)
		for _, s := range steps {
			want := at
			switch s.sym {
			case xkb.KeyDown:
				want += line
			case xkb.KeyUp:
				want -= line
			case xkb.KeyRight:
				want += item
			case xkb.KeyLeft:
				want -= item
			}
			hs.press(s.sym)
			if got := hs.quicklookIndex(t); got != want {
				t.Errorf("%s from %d shows %d, want %d", s.name, at, got, want)
			}
			at = want
		}
	}
	walk(1, 1)
	hs.key(xkb.KeySpace, 0, nil)
	hs.click(t, "Toggle grid view")
	var cols int
	hs.run(func() {
		lay(hs.cfg.Root, 1400, 1000)
		cols = hs.c.ui.grid.Columns()
	})
	if cols < 2 {
		t.Fatalf("the grid lays out %d columns, want a row of several", cols)
	}
	hs.pick(t, "note00.txt")
	hs.key(xkb.KeySpace, 0, nil)
	walk(cols, 1)
	hs.key(xkb.KeyDown, 0, nil)
	if got := hs.quicklookIndex(t); got != cols {
		t.Errorf("Down with the grid unfocused shows %d, want %d", got, cols)
	}
	hs.hide()
}
