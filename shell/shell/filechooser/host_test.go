package filechooser

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/fileuri"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/apptheme"
	"github.com/stubbedev/lys/desktop/shell/reveal"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/portaldialogs"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/lib/xkb"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/shell/shell/dialogsurface"
	"github.com/stubbedev/lys/shell/shell/filebrowser"
	"github.com/stubbedev/lys/shell/shell/filebrowser/browsertest"
)

type fakeWindow struct {
	closed bool
	focus  widget.Widget
	// refuse fails every SetFocus, as an overlay whose surface is gone.
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

// panel is the request's floating panel, which the overlay's root
// reveals.
func (hs *harness) panel(t *testing.T) *widget.FloatingPanel {
	t.Helper()
	if _, ok := hs.cfg.Root.(*widget.Revealer); !ok {
		t.Fatalf("root %T, want the revealer around the panel", hs.cfg.Root)
	}
	return treetest.First[*widget.FloatingPanel](t, hs.cfg.Root)
}

// center is the middle of w's bounds.
func center(w widget.Boundser) widget.Point {
	b := w.Bounds()
	return widget.Point{X: b.X + b.W/2, Y: b.Y + b.H/2}
}

// harness runs the chooser with a lock-step loop: Invoke and the
// test's gestures take turns, as on the one UI goroutine.
type harness struct {
	t      *testing.T
	mu     sync.Mutex
	c      *Chooser
	root   string
	opened chan app.LayerConfig
	// specs are the dialogs opened, in order.
	specs   []dialogsurface.Spec
	windows []*fakeWindow
	openErr error
	// surface is what the dialogs open on.
	surface dialogsurface.Surface
	// focusErr fails every opened overlay's SetFocus.
	focusErr error
	cfg      app.LayerConfig
	// conf is the live config; the chooser animates as it says.
	conf *config.Config
	// theme is the stylesheet and palette the chooser paints with.
	theme *apptheme.Theme
	// forgotten are the thumbnails the renderer was told to forget.
	forgotten []render.ImageID
	queue     chan func()
	qmu       sync.Mutex
	stopped   bool
	drained   chan struct{}
	asks      sync.WaitGroup
}

func newHarness(t *testing.T, viewFile string, browse ...func(*filebrowser.Deps)) *harness {
	t.Helper()
	face := treetest.Font(t)
	conf := config.Defaults()
	conf.Animations.Enabled = false
	hs := &harness{t: t, root: browsertest.Tree(t), opened: make(chan app.LayerConfig, 4), conf: conf, theme: apptheme.New(conf)}
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
	// chooser closes, answering what it was asked.
	t.Cleanup(hs.end)
	bd := filebrowser.Deps{
		Invoke: hs.invoke,
		Font:   face,
		MIME:   browsertest.MIME(t),
		Home:   hs.root,
		Places: func() []filebrowser.Place {
			return []filebrowser.Place{{Label: "Home", Path: hs.root, Icon: "user-home-symbolic"}}
		},
		Locations: func() []filebrowser.Place {
			return []filebrowser.Place{{Label: "Computer", Path: "/", Icon: "drive-harddisk-symbolic"}}
		},
		ViewFile: viewFile,
		ForgetImage: func(id render.ImageID) {
			hs.forgotten = append(hs.forgotten, id)
		},
	}
	for _, b := range browse {
		b(&bd)
	}
	hs.c = New(Deps{
		Config: func() *config.Config { return hs.conf },
		Open: func(s dialogsurface.Spec) (Window, error) {
			hs.mu.Lock()
			defer hs.mu.Unlock()
			hs.specs = append(hs.specs, s)
			if hs.openErr != nil {
				return nil, hs.openErr
			}
			w := &fakeWindow{refuse: hs.focusErr}
			hs.windows = append(hs.windows, w)
			cfg := s.Layer
			cfg.Root = s.Root(hs.surface)
			hs.opened <- cfg
			return w, nil
		},
		KeySym:  treetest.KeySym,
		Sheet:   hs.theme.Sheet,
		Browser: bd,
	})
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
	hs.c.Close()
	asked := make(chan struct{})
	go func() {
		hs.asks.Wait()
		close(asked)
	}()
	waittest.For(hs.t, asked, "every request to end with the closed chooser")
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

// run runs fn as the loop: the dialog's work and the test's gestures
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

// settle waits until no listing is being read and the loop ran what
// that queued.
func (hs *harness) settle() {
	hs.t.Helper()
	waittest.Until(hs.t, func() bool {
		var loading bool
		hs.run(func() { loading = hs.c.b.Busy() })
		return !loading
	}, "the listing to arrive")
	hs.flush()
}

// ask starts a request and waits for its surface.
func (hs *harness) ask(t *testing.T, fn func() []string) chan []string {
	t.Helper()
	got := make(chan []string, 1)
	hs.asks.Go(func() { got <- fn() })
	hs.cfg = waittest.For(t, hs.opened, "the chooser to open")
	hs.settle()
	return got
}

func (hs *harness) open(t *testing.T, r OpenRequest) chan []string {
	t.Helper()
	if r.CurrentFolder == "" {
		r.CurrentFolder = hs.root
	}
	return hs.ask(t, func() []string { return hs.c.Open(testCaller, r).URIs })
}

func answer[T any](t *testing.T, got chan T) T {
	t.Helper()
	return waittest.For(t, got, "the request's answer")
}

func pending(t *testing.T, got chan []string) {
	t.Helper()
	select {
	case uris := <-got:
		t.Fatalf("answered %v while it should still be open", uris)
	case <-time.After(quietWindow):
	}
}

// click finds the named button as the loop, which owns the tree a
// listing may be rebuilding, and clicks it.
func (hs *harness) click(t *testing.T, name string) {
	t.Helper()
	var b *widget.Button
	hs.run(func() { b = treetest.Button(t, hs.cfg.Root, name) })
	hs.do(b.OnClick)
}

// shown is the names of the active view's rows, in order.
func (hs *harness) shown() []string {
	var out []string
	hs.do(func() { out = names(hs.c.b.Entries()) })
	return out
}

func names(entries []filebrowser.Entry) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Name()
	}
	return out
}

// fileView is the browser's file view showing, which takes typing.
type fileView interface {
	widget.Widget
	Select(int)
	SelectAll()
	Selection() []int
	SelectionMode() widget.SelectionMode
	KeyAction(widget.KeyAction, widget.Mods)
}

// view is the file view showing.
func (hs *harness) view() fileView { return hs.c.b.FileView().(fileView) }

