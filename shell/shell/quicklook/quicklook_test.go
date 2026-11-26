package quicklook

import (
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"github.com/stubbedev/lys/desktop/mime"
	"github.com/stubbedev/lys/desktop/shell/layers"
	"github.com/stubbedev/lys/desktop/shell/preview"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/lib/xkb"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"
)

type fakeWindow struct {
	cfg    app.LayerConfig
	closed bool
}

func (w *fakeWindow) Close() { w.closed = true }

type harness struct {
	t       *testing.T
	mu      sync.Mutex
	queue   []func()
	work    sync.WaitGroup
	q       *QuickLook
	windows []*fakeWindow
	opened  []string
	closed  int
}

func newHarness(t *testing.T, opener bool) *harness {
	t.Helper()
	h := &harness{t: t}
	d := Deps{
		Open: func(cfg app.LayerConfig) (Window, error) {
			w := &fakeWindow{cfg: cfg}
			h.windows = append(h.windows, w)
			return w, nil
		},
		KeySym: func(code uint32) xkb.Keysym { return xkb.Keysym(code) },
		Invoke: func(fn func()) {
			h.mu.Lock()
			h.queue = append(h.queue, fn)
			h.mu.Unlock()
		},
		Spawn:     h.work.Go,
		Font:      treetest.Font(t),
		Scale:     func() float64 { return 1 },
		MIME:      mime.System(),
		Namespace: layers.ScreenshotThumbnail,
	}
	if opener {
		d.OpenFile = func(path string) { h.opened = append(h.opened, path) }
	}
	h.q = New(d)
	t.Cleanup(func() {
		h.q.Close()
		h.pump()
		h.work.Wait()
	})
	return h
}

func (h *harness) pump() {
	for {
		h.mu.Lock()
		q := h.queue
		h.queue = nil
		h.mu.Unlock()
		if len(q) == 0 {
			return
		}
		for _, fn := range q {
			fn()
		}
	}
}

func (h *harness) file() string {
	h.t.Helper()
	path := filepath.Join(h.t.TempDir(), "notes.txt")
	if err := os.WriteFile(path, []byte("first line\nsecond line\n"), 0o600); err != nil {
		h.t.Fatal(err)
	}
	return path
}

// show shows path and waits for its text to load.
func (h *harness) show(path string) *fakeWindow {
	h.t.Helper()
	if err := h.q.Show(path, nil, func() { h.closed++ }); err != nil {
		h.t.Fatal(err)
	}
	waittest.UntilShowing(h.t, func() bool {
		h.pump()
		return h.q.cur != nil && h.q.cur.view.Renderer() == preview.Text
	}, "the file's text", func() string { return "" })
	return h.windows[len(h.windows)-1]
}

func (h *harness) key(w *fakeWindow, sym xkb.Keysym, mods app.Mods) {
	w.cfg.OnKey(nil, uint32(sym), mods)
}

// Quick Look maps a surface over the output that holds the keyboard,
// the file in a card centered on it, and Space closes it once.
func TestQuickLookShowsTheFileAndSpaceClosesIt(t *testing.T) {
	h := newHarness(t, false)
	path := h.file()
	w := h.show(path)
	if w.cfg.Keyboard != app.KeyboardExclusive || w.cfg.Layer != app.LayerOverlay || w.cfg.Namespace != layers.ScreenshotThumbnail.Name() {
		t.Errorf("the surface is %+v", w.cfg)
	}
	if got, ok := h.q.Shown(); !ok || got != path {
		t.Errorf("Quick Look shows %q %v, want %q", got, ok, path)
	}
	w.cfg.Root.Arrange(render.Rect{W: 1920, H: 1080})
	card := h.q.cur.view.Root().Bounds()
	if card.W == 0 || card.X != (1920-card.W)/2 || card.Y != (1080-card.H)/2 {
		t.Errorf("the card lies at %+v, not centered", card)
	}
	h.key(w, 'a', 0)
	h.key(w, xkb.KeySpace, app.ModCtrl)
	if w.closed || h.closed != 0 {
		t.Fatal("a key that is not Quick Look's closed it")
	}
	h.key(w, xkb.KeySpace, 0)
	if !w.closed || h.closed != 1 {
		t.Fatalf("Space: window closed %v, closed ran %d times", w.closed, h.closed)
	}
	if _, ok := h.q.Shown(); ok {
		t.Error("Quick Look still shows after Space")
	}
	h.key(w, xkb.KeySpace, 0)
	if h.closed != 1 {
		t.Error("closed ran again for a surface already gone")
	}
}

// Escape closes, as does a press outside the card; a press on the card
// does not.
func TestEscapeAndAPressOutsideClose(t *testing.T) {
	h := newHarness(t, false)
	path := h.file()
	w := h.show(path)
	h.key(w, xkb.KeyEscape, app.ModShift)
	if !w.closed || h.closed != 1 {
		t.Fatal("Escape did not close Quick Look")
	}
	w = h.show(path)
	w.cfg.OnPress(0x110, h.q.cur.view.Root())
	if w.closed {
		t.Fatal("a press on the card closed Quick Look")
	}
	w.cfg.OnPress(0x110, w.cfg.Root)
	if !w.closed || h.closed != 2 {
		t.Fatal("a press outside the card did not close Quick Look")
	}
}

// Showing another file closes the first; the compositor taking the
// surface away closes it too.
func TestAnotherShowReplacesTheFirst(t *testing.T) {
	h := newHarness(t, false)
	path := h.file()
	first := h.show(path)
	second := h.show(path)
	if !first.closed || second.closed || h.closed != 1 {
		t.Fatalf("first closed %v, second closed %v, closed ran %d", first.closed, second.closed, h.closed)
	}
	second.cfg.OnClosed()
	if h.closed != 2 {
		t.Error("the surface going away did not close Quick Look")
	}
}

// Open opens the file through the opener and closes Quick Look; without
// an opener there is no Open.
func TestOpenOpensThroughTheOpener(t *testing.T) {
	h := newHarness(t, true)
	path := h.file()
	w := h.show(path)
	buttons := treetest.All[*widget.Button](w.cfg.Root)
	if len(buttons) != 1 {
		t.Fatalf("Quick Look offers %d buttons, want Open", len(buttons))
	}
	buttons[0].OnClick()
	if !slices.Equal(h.opened, []string{path}) || !w.closed || h.closed != 1 {
		t.Fatalf("opened %q, closed %v %d", h.opened, w.closed, h.closed)
	}

	none := newHarness(t, false)
	w = none.show(none.file())
	if n := len(treetest.All[*widget.Button](w.cfg.Root)); n != 0 {
		t.Errorf("without an opener Quick Look offers %d buttons", n)
	}
}
