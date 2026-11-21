package app

import (
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/third_party/wayland/xdg"

	"github.com/stubbedev/lys/ui/internal/popup"
	"github.com/stubbedev/lys/ui/internal/wlsession"
	"github.com/stubbedev/lys/ui/widget"
)

// sizedContent asks for the size it was last given, as content that
// takes a resize does.
type sizedContent struct {
	widget.Base
	size widget.Size
}

func (s *sizedContent) Measure(axis widget.Axis, _ int) widget.Sizes {
	return widget.FixedSizes(s.size.Along(axis))
}
func (s *sizedContent) Arrange(r render.Rect)              { s.ArrangeSelf(r) }
func (s *sizedContent) Paint(*render.Canvas)               {}
func (s *sizedContent) HitTest(widget.Point) widget.Widget { return s }

// resizable is a 200x100 popover on side whose content takes each size
// a drag asks for, with what it heard.
type resizable struct {
	op      *openPopover
	surf    *fakePopSurface
	content *sizedContent
	asked   []widget.Size
	resized []widget.Size
}

func newResizable(t *testing.T, side Gravity, lo, hi widget.Size) *resizable {
	t.Helper()
	r := &resizable{content: &sizedContent{size: widget.Size{W: 200, H: 100}}}
	op, surf, _, _ := newLoopPopover(r.content)
	surf.side, surf.x, surf.y = side, 300, 40
	surf.w, surf.h = 204, 104
	r.op, r.surf = op, surf
	resize := PopoverResize{
		Min: lo, Max: hi,
		Resizing: func(s widget.Size) {
			r.asked = append(r.asked, s)
			r.content.size = s
		},
		Resized: func(s widget.Size) { r.resized = append(r.resized, s) },
	}
	if err := resize.validate(); err != nil {
		t.Fatal(err)
	}
	op.keyRoot.grip = newPopoverGrip(op, resize, func() []*wlsession.Output { return nil })
	op.keyRoot.Arrange(render.Rect{W: 200, H: 100})
	return r
}

func (r *resizable) drag(from, to widget.Point) {
	r.op.router.Move(from)
	r.op.router.Press(widget.BTNLeft, from, 0)
	r.op.router.Move(to)
	r.op.router.Release(widget.BTNLeft, to)
}

func TestDraggingTheFreeCornerResizesThePopover(t *testing.T) {
	r := newResizable(t, GravityBottom, widget.Size{W: 50, H: 50}, widget.Size{W: 1000, H: 1000})
	if got := widget.HoverCursor(r.op.keyRoot.HitTest(widget.Point{X: 198, Y: 98})); got != wlsession.CursorBottomRightCorner {
		t.Errorf("the free corner shows %q", got)
	}
	r.drag(widget.Point{X: 198, Y: 98}, widget.Point{X: 238, Y: 128})
	want := widget.Size{W: 240, H: 130}
	if !slices.Equal(r.asked, []widget.Size{want}) {
		t.Errorf("the drag asked for %v, want %v", r.asked, want)
	}
	if want := [][2]int{{244 + resizeSlack, 134 + resizeSlack}, {244, 134}}; !slices.Equal(r.surf.resizes, want) {
		t.Errorf("the surface was resized %v, want %v: room to drag in, then the content's size", r.surf.resizes, want)
	}
	if r.surf.catchAll {
		t.Error("the surface still takes input beyond the content after the drag")
	}
	if !slices.Equal(r.surf.holds, []popup.Hold{popup.HoldStart}) {
		t.Errorf("a bottom-right drag held %v, want the left edge", r.surf.holds)
	}
	if !slices.Equal(r.resized, []widget.Size{want}) {
		t.Errorf("the drag ended at %v, want %v", r.resized, want)
	}
	if r.op.popover.resizing {
		t.Error("the popover still resizes after the release")
	}
}

func TestADragAwayFromTheFreeEdgeDoesNotResize(t *testing.T) {
	r := newResizable(t, GravityBottom, widget.Size{W: 50, H: 50}, widget.Size{W: 1000, H: 1000})
	for _, from := range []widget.Point{{X: 100, Y: 50}, {X: 100, Y: 1}, {X: 1, Y: 50}, {X: 198, Y: 50}} {
		if hit := r.op.keyRoot.HitTest(from); hit == widget.Widget(r.op.keyRoot.grip) {
			t.Errorf("%v grips the popover", from)
		}
		r.drag(from, widget.Point{X: from.X + 30, Y: from.Y + 30})
	}
	if len(r.asked) != 0 || len(r.resized) != 0 || len(r.surf.resizes) != 0 {
		t.Errorf("a drag off the free edge resized: asked %v, ended %v, surface %v", r.asked, r.resized, r.surf.resizes)
	}
}

// Above its anchor the free edge is the top one: dragging its left
// corner up and out grows the popover while the surface moves under the
// pointer, and keeps the right edge in place.
func TestAPopoverAboveItsAnchorGrowsUp(t *testing.T) {
	r := newResizable(t, GravityTop, widget.Size{W: 50, H: 50}, widget.Size{W: 1000, H: 1000})
	if hit := r.op.keyRoot.HitTest(widget.Point{X: 100, Y: 98}); hit == widget.Widget(r.op.keyRoot.grip) {
		t.Error("the bottom edge, facing the anchor, grips the popover")
	}
	from := widget.Point{X: 1, Y: 1}
	r.op.router.Move(from)
	r.op.router.Press(widget.BTNLeft, from, 0)
	r.surf.x, r.surf.y = r.surf.x-20, r.surf.y-30
	r.op.router.Move(widget.Point{X: 1, Y: 1})
	r.op.router.Release(widget.BTNLeft, widget.Point{X: 1, Y: 1})
	if want := (widget.Size{W: 220, H: 130}); !slices.Equal(r.asked, []widget.Size{want}) {
		t.Errorf("the drag asked for %v, want %v: the surface moved under the pointer", r.asked, want)
	}
	if !slices.Equal(r.surf.holds, []popup.Hold{popup.HoldEnd}) {
		t.Errorf("a top-left drag held %v, want the right edge", r.surf.holds)
	}
	r.op.keyRoot.Arrange(render.Rect{W: 400, H: 300})
	if want := (render.Rect{X: 180, Y: 170, W: 220, H: 130}); r.op.keyRoot.contentRect != want {
		t.Errorf("in a larger surface the content sits at %+v, want %+v against the held edges", r.op.keyRoot.contentRect, want)
	}
}

