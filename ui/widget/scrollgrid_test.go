package widget

import (
	"testing"

	"github.com/stubbedev/lys/lib/render"
)

func TestAScrollStepAtAFractionalScaleMovesItsPixels(t *testing.T) {
	for _, num := range []int{150, 180, 210} {
		sc, _ := longScroll(t, 400)
		moves, painted := movedFrame(t, sc, 400, 300, num, func() { sc.ScrollPixels(0, 7.3) })
		if len(moves) != 1 {
			t.Fatalf("scale %d/120: moves %+v, want the step copied", num, moves)
		}
		if dy := moves[0].DY; dy*num%120 != 0 {
			t.Errorf("scale %d/120: moved %d logical px, between device pixels", num, dy)
		}
		if full := 400 * 300 * num * num / 120 / 120; painted*4 > full {
			t.Errorf("scale %d/120: repainted %d of %d device px", num, painted, full)
		}
	}
}

func TestScrollOffsetsKeepToTheDeviceGrid(t *testing.T) {
	sc, _ := longScroll(t, 400)
	SetDeviceScale(sc, 180, 120)
	r := render.Rect{W: 400, H: 300}
	SizeWithin(sc, Size{W: r.W, H: r.H})
	sc.Arrange(r)
	moved := 0.0
	for range 25 {
		sc.ScrollPixels(0, 1)
		moved++
		_, y := sc.Offset()
		if y%2 != 0 {
			t.Fatalf("offset %d at scale 1.5 is between device pixels", y)
		}
		if float64(y) > moved || moved-float64(y) >= 2 {
			t.Fatalf("after %v px of motion the offset is %d: the remainder must carry to the next step", moved, y)
		}
	}
	sc.SetOffset(0, 1<<20)
	_, end := sc.Offset()
	if want := sc.AxisRange(Column).Total - sc.AxisRange(Column).Page; end != want {
		t.Errorf("scrolled to the end at %d, want the content's end %d whatever grid it falls on", end, want)
	}
	sc.SetOffset(0, 37)
	if _, y := sc.Offset(); y != 38 && y != 36 {
		t.Errorf("SetOffset(37) at scale 1.5 landed on %d, want the nearest grid step", y)
	}
}

func TestScrollOffsetsAtAWholeScaleMoveByThePixel(t *testing.T) {
	sc, _ := longScroll(t, 400)
	SetDeviceScale(sc, 240, 120)
	r := render.Rect{W: 400, H: 300}
	SizeWithin(sc, Size{W: r.W, H: r.H})
	sc.Arrange(r)
	for want := 1; want <= 5; want++ {
		sc.ScrollPixels(0, 1)
		if _, y := sc.Offset(); y != want {
			t.Fatalf("offset %d at scale 2, want %d", y, want)
		}
	}
}
