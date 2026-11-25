package app

import (
	"errors"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/render"
	"golang.org/x/image/font/gofont/goregular"

	"github.com/stubbedev/lys/ui/anim"
	"github.com/stubbedev/lys/ui/internal/wlsession"
	"github.com/stubbedev/lys/ui/widget"
)

// testFace loads the Go regular face for widget constructors; the
// nil-face contract makes a face mandatory everywhere.
func testFace(t testing.TB) *render.Typeface {
	t.Helper()
	f, err := render.LoadFont(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// spyRoot records the Arrange rect, standing in for a window's root
// widget.
type spyRoot struct {
	widget.Nodeless
	arranged render.Rect
	nat      widget.Size
	painted  int
}

func (s *spyRoot) Measure(axis widget.Axis, _ int) widget.Sizes {
	return widget.FixedSizes(s.nat.Along(axis))
}

func (s *spyRoot) Arrange(r render.Rect) { s.arranged = r }

func (s *spyRoot) Paint(*render.Canvas) { s.painted++ }

func (s *spyRoot) HitTest(widget.Point) widget.Widget { return nil }

func newToastLayerForTest(root *spyRoot) *toastLayer {
	return &toastLayer{root: root, clock: new(anim.Clock), margin: toastMargin, max: toastMaxStack}
}

func TestToastLayerDropsOldestPastCap(t *testing.T) {
	l := newToastLayerForTest(&spyRoot{nat: widget.Size{W: 50, H: 40}})
	l.max = 2
	toasts := make([]*widget.Toast, 3)
	for i := range toasts {
		toasts[i] = widget.NewToast(testFace(t), "toast", time.Second)
		toasts[i].OnDismissed = func() { l.remove(toasts[i]) }
		l.add(toasts[i], false)
	}

	t.Run("the cap holds and the oldest drops", func(t *testing.T) {
		if len(l.slots) != 2 {
			t.Fatalf("stack holds %d toasts, want 2", len(l.slots))
		}
		if !toasts[0].Closed() {
			t.Error("the oldest toast survived the cap")
		}
		if toasts[1].Closed() || toasts[2].Closed() {
			t.Error("a live toast was dropped instead of the oldest")
		}
		if l.slots[0] != toasts[1] || l.slots[1] != toasts[2] {
			t.Error("stack order lost: want oldest live first, newest last")
		}
	})
}

func TestToastLayerArrange(t *testing.T) {
	root := &spyRoot{nat: widget.Size{W: 50, H: 40}}
	l := newToastLayerForTest(root)

	t.Run("the wrapped root keeps the whole window", func(t *testing.T) {
		full := render.Rect{X: 0, Y: 0, W: 300, H: 200}
		if got := widget.SizeWithin(l, widget.Size{W: full.W, H: full.H}); got != root.nat {
			t.Errorf("Measure = %v, want the root's natural %v", got, root.nat)
		}
		l.Arrange(full)
		if root.arranged != full {
			t.Errorf("root arranged into %v, want %v", root.arranged, full)
		}
	})

	first := widget.NewToast(testFace(t), "one", 0)
	l.add(first, false)
	l.Arrange(render.Rect{X: 0, Y: 0, W: 300, H: 200})

	t.Run("a single toast sits bottom-center inside the margin", func(t *testing.T) {
		b := first.Bounds()
		if b.Empty() {
			t.Fatal("toast never arranged")
		}
		if b.Y+b.H != 200-toastMargin {
			t.Errorf("toast bottom edge at %d, want %d", b.Y+b.H, 200-toastMargin)
		}
		if b.X != (300-b.W)/2 {
			t.Errorf("toast x = %d, want centered %d", b.X, (300-b.W)/2)
		}
		if b.W > 300-2*toastMargin {
			t.Errorf("toast width %d past the margin box", b.W)
		}
	})

	second := widget.NewToast(testFace(t), "two", 0)
	l.add(second, false)
	l.Arrange(render.Rect{X: 0, Y: 0, W: 300, H: 200})

	t.Run("the newest hugs the edge, older stacks above", func(t *testing.T) {
		top, bottom := first.Bounds(), second.Bounds()
		if top.Y+top.H+toastSpacing != bottom.Y {
			t.Errorf("older toast bottom %d + spacing != newer top %d", top.Y+top.H, bottom.Y)
		}
		if bottom.Y+bottom.H != 200-toastMargin {
			t.Errorf("newest toast not at the edge: bottom %d", bottom.Y+bottom.H)
		}
	})

	l.pos = ToastTop
	l.Arrange(render.Rect{X: 0, Y: 0, W: 300, H: 200})

	t.Run("a top stack mirrors downward", func(t *testing.T) {
		newest, older := second.Bounds(), first.Bounds()
		if newest.Y != toastMargin {
			t.Errorf("newest toast top = %d, want the margin %d", newest.Y, toastMargin)
		}
		if older.Y != newest.Y+newest.H+toastSpacing {
			t.Errorf("older toast top = %d, want below the newest", older.Y)
		}
	})
}

func TestToastLayerRemoveUnwraps(t *testing.T) {
	root := &spyRoot{nat: widget.Size{W: 50, H: 40}}
	l := newToastLayerForTest(root)
	a := widget.NewToast(testFace(t), "a", 0)
	b := widget.NewToast(testFace(t), "b", 0)
	a.OnDismissed = func() { l.remove(a) }
	b.OnDismissed = func() { l.remove(b) }
	l.add(a, false)
	l.add(b, false)

	// A dismissed toast detaches; the survivor stays.
	a.OnDismissed()
	if len(l.slots) != 1 || l.slots[0] != b {
		t.Fatalf("stack = %v, want just b", l.slots)
	}
	if !a.Closed() {
		t.Error("removed toast not closed")
	}

	// The last removal tears the layer down and detaches it from the
	// window.
	hw := &hostWindow{}
	hw.router = &widget.Router{Root: widget.Widget(l)}
	l.hw = hw
	b.OnDismissed()
	if len(l.slots) != 0 {
		t.Errorf("stack holds %d toasts after the last removal", len(l.slots))
	}
	if hw.router.Root != widget.Widget(root) {
		t.Errorf("window root = %T, want the bare root back", hw.router.Root)
	}
	if l.hw != nil {
		t.Error("layer still attached to its window")
	}

	// Closing an already-detached layer is inert.
	l.close()
}

func TestShowToastWithoutAWindowIsNil(t *testing.T) {
	a := &Application{}
	if got, err := a.ShowToast("hi", time.Second, nil); got != nil || !errors.Is(err, ErrNoToastWindow) {
		t.Errorf("ShowToast without windows = %v, %v; want nil, ErrNoToastWindow", got, err)
	}
	if got, err := a.ShowToast("hi", time.Second, &ToastConfig{}); got != nil || err == nil {
		t.Errorf("ShowToast with a default config = %v, %v", got, err)
	}
}

// A closing window hands its live toasts to the next toast window, and
// SetToastHost picks that window for configs naming none.
func TestToastsMoveOffAClosingWindow(t *testing.T) {
	face := testFace(t)
	h1, h2 := &fakeHost{w: 400, h: 300}, &fakeHost{w: 400, h: 300}
	root1, root2 := &spyRoot{}, &spyRoot{}
	a := &Application{sess: &wlsession.Session{}}
	w1 := &hostWindow{host: h1, router: &widget.Router{Root: root1}, anim: &a.anim}
	w2 := &hostWindow{host: h2, router: &widget.Router{Root: root2}, anim: &a.anim}
	a.windows = []*hostWindow{w1, w2}
	a.SetToastHost(h1)
	toast, err := a.ShowToast("saved", time.Minute, &ToastConfig{Face: face, Position: ToastTop})
	if err != nil || w1.router.Root == widget.Widget(root1) {
		t.Fatalf("toast not on the toast host: %v", err)
	}

	m, ok := a.toasts.takeHost(h1)
	if !ok || w1.router.Root != widget.Widget(root1) || toast.Closed() {
		t.Fatalf("take: ok %v, root restored %v, closed %v", ok, w1.router.Root == widget.Widget(root1), toast.Closed())
	}
	a.windows = []*hostWindow{w2}
	a.rehostToasts([]movedToasts{m})
	l, isLayer := w2.router.Root.(*toastLayer)
	if !isLayer || len(l.slots) != 1 || l.slots[0] != toast || l.pos != ToastTop {
		t.Fatalf("toast did not move to the remaining window: %T", w2.router.Root)
	}
	toast.Close()
	toast.OnDismissed()
	if w2.router.Root != widget.Widget(root2) {
		t.Error("dismissing the moved toast left its layer on the new window")
	}

	a.windows = nil
	a.rehostToasts([]movedToasts{{toasts: []*widget.Toast{widget.NewToast(face, "x", time.Minute)}}})
}

// rectAt is a fixed rect standing in for the widget a toast stack
// centres on.
type rectAt render.Rect

func (r rectAt) Bounds() render.Rect { return render.Rect(r) }

// A stack given a widget to centre on sits on that widget's centre and
// hugs its bottom edge, not the window's: a content pane beside a
// sidebar keeps its toast in its own middle (#279).
func TestAToastStackCentresOnTheWidgetItIsOver(t *testing.T) {
	l := newToastLayerForTest(&spyRoot{nat: widget.Size{W: 50, H: 40}})
	pane := render.Rect{X: 200, Y: 0, W: 400, H: 300}
	l.over = rectAt(pane)
	toast := widget.NewToast(testFace(t), "Removed", 0)
	l.add(toast, false)
	l.Arrange(render.Rect{W: 600, H: 400})
	b := toast.Bounds()
	if c := b.X + b.W/2; c < pane.X+pane.W/2-1 || c > pane.X+pane.W/2+1 {
		t.Errorf("toast centred at x=%d, want the pane's centre %d", c, pane.X+pane.W/2)
	}
	if b.Y+b.H != pane.Y+pane.H-toastMargin {
		t.Errorf("toast bottom at %d, want the pane's bottom less the margin %d", b.Y+b.H, pane.Y+pane.H-toastMargin)
	}
	l.over = nil
	l.Arrange(render.Rect{W: 600, H: 400})
	if b := toast.Bounds(); b.X+b.W/2 != 300 && b.X+b.W/2 != 299 {
		t.Errorf("without a widget the toast centres at x=%d, want the window's 300", b.X+b.W/2)
	}
}

// A toast that closes on a press goes on the next press outside it,
// stays for a press on it (its Undo), and a plain toast outlives any
// press (#279).
func TestAToastClosingOnPressGoesOnTheNextPressOutsideIt(t *testing.T) {
	l := newToastLayerForTest(&spyRoot{nat: widget.Size{W: 50, H: 40}})
	undo := widget.NewToast(testFace(t), "Removed", time.Minute)
	plain := widget.NewToast(testFace(t), "Saved", time.Minute)
	var gone []string
	undo.OnDismissed = func() { gone = append(gone, "undo"); l.remove(undo) }
	plain.OnDismissed = func() { gone = append(gone, "plain"); l.remove(plain) }
	l.add(plain, false)
	l.add(undo, true)
	l.Arrange(render.Rect{W: 600, H: 400})
	on := undo.Bounds()
	l.pressed(widget.Point{X: on.X + on.W/2, Y: on.Y + on.H/2})
	if len(gone) != 0 || undo.Closed() {
		t.Fatalf("a press on the toast closed %v", gone)
	}
	l.pressed(widget.Point{X: 5, Y: 5})
	now := time.Now()
	for range 120 {
		now = now.Add(16 * time.Millisecond)
		l.clock.Tick(now)
	}
	if !undo.Closed() || len(gone) != 1 || gone[0] != "undo" {
		t.Error("a press outside left the toast that closes on a press up")
	}
	if plain.Closed() {
		t.Error("a press closed a toast that does not close on one")
	}
}