func TestTheSurfaceTakesInputBeyondTheContentOnlyWhileDragging(t *testing.T) {
	r := newResizable(t, GravityBottom, widget.Size{W: 50, H: 50}, widget.Size{W: 1000, H: 1000})
	from := widget.Point{X: 100, Y: 98}
	r.op.router.Move(from)
	r.op.router.Press(widget.BTNLeft, from, 0)
	if !r.surf.catchAll {
		t.Error("a drag left input to the content alone")
	}
	r.op.router.Move(widget.Point{X: 100, Y: 120})
	if n := len(r.surf.resizes); n == 0 || r.surf.resizes[n-1] != [2]int{204, 126 + resizeSlack} {
		t.Errorf("a drag of the bottom edge resized the surface %v, want room below only", r.surf.resizes)
	}
	r.op.router.Release(widget.BTNLeft, widget.Point{X: 100, Y: 120})
	if r.surf.catchAll {
		t.Error("the surface took input beyond the content after the release")
	}
}

func TestADragStopsAtTheBounds(t *testing.T) {
	r := newResizable(t, GravityBottom, widget.Size{W: 150, H: 80}, widget.Size{W: 260, H: 300})
	r.drag(widget.Point{X: 198, Y: 98}, widget.Point{X: 600, Y: 40})
	if want := (widget.Size{W: 260, H: 80}); !slices.Equal(r.resized, []widget.Size{want}) {
		t.Errorf("an outsized drag ended at %v, want %v", r.resized, want)
	}
}

func TestAResizeDragHoldsAutohide(t *testing.T) {
	r := newResizable(t, GravityBottom, widget.Size{W: 50, H: 50}, widget.Size{W: 1000, H: 1000})
	a := &Application{openPopovers: []*openPopover{r.op}}
	p := r.op.popover
	p.host, p.autohide = r.op.host, true
	dismissed := 0
	p.closeFn = func() { dismissed++ }
	a.popovers.openOrReplace(r.op.host, p)
	from := widget.Point{X: 100, Y: 98}
	r.op.router.Move(from)
	r.op.router.Press(widget.BTNLeft, from, 0)
	r.op.router.Move(widget.Point{X: 100, Y: 140})
	if a.dismissAutohide(r.op.host) {
		t.Error("a press on the host closed the popover mid-drag")
	}
	p.dismissAway()
	if dismissed != 0 {
		t.Errorf("a click away closed the popover mid-drag %d times", dismissed)
	}
	r.op.router.Release(widget.BTNLeft, widget.Point{X: 100, Y: 140})
	p.dismissAway()
	if dismissed != 1 {
		t.Errorf("a click away after the drag closed it %d times, want once", dismissed)
	}
}

func TestAPopoverResizeIsValidated(t *testing.T) {
	ok := func(widget.Size) {}
	for _, tc := range []struct {
		resize PopoverResize
		want   string
	}{
		{PopoverResize{Min: widget.Size{W: 10, H: 10}, Max: widget.Size{W: 20, H: 20}}, "needs Resizing"},
		{PopoverResize{Max: widget.Size{W: 20, H: 20}, Resizing: ok}, "not positive"},
		{PopoverResize{Min: widget.Size{W: 30, H: 10}, Max: widget.Size{W: 20, H: 20}, Resizing: ok}, "below its minimum"},
	} {
		if err := tc.resize.validate(); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%+v: %v, want %q", tc.resize, err, tc.want)
		}
	}
	if err := (&PopoverResize{Min: widget.Size{W: 10, H: 10}, Max: widget.Size{W: 10, H: 10}, Resizing: ok}).validate(); err != nil {
		t.Errorf("a fixed range was refused: %v", err)
	}
}

func TestDragSizeFollowsTheDraggedEdges(t *testing.T) {
	start, lo, hi := widget.Size{W: 200, H: 100}, widget.Size{W: 10, H: 10}, widget.Size{W: 1000, H: 1000}
	from, at := widget.Point{X: 50, Y: 50}, widget.Point{X: 80, Y: 70}
	for _, tc := range []struct {
		edges uint32
		want  widget.Size
	}{
		{xdg.ToplevelResizeEdgeBottom, widget.Size{W: 200, H: 120}},
		{xdg.ToplevelResizeEdgeTop, widget.Size{W: 200, H: 80}},
		{xdg.ToplevelResizeEdgeBottomRight, widget.Size{W: 230, H: 120}},
		{xdg.ToplevelResizeEdgeBottomLeft, widget.Size{W: 170, H: 120}},
		{xdg.ToplevelResizeEdgeRight, widget.Size{W: 230, H: 100}},
	} {
		if got := dragSize(tc.edges, from, at, start, lo, hi); got != tc.want {
			t.Errorf("edges %d: %v, want %v", tc.edges, got, tc.want)
		}
	}
}
