package cavaview

import (
	"testing"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/desktop/config"
)

var (
	red   = render.RGB(0xff, 0, 0)
	black = render.RGB(0, 0, 0)
)

// paintSpectrum paints one frame of s into a w x h canvas and returns
// a pixel reader.
func paintSpectrum(s *Spectrum, w, h int, values ...float64) func(x, y int) render.Color {
	widget.SizeWithin(s, widget.Size{W: 1000, H: 1000})
	s.Arrange(render.Rect{W: w, H: h})
	data := make([]byte, render.Stride(w)*h)
	cv := render.New(data, render.Stride(w), w, h)
	cv.Clear(cv.Rect(), black)
	if values != nil {
		s.SetFrame(values)
	}
	s.Paint(cv)
	return func(x, y int) render.Color {
		start := y*render.Stride(w) + x*4
		return render.ColorFromBytes(data[start : start+4])
	}
}

func testConfig(bars int) config.CavaConfig {
	cfg := config.DefaultsCava()
	cfg.Bars = config.BarCount(bars)
	cfg.BarWidth = 4
	cfg.BarGap = 2
	cfg.InternalPadding = config.Size{}
	return cfg
}

func TestLength(t *testing.T) {
	cfg := testConfig(10)
	cfg.BarWidth, cfg.BarGap = 6, 1
	cfg.InternalPadding = config.Size{Value: 8, Unit: config.SizePixels}
	s := NewSpectrum(cfg, 10, red, 1, false)
	if s.length != 2*8+10*6+9*1 {
		t.Errorf("length = %d, want %d", s.length, 2*8+10*6+9*1)
	}
	got := widget.SizeWithin(s, widget.Size{W: 1000, H: 1000})
	if got != (widget.Size{W: s.length, H: s.thick}) || s.Measure(widget.Row, widget.Unknown).Min != s.length {
		t.Errorf("horizontal size = %+v", got)
	}
	v := NewSpectrum(cfg, 10, red, 1, true)
	if got := widget.SizeWithin(v, widget.Size{W: 1000, H: 1000}); got != (widget.Size{W: v.thick, H: v.length}) {
		t.Errorf("vertical size = %+v, want the length along the height", got)
	}
	if widgetLength(1, 3, 1, 0) != 3 || widgetLength(20, 3, 1, 8) != 95 || widgetLength(0, 3, 1, 0) != 1 {
		t.Error("calculate_widget_length")
	}
}

func TestPaintsBarsByDirection(t *testing.T) {
	for _, tc := range []struct {
		dir           config.CavaDirection
		top, mid, bot render.Color // probes at y 1, 7, 18 of a 20px canvas
	}{
		{config.CavaDirectionNormal, black, black, red}, // 10..20
		{config.CavaDirectionReverse, red, red, black},  // 0..10
		{config.CavaDirectionMirror, black, red, black}, // 5..15
	} {
		cfg := testConfig(2)
		cfg.Direction = tc.dir
		px := paintSpectrum(NewSpectrum(cfg, 2, red, 1, false), 30, 20, 0.5, 0.5)
		if px(1, 1) != tc.top || px(1, 7) != tc.mid || px(1, 18) != tc.bot {
			t.Errorf("%s: probes %#08x %#08x %#08x", tc.dir, px(1, 1), px(1, 7), px(1, 18))
		}
		if px(5, 18) != black && tc.dir == config.CavaDirectionNormal {
			t.Error("the gap between bars was painted")
		}
	}
}

func TestSilentBarsKeepTheirMinimum(t *testing.T) {
	px := paintSpectrum(NewSpectrum(testConfig(1), 1, red, 1, false), 10, 20, 0)
	if px(1, 19) != red || px(1, 18) != red || px(1, 17) != black {
		t.Error("a silent bar must Still draw the minimum bar height (2px)")
	}
	// Before the first frame nothing paints.
	empty := NewSpectrum(testConfig(0), 0, red, 1, false)
	if px := paintSpectrum(empty, 10, 20); px(1, 19) != black {
		t.Error("an empty frame painted")
	}
}

func TestPadsOnlyAlongTheBar(t *testing.T) {
	cfg := testConfig(1)
	cfg.InternalPadding = config.Size{Value: 5, Unit: config.SizePixels}
	px := paintSpectrum(NewSpectrum(cfg, 1, red, 1, false), 20, 20, 1)
	if px(2, 10) != black || px(6, 10) != red {
		t.Error("the bar does not start after the padding")
	}
	if px(6, 0) != red || px(6, 19) != red {
		t.Error("a full bar must run the whole height: the padding is only along the bar")
	}
}

