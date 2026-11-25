package settings

import (
	"github.com/stubbedev/lys/desktop/shell/widgets"
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
	own map[*widget.SpinButton]int
	// chars is the width every spin of the page takes.
	chars int
	units *widget.SizeGroup
	// readings are the page's sliders' readings, one width, so every
	// slider of the page ends its track at one x whether a label or a
	// number field reads it.
	readings *widget.SizeGroup
}

// spinWidestChars caps the width a value range gives a spin, so one
// field of an unbounded range does not widen every spin of its page; a
// longer value pans in its field.
const spinWidestChars = 6

func newSpinWidths() *spinWidths {
	return &spinWidths{
		own:      map[*widget.SpinButton]int{},
		units:    widget.NewSizeGroup(widget.SizeGroupHorizontal),
		readings: widget.NewSizeGroup(widget.SizeGroupHorizontal),
	}
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
	g.chars = max(g.chars, g.own[s])
	for _, o := range g.spins {
		g.hold(o)
	}
}

// hold gives s, one of the page's spins, the page's width again after
// something of its own (new digits) refitted it.
func (g *spinWidths) hold(s *widget.SpinButton) { s.SetWidthChars(g.own[s], g.chars) }

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

// readOf joins d's reading to the page's reading width while a page is
// being built.
func (k *kit) readOf(d *widgets.DebouncedSlider) *widgets.DebouncedSlider {
	if k.spins != nil {
		d.PlaceReading(k.spins.readings.Add)
	}
	return d
}

// buildingOn has widgets built until the returned restore join spins, the
// widths of the page they are built on after the page itself was: a
// card added or rebuilt later lines up with the page's fields.
func (k *kit) buildingOn(spins *spinWidths) (restore func()) {
	prev := k.spins
	k.spins = spins
	return func() { k.spins = prev }
}
