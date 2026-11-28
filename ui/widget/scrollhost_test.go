package widget

import (
	"bytes"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/render"
)

// movedFrame lays root out and paints it whole at w x h logical pixels
// and device scale num/120, runs change, and draws the next frame as a
// window draws it: each move at a whole device step copied in place,
// any other owed whole, and the owed rects repainted. It fails unless
// that frame matches root painted whole afresh, and returns the moves
// and the device pixels it repainted.
func movedFrame(t *testing.T, root Widget, w, h, num int, change func()) ([]ScrollMove, int) {
	t.Helper()
	dw, dh := w*num/120, h*num/120
	stride := render.Stride(dw)
	bg := Current().Bg
	paint := func(cv *render.Canvas, regions []render.Rect) int {
		painted := 0
		for _, region := range regions {
			prev := cv.PushClipDevice(region)
			cv.ClearDevice(region, bg)
			cv.BeginOverlays()
			root.Paint(cv)
			cv.FlushOverlays()
			cv.PopClip(prev)
			painted += region.W * region.H
		}
		return painted
	}
	layout := func() {
		SetDeviceScale(root, num, 120)
		for range 2 {
			SizeWithin(root, Size{W: w, H: h})
			root.Arrange(render.Rect{W: w, H: h})
		}
	}
	full := []render.Rect{{W: dw, H: dh}}
	buf := make([]byte, stride*dh)
	cv := render.NewScaled(buf, stride, dw, dh, num, 120)
	var damage DamageCollector
	layout()
	damage.CollectFrame(root)
	paint(cv, full)

	change()
	layout()
	rects, moves, _ := damage.CollectFrame(root)
	moves = append([]ScrollMove(nil), moves...)
	var owed []render.Rect
	for _, m := range moves {
		view := render.MapRect(m.View, num, 120)
		if m.DX*num%120 != 0 || m.DY*num%120 != 0 {
			owed = append(owed, view)
			continue
		}
		moveDevice(buf, stride, view, m.DX*num/120, m.DY*num/120)
	}
	for _, r := range rects {
		if d := render.MapRect(r, num, 120).Intersect(full[0]); !d.Empty() {
			owed = append(owed, d)
		}
	}
	painted := paint(cv, owed)

	want := make([]byte, stride*dh)
	paint(render.NewScaled(want, stride, dw, dh, num, 120), full)
	if !bytes.Equal(buf, want) {
		for i := range buf {
			if buf[i] != want[i] {
				px := i / 4
				t.Fatalf("the moved frame differs from a whole repaint first at device (%d, %d); moves %+v, owed %+v", px%dw, px/dw, moves, owed)
			}
		}
	}
	return moves, painted
}

// moveDevice copies area's pixels in buf by dx, dy, dropping what falls
// outside area, as a window's buffer copy does.
func moveDevice(buf []byte, stride int, area render.Rect, dx, dy int) {
	to := render.Rect{X: area.X + dx, Y: area.Y + dy, W: area.W, H: area.H}.Intersect(area)
	if to.Empty() {
		return
	}
	for i := range to.H {
		y := to.Y + i
		if dy > 0 {
			y = to.Y + to.H - 1 - i
		}
		at := y*stride + to.X*4
		from := (y-dy)*stride + (to.X-dx)*4
		copy(buf[at:at+to.W*4], buf[from:from+to.W*4])
	}
}

// pageScroll is a settings page's shape: a clamped column of items in a
// scroll, shown in a stack under an overlay that floats a button over
// the page's top right corner.
func pageScroll(t *testing.T) (root Widget, sc *Scroll, stack *Stack, items []*Item) {
	t.Helper()
	col, items := itemColumn(t, 40)
	sc = NewScroll(NewClamp(500, col))
	sc.VerticalOnly = true
	stack = NewStack()
	stack.Add("page", sc)
	stack.Add("other", NewBox(Column, 0, 0))
	stack.Show("page")
	close := NewButton(NewLabel(entryFace(t), 14, "x", 0), 4, 4)
	return NewOverlay().Append(stack).AppendAligned(close, AlignEnd, AlignStart), sc, stack, items
}