// index is the row of the entry named name, failing without one.
func (hs *harness) index(t *testing.T, name string) int {
	t.Helper()
	j := slices.Index(names(hs.c.b.Entries()), name)
	if j < 0 {
		t.Fatalf("no %s in %v", name, names(hs.c.b.Entries()))
	}
	return j
}

// activate activates the named entry as Enter on it does.
func (hs *harness) activate(t *testing.T, name string) {
	t.Helper()
	hs.do(func() {
		v := hs.view()
		v.Select(hs.index(t, name))
		v.KeyAction(widget.KeyEnter, 0)
	})
}

// notice is the notice's text and whether it shows.
func (hs *harness) notice(t *testing.T) (string, bool) {
	t.Helper()
	var text string
	var shown bool
	hs.run(func() {
		text = classed(t, hs.cfg.Root, "file-chooser-notice-label").(*widget.Label).Text()
		shown = widget.IsVisible(classed(t, hs.cfg.Root, "file-chooser-notice").(widget.Widget))
	})
	return text, shown
}

// quicklook reports whether Quick Look shows.
func (hs *harness) quicklook(t *testing.T) bool {
	t.Helper()
	var shown bool
	hs.run(func() { shown = widget.IsVisible(classed(t, hs.cfg.Root, "file-chooser-quicklook").(widget.Widget)) })
	return shown
}

// pick selects the named entry in the active view.
func (hs *harness) pick(t *testing.T, name string) {
	t.Helper()
	hs.do(func() { hs.view().Select(hs.index(t, name)) })
}

func (hs *harness) uri(rel string) string { return "file://" + filepath.Join(hs.root, rel) }

func TestOpenAnswersTheSelectedFile(t *testing.T) {
	hs := newHarness(t, "")
	got := hs.open(t, OpenRequest{Title: "Pick one"})
	if hs.cfg.Layer != app.LayerOverlay || hs.cfg.Namespace != "lys-shell-file-chooser" || hs.cfg.Keyboard != app.KeyboardExclusive {
		t.Errorf("surface = %+v", hs.cfg)
	}
	var l []string
	hs.run(func() { l = treetest.Labels(hs.cfg.Root) })
	if !slices.Contains(l, "Pick one") {
		t.Errorf("no title in %q", l)
	}
	if got := hs.shown(); !slices.Equal(got, []string{"docs", "pics", "a.png", "b.txt"}) {
		t.Errorf("listing = %v", got)
	}
	// Nothing selected: Open keeps the dialog.
	hs.click(t, "Open")
	pending(t, got)
	hs.pick(t, "b.txt")
	hs.click(t, "Open")
	if uris := answer(t, got); !slices.Equal(uris, []string{hs.uri("b.txt")}) {
		t.Errorf("answered %v", uris)
	}
	hs.mu.Lock()
	defer hs.mu.Unlock()
	if !hs.windows[0].closed {
		t.Error("an answered chooser stayed mapped")
	}
}

func TestOpenMultipleAnswersEveryFile(t *testing.T) {
	hs := newHarness(t, "")
	got := hs.open(t, OpenRequest{Multiple: true})
	hs.do(func() {
		v := hs.view()
		if v.SelectionMode() != widget.SelectionMultiple {
			t.Errorf("selection mode = %v, want multiple", v.SelectionMode())
		}
		v.SelectAll()
	})
	hs.click(t, "Open")
	if uris := answer(t, got); !slices.Equal(uris, []string{hs.uri("a.png"), hs.uri("b.txt")}) {
		t.Errorf("answered %v, want the two files and no folders", uris)
	}
}

// Activating a file answers it; activating a folder enters it, the
// dialog left open.
func TestActivatingAFileOpensItAndAFolderEntersIt(t *testing.T) {
	hs := newHarness(t, "")
	got := hs.open(t, OpenRequest{})
	hs.activate(t, "pics")
	if got := hs.shown(); !slices.Equal(got, []string{"a.png"}) {
		t.Fatalf("after entering pics = %v", got)
	}
	pending(t, got)
	hs.activate(t, "a.png")
	if uris := answer(t, got); !slices.Equal(uris, []string{hs.uri("pics/a.png")}) {
		t.Errorf("activating a file answered %v", uris)
	}
}

func TestCancelEscapeAndQueueing(t *testing.T) {
	hs := newHarness(t, "")
	got := hs.open(t, OpenRequest{Filters: []Filter{{"Images", []Rule{{RuleGlob, "*.png"}}}}})
	// Escape closes quick look first, then the dialog.
	hs.pick(t, "a.png")
	hs.do(func() { hs.cfg.OnKey(&widget.Router{}, treetest.Code(xkb.KeySpace), 0) })
	if !hs.quicklook(t) {
		t.Fatal("space did not open quick look")
	}
	hs.do(func() { hs.cfg.OnKey(&widget.Router{}, treetest.Code(xkb.KeyEscape), 0) })
	pending(t, got)
	hs.do(func() { hs.cfg.OnKey(&widget.Router{}, treetest.Code(xkb.KeyEscape), 0) })
	if uris := answer(t, got); uris != nil {
		t.Errorf("Escape answered %v", uris)
	}

	// A second request waits for the first rather than cancelling it.
	first := hs.open(t, OpenRequest{})
	second := make(chan []string, 1)
	go func() {
		second <- hs.c.Save(portaldialogs.Caller{Handle: "/r/second", Modal: true}, SaveRequest{CurrentFolder: hs.root, CurrentName: "x"}).URIs
	}()
	pending(t, first)
	hs.click(t, "Cancel")
	if uris := answer(t, first); uris != nil {
		t.Errorf("Cancel answered %v", uris)
	}
	hs.cfg = waittest.For(t, hs.opened, "the waiting request to show")
	hs.click(t, "Cancel")
	if uris := answer(t, second); uris != nil {
		t.Errorf("Cancel answered %v", uris)
	}
}

func TestSaveAnswersTheNameInTheFolder(t *testing.T) {
	hs := newHarness(t, "")
	got := hs.ask(t, func() []string {
		return hs.c.Save(testCaller, SaveRequest{Title: "Save", CurrentName: "draft.txt", CurrentFolder: hs.root}).URIs
	})
	var focused bool
	var text string
	hs.do(func() { focused, text = hs.windows[0].focus == hs.c.name, hs.c.name.Text() })
	if !focused || text != "draft.txt" {
		t.Errorf("name entry focused %v, text %q", focused, text)
	}
	// Activating a file takes its name.
	hs.activate(t, "b.txt")
	if hs.c.name.Text() != "b.txt" {
		t.Errorf("name = %q after activating b.txt", hs.c.name.Text())
	}
	hs.do(func() { hs.c.name.SetText("") })
	hs.click(t, "Save")
	pending(t, got)
	hs.do(func() {
		hs.c.name.SetText("new.txt")
		hs.c.name.OnActivate("new.txt")
	})
	if uris := answer(t, got); !slices.Equal(uris, []string{hs.uri("new.txt")}) {
		t.Errorf("save answered %v", uris)
	}
}

