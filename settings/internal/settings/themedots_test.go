package settings

import (
	"testing"

	"github.com/stubbedev/lys/lib/render"
)

// The theme preview's signal dots started where the window does and ran
// down to 7px above the bottom, 22px under the bar. The column is centred
// in the room under the bar.
func TestTheThemePreviewsDotsAreCentredUnderTheBar(t *testing.T) {
	colors := make([]render.Color, 10)
	for i := range colors {
		colors[i] = render.RGB(uint8(10*i), 0, 0)
	}
	signal := colors[6]
	last := colors[9]
	const w, h = 320, 160
	data := make([]byte, render.Stride(w)*h)
	r := render.Rect{W: w, H: h}
	miniDesktop(colors)(render.New(data, render.Stride(w), w, h), r)
	size := max(3, h/8)
	x := w - size - w/16 + size/2
	first, end := -1, -1
	for y := range h {
		i := y*render.Stride(w) + 4*x
		c := render.ColorFromBytes(data[i : i+4])
		if c == signal && first < 0 {
			first = y
		}
		if c == last {
			end = y + 1
		}
	}
	bar := max(4, h/6)
	if first < 0 || end < 0 {
		t.Fatalf("the dots are not at x=%d", x)
	}
	if above, below := first-bar, h-end; above-below > 1 || below-above > 1 {
		t.Errorf("the dots stand %dpx under the bar and %dpx above the bottom", above, below)
	}
}
