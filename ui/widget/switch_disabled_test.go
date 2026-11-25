package widget

import (
	"testing"

	"github.com/stubbedev/lys/lib/render"
)

// A disabled switch fades without changing its colors: off, its knob
// stays lighter or darker than its track as an enabled one's does,
// on either palette (#270).
func TestADisabledSwitchKeepsItsKnobAgainstItsTrack(t *testing.T) {
	sum := func(c render.Color) int { return int(c.R()) + int(c.G()) + int(c.B()) }
	for _, th := range []*Theme{DarkTheme(), LightTheme()} {
		SetTheme(th)
		knobOverTrack := func(enabled bool) int {
			s := NewSwitch(false)
			s.SetEnabled(enabled)
			sz := SizeWithin(s, Size{W: 200, H: 100})
			s.Arrange(render.Rect{W: sz.W, H: sz.H})
			stride := render.Stride(sz.W)
			data := make([]byte, stride*sz.H)
			cv := render.New(data, stride, sz.W, sz.H)
			cv.Clear(cv.Rect(), th.Bg)
			s.Paint(cv)
			k := s.knob.Bounds()
			knob := render.ColorFromBytes(data[(k.Y+k.H/2)*stride+(k.X+k.W/2)*4:])
			track := render.ColorFromBytes(data[(sz.H/2)*stride+(sz.W-sz.H/2)*4:])
			return sum(knob) - sum(track)
		}
		on, off := knobOverTrack(true), knobOverTrack(false)
		if on == 0 || off == 0 || (on > 0) != (off > 0) {
			t.Errorf("dark %v: the off knob is %+d over its track enabled, %+d disabled; want the same side", th.IsDark(), on, off)
		}
	}
	SetTheme(nil)
}
