package bar

import (
	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"
)

// statRing is a dashboard gauge: lys-ui's ProgressRing with its value
// written in the middle, in a threshold's color.
type statRing struct {
	ring  *widget.ProgressRing
	label *widget.Label
}

func newStatRing(label *widget.Label) *statRing {
	r := &statRing{ring: widget.NewProgressRing(0), label: label}
	r.ring.AddClass("stat-ring")
	label.AddClass(css.Heading)
	r.ring.SetChild(label)
	return r
}

// set shows fraction (clamped by the ring) and its text, in color, or
// as a success while no threshold gives one.
func (r *statRing) set(fraction float64, text string, color *config.ColorValue) {
	tintGauge(r.ring, color)
	r.label.SetText(text)
	r.ring.SetValue(fraction)
}

type tintable interface {
	classer
	SetInlineStyle(css string)
}

func tintGauge(g tintable, color *config.ColorValue) {
	setClass(g, css.Success, color == nil)
	g.SetInlineStyle(colorDecl("accent-color", color))
}

// glyphRing is a row's media that reads as a percentage at a glance:
// lys-ui's ProgressRing filled to it around a glyph, a network's signal
// around the wifi glyph or a device's battery around its icon, where a
// 16px stepped glyph did not read.
type glyphRing struct {
	*widget.ProgressRing
	glyph *widget.Icon
}

func newGlyphRing(icon string, percent uint8) *glyphRing {
	r := &glyphRing{
		ProgressRing: widget.NewProgressRing(percentFraction(percent)),
		glyph:        widget.NewThemeIcon(icon, itemIconPx),
	}
	r.AddClass("glyph-ring")
	r.SetChild(r.glyph)
	return r
}

// setPercent fills the ring to percent.
func (r *glyphRing) setPercent(percent uint8) { r.SetValue(percentFraction(percent)) }

// percentFraction is a percentage as a ring's fill.
func percentFraction(percent uint8) float64 { return float64(min(percent, 100)) / 100 }
