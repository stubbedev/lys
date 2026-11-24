package settings

import (
	"math"
	"slices"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

type namedOrWhole struct {
	*customDropdown
	slot  slot
	code  *widget.SpinButton
	names []string
}

func newNamedOrWhole(k *kit, s slot, meta config.FieldMeta) *namedOrWhole {
	t := locale.T()
	labels := slices.Concat([]string{t.Get("settings-inherit")}, enumLabels(meta))
	c := &namedOrWhole{slot: s, names: meta.Variants}
	c.code = widget.NewSpinButton(k.face, k.textPx(), 0, meta.Min, meta.Max, 1, 0)
	c.code.SetTooltip(t.Attr("wm-button-code", "description"))
	c.code.OnValueChanged = func(v float64) { _ = c.slot.set(int64(math.Round(v))) }
	c.customDropdown = newCustomDropdown(k, labels, t.Get("wm-button-code"), c.code)
	c.OnSelect = c.choose
	c.refresh()
	return c
}

func (c *namedOrWhole) choose(i int) {
	switch {
	case i == 0:
		c.slot.unset()
	case i == c.customIndex():
		_ = c.slot.set(int64(math.Round(c.code.Value())))
	default:
		_ = c.slot.set(c.names[i-1])
	}
	c.refresh()
}

// fit keeps the button choice at its own width (controlFit): the code
// spin appears beside the dropdown while a code is chosen, and in the
// card's control column it would widen every field of the card while it
// shows, as the action editor's command entry would.
func (c *namedOrWhole) fit() widget.ItemControlFit { return widget.ItemControlNatural }

func (c *namedOrWhole) refresh() {
	selected := 0
	switch v := c.slot.get().(type) {
	case int64:
		selected = c.customIndex()
		c.code.SetValue(float64(v))
	case string:
		selected = slices.Index(c.names, v) + 1
	}
	c.show(selected)
}

var namedOrWholeRow = withEditor(func(k *kit, s slot, meta config.FieldMeta) control { return newNamedOrWhole(k, s, meta) })