func TestPeakCapsFall(t *testing.T) {
	cfg := testConfig(1)
	cfg.BarWidth = 10
	cfg.Style = config.CavaStylePeaks
	s := NewSpectrum(cfg, 1, red, 1, false)
	paintSpectrum(s, 20, 40, 0.9)
	px := paintSpectrum(s, 20, 40, 0.1)
	if s.frame.peaks[0] != 0.9-peakGravity {
		t.Errorf("peak = %v, want 0.9 less one frame of gravity", s.frame.peaks[0])
	}
	// The cap sits just above the fallen peak, 40*(1-0.885) = 4.6px down.
	if px(2, 3) != red || px(2, 10) != black {
		t.Errorf("cap probes %#08x / %#08x", px(2, 3), px(2, 10))
	}
	bars := NewSpectrum(testConfig(1), 1, red, 1, false)
	paintSpectrum(bars, 20, 40, 0.9)
	if px := paintSpectrum(bars, 20, 40, 0.1); px(2, 3) != black {
		t.Error("the bars style drew a peak cap")
	}
}

func TestWave(t *testing.T) {
	for _, tc := range []struct {
		dir           config.CavaDirection
		top, mid, bot render.Color
	}{
		{config.CavaDirectionNormal, black, black, red},
		{config.CavaDirectionReverse, red, black, black},
		{config.CavaDirectionMirror, black, red, black},
	} {
		cfg := testConfig(3)
		cfg.Style = config.CavaStyleWave
		cfg.Direction = tc.dir
		px := paintSpectrum(NewSpectrum(cfg, 3, red, 1, false), 30, 40, 0.4, 0.4, 0.4)
		if px(15, 2) != tc.top || px(15, 20) != tc.mid || px(15, 37) != tc.bot {
			t.Errorf("%s wave probes %#08x %#08x %#08x", tc.dir, px(15, 2), px(15, 20), px(15, 37))
		}
	}
	// A peak in the middle rises above its flat neighbors.
	cfg := testConfig(3)
	cfg.Style = config.CavaStyleWave
	px := paintSpectrum(NewSpectrum(cfg, 3, red, 1, false), 30, 40, 0.1, 0.9, 0.1)
	if px(15, 10) != red || px(1, 10) != black {
		t.Error("the curve does not follow the values")
	}
}

func TestVerticalTurnsAQuarter(t *testing.T) {
	cfg := testConfig(2)
	s := NewSpectrum(cfg, 2, red, 1, true)
	// On a vertical bar the first bar sits at the bottom and grows from
	// the right edge leftward (translate to the bottom, rotate -90°).
	px := paintSpectrum(s, 20, 30, 0.5, 0)
	if px(15, 28) != red || px(5, 28) != black {
		t.Errorf("first bar probes %#08x / %#08x", px(15, 28), px(5, 28))
	}
	if px(15, 23) != black || px(19, 23) != red {
		t.Error("the second (silent) bar must be its 2px minimum at the right edge")
	}
}

func TestBarsSplitForStereo(t *testing.T) {
	cfg := config.DefaultsCava()
	cfg.Bars = 21
	if b, ch := Bars(cfg); b != 21 || ch != 1 {
		t.Errorf("mono = %d/%d", b, ch)
	}
	cfg.Stereo = true
	if b, ch := Bars(cfg); b != 22 || ch != 2 {
		t.Errorf("stereo of 21 = %d/%d, want 22 over 2 channels", b, ch)
	}
	cfg.Bars = 20
	if b, _ := Bars(cfg); b != 20 {
		t.Errorf("stereo of 20 = %d", b)
	}
}

func TestIgnoresChangesBelowAPixel(t *testing.T) {
	s := NewSpectrum(testConfig(1), 1, red, 1, false)
	if !s.SetFrame([]float64{0.5}) {
		t.Fatal("a real change reported no movement")
	}
	if s.SetFrame([]float64{0.5 + Still/2}) {
		t.Error("a sub-pixel change asked for a paint")
	}
	if !s.SetFrame([]float64{0.5 + Still}) {
		t.Error("the drift never added up to a paint")
	}
	peaks := testConfig(1)
	peaks.Style = config.CavaStylePeaks
	p := NewSpectrum(peaks, 1, red, 1, false)
	p.SetFrame([]float64{0.9})
	if !p.SetFrame([]float64{0.9 - 0.3}) {
		t.Error("a fallen bar did not paint")
	}
	if !p.SetFrame([]float64{0.9 - 0.3}) {
		t.Error("a falling cap must keep painting while the bar holds Still")
	}
}
