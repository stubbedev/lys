package settings

import (
	"slices"

	"github.com/stubbedev/lys/ui/widget"
)

// customInput is what a customDropdown shows for its custom choice.
type customInput interface {
	widget.Widget
	SetVisible(on bool)
}

// customDropdown is a dropdown of named choices that ends in a custom
// one, whose input follows the dropdown inline while it is chosen: the
// one shape of a long list of presets with a value of the user's own.
type customDropdown struct {
	*widget.Box
	drop    *widget.Dropdown
	input   customInput
	custom  int
	showing bool
	// OnSelect hears the choice the user picked, never one shown.
	OnSelect func(i int)
}

func newCustomDropdown(k *kit, choices []string, custom string, input customInput) *customDropdown {
	d := &customDropdown{Box: gapBox(widget.Row), input: input, custom: len(choices)}
	d.drop = widget.NewDropdown(k.face, k.textPx(), append(slices.Clone(choices), custom), 0)
	d.drop.OnSelect = func(i int) {
		d.input.SetVisible(i == d.custom)
		if !d.showing && d.OnSelect != nil {
			d.OnSelect(i)
		}
	}
	d.AppendAligned(d.drop, false, widget.AlignCenter)
	d.AppendAligned(input, false, widget.AlignCenter)
	input.SetVisible(false)
	return d
}

// customIndex is the custom choice's index.
func (d *customDropdown) customIndex() int { return d.custom }

// show selects choice i, with the input while it is the custom one,
// without OnSelect hearing it.
func (d *customDropdown) show(i int) {
	d.showing = true
	defer func() { d.showing = false }()
	d.drop.SetSelected(i)
	d.input.SetVisible(i == d.custom)
}
