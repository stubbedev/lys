package settings

import (
	"testing"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

// trackLuma is c's Rec. 709 luma.
func trackLuma(c render.Color) float64 {
	return 0.2126*float64(c.R()) + 0.7152*float64(c.G()) + 0.0722*float64(c.B())
}

// pixelOf is the premultiplied color w painted, laid out in r, at (x, y).
func pixelOf(w widget.Widget, r render.Rect, x, y int) render.Color {
	stride := render.Stride(r.W)
	data := make([]byte, stride*r.H)
	w.Arrange(r)
	w.Paint(render.New(data, stride, r.W, r.H))
	i := y*stride + 4*x
	return render.RGBA(data[i+2], data[i+1], data[i], data[i+3])
}

// The thresholds and range sliders drew their own track, a field's fill
// that all but vanished on the card, and hollow ring thumbs, unlike
// every other slider. Their track and thumbs paint as lys-ui's slider
// does, in the same place of the same theme.
func TestThresholdAndRangeSlidersPaintAsTheSlider(t *testing.T) {
	k := testKit(t, "")
	bar := newThresholdBar(k, slot{get: func() any { return nil }, set: func(any) error { return nil }, unset: func() {}}, nil)
	ref := widget.NewSlider(0, 100, 1, 0)
	host := themed(k, gapBox(widget.Column))
	host.Append(bar, false)
	host.Append(ref, false)
	r := render.Rect{W: 320, H: bar.barH()}
	bar.Arrange(r)
	tr := sliderTrough(bar.look, bar.track())
	x, y := tr.X+tr.W*3/4, tr.Y+tr.H/2
	ref.Arrange(render.Rect{W: 320, H: r.H})
	refY := ref.Bounds().Y + ref.Bounds().H/2
	if got, want := pixelOf(bar, r, x, y), pixelOf(ref, render.Rect{W: 320, H: r.H}, x, refY); got != want {
		t.Errorf("the threshold track paints %08x, a slider's trough %08x", uint32(got), uint32(want))
	}
}
