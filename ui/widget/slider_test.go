package widget

import (
	"testing"

	"github.com/stubbedev/lys/lib/render"
)

func TestSliderKeyboard(t *testing.T) {
	t.Run("arrows nudge by the step", func(t *testing.T) {
		s := NewSlider(0, 100, 5, 50)
		s.KeyAction(KeyRight, 0)
		if got := s.Value(); got != 55 {
			t.Errorf("after right = %v, want 55", got)
		}
		s.KeyAction(KeyLeft, 0)
		s.KeyAction(KeyLeft, 0)
		if got := s.Value(); got != 45 {
			t.Errorf("after lefts = %v, want 45", got)
		}
	})

	t.Run("home and end jump to the extremes", func(t *testing.T) {
		s := NewSlider(10, 20, 0, 15)
		s.KeyAction(KeyHome, 0)
		if got := s.Value(); got != 10 {
			t.Errorf("home = %v, want 10", got)
		}
		s.KeyAction(KeyEnd, 0)
		if got := s.Value(); got != 20 {
			t.Errorf("end = %v, want 20", got)
		}
	})

	t.Run("without a step a tenth of the range nudges", func(t *testing.T) {
		s := NewSlider(0, 100, 0, 50)
		s.KeyAction(KeyRight, 0)
		if got := s.Value(); got != 60 {
			t.Errorf("after right = %v, want 60", got)
		}
	})
}

// The slider takes the reference's proportions: a 6px track under a
// 16px thumb, white with a primary ring in both palettes. The track
// was 4px, the thumb 14px and filled with the window background, a
// dark ring on the dark palette (#281).
func TestSliderTrackAndThumb(t *testing.T) {
	prev := Current()
	t.Cleanup(func() { SetTheme(prev) })
	for _, th := range []*Theme{DarkTheme(), LightTheme()} {
		SetTheme(th)
		s := NewSlider(0, 100, 1, 50)
		arrangeTree(t, s, 200, 40)
		if tr := s.troughRect(); tr.H != 6 {
			t.Errorf("track %v, want 6px thick", tr)
		}
		if w, h := s.knobSize(); w != 16 || h != 16 {
			t.Errorf("thumb %dx%d, want 16x16", w, h)
		}
		if bg := s.trough.knob.style(&s.trough.knob).Background; bg != render.RGB(255, 255, 255) {
			t.Errorf("dark=%v: thumb fill %#x, want white", th.IsDark(), uint32(bg))
		}
	}
}

// A track paints the trough's colours from the minimum's end to the
// maximum's, with no highlight up to the knob; cleared, inverted or
// disabled, the slider paints what it painted before.
func TestSliderTrackPaintsAGradient(t *testing.T) {
	prev := Current()
	t.Cleanup(func() { SetTheme(prev) })
	SetTheme(DarkTheme())
	red, blue := render.RGB(255, 0, 0), render.RGB(0, 0, 255)
	const w, h = 200, 40
	ends := func(s *Slider) (render.Color, render.Color) {
		data := paintPixels(t, s, w, h)
		tr := s.troughRect()
		y := tr.Y + tr.H/2
		return pxAt(data, w, tr.X+3, y), pxAt(data, w, tr.X+tr.W-4, y)
	}
	reddish := func(c render.Color) bool { return c.R() > 200 && c.B() < 60 }
	bluish := func(c render.Color) bool { return c.B() > 200 && c.R() < 60 }

	s := NewSlider(0, 100, 1, 50)
	plainLo, plainHi := ends(s)
	s.SetTrack(red, blue)
	lo, hi := ends(s)
	if !reddish(lo) || !bluish(hi) {
		t.Errorf("track ends %#x .. %#x, want red .. blue", uint32(lo), uint32(hi))
	}
	s.SetInverted(true)
	if lo, hi := ends(s); !bluish(lo) || !reddish(hi) {
		t.Errorf("inverted track ends %#x .. %#x, want blue .. red", uint32(lo), uint32(hi))
	}
	s.SetInverted(false)
	s.SetEnabled(false)
	if lo, _ := ends(s); reddish(lo) {
		t.Errorf("disabled track still paints %#x", uint32(lo))
	}
	s.SetEnabled(true)
	s.SetTrack()
	if lo, hi := ends(s); lo != plainLo || hi != plainHi {
		t.Errorf("cleared track %#x .. %#x, want the plain %#x .. %#x", uint32(lo), uint32(hi), uint32(plainLo), uint32(plainHi))
	}
}
