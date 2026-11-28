package filebrowser

import (
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/apptheme"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/desktop/vfs"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/lib/xkb"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/shell/filebrowser/browsertest"
)

type fakeWindow struct {
	closed bool
	focus  widget.Widget
	// refuse fails every SetFocus, as a window whose surface is gone.
	refuse error
}

func (w *fakeWindow) Close() { w.closed = true }

func (w *fakeWindow) SetFocus(f widget.Widget) error {
	if w.refuse != nil {
		return w.refuse
	}
	w.focus = f
	return nil
}

// lay lays root out on a w x h output, as its window does.
func lay(root widget.Widget, w, h int) {
	widget.SizeWithin(root, widget.Size{W: w, H: h})
	root.Arrange(render.Rect{W: w, H: h})
}

// drag presses at from, moves to to and releases there.
func drag(r *widget.Router, from, to widget.Point) {
	r.Move(from)
	r.Press(widget.BTNLeft, from, 0)
	r.Move(to)
	r.Release(widget.BTNLeft, to)
}

// center is the middle of w's bounds.
func center(w widget.Boundser) widget.Point {
	b := w.Bounds()
	return widget.Point{X: b.X + b.W/2, Y: b.Y + b.H/2}
}

// harness runs the browser in its standalone window with a lock-step
// loop: Invoke and the test's gestures take turns, as on the one UI
// goroutine.
type harness struct {
	t    *testing.T
	mu   sync.Mutex
	s    *Standalone
	c    *Browser
	root string
	// specs are the windows opened, in order, and cfg the last.
	specs   []WindowSpec
	windows []*fakeWindow
	openErr error
	// focusErr fails every opened window's SetFocus.
	focusErr error
	cfg      WindowSpec
	// theme is the stylesheet and palette the browser paints with.
	theme *apptheme.Theme
	// mounts are the filesystems the browser lists, the local disk and
	// what a test connects.
	mounts *vfs.Mounts
	// forgotten are the thumbnails the renderer was told to forget.
	forgotten []render.ImageID
	queue     chan func()
	qmu       sync.Mutex
	stopped   bool
	drained   chan struct{}
}

func newHarness(t *testing.T, viewFile string) *harness {
	t.Helper()
	conf := config.Defaults()
	conf.Animations.Enabled = false
	hs := &harness{t: t, root: browsertest.Tree(t), theme: apptheme.New(conf), mounts: vfs.NewMounts()}
	hs.queue = make(chan func(), 256)
	hs.drained = make(chan struct{})
	go func() {
		defer close(hs.drained)
		for fn := range hs.queue {
			hs.run(fn)
		}
	}()
	// The loop ends with the test: work already queued runs out before
	// the next test touches the widget package's loop state, and the
	// browser closes.
	t.Cleanup(hs.end)
	hs.s = NewStandalone(Deps{
		Invoke:     hs.invoke,
		Font:       treetest.Font(t),
		MIME:       browsertest.MIME(t),
		Home:       hs.root,
		Places:     func() []Place { return []Place{{Label: "Home", Path: hs.root, Icon: "user-home-symbolic"}} },
		Locations:  func() []Place { return []Place{{Label: "Computer", Path: "/", Icon: "drive-harddisk-symbolic"}} },
		ViewFile:   viewFile,
		Mounts:     hs.mounts,
		Animations: func() config.AnimationsConfig { return conf.Animations },
		ForgetImage: func(id render.ImageID) {
			hs.forgotten = append(hs.forgotten, id)
		},
	}, WindowDeps{
		Open: func(s WindowSpec) (Window, error) {
			hs.mu.Lock()
			defer hs.mu.Unlock()
			hs.specs = append(hs.specs, s)
			if hs.openErr != nil {
				return nil, hs.openErr
			}
			w := &fakeWindow{refuse: hs.focusErr}
			hs.windows = append(hs.windows, w)
			hs.cfg = s
			return w, nil
		},
		KeySym: treetest.KeySym,
		Sheet:  hs.theme.Sheet,
	})
	hs.c = hs.s.Browser()
	return hs
}

// endLoop ends the loop as Run returning does: what is queued runs
// out, and every later invoke is dropped.
func (hs *harness) endLoop() {
	hs.qmu.Lock()
	if !hs.stopped {
		hs.stopped = true
		close(hs.queue)
	}
	hs.qmu.Unlock()
	<-hs.drained
}

func (hs *harness) end() {
	hs.endLoop()
	hs.s.Close()
}

// flush waits until the loop ran everything queued so far.
func (hs *harness) flush() {
	done := make(chan struct{})
	hs.invoke(func() { close(done) })
	<-done
}

// invoke queues fn for the loop, in order, as Application.Invoke does;
// after the test, like Invoke after Run, it drops fn.
func (hs *harness) invoke(fn func()) {
	hs.qmu.Lock()
	defer hs.qmu.Unlock()
	if !hs.stopped {
		hs.queue <- fn
	}
}

