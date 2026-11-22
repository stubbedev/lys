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

// The theme's slider is a 6px track under an 18px white thumb in both
// palettes. The thumb carries no border: a hairline ring of shadow and
// a drop shadow set it apart, so even on a pale accent fill a darker
// ring runs between the fill and the thumb, where an accent ring
// blended into the fill.
func TestSliderTrackAndThumb(t *testing.T) {
	prev := Current()
	t.Cleanup(func() { SetTheme(prev) })
	for _, th := range []*Theme{DarkTheme(), LightTheme()} {
		th.Accent = render.RGB(0xbd, 0xc2, 0xff)
		SetTheme(th)
		s := NewSlider(0, 100, 1, 50)
		const w, h = 200, 40
		data := paintPixels(t, s, w, h)
		tr := s.troughRect()
		if tr.H != 6 {
			t.Errorf("track %v, want 6px thick", tr)
		}
		kw, kh := s.knobSize()
		if kw != 18 || kh != 18 {
			t.Errorf("thumb %dx%d, want 18x18", kw, kh)
		}
		if bg := s.trough.knob.style(&s.trough.knob).Background; bg != render.RGB(255, 255, 255) {
			t.Errorf("dark=%v: thumb fill %#x, want white", th.IsDark(), uint32(bg))
		}
		origin, span := s.along(tr)
		at := origin + span/2
		y := tr.Y + tr.H/2
		fill := luma(pxAt(data, w, origin+4, y))
		ring := 255
		for x := at - kw/2 - 2; x <= at-kw/2; x++ {
			ring = min(ring, luma(pxAt(data, w, x, y)))
		}
		if ring+48 > fill {
			t.Errorf("dark=%v: between the fill (luma %d) and the thumb the darkest pixel has luma %d, want a ring well darker than the fill", th.IsDark(), fill, ring)
		}
	}
}

func luma(c render.Color) int { return (int(c.R())*299 + int(c.G())*587 + int(c.B())*114) / 1000 }

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

// A new range clamps the value into it, saying so only when it moved,
// and the ends it reports are the new ones.
func TestSliderSetRangeClampsTheValue(t *testing.T) {
	s := NewSlider(0, 100, 0, 80)
	var heard []float64
	s.OnChanged = func(v float64) { heard = append(heard, v) }
	s.SetRange(0, 200)
	if lo, hi := s.Range(); lo != 0 || hi != 200 || s.Value() != 80 || len(heard) != 0 {
		t.Errorf("widened to [%v, %v]: value %v, heard %v; want 80 kept unannounced", lo, hi, s.Value(), heard)
	}
	s.SetRange(0, 50)
	if s.Value() != 50 || len(heard) != 1 || heard[0] != 50 {
		t.Errorf("narrowed below the value: value %v, heard %v; want 50 announced once", s.Value(), heard)
	}
}
