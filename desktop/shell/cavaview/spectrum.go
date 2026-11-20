// Package cavaview draws the cava module's visualizer from its config:
// the bars, the wave or the bars with falling caps that lys-shell paints
// from each analyzer frame, and that lys-settings previews from a draft
// with a sample frame.
package cavaview

import (
	"log"
	"math"
	"slices"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/styling"
)

// heightRem is the visualizer's natural thickness (the icon-lg
// token, $base-icon-lg).
const heightRem = 1.6

// Renderer constants (rendering/*.rs).
const (
	minBarHeight  = 2.0
	minWaveHeight = 2.0
	peakCap       = 2.0
	peakGravity   = 0.015 // per frame
)

// Still is the change in a bar, as a fraction of its height, too small
// to reach a pixel on any bar.
const Still = 1.0 / 1024

// Spectrum paints an analyzer frame: bars, a filled wave, or bars with falling peak caps,
// laid along the bar (rotated a quarter turn on a vertical bar) with
// the internal padding at both ends.
type Spectrum struct {
	widget.Base
	frame    Frame
	dir      config.CavaDirection
	color    render.Color
	width    float64
	gap      float64
	pad      float64
	length   int
	thick    int
	vertical bool
}

// NewSpectrum builds the painter for one resolved cava config with
// bars values a frame, at a bar scale, along a vertical bar or not.
func NewSpectrum(cfg config.CavaConfig, bars int, color render.Color, scale float64, vertical bool) *Spectrum {
	pad := cfg.InternalPadding.ResolvePx(styling.RemBase, scale)
	return &Spectrum{
		frame:    NewFrame(bars, cfg.Style),
		dir:      cfg.Direction,
		color:    color,
		width:    float64(cfg.BarWidth),
		gap:      float64(cfg.BarGap),
		pad:      pad,
		length:   widgetLength(bars, int(cfg.BarWidth), int(cfg.BarGap), pad),
		thick:    int(math.Round(heightRem * styling.RemBase * scale)),
		vertical: vertical,
	}
}

// widgetLength is every bar and gap plus
// the padding at both ends, at least one pixel.
func widgetLength(bars, barWidth, barGap int, padding float64) int {
	total := float64(bars*barWidth) + float64(max(bars-1, 0)*barGap) + float64(2*padding)
	return max(int(math.Round(total)), 1)
}

// Frame is what a frame leaves on screen: the bars and their caps.
type Frame struct {
	values []float64
	peaks  []float64
	style  config.CavaStyle
}

// NewFrame is a silent frame of bars values drawn in style.
func NewFrame(bars int, style config.CavaStyle) Frame {
	return Frame{values: make([]float64, bars), peaks: make([]float64, bars), style: style}
}

// Advance swaps in the latest values and, in the peaks style, lets each cap
// rise to its bar or fall by the gravity. It reports whether the
// frame moved what is on screen; one that did not keeps the painted values, so
// a slow drift still repaints once it adds up.
func (f *Frame) Advance(values []float64) bool {
	moved := false
	for i, v := range values {
		if math.Abs(v-f.values[i]) >= Still {
			moved = true
			break
		}
	}
	if moved {
		copy(f.values, values)
	}
	if f.style == config.CavaStylePeaks {
		for i, v := range f.values {
			if v >= f.peaks[i] {
				f.peaks[i] = v
				continue
			}
			f.peaks[i] = max(f.peaks[i]-peakGravity, 0)
			moved = true
		}
	}
	return moved
}

// SetFrame advances to the frame and asks for a paint when it moved.
func (s *Spectrum) SetFrame(values []float64) bool {
	moved := s.frame.Advance(values)
	if moved {
		s.Invalidate()
	}
	return moved
}

// size is the natural size: the length along the bar, the thickness
// across it.
func (s *Spectrum) size() widget.Size {
	if s.vertical {
		return widget.Size{W: s.thick, H: s.length}
	}
	return widget.Size{W: s.length, H: s.thick}
}

// Measure reports the fixed natural size as its minimum too: a
// visualizer squeezed shorter would misrepresent the bar layout.
func (s *Spectrum) Measure(axis widget.Axis, _ int) widget.Sizes {
	return widget.FixedSizes(s.size().Along(axis))
}

// Arrange records the rect.
func (s *Spectrum) Arrange(r render.Rect) { s.Base.Arrange(r) }

// canvas is the draw func's coordinate space: u along the bar from the
// padding's start, v across it from the top; a vertical bar turns it a
// quarter (translate to the bottom, rotate -90°), so u runs upward and
// v rightward.
type canvasSpace struct {
	r        render.Rect
	vertical bool
	w, h     float64 // the canvas extent along u and v
}

func (s *Spectrum) canvas() canvasSpace {
	r := s.Bounds()
	c := canvasSpace{r: r, vertical: s.vertical, w: float64(r.W), h: float64(r.H)}
	if s.vertical {
		c.w, c.h = float64(r.H), float64(r.W)
	}
	return c
}

// point maps a canvas point to the widget's logical pixels.
func (c canvasSpace) point(u, v float64) (float64, float64) {
	if c.vertical {
		return float64(c.r.X) + v, float64(c.r.Y) + c.w - u
	}
	return float64(c.r.X) + u, float64(c.r.Y) + v
}

