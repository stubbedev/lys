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
	// own is each spin's own least width, what its value range makes
	// it: the width it gives way to where its row is too narrow for the
	// page's.
	own   map[*widget.SpinButton]int
	units *widget.SizeGroup
}

// spinWidestChars caps the width a value range gives a spin, so one
// field of an unbounded range does not widen every spin of its page; a
// longer value pans in its field.
const spinWidestChars = 6

func newSpinWidths() *spinWidths {
	return &spinWidths{own: map[*widget.SpinButton]int{}, units: widget.NewSizeGroup(widget.SizeGroupHorizontal)}
}

// addSpin sizes s and the page's spins to the widest of them, each
// giving way down to its own width.
func (g *spinWidths) addSpin(s *widget.SpinButton) {
	if _, ok := g.own[s]; ok {
		return
	}
	least, _ := s.WidthChars()
	g.own[s] = min(least, spinWidestChars)
	g.spins = append(g.spins, s)
	chars := 0
	for _, o := range g.spins {
		chars = max(chars, g.own[o])
	}
	for _, o := range g.spins {
		o.SetWidthChars(g.own[o], chars)
	}
}

// unit is a unit after a spin, as wide as the page's widest.
func (g *spinWidths) unit(label *widget.Label) widget.Widget { return g.units.Add(label) }

// spinField is a spin a row or a form holds as its control.
type spinField interface{ spinButton() *widget.SpinButton }

// spinOf joins ctl, when it is a spin, to the page's spin widths while a
// page is being built: a row's or a form's spin.
func (k *kit) spinOf(ctl widget.Widget) {
	if s, ok := ctl.(spinField); ok && k.spins != nil {
		k.spins.addSpin(s.spinButton())
	}
}
