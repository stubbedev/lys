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

// customDropdown is a closed choice of named presets that ends in a
// custom one, whose input follows the choice inline while it is chosen:
// the one shape of presets with a value of the user's own. The choice
// is a dropdown however few presets it has, as wide as the line it is
// on, so it reads alike wherever it stands.
type customDropdown struct {
	*widget.Box
	pick   *choicePick
	input  customInput
	custom int
	// OnSelect hears the choice the user picked, never one shown.
	OnSelect func(i int)
}

func newCustomDropdown(k *kit, choices []string, custom string, input customInput) *customDropdown {
	d := &customDropdown{Box: gapBox(widget.Row), input: input, custom: len(choices)}
	d.pick = newDropdownPick(k, append(slices.Clone(choices), custom), 0)
	d.pick.OnSelect = func(i int) {
		d.input.SetVisible(i == d.custom)
		if d.OnSelect != nil {
			d.OnSelect(i)
		}
	}
	d.AppendAligned(d.pick, true, widget.AlignCenter)
	d.AppendAligned(input, false, widget.AlignCenter)
	input.SetVisible(false)
	return d
}

// customIndex is the custom choice's index.
func (d *customDropdown) customIndex() int { return d.custom }

// show selects choice i, with the input while it is the custom one,
// without OnSelect hearing it.
func (d *customDropdown) show(i int) {
	d.pick.show(i)
	d.input.SetVisible(i == d.custom)
}
