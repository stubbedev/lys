package render

import (
	"fmt"
	"testing"
)

// The per-pixel loops solidSpan shortcuts, kept as the reference.

func refPaintGradient(c *Canvas, r Rect, radii Corners, g Gradient) {
	b, rc := c.devBox(r, radii)
	m, ok := g.mapping(b, devScale(c.num, c.denom))
	if !ok {
		return
	}
	b32, rc32 := b.array(), rc
	sp := c.span(b)
	for y := sp.Y; y < sp.Y+sp.H; y++ {
		for x := sp.X; x < sp.X+sp.W; x++ {
			px, py := float32(x)+0.5, float32(y)+0.5
			cov := coverage(sdBoxF(px, py, b32, rc32))
			if cov <= 0 {
				continue
			}
			blendCovF(c, x, y, sampleStops(g.Stops, g.Extend, m.At(px, py)), cov)
		}
	}
}

func refRoundedMask(l *Layer, r Rect, radii Corners, region Rect) {
	b, rc := DeviceBox(r, radii, l.cv.num, l.cv.denom)
	for y := region.Y; y < region.Y+region.H; y++ {
		py := float32(y) + 0.5
		for x := region.X; x < region.X+region.W; x++ {
			cov := cov255(coverage(sdBoxF(float32(x)+0.5, py, b, rc)))
			if cov == 255 {
				continue
			}
			if cov == 0 {
				l.cv.set(x, y, 0)
				continue
			}
			l.cv.set(x, y, coverageScale(l.cv.get(x, y), cov))
		}
	}
}

type boxCase struct {
	r          Rect
	radii      Corners
	num, denom int
	clip       Rect
}

func boxCases() []boxCase {
	var cases []boxCase
	for _, sc := range [][2]int{{1, 1}, {3, 2}, {5, 4}, {2, 1}} {
		for _, r := range []Rect{{X: 3, Y: 2, W: 40, H: 30}, {X: 0, Y: 0, W: 1, H: 1}, {X: 5, Y: 7, W: 2, H: 20}, {X: -4, Y: 3, W: 30, H: 3}} {
			for _, radii := range []Corners{{}, {TopLeft: 4, TopRight: 4, BottomRight: 4, BottomLeft: 4}, {TopLeft: 9, BottomRight: 2}, {TopLeft: 50, TopRight: 50, BottomRight: 50, BottomLeft: 50}} {
				for _, clip := range []Rect{{W: 100, H: 80}, {X: 10, Y: 9, W: 13, H: 11}} {
					cases = append(cases, boxCase{r: r, radii: radii, num: sc[0], denom: sc[1], clip: clip})
				}
			}
		}
	}
	return cases
}

func (bc boxCase) canvas() *Canvas {
	const w, h = 100, 80
	c := NewScaled(make([]byte, Stride(w)*h), Stride(w), w, h, bc.num, bc.denom)
	c.ClearDevice(c.Rect(), RGB(20, 30, 40))
	c.clip = bc.clip
	return c
}

func TestPaintGradientMatchesThePerPixelCoverage(t *testing.T) {
	grads := []Gradient{
		Linear(135, GradientStop{Pos: 0, Color: RGBA(200, 100, 20, 0)}, GradientStop{Pos: 1, Color: RGBA(200, 100, 20, 120)}),
		Radial(GradientStop{Pos: 0, Color: RGB(255, 0, 0)}, GradientStop{Pos: 0.5, Color: RGBA(0, 0, 255, 90)}, GradientStop{Pos: 1, Color: RGB(0, 255, 0)}),
		Conic(30, GradientStop{Pos: 0, Color: RGB(10, 200, 10)}, GradientStop{Pos: 1, Color: RGBA(10, 10, 200, 200)}),
	}
	for i, bc := range boxCases() {
		for j, g := range grads {
			got, want := bc.canvas(), bc.canvas()
			got.PaintGradient(bc.r, bc.radii, g)
			refPaintGradient(want, bc.r, bc.radii, g)
			assertSamePixels(t, fmt.Sprintf("case %d %+v gradient %d", i, bc, j), got, want)
		}
	}
}

func TestRoundedMaskMatchesThePerPixelCoverage(t *testing.T) {
	for i, bc := range boxCases() {
		mk := func() *Layer {
			c := bc.canvas()
			l := c.Layer(nil, c.Rect())
			l.cv.ClearDevice(l.cv.Rect(), RGBA(90, 160, 220, 200))
			l.cv.touched = 0
			return l
		}
		got, want := mk(), mk()
		got.applyRoundedMask(bc.r, bc.radii, bc.clip)
		refRoundedMask(want, bc.r, bc.radii, bc.clip)
		assertSamePixels(t, fmt.Sprintf("case %d %+v", i, bc), got.cv, want.cv)
	}
}

func TestSolidSpanStaysInsideTheFullCoverage(t *testing.T) {
	for i, bc := range boxCases() {
		b, rc := DeviceBox(bc.r, bc.radii, bc.num, bc.denom)
		sp := bc.canvas().Rect()
		solid := solidSpan(b, rc, sp)
		for y := solid.Y; y < solid.Y+solid.H; y++ {
			for x := solid.X; x < solid.X+solid.W; x++ {
				if cov := coverage(sdBoxF(float32(x)+0.5, float32(y)+0.5, b, rc)); cov != 1 {
					t.Fatalf("case %d %+v: (%d,%d) in the solid span %+v has coverage %v", i, bc, x, y, solid, cov)
				}
			}
		}
		square := bc.radii == Corners{} && bc.r.W > 2 && bc.r.H > 2 && bc.num == bc.denom
		if square && solid != sp.Intersect(bc.r) {
			t.Errorf("case %d: a square box's solid span is %+v, want all of it, %+v", i, solid, sp.Intersect(bc.r))
		}
	}
}

func TestSolidSpanOfABoxTooThinIsEmpty(t *testing.T) {
	b := [4]float32{4, 4, 7, 40}
	if got := solidSpan(b, [4]float32{2, 2, 2, 2}, Rect{W: 100, H: 100}); !got.Empty() {
		t.Errorf("a 3 px wide box with 2 px corners has the solid span %+v, want none", got)
	}
}