func TestFolderPickAnswersTheFolder(t *testing.T) {
	hs := newHarness(t, "")
	got := hs.open(t, OpenRequest{Directory: true})
	if got := hs.shown(); !slices.Equal(got, []string{"docs", "pics"}) {
		t.Errorf("folder pick lists %v", got)
	}
	hs.click(t, "Select")
	if uris := answer(t, got); !slices.Equal(uris, []string{"file://" + hs.root}) {
		t.Errorf("answered %v", uris)
	}
}

// The dialog lists what its filter passes, the folders always, and the
// filter dropdown offers All Files after the app's filters.
func TestFiltersHiddenAndSort(t *testing.T) {
	hs := newHarness(t, "")
	filtered := hs.open(t, OpenRequest{Filters: []Filter{{"Images", []Rule{{RuleGlob, "*.png"}}}}})
	if got := hs.shown(); !slices.Equal(got, []string{"docs", "pics", "a.png"}) {
		t.Errorf("filtered = %v", got)
	}
	hs.do(func() { hs.c.filter.SetSelected(1) })
	if got := hs.shown(); !slices.Equal(got, []string{"docs", "pics", "a.png", "b.txt"}) {
		t.Errorf("all files = %v", got)
	}
	if got := hs.c.filter.Selection(); got != "All Files" {
		t.Errorf("filter shows %q", got)
	}
	hs.do(func() { hs.c.selectFilter(7) })
	if hs.c.active != 1 {
		t.Errorf("an out-of-range filter changed the active one to %d", hs.c.active)
	}
	hs.click(t, "Show hidden files")
	if got := hs.shown(); !slices.Contains(got, ".dot.txt") {
		t.Errorf("hidden shown = %v", got)
	}
	hs.click(t, "Cancel")
	answer(t, filtered)
}

func TestOpenFailureAnswersNothing(t *testing.T) {
	hs := newHarness(t, "")
	hs.openErr = errors.New("no layer shell")
	if uris := hs.c.Open(testCaller, OpenRequest{CurrentFolder: hs.root}).URIs; uris != nil {
		t.Errorf("an unmapped chooser answered %v", uris)
	}
}

// An overlay whose first control cannot take focus could not be typed
// into: the request answers nothing and the overlay closes.
func TestUnfocusableOverlayAnswersNothing(t *testing.T) {
	hs := newHarness(t, "")
	hs.focusErr = app.ErrClosed
	if uris := hs.c.Open(testCaller, OpenRequest{CurrentFolder: hs.root}).URIs; uris != nil {
		t.Errorf("an unfocusable chooser answered %v", uris)
	}
	var closed bool
	hs.do(func() { closed = len(hs.windows) == 1 && hs.windows[0].closed })
	if !closed {
		t.Error("the unfocusable overlay stayed mapped")
	}
}

// The overlay spans the target output (the focused one) and takes input
// only over the panel, which centres there at its size, its content
// filling it. The header drags it while the header's buttons keep
// their presses, and its place sticks across requests on that output;
// another output centres it again.
func TestPanelFloatsOnTheOutput(t *testing.T) {
	hs := newHarness(t, "")
	dp1, dp2 := &app.Output{Name: "DP-1"}, &app.Output{Name: "DP-2"}
	target := dp1
	hs.c.d.Target = func() *app.Output { return target }
	got := hs.open(t, OpenRequest{})
	c := hs.cfg
	if c.Output != dp1 {
		t.Errorf("mapped on %v, want the target output", c.Output)
	}
	if all := app.AnchorTop | app.AnchorBottom | app.AnchorLeft | app.AnchorRight; c.Anchor != all || c.Margin != (app.Margins{}) || c.Width != 0 || c.Height != 0 {
		t.Errorf("layer anchor %v margin %+v size %dx%d, want it spanning the output", c.Anchor, c.Margin, c.Width, c.Height)
	}
	panel := hs.panel(t)
	centred := render.Rect{X: (1920 - panelSize.W) / 2, Y: (1080 - panelSize.H) / 2, W: panelSize.W, H: panelSize.H}
	var r *widget.Router
	var grab, up widget.Point
	hs.run(func() {
		r, grab = treetest.Pointer(hs.cfg.Root, treetest.WithClass(panel, "file-chooser-title")[0].(widget.Boundser), 1920, 1080)
		up = center(treetest.Button(t, panel, "Up"))
		if p := panel.PanelRect(); p != centred {
			t.Errorf("panel at %v, want centred at %v", p, centred)
		}
		if reg, ok := widget.InputRegionOf(hs.cfg.Root); !ok || len(reg) != 1 || reg[0] != centred {
			t.Errorf("the overlay's input region %v (found %v), want the panel alone through its revealer", reg, ok)
		}
		if hit := panel.HitTest(widget.Point{X: 5, Y: 5}); hit != nil {
			t.Errorf("a press beside the panel hit %T, want it passed through", hit)
		}
		header := treetest.WithClass(panel, "file-chooser-header")[0].(widget.Boundser).Bounds()
		footer := treetest.WithClass(panel, "file-chooser-footer")[0].(widget.Boundser).Bounds()
		content := within(centred)
		if header.Y != content.Y || footer.Y+footer.H != content.Y+content.H || header.W != content.W || footer.W != content.W {
			t.Errorf("header %v and footer %v, want them spanning the panel %v inside its border", header, footer, centred)
		}
	})
	dir := hs.dir()
	hs.do(func() {
		drag(r, up, widget.Point{X: up.X + 3, Y: up.Y})
		lay(panel, 1920, 1080)
	})
	if p := panel.PanelRect(); p != centred || hs.dir() == dir {
		t.Errorf("a press on Up left the panel at %v in %s, want it clicked and the panel unmoved", p, hs.dir())
	}
	moved := centred
	moved.X, moved.Y = moved.X+40, moved.Y-30
	hs.do(func() {
		drag(r, grab, widget.Point{X: grab.X + 40, Y: grab.Y - 30})
		lay(panel, 1920, 1080)
	})
	if p := panel.PanelRect(); p != moved {
		t.Errorf("dragged the header to %v, want %v", p, moved)
	}
	hs.do(func() { hs.c.finish(nil) })
	answer(t, got)
	got = hs.open(t, OpenRequest{})
	hs.run(func() { lay(hs.cfg.Root, 1920, 1080) })
	if p := hs.panel(t).PanelRect(); p != moved {
		t.Errorf("the next request opened at %v, want the dragged %v", p, moved)
	}
	hs.do(func() { hs.c.finish(nil) })
	answer(t, got)
	target = dp2
	hs.open(t, OpenRequest{})
	hs.run(func() { lay(hs.cfg.Root, 2560, 1440) })
	if want := (render.Rect{X: (2560 - panelSize.W) / 2, Y: (1440 - panelSize.H) / 2, W: panelSize.W, H: panelSize.H}); hs.cfg.Output != dp2 || hs.panel(t).PanelRect() != want {
		t.Errorf("on another output: %v at %v, want DP-2 at %v", hs.cfg.Output, hs.panel(t).PanelRect(), want)
	}
}