func itemColumn(t *testing.T, rows int) (*Box, []*Item) {
	t.Helper()
	sc, items := itemScroll(t, rows)
	col, ok := sc.child.(*Box)
	if !ok {
		t.Fatalf("itemScroll's child is a %T", sc.child)
	}
	sc.SetChild(nil)
	return col, items
}

func TestAScrolledPageMovesUnderItsHosts(t *testing.T) {
	face := entryFace(t)
	for name, build := range map[string]func(t *testing.T) (Widget, *Scroll){
		"a stack under an overlay with a floating button": func(t *testing.T) (Widget, *Scroll) {
			root, sc, _, _ := pageScroll(t)
			return root, sc
		},
		"a sidebar beside it": func(t *testing.T) (Widget, *Scroll) {
			root, sc, _, _ := pageScroll(t)
			return NewSidebar(face, root), sc
		},
		"a label below repainting on the step": func(t *testing.T) (Widget, *Scroll) {
			root, sc, _, items := pageScroll(t)
			items[0].desc.Invalidate()
			return root, sc
		},
	} {
		t.Run(name, func(t *testing.T) {
			root, sc := build(t)
			moves, painted := movedFrame(t, root, 640, 360, 120, func() { sc.SetOffset(0, 30) })
			if len(moves) != 1 || moves[0].DY != -30 {
				t.Fatalf("moves %+v, want the page moved up by 30", moves)
			}
			if limit := 640 * 360 / 4; painted > limit {
				t.Errorf("repainted %d px, want the exposed strip and the button, at most %d", painted, limit)
			}
		})
	}
}

func TestAScrollStepMovesPastAnEmptyBox(t *testing.T) {
	col, _ := itemColumn(t, 30)
	col.InsertAt(3, NewBox(Column, 0, 0), false)
	sc := NewScroll(col)
	moves, _ := movedFrame(t, sc, 400, 300, 120, func() { sc.SetOffset(0, 20) })
	if len(moves) != 1 {
		t.Errorf("moves %+v, want the view moved past the empty box", moves)
	}
}

func TestAScrollRepaintsItsViewWhenItCannotMove(t *testing.T) {
	for name, tc := range map[string]struct {
		build  func(t *testing.T) (Widget, *Scroll, func())
		reason string
	}{
		"the stack switching pages": {func(t *testing.T) (Widget, *Scroll, func()) {
			root, sc, stack, _ := pageScroll(t)
			stack.SetTransition(StackCrossfade, time.Second)
			return root, sc, func() {
				stack.Show("other")
				stack.Show("page")
			}
		}, "a switch paints the page through a layer"},
		"a row changing its layout": {func(t *testing.T) (Widget, *Scroll, func()) {
			root, sc, _, items := pageScroll(t)
			return root, sc, func() { items[2].SetDescription("Short.") }
		}, "the column lays out again"},
		"content hanging paint past the clip": {func(t *testing.T) (Widget, *Scroll, func()) {
			root, sc, _, items := pageScroll(t)
			items[1].AddTrailing(&escaping{})
			return root, sc, func() {}
		}, "the copy would carry what the clip does not hold"},
	} {
		t.Run(name, func(t *testing.T) {
			root, sc, before := tc.build(t)
			moves, _ := movedFrame(t, root, 640, 360, 120, func() {
				before()
				sc.SetOffset(0, 30)
			})
			if len(moves) != 0 {
				t.Errorf("moved %+v, want the view repainted: %s", moves, tc.reason)
			}
		})
	}
}

// escaping paints a band on the frame's top layer, past any clip, as an
// open dropdown hangs its list.
type escaping struct{ Base }

func (e *escaping) Measure(Axis, int) Sizes { return FixedSizes(10) }

func (e *escaping) Paint(cv *render.Canvas) {
	b := e.Bounds()
	cv.Overlay(func(cv *render.Canvas) {
		cv.FillRect(render.Rect{X: b.X, Y: b.Y + b.H, W: 40, H: 200}, render.RGB(200, 0, 0))
	})
}

func (e *escaping) HitTest(Point) Widget { return nil }
