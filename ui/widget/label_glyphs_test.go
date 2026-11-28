package widget

import (
	"testing"

	"github.com/stubbedev/lys/lib/render"
)

func inkBox(pix []byte, w, h int) (render.Rect, bool) {
	x0, y0, x1, y1 := w, h, -1, -1
	for y := range h {
		for x := range w {
			if pix[4*(y*w+x)+3] != 0 {
				x0, x1, y0, y1 = min(x0, x), max(x1, x), min(y0, y), max(y1, y)
			}
		}
	}
	return render.Rect{X: x0, Y: y0, W: x1 - x0 + 1, H: y1 - y0 + 1}, x1 >= 0
}

func TestPaintGlyphsDrawsOnlyTheGlyphsInOneColorShifted(t *testing.T) {
	face := goldenFace(t)
	const w, h = 240, 80
	loadCSS(t, "label { color: red; text-shadow: 0 0 6px blue; text-decoration: underline; background-color: green; }"+
		"label::first-letter { color: yellow; }")
	label := NewLabel(face, 24, "Glyphs", 0)
	SizeWithin(label, Size{W: w, H: h})
	label.Arrange(render.Rect{X: 100, Y: 50, W: 120, H: 40})

	pix := make([]byte, render.Stride(w)*h)
	label.PaintGlyphs(render.New(pix, render.Stride(w), w, h), render.RGB(255, 255, 255), -100, -50)
	box, ok := inkBox(pix, w, h)
	if !ok {
		t.Fatal("PaintGlyphs drew nothing")
	}
	for i := 0; i < len(pix); i += 4 {
		if pix[i] != pix[i+3] || pix[i+1] != pix[i+3] || pix[i+2] != pix[i+3] {
			t.Fatalf("pixel %d is %v, not the white ink: a shadow, background or first-letter color drew", i/4, pix[i:i+4])
		}
	}
	if box.X >= 120 || box.Y >= 40 || box.X+box.W > 120 || box.Y+box.H > 40 {
		t.Errorf("the glyphs landed at %v, outside the label's box shifted to the origin", box)
	}

	ref := make([]byte, render.Stride(w+100)*(h+50))
	loadCSS(t, "label { color: white; }")
	label.Paint(render.New(ref, render.Stride(w+100), w+100, h+50))
	want, _ := inkBox(ref, w+100, h+50)
	want.X -= 100
	want.Y -= 50
	if box != want {
		t.Errorf("the glyphs cover %v, Paint's text covers %v once shifted", box, want)
	}
}

func TestPaintGlyphsOfAnEmptyLabelDrawsNothing(t *testing.T) {
	label := NewLabel(goldenFace(t), 24, "", 0)
	SizeWithin(label, Size{W: 100, H: 40})
	label.Arrange(render.Rect{W: 100, H: 40})
	pix := make([]byte, render.Stride(100)*40)
	label.PaintGlyphs(render.New(pix, render.Stride(100), 100, 40), render.RGB(255, 255, 255), 0, 0)
	if _, ok := inkBox(pix, 100, 40); ok {
		t.Error("an empty label drew glyphs")
	}
}