// The corner grip resizes the panel within its floor and ceiling, its
// top left fixed, and the next request opens at the size it was left.
func TestPanelSizeIsLimitedAndKept(t *testing.T) {
	hs := newHarness(t, "")
	got := hs.open(t, OpenRequest{})
	panel := hs.panel(t)
	r := &widget.Router{Root: panel}
	resize := func(dx, dy int) render.Rect {
		var p render.Rect
		hs.do(func() {
			lay(panel, 3840, 2160)
			p = panel.PanelRect()
			corner := widget.Point{X: p.X + p.W - 4, Y: p.Y + p.H - 4}
			drag(r, corner, widget.Point{X: corner.X + dx, Y: corner.Y + dy})
			lay(panel, 3840, 2160)
		})
		return p
	}
	from := resize(5000, 5000)
	if p := panel.PanelRect(); p != (render.Rect{X: from.X, Y: from.Y, W: panelMax.W, H: panelMax.H}) {
		t.Errorf("grown to %v from %v, want the %dx%d ceiling at the same corner", p, from, panelMax.W, panelMax.H)
	}
	resize(-5000, -5000)
	if p := panel.PanelRect(); p != (render.Rect{X: from.X, Y: from.Y, W: panelMin.W, H: panelMin.H}) {
		t.Errorf("shrunk to %v from %v, want the %dx%d floor at the same corner", p, from, panelMin.W, panelMin.H)
	}
	resize(100, 60)
	kept := panel.PanelRect()
	if kept.W != panelMin.W+100 || kept.H != panelMin.H+60 {
		t.Fatalf("resized to %v, want %dx%d", kept, panelMin.W+100, panelMin.H+60)
	}
	hs.do(func() { hs.c.finish(nil) })
	answer(t, got)
	hs.open(t, OpenRequest{})
	hs.run(func() { lay(hs.cfg.Root, 3840, 2160) })
	if p := hs.panel(t).PanelRect(); p != kept {
		t.Errorf("the next request opened at %v, want the %v it was left at", p, kept)
	}
}

// A new chooser opens at its default size, centred.
func TestPanelOpensAtItsDefaultSize(t *testing.T) {
	hs := newHarness(t, "")
	hs.open(t, OpenRequest{})
	hs.run(func() { lay(hs.cfg.Root, 1920, 1080) })
	if p := hs.panel(t).PanelRect(); p != (render.Rect{X: (1920 - panelSize.W) / 2, Y: (1080 - panelSize.H) / 2, W: panelSize.W, H: panelSize.H}) {
		t.Errorf("a new chooser opened at %v, want %dx%d centred", p, panelSize.W, panelSize.H)
	}
}

// A panel squeezed by a small output, moved there, opens the next
// request at the size it was chosen at, not the size it was squeezed
// to.
func TestASqueezedPanelKeepsTheSizeItWasChosenAt(t *testing.T) {
	for name, room := range map[string]widget.Size{
		"below its floor":        {W: panelMin.W - 80, H: panelMin.H - 60},
		"below its default size": {W: panelSize.W - 60, H: panelSize.H - 40},
	} {
		t.Run(name, func(t *testing.T) {
			hs := newHarness(t, "")
			got := hs.open(t, OpenRequest{})
			panel := hs.panel(t)
			hs.do(func() {
				lay(hs.cfg.Root, room.W, room.H)
				r, grab := treetest.Pointer(hs.cfg.Root, classed(t, panel, "file-chooser-title"), room.W, room.H)
				drag(r, grab, widget.Point{X: grab.X + 5, Y: grab.Y + 5})
				lay(hs.cfg.Root, room.W, room.H)
			})
			if p := panel.PanelRect(); p.W != room.W || p.H != room.H {
				t.Fatalf("the panel on a %v area is %v, want it squeezed to the area", room, p)
			}
			hs.do(func() { hs.c.finish(nil) })
			answer(t, got)
			hs.open(t, OpenRequest{})
			hs.run(func() { lay(hs.cfg.Root, 1920, 1080) })
			if p := hs.panel(t).PanelRect(); p.W != panelSize.W || p.H != panelSize.H {
				t.Errorf("after a squeezed panel the next request opened at %v, want the %v it was chosen at", p, panelSize)
			}
		})
	}
}

// A panel the user shrinks by its grip on a small output opens the
// next request at the size it was shrunk to.
func TestAPanelShrunkOnASmallOutputKeepsThatSize(t *testing.T) {
	room := widget.Size{W: panelSize.W - 60, H: panelSize.H - 40}
	shrunk := widget.Size{W: room.W - 30, H: room.H - 20}
	hs := newHarness(t, "")
	got := hs.open(t, OpenRequest{})
	panel := hs.panel(t)
	hs.do(func() {
		lay(hs.cfg.Root, room.W, room.H)
		p := panel.PanelRect()
		corner := widget.Point{X: p.X + p.W - 4, Y: p.Y + p.H - 4}
		drag(&widget.Router{Root: hs.cfg.Root}, corner, widget.Point{X: corner.X - (room.W - shrunk.W), Y: corner.Y - (room.H - shrunk.H)})
		lay(hs.cfg.Root, room.W, room.H)
	})
	if p := panel.PanelRect(); p.W != shrunk.W || p.H != shrunk.H {
		t.Fatalf("the panel on a %v area shrunk to %v, want %v", room, p, shrunk)
	}
	hs.do(func() { hs.c.finish(nil) })
	answer(t, got)
	hs.open(t, OpenRequest{})
	hs.run(func() { lay(hs.cfg.Root, 1920, 1080) })
	if p := hs.panel(t).PanelRect(); p.W != shrunk.W || p.H != shrunk.H {
		t.Errorf("the next request opened at %v, want the %v it was shrunk to", p, shrunk)
	}
}

