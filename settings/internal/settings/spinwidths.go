package settings

import (
	"github.com/stubbedev/lys/ui/widget"
)

// spinWidths makes the spins of one page one width: every spin as wide
// as the widest its value range makes one, and every unit after a spin
// as wide as the widest unit, so the spins of a page end on the control
// edge with their arrows on one x whatever their unit.
type spinWidths struct {
	spins []*widget.SpinButton
	units *widget.SizeGroup
}

// spinWidestChars caps the width a value range gives a spin, so one
// field of an unbounded range does not widen every spin of its page; a
// longer value pans in its field.
const spinWidestChars = 6

func newSpinWidths() *spinWidths {
	return &spinWidths{units: widget.NewSizeGroup(widget.SizeGroupHorizontal)}
}

// addSpin sizes s and the page's spins to the widest of them.
func (g *spinWidths) addSpin(s *widget.SpinButton) {
	g.spins = append(g.spins, s)
	chars := 0
	for _, o := range g.spins {
		least, _ := o.WidthChars()
		chars = max(chars, min(least, spinWidestChars))
	}
	for _, o := range g.spins {
		o.SetWidthChars(chars, chars)
	}
}

// unit is a unit after a spin, as wide as the page's widest.
func (g *spinWidths) unit(label *widget.Label) widget.Widget { return g.units.Add(label) }

// spinField is a spin a row or a form holds as its control.
type spinField interface{ spinButton() *widget.SpinButton }

// spinOf joins ctl, when it is a spin, to the page's spin widths while a
// page is being built: a row's or a form's spin, not a slider's field,
// which keeps the width of its slider's values.
func (k *kit) spinOf(ctl widget.Widget) {
	if s, ok := ctl.(spinField); ok && k.spins != nil {
		k.spins.addSpin(s.spinButton())
	}
}
