package settings

import (
	"fmt"
	"strconv"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// unit is what a number counts: the message whose text is the unit
// alone, the addon after a field typing the number, and whose value
// attribute renders a number in it ("{ $n } px"), the reading of a
// slider. The zero unit is a bare count.
type unit string

// The units the window's numbers come in.
const (
	bareCount        unit = ""
	unitPixels       unit = "settings-unit-pixels"
	unitMilliseconds unit = "settings-unit-milliseconds"
	unitPerSecond    unit = "settings-unit-per-second"
	unitPercent      unit = "settings-unit-percent"
	unitDegrees      unit = "settings-unit-degrees"
	unitKelvin       unit = "settings-unit-kelvin"
	unitZoom         unit = "settings-unit-zoom"
	unitSeconds      unit = "settings-duration-unit-seconds"
	unitMinutes      unit = "settings-duration-unit-minutes"
	unitDays         unit = "settings-duration-unit-days"
	unitEntries      unit = "settings-unit-entries"
	unitFrames       unit = "settings-framerate-unit"
	unitScale        unit = "settings-unit-scale"
)

// addon is the unit alone, "" for a bare count.
func (u unit) addon() string {
	if u == bareCount {
		return ""
	}
	return locale.T().Get(string(u))
}

// format renders a number at digits decimals in the unit. A unit whose
// message renders no value is a bug in the caller.
func (u unit) format(digits int) func(float64) string {
	if u != bareCount && !locale.T().HasAttr(string(u), "value") {
		panic(fmt.Sprintf("settings: the unit %s renders no value", u))
	}
	return func(v float64) string {
		n := strconv.FormatFloat(v, 'f', digits, 64)
		if u == bareCount {
			return n
		}
		return locale.T().Attr(string(u), "value", i18n.Str("n", n))
	}
}

// inUnit is input inside one border with u's addon after it, in the
// muted color lys-ui's input group gives its addons.
func (k *kit) inUnit(input widget.Widget, u unit) *widget.InputGroup {
	return k.unitGroup(input, u, func(l *widget.Label) widget.Widget { return l })
}

// unitGroup is inUnit with the addon placed through place.
func (k *kit) unitGroup(input widget.Widget, u unit, place func(*widget.Label) widget.Widget) *widget.InputGroup {
	g := widget.NewInputGroup(input)
	if a := u.addon(); a != "" {
		g.Append(place(k.label(a)))
	}
	return g
}

// spinControl is a spin a field holds: a number, required or optional.
type spinControl interface {
	control
	spinButton() *widget.SpinButton
}

// unitNumber is a number field with its unit as the trailing addon.
type unitNumber struct {
	*widget.InputGroup
	number spinControl
}

func (c *unitNumber) refresh() { c.number.refresh() }

// spin holds the field at its own width on the control edge
// (controlFit).
func (c *unitNumber) spin() {}

func (c *unitNumber) spinButton() *widget.SpinButton { return c.number.spinButton() }

// numberField is n in u inside one border with the unit's addon after
// it. While a page is being built the field joins the page's spin
// widths: n as wide as the page's other spins, its unit as wide as
// theirs, and a bare count an empty addon as wide, so every number field
// of the page, a row's or a slider's, starts and ends at one x with its
// arrows on one x. Outside a page a bare count is n alone.
func (k *kit) numberField(n spinControl, u unit) control {
	if k.spins == nil {
		if u == bareCount {
			return n
		}
		return &unitNumber{InputGroup: k.inUnit(n, u), number: n}
	}
	k.spins.addSpin(n.spinButton())
	g := widget.NewInputGroup(n)
	g.Append(k.spins.unit(k.label(u.addon())))
	return &unitNumber{InputGroup: g, number: n}
}