func TestLongTitleKeepsClearOfTheHeaderButtons(t *testing.T) {
	const title = "File Upload - Kontainer : Test Kontainer — Mozilla Firefox, with a title longer than any header holds"
	for _, mode := range []struct {
		name    string
		request func(hs *harness) []string
		buttons int
	}{
		{name: "open", buttons: 5, request: func(hs *harness) []string {
			return hs.c.Open(testCaller, OpenRequest{Title: title, CurrentFolder: hs.root}).URIs
		}},
		{name: "save", buttons: 6, request: func(hs *harness) []string {
			return hs.c.Save(testCaller, SaveRequest{Title: title, CurrentFolder: hs.root, CurrentName: "a.txt"}).URIs
		}},
	} {
		t.Run(mode.name, func(t *testing.T) {
			hs := newHarness(t, "")
			hs.c.size = widget.Size{W: panelMin.W, H: panelMin.H}
			hs.ask(t, func() []string { return mode.request(hs) })
			hs.run(func() {
				panel := hs.laidOut(t)
				if panel.W != panelMin.W-2*panelBorder {
					t.Fatalf("panel %v, want it at its %dpx floor", panel, panelMin.W)
				}
				header := classed(t, hs.cfg.Root, "file-chooser-header").Bounds()
				label := classed(t, hs.cfg.Root, "file-chooser-title").(*widget.Label)
				tb := label.Bounds()
				if tb.W <= 0 || !inside(tb, header) {
					t.Fatalf("title at %v, want it inside the header %v", tb, header)
				}
				if label.CharsWithin(tb.W) >= len([]rune(title)) {
					t.Errorf("the whole title fits its %dpx: the test never squeezed it", tb.W)
				}
				buttons := treetest.WithClass(hs.cfg.Root, "file-chooser-nav")
				var inHeader int
				for _, w := range buttons {
					b := w.(widget.Boundser).Bounds()
					if !inside(b, header) {
						continue
					}
					inHeader++
					if !b.Intersect(tb).Empty() {
						t.Errorf("title at %v covers %q at %v", tb, widget.Describe(w).Name, b)
					}
				}
				if inHeader != mode.buttons {
					t.Errorf("%d header buttons, want %d", inHeader, mode.buttons)
				}
			})
		})
	}
}

// A modal request holds the keyboard; a request with modal off takes it
// only on demand.
func TestModalDecidesTheKeyboard(t *testing.T) {
	hs := newHarness(t, "")
	got := hs.ask(t, func() []string {
		return hs.c.Open(portaldialogs.Caller{Handle: "/r/m", Modal: true}, OpenRequest{CurrentFolder: hs.root}).URIs
	})
	if hs.cfg.Keyboard != app.KeyboardExclusive {
		t.Errorf("modal keyboard = %v, want exclusive", hs.cfg.Keyboard)
	}
	hs.do(func() { hs.c.finish(nil) })
	answer(t, got)
	hs.ask(t, func() []string {
		return hs.c.Open(portaldialogs.Caller{Handle: "/r/n", Modal: false}, OpenRequest{CurrentFolder: hs.root}).URIs
	})
	if hs.cfg.Keyboard != app.KeyboardOnDemand {
		t.Errorf("non-modal keyboard = %v, want on demand", hs.cfg.Keyboard)
	}
}

// Two requests at once both get answered: the second waits its turn.
func TestConcurrentRequestsBothGetAnswers(t *testing.T) {
	hs := newHarness(t, "")
	first := hs.open(t, OpenRequest{})
	second := make(chan []string, 1)
	go func() {
		second <- hs.c.Open(portaldialogs.Caller{Handle: "/r/2"}, OpenRequest{CurrentFolder: hs.root}).URIs
	}()
	select {
	case uris := <-first:
		t.Fatalf("a second request answered the first with %v", uris)
	case <-hs.opened:
		t.Fatal("the second request mapped over the first")
	case <-time.After(quietWindow):
	}
	want := fileuri.FromPath(filepath.Join(hs.root, "a"))
	hs.do(func() { hs.c.finish([]string{want}) })
	if uris := answer(t, first); len(uris) != 1 || uris[0] != want {
		t.Errorf("first = %v", uris)
	}
	hs.cfg = waittest.For(t, hs.opened, "the second request to show")
	hs.do(func() { hs.c.finish([]string{want + "2"}) })
	if uris := answer(t, second); len(uris) != 1 || uris[0] != want+"2" {
		t.Errorf("second = %v", uris)
	}
}

// The chooser's containers report a focused control to the focus ring,
// in the header and in the body alike.
func TestLayoutWidgetsReportTheirChildToTheFocusRing(t *testing.T) {
	hs := newHarness(t, "")
	hs.open(t, OpenRequest{})
	hs.run(func() {
		root := hs.cfg.Root
		for _, name := range []string{"Up", "Cancel"} {
			b := treetest.Button(t, root, name)
			lay(root, 800, 600)
			cv := render.New(make([]byte, render.Stride(800)*600), render.Stride(800), 800, 600)
			drawn := false
			cv.MarkFocus(b, func(*render.Canvas) { drawn = true })
			root.Paint(cv)
			if !drawn {
				t.Errorf("%s was not reported", name)
			}
			cv.FinishFocus()
		}
	})
}

// quietWindow is how long what must not happen gets to happen anyway
// before a test concludes it does not.
const quietWindow = 50 * time.Millisecond

// testCaller is the request the tests' dialogs serve.
var testCaller = portaldialogs.Caller{Handle: "/r/test", AppID: "org.app", Modal: true}

// Dismissing the request the chooser shows answers it with nothing and
// takes it down; another request's dismissal leaves it up.
func TestDismissTakesDownItsRequestOnly(t *testing.T) {
	hs := newHarness(t, "")
	got := hs.open(t, OpenRequest{})
	hs.c.Dismiss("/r/other")
	pending(t, got)
	hs.c.Dismiss(testCaller.Handle)
	if uris := answer(t, got); len(uris) != 0 {
		t.Errorf("a dismissed chooser answered %v", uris)
	}
}

