package bar

import (
	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"
)

// statRing is a dashboard gauge: lys-ui's ProgressRing with its value
// written in the middle, in a threshold's color, and the unit trailing
// it smaller. An invisible copy of the unit leads the value, so the
// number itself sits on the ring's centre and the unit hangs off it.
type statRing struct {
	ring  *widget.ProgressRing
	value *widget.Label
}

func newStatRing(label func(string) *widget.Label, unit string) *statRing {
	r := &statRing{ring: widget.NewProgressRing(0), value: label("")}
	r.ring.AddClass("stat-ring")
	r.value.AddClass("stat-value", css.Heading, css.Numeric)
	balance, trailing := label(unit), label(unit)
	balance.AddClass("stat-unit", "stat-unit-balance", css.Caption)
	trailing.AddClass("stat-unit", css.Caption, css.DimLabel)
	row := widget.NewBox(widget.Row, 0, 0)
	row.AddClass("stat-reading")
	row.AppendAligned(balance, false, widget.AlignBaseline)
	row.AppendAligned(r.value, false, widget.AlignBaseline)
	row.AppendAligned(trailing, false, widget.AlignBaseline)
	r.ring.SetChild(widgets.Center(row))
	return r
}

// set shows fraction (clamped by the ring) and its value, in color, or
// as a success while no threshold gives one.
func (r *statRing) set(fraction float64, value string, color *config.ColorValue) {
	tintGauge(r.ring, color)
	r.value.SetText(value)
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