// run runs fn as the loop: the browser's work and the test's gestures
// take turns, as on the one UI goroutine.
func (hs *harness) run(fn func()) {
	treetest.Loop.Lock()
	defer treetest.Loop.Unlock()
	fn()
}

// do runs a gesture as the loop and waits for what it set off to
// settle: a folder listing being read lands before the test goes on.
func (hs *harness) do(fn func()) {
	hs.run(fn)
	hs.settle()
}

// settle waits until no listing is being read and no folder counted,
// and the loop ran what that queued.
func (hs *harness) settle() {
	hs.t.Helper()
	waittest.Until(hs.t, func() bool {
		var busy bool
		hs.run(func() { busy = hs.c.Busy() || hs.c.counting > 0 })
		return !busy
	}, "the listing and the folder counts to arrive")
	hs.flush()
}

// open shows a browse as o says, in the tree's root unless o names a
// folder, and waits for its listing.
func (hs *harness) open(t *testing.T, o Options) {
	t.Helper()
	if o.Folder == "" {
		o.Folder = hs.root
	}
	var err error
	hs.run(func() { err = hs.s.Show(o) })
	if err != nil {
		t.Fatalf("show the browser: %v", err)
	}
	hs.settle()
}

// hide hides the browse showing.
func (hs *harness) hide() { hs.do(hs.s.Hide) }

// The browses the tests open, as the file chooser's modes do: picking
// files (one, or several) offers Recent and Trash, picking folders
// lists folders, and both pick folders and saving offer Create Folder.
var (
	openFile   = Options{Gathered: true}
	openFiles  = Options{Gathered: true, Selection: widget.SelectionMultiple}
	pickFolder = Options{Kinds: FoldersOnly, NewFolder: true}
	saving     = Options{NewFolder: true}
)

// in is o starting in folder.
func in(o Options, folder string) Options {
	o.Folder = folder
	return o
}

// click finds the named button as the loop, which owns the tree a
// listing may be rebuilding, and clicks it.
func (hs *harness) click(t *testing.T, name string) {
	t.Helper()
	var b *widget.Button
	hs.run(func() { b = treetest.Button(t, hs.cfg.Root, name) })
	hs.do(b.OnClick)
}

// shown is the active view's labels, in order.
func (hs *harness) shown() []string {
	var out []string
	hs.do(func() {
		for _, e := range hs.c.s.entries {
			out = append(out, hs.c.s.displayName(e))
		}
	})
	return out
}

// pick selects the named entry in the active view.
func (hs *harness) pick(t *testing.T, name string) {
	t.Helper()
	hs.do(func() {
		j := slices.Index(names(hs.c.s.entries), name)
		if j < 0 {
			t.Fatalf("no %s in %v", name, names(hs.c.s.entries))
		}
		hs.c.activeView().Select(j)
	})
}

func (hs *harness) uri(rel string) string { return "file://" + filepath.Join(hs.root, rel) }

// key presses a key on the window with focus on focused (none for nil).
func (hs *harness) key(sym xkb.Keysym, mods app.Mods, focused widget.Widget) {
	hs.do(func() {
		r := &widget.Router{Root: hs.cfg.Root}
		if focused != nil {
			r.SetFocus(focused)
		}
		hs.cfg.OnKey(r, treetest.Code(sym), mods)
	})
}

// dir is the folder shown.
func (hs *harness) dir() string {
	var d string
	hs.do(func() { d = hs.c.s.dir })
	return d
}

// hasButton reports a button named name (its label or tooltip) in
// the browser's tree, walked as the loop.
func (hs *harness) hasButton(name string) bool {
	found := false
	hs.run(func() {
		treetest.Walk(hs.cfg.Root, func(w widget.Widget) {
			if _, ok := w.(*widget.Button); ok && widget.Describe(w).Name == name {
				found = true
			}
		})
	})
	return found
}

// frame paints the whole surface at w x h as its window does and reads
// back its pixels.
func (hs *harness) frame(w, h int) func(x, y int) render.Color {
	px := make([]uint8, w*h*4)
	hs.do(func() {
		lay(hs.cfg.Root, w, h)
		hs.cfg.Root.Paint(render.New(px, w*4, w, h))
	})
	return func(x, y int) render.Color {
		i := (y*w + x) * 4
		return render.RGBA(px[i+2], px[i+1], px[i], px[i+3])
	}
}

// token is the palette color the stylesheet resolves var(--tok) to.
func (hs *harness) token(t *testing.T, tok config.CssToken) render.Color {
	t.Helper()
	c, ok := hs.theme.RenderPalette().Token(tok)
	if !ok {
		t.Fatalf("no palette token %s", tok)
	}
	return c
}

// classed is the one widget carrying class.
func classed(t *testing.T, root widget.Widget, class string) widget.Boundser {
	t.Helper()
	ws := treetest.WithClass(root, class)
	if len(ws) != 1 {
		t.Fatalf("%d widgets carry %s, want one", len(ws), class)
	}
	return ws[0].(widget.Boundser)
}

// defaultW and defaultH are the size the file chooser's panel opens
// at, the browser's room in the tests that measure it there.
const defaultW, defaultH = 760, 520