// askResult starts a request and waits for its surface, answering the
// whole Result.
func (hs *harness) askResult(t *testing.T, fn func() Result) chan Result {
	t.Helper()
	got := make(chan Result, 1)
	hs.asks.Go(func() { got <- fn() })
	hs.cfg = waittest.For(t, hs.opened, "the chooser to open")
	hs.settle()
	return got
}

var (
	txtFilter = Filter{Name: "Text", Rules: []Rule{{RuleGlob, "*.txt"}}}
	pngFilter = Filter{Name: "Images", Rules: []Rule{{RuleGlob, "*.[pP][nN][gG]"}}}
)

// The confirm button takes the accept label, the dialog starts on the
// current filter, and the answer carries the filter active and the
// choices made; a cancel carries neither.
func TestOpenTakesTheAppsOptions(t *testing.T) {
	hs := newHarness(t, "")
	choices := []portaldialogs.Choice{
		{ID: "encoding", Label: "Encoding", Options: []portaldialogs.Option{{ID: "utf8", Label: "UTF-8"}, {ID: "latin1", Label: "Latin-1"}}, Initial: "latin1"},
		{ID: "readonly", Label: "Open read-only", Initial: "true"},
	}
	got := hs.askResult(t, func() Result {
		return hs.c.Open(testCaller, OpenRequest{
			CurrentFolder: hs.root, AcceptLabel: "Import", Filters: []Filter{txtFilter, pngFilter}, CurrentFilter: 1, Choices: choices,
		})
	})
	if shown := hs.shown(); !slices.Equal(shown, []string{"docs", "pics", "a.png"}) {
		t.Errorf("listing = %v, want the image filter applied from the start", shown)
	}
	checks := treetest.All[*widget.CheckButton](hs.cfg.Root)
	if len(checks) != 1 || !checks[0].Checked() {
		t.Fatalf("%d check buttons, want the read-only choice checked", len(checks))
	}
	hs.do(func() { checks[0].SetChecked(false) })
	hs.pick(t, "a.png")
	hs.click(t, "Import")
	r := answer(t, got)
	if !slices.Equal(r.URIs, []string{hs.uri("a.png")}) || r.Filter.Name != "Images" {
		t.Errorf("result %+v", r)
	}
	if want := []portaldialogs.Selection{{ID: "encoding", Value: "latin1"}, {ID: "readonly", Value: "false"}}; !slices.Equal(r.Choices, want) {
		t.Errorf("choices = %v, want %v", r.Choices, want)
	}
	got = hs.askResult(t, func() Result { return hs.c.Open(testCaller, OpenRequest{CurrentFolder: hs.root, Choices: choices}) })
	hs.click(t, "Cancel")
	if r := answer(t, got); r.URIs != nil || r.Choices != nil || len(r.Filter.Rules) != 0 {
		t.Errorf("a cancel answered %+v", r)
	}
}

// A pick of several folders answers each selected one.
func TestOpenSeveralFolders(t *testing.T) {
	hs := newHarness(t, "")
	got := hs.open(t, OpenRequest{Directory: true, Multiple: true})
	hs.do(func() {
		hs.view().Select(hs.index(t, "docs"))
		hs.view().SelectAll()
	})
	hs.click(t, "Select")
	if uris := answer(t, got); !slices.Equal(uris, []string{hs.uri("docs"), hs.uri("pics")}) {
		t.Errorf("answered %v, want both folders", uris)
	}
}

// The save name may be a path: absolute, under ~ (home), or into a
// subfolder; a folder it names opens instead.
func TestSaveResolvesTheNamedPath(t *testing.T) {
	hs := newHarness(t, "")
	elsewhere := t.TempDir()
	for name, want := range map[string]string{
		filepath.Join(elsewhere, "abs.txt"): "file://" + filepath.Join(elsewhere, "abs.txt"),
		"~/docs/home.txt":                   hs.uri("docs/home.txt"),
		"docs/sub.txt":                      hs.uri("docs/sub.txt"),
	} {
		got := hs.ask(t, func() []string { return hs.c.Save(testCaller, SaveRequest{CurrentFolder: hs.root}).URIs })
		hs.do(func() { hs.c.name.SetText(name) })
		hs.click(t, "Save")
		if uris := answer(t, got); !slices.Equal(uris, []string{want}) {
			t.Errorf("%q answered %v, want %s", name, uris, want)
		}
	}
	got := hs.ask(t, func() []string { return hs.c.Save(testCaller, SaveRequest{CurrentFolder: hs.root}).URIs })
	hs.do(func() { hs.c.name.SetText("docs") })
	hs.click(t, "Save")
	pending(t, got)
	var dir, text string
	hs.do(func() { dir, text = hs.c.b.Dir(), hs.c.name.Text() })
	if dir != filepath.Join(hs.root, "docs") || text != "" {
		t.Errorf("naming a folder left %s with %q, want it opened and the name cleared", dir, text)
	}
	hs.click(t, "Cancel")
	answer(t, got)
}

// An existing file is replaced only on the notice's confirm; changing
// the name drops the prompt. A folder missing or not writable warns.
func TestSaveAsksBeforeReplacing(t *testing.T) {
	hs := newHarness(t, "")
	got := hs.ask(t, func() []string {
		return hs.c.Save(testCaller, SaveRequest{CurrentFolder: hs.root, CurrentName: "b.txt"}).URIs
	})
	hs.click(t, "Save")
	pending(t, got)
	var notice string
	var visible bool
	notice, visible = hs.notice(t)
	if !visible || notice != i18n.T("file-chooser-save-exists", i18n.Str("name", "b.txt"), i18n.Str("folder", filepath.Base(hs.root))) {
		t.Errorf("notice %v %q", visible, notice)
	}
	hs.do(func() { hs.c.name.SetText("b.txt ") })
	_, visible = hs.notice(t)
	if visible {
		t.Error("editing the name kept the replace prompt")
	}
	hs.click(t, "Save")
	pending(t, got)
	hs.click(t, "Replace")
	if uris := answer(t, got); !slices.Equal(uris, []string{hs.uri("b.txt")}) {
		t.Errorf("replace answered %v", uris)
	}

	got = hs.ask(t, func() []string { return hs.c.Save(testCaller, SaveRequest{CurrentFolder: hs.root}).URIs })
	hs.do(func() { hs.c.name.SetText("missing/x.txt") })
	hs.click(t, "Save")
	pending(t, got)
	notice, _ = hs.notice(t)
	if notice != i18n.T("file-chooser-save-no-folder", i18n.Str("folder", filepath.Join(hs.root, "missing"))) {
		t.Errorf("missing folder notice %q", notice)
	}
	locked := filepath.Join(hs.root, "locked")
	if err := os.Mkdir(locked, 0o500); err != nil {
		t.Fatal(err)
	}
	hs.do(func() { hs.c.name.SetText("locked/x.txt") })
	hs.click(t, "Save")
	pending(t, got)
	notice, _ = hs.notice(t)
	if os.Geteuid() != 0 && notice != i18n.T("file-chooser-save-not-writable", i18n.Str("folder", locked)) {
		t.Errorf("read-only folder notice %q", notice)
	}
	hs.click(t, "Cancel")
	answer(t, got)
}