// fillRect fills a canvas rect as a path, so fractional bar geometry
// lands as cairo would put it.
func (c canvasSpace) fillRect(cv *render.Canvas, u, v, w, h float64, col render.Color) {
	var p render.Path
	p.MoveTo(c.point(u, v))
	p.LineTo(c.point(u+w, v))
	p.LineTo(c.point(u+w, v+h))
	p.LineTo(c.point(u, v+h))
	p.Close()
	cv.FillPath(&p, col)
}

func barOrigin(dir config.CavaDirection, barHeight, canvasHeight float64) float64 {
	switch dir {
	case config.CavaDirectionReverse:
		return 0
	case config.CavaDirectionMirror:
		return (canvasHeight - barHeight) / 2
	}
	return canvasHeight - barHeight
}

// Paint is the draw func: nothing before the first frame, then the
// configured style.
func (s *Spectrum) Paint(cv *render.Canvas) {
	if len(s.frame.values) == 0 {
		return
	}
	c := s.canvas()
	switch s.frame.style {
	case config.CavaStyleWave:
		s.paintWave(cv, c)
	default:
		s.paintBars(cv, c)
	}
}

// paintBars paints the bars, with falling caps in the peaks style.
func (s *Spectrum) paintBars(cv *render.Canvas, c canvasSpace) {
	stride := s.width + s.gap
	for i, amp := range s.frame.values {
		u := s.pad + float64(float64(i)*stride)
		h := math.Min(math.Max(amp*c.h, minBarHeight), c.h)
		c.fillRect(cv, u, barOrigin(s.dir, h, c.h), s.width, h, s.color)
		if s.frame.style != config.CavaStylePeaks {
			continue
		}
		peak := float64(s.frame.peaks[i] * c.h)
		if peak <= h {
			continue
		}
		capH := math.Min(peakCap, c.h)
		switch s.dir {
		case config.CavaDirectionReverse:
			c.fillRect(cv, u, peak, s.width, capH, s.color)
		case config.CavaDirectionMirror:
			center := float64(c.h / 2)
			c.fillRect(cv, u, center-float64(peak/2)-capH, s.width, capH, s.color)
			c.fillRect(cv, u, center+float64(peak/2), s.width, capH, s.color)
		default:
			c.fillRect(cv, u, c.h-peak-capH, s.width, capH, s.color)
		}
	}
}

// paintWave is a curve through each value, the control
// points halfway between neighbors, closed along the base (or, mirrored,
// back along its reflection).
func (s *Spectrum) paintWave(cv *render.Canvas, c canvasSpace) {
	n := len(s.frame.values)
	width := math.Max(c.w-float64(2*s.pad), 0)
	spacing := width
	if n > 1 {
		spacing = width / float64(n-1)
	}
	minAmp := minWaveHeight / c.h
	y := func(amp float64) float64 {
		amp = math.Max(amp, minAmp)
		switch s.dir {
		case config.CavaDirectionReverse:
			return float64(c.h * amp)
		case config.CavaDirectionMirror:
			return float64(c.h * (1 - amp) / 2)
		}
		return float64(c.h * (1 - amp))
	}
	var p render.Path
	at := func(u, v float64) (float64, float64) { return c.point(s.pad+u, v) }
	p.MoveTo(at(0, y(s.frame.values[0])))
	curve := func(u0, v0, u1, v1 float64) {
		mid := float64((u0 + u1) / 2)
		x1, y1 := at(mid, v0)
		x2, y2 := at(mid, v1)
		x3, y3 := at(u1, v1)
		p.CubeTo(x1, y1, x2, y2, x3, y3)
	}
	for i := 1; i < n; i++ {
		curve(float64(float64(i-1)*spacing), y(s.frame.values[i-1]), float64(float64(i)*spacing), y(s.frame.values[i]))
	}
	switch s.dir {
	case config.CavaDirectionReverse:
		p.LineTo(at(width, 0))
		p.LineTo(at(0, 0))
	case config.CavaDirectionMirror:
		center := float64(c.h / 2)
		mirror := func(i int) float64 { return center + float64(math.Max(s.frame.values[i], minAmp)*c.h/2) }
		p.LineTo(at(float64(float64(n-1)*spacing), mirror(n-1)))
		for i := n - 2; i >= 0; i-- {
			curve(float64(float64(i+1)*spacing), mirror(i+1), float64(float64(i)*spacing), mirror(i))
		}
	default:
		p.LineTo(at(width, c.h))
		p.LineTo(at(0, c.h))
	}
	p.Close()
	cv.FillPath(&p, s.color)
}

// HitTest misses: the visualizer takes no input.
func (s *Spectrum) HitTest(widget.Point) widget.Widget { return nil }

// Bars is how many values a frame of s holds.
func (s *Spectrum) Bars() int { return len(s.frame.values) }

// Values are the bars s paints, a copy.
func (s *Spectrum) Values() []float64 { return slices.Clone(s.frame.values) }

// Bars is the bar count and channel count the service runs with:
// stereo splits the bars between left and right, so an odd count is
// rounded up.
func Bars(cfg config.CavaConfig) (bars, channels int) {
	bars, channels = int(cfg.Bars), 1
	if cfg.Stereo {
		channels = 2
		if bars%2 != 0 {
			log.Printf("cava: odd bar count %d rounded up to %d for stereo output", bars, bars+1)
			bars++
		}
	}
	return bars, channels
}