// Picking a filter gives the save name its extension.
func TestSaveFilterSetsTheExtension(t *testing.T) {
	hs := newHarness(t, "")
	got := hs.ask(t, func() []string {
		return hs.c.Save(testCaller, SaveRequest{CurrentFolder: hs.root, CurrentName: "report.txt", Filters: []Filter{txtFilter, pngFilter}}).URIs
	})
	hs.do(func() { hs.c.filter.SetSelected(1) })
	var text string
	hs.do(func() { text = hs.c.name.Text() })
	if text != "report.png" {
		t.Errorf("name = %q after the image filter", text)
	}
	hs.do(func() { hs.c.filter.SetSelected(2) }) // All Files: no extension
	hs.do(func() { text = hs.c.name.Text() })
	if text != "report.png" {
		t.Errorf("name = %q after All Files", text)
	}
	hs.click(t, "Cancel")
	answer(t, got)
}

// key presses a key on the dialog with focus on focused (none for nil).
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
	hs.do(func() { d = hs.c.b.Dir() })
	return d
}

// A typed path naming a file answers it when opening files; saving, a
// typed path is the folder and the name; a folder pick warns.
func TestTypeAheadAndLocation(t *testing.T) {
	hs := newHarness(t, "")
	locate := func(text string) {
		t.Helper()
		hs.key('l', app.ModCtrl, nil)
		hs.do(func() {
			loc := classed(t, hs.cfg.Root, "file-chooser-location")
			loc.(interface{ SetText(string) }).SetText(text)
			r := &widget.Router{Root: hs.cfg.Root}
			if err := r.SetFocus(hs.windows[len(hs.windows)-1].focus); err != nil {
				t.Fatalf("focus the location entry: %v", err)
			}
			r.KeyAction(widget.KeyEnter, 0)
		})
	}
	got := hs.open(t, OpenRequest{CurrentFolder: filepath.Join(hs.root, "pics")})
	locate("a.png")
	if uris := answer(t, got); !slices.Equal(uris, []string{hs.uri("pics/a.png")}) {
		t.Errorf("a typed file answered %v", uris)
	}
	got = hs.ask(t, func() []string { return hs.c.Save(testCaller, SaveRequest{CurrentFolder: hs.root}).URIs })
	locate("docs/report.odt")
	var text string
	hs.do(func() { text = hs.c.name.Text() })
	if hs.dir() != filepath.Join(hs.root, "docs") || text != "report.odt" || hs.windows[len(hs.windows)-1].focus != hs.c.name {
		t.Errorf("a typed save path left %s with %q", hs.dir(), text)
	}
	hs.click(t, "Cancel")
	answer(t, got)
	got = hs.open(t, OpenRequest{Directory: true})
	locate("b.txt")
	pending(t, got)
	if notice, _ := hs.notice(t); notice != i18n.T("file-chooser-location-missing", i18n.Str("path", hs.uri("b.txt")[len("file://"):])) {
		t.Errorf("a typed file in a folder pick noticed %q", notice)
	}
	hs.click(t, "Cancel")
	answer(t, got)
}

// A save and a folder pick offer Create Folder; opening files does not.
func TestCreateFolder(t *testing.T) {
	hs := newHarness(t, "")
	got := hs.ask(t, func() []string { return hs.c.Save(testCaller, SaveRequest{CurrentFolder: hs.root}).URIs })
	if !hs.hasButton("Create Folder") {
		t.Error("a save offers no Create Folder")
	}
	hs.click(t, "Cancel")
	answer(t, got)
	got = hs.open(t, OpenRequest{Directory: true})
	if !hs.hasButton("Create Folder") {
		t.Error("a folder pick offers no Create Folder")
	}
	hs.click(t, "Cancel")
	answer(t, got)
	got = hs.open(t, OpenRequest{})
	if hs.hasButton("Create Folder") {
		t.Error("an open dialog offers Create Folder")
	}
	hs.click(t, "Cancel")
	answer(t, got)
}

// hasButton reports a button named name (its label or tooltip) in
// the chooser's tree, walked as the loop.
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

// Recent is offered when opening files, never when saving.
func TestRecentAndListingErrors(t *testing.T) {
	recent := filepath.Join(t.TempDir(), "recently-used.xbel")
	hs := newHarness(t, "", func(d *filebrowser.Deps) { d.RecentFile = recent })
	body := `<xbel><bookmark href="` + hs.uri("b.txt") + `" modified="2026-10-01T00:00:00Z"/></xbel>`
	if err := os.WriteFile(recent, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	got := hs.open(t, OpenRequest{})
	if !hs.hasButton("Recent") {
		t.Fatal("no Recent place")
	}
	hs.click(t, "Recent")
	hs.pick(t, "b.txt")
	hs.click(t, "Open")
	if uris := answer(t, got); !slices.Equal(uris, []string{hs.uri("b.txt")}) {
		t.Errorf("opened %v from Recent, want the file", uris)
	}
	got = hs.ask(t, func() []string { return hs.c.Save(testCaller, SaveRequest{CurrentFolder: hs.root}).URIs })
	if hs.hasButton("Recent") {
		t.Error("a save offers Recent")
	}
	hs.click(t, "Cancel")
	answer(t, got)
}

// Trash is offered when opening files and answers the trashed file
// itself; a save or a folder pick never offers it.
func TestTrashPlace(t *testing.T) {
	trash := filepath.Join(t.TempDir(), "Trash")
	trashed := browsertest.Trashed(t, trash, "notes.txt.2", browsertest.TrashInfo("/home/u/notes.txt"))
	hs := newHarness(t, "", func(d *filebrowser.Deps) { d.TrashDirs = func() []string { return []string{trash} } })
	got := hs.open(t, OpenRequest{})
	if !hs.hasButton("Trash") {
		t.Fatal("no Trash place")
	}
	hs.click(t, "Trash")
	if shown := hs.shown(); !slices.Equal(shown, []string{"notes.txt"}) {
		t.Errorf("Trash shows %v", shown)
	}
	hs.pick(t, "notes.txt")
	hs.click(t, "Open")
	if uris := answer(t, got); !slices.Equal(uris, []string{fileuri.FromPath(trashed)}) {
		t.Errorf("opened %v, want the trashed file", uris)
	}

	got = hs.ask(t, func() []string { return hs.c.Save(testCaller, SaveRequest{CurrentFolder: hs.root}).URIs })
	if hs.hasButton("Trash") {
		t.Error("a save offers Trash")
	}
	hs.click(t, "Cancel")
	answer(t, got)
	got = hs.open(t, OpenRequest{Directory: true})
	if hs.hasButton("Trash") {
		t.Error("a folder pick offers Trash")
	}
	hs.click(t, "Cancel")
	answer(t, got)
}

// The chooser stands on lys-ui's parts: a floating panel around the
// browser, an outline Cancel and the suggested confirm.
func TestChooserTakesLysUiParts(t *testing.T) {
	hs := newHarness(t, "")
	got := hs.open(t, OpenRequest{})
	hs.run(func() {
		root := hs.cfg.Root
		hs.panel(t)
		if !widget.HasClass(classed(t, root, "file-chooser-cancel").(widget.Widget), css.Outline) {
			t.Error("Cancel is no outline button")
		}
		if !widget.HasClass(classed(t, root, "file-chooser-confirm").(widget.Widget), css.SuggestedAction) {
			t.Error("the confirm button is not the suggested action")
		}
		if widget.HasClass(classed(t, root, "file-chooser-cancel").(widget.Widget), css.SuggestedAction) {
			t.Error("Cancel is the suggested action")
		}
	})
	hs.click(t, "Cancel")
	answer(t, got)
}

// The chooser reveals its panel with the file-chooser animation and
// keeps the overlay until the panel has hidden; with animations off
// the overlay closes as the dialog answers.
func TestPanelRevealsAndHidesWithItsAnimation(t *testing.T) {
	hs := newHarness(t, "")
	hs.conf.Animations.Enabled = true
	got := hs.open(t, OpenRequest{})
	rev, ok := hs.cfg.Root.(*widget.Revealer)
	if !ok {
		t.Fatalf("root %T, want the revealer around the panel", hs.cfg.Root)
	}
	want := reveal.Transition(hs.conf.Animations.TransitionFor(config.AnimFileChooser, false))
	if !rev.Revealed() || rev.Transition() != want {
		t.Errorf("revealed %v with %v, want revealed with the file chooser's %v", rev.Revealed(), rev.Transition(), want)
	}
	win := hs.windows[len(hs.windows)-1]
	hs.do(func() { hs.cfg.OnKey(&widget.Router{}, treetest.Code(xkb.KeyEscape), 0) })
	answer(t, got)
	if rev.Revealed() {
		t.Error("cancelling left the panel revealed")
	}
	if win.closed {
		t.Error("the overlay closed before its panel hid")
	}

	hs.conf.Animations.Enabled = false
	got = hs.open(t, OpenRequest{})
	win = hs.windows[len(hs.windows)-1]
	hs.do(func() { hs.cfg.OnKey(&widget.Router{}, treetest.Code(xkb.KeyEscape), 0) })
	answer(t, got)
	if !win.closed {
		t.Error("with animations off the overlay outlived the answer")
	}
}

func TestTheChooserMapsWhilePlacesAndLocationsHold(t *testing.T) {
	release := make(chan struct{})
	t.Cleanup(sync.OnceFunc(func() { close(release) }))
	asked := make(chan string, 4)
	var hs *harness
	hs = newHarness(t, "", func(d *filebrowser.Deps) {
		d.Places = func() []filebrowser.Place {
			asked <- "places"
			<-release
			return []filebrowser.Place{{Label: "Held", Path: hs.root, Icon: "user-home-symbolic"}}
		}
		d.Locations = func() []filebrowser.Place {
			asked <- "locations"
			<-release
			return nil
		}
	})
	got := make(chan []string, 1)
	hs.asks.Go(func() { got <- hs.c.Open(testCaller, OpenRequest{CurrentFolder: hs.root}).URIs })
	waittest.For(t, hs.opened, "the chooser to map without waiting on its sidebar")
	waittest.For(t, asked, "the sidebar to be read")
	hs.flush()
}

// The chooser is opened for the request's window, modality and title; a
// chooser that lost its parent and could not map again cancels.
func TestTheChooserIsOpenedForTheRequestWindow(t *testing.T) {
	hs := newHarness(t, "")
	caller := portaldialogs.Caller{Handle: "/r/parent", AppID: "org.app", ParentWindow: "wayland:abc", Modal: true}
	got := hs.ask(t, func() []string {
		return hs.c.Open(caller, OpenRequest{Title: "Pick a file", CurrentFolder: hs.root}).URIs
	})
	hs.mu.Lock()
	spec := hs.specs[len(hs.specs)-1]
	hs.mu.Unlock()
	if spec.Caller != caller || spec.Title != "Pick a file" || spec.Failed == nil {
		t.Fatalf("opened for %+v %q (failed set %v), want %+v", spec.Caller, spec.Title, spec.Failed != nil, caller)
	}
	hs.do(spec.Failed)
	if uris := answer(t, got); uris != nil {
		t.Errorf("a chooser nothing shows answered %v", uris)
	}
}

func TestCloseAnswersTheRequestShowingAndThoseWaiting(t *testing.T) {
	hs := newHarness(t, "")
	showing := hs.open(t, OpenRequest{})
	waiting := make(chan []string, 1)
	hs.asks.Go(func() {
		waiting <- hs.c.Open(portaldialogs.Caller{Handle: "/r/second", AppID: "org.app"}, OpenRequest{CurrentFolder: hs.root}).URIs
	})
	waittest.Until(t, func() bool {
		var queued int
		hs.run(func() { queued = hs.c.queue.Len() })
		return queued == 1
	}, "the second request to queue")
	pending(t, showing)
	hs.endLoop()
	hs.c.Close()
	if uris := answer(t, showing); len(uris) != 0 {
		t.Errorf("closing answered the request showing with %v", uris)
	}
	if uris := answer(t, waiting); len(uris) != 0 {
		t.Errorf("closing answered the waiting request with %v", uris)
	}
}
