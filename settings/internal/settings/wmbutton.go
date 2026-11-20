package settings

import (
	"math"
	"slices"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

type namedOrWhole struct {
	*widget.Box
	slot    slot
	choice  *widget.Dropdown
	code    *widget.SpinButton
	names   []string
	syncing bool
}

func newNamedOrWhole(k *kit, s slot, meta config.FieldMeta) *namedOrWhole {
	t := locale.T()
	labels := slices.Concat([]string{t.Get("settings-inherit")}, enumLabels(meta), []string{t.Get("wm-button-code")})
	c := &namedOrWhole{Box: gapBox(widget.Row), slot: s, names: meta.Variants}
	c.choice = widget.NewDropdown(k.face, k.textPx(), labels, 0)
	c.code = widget.NewSpinButton(k.face, k.textPx(), 0, meta.Min, meta.Max, 1, 0)
	c.code.SetTooltip(t.Attr("wm-button-code", "description"))
	c.choice.OnSelect = c.choose
	c.code.OnValueChanged = func(v float64) {
		if !c.syncing {
			_ = c.slot.set(int64(math.Round(v)))
		}
	}
	c.Append(c.choice, false)
	c.Append(c.code, false)
	c.refresh()
	return c
}

func (c *namedOrWhole) codeIndex() int { return len(c.names) + 1 }

func (c *namedOrWhole) choose(i int) {
	if c.syncing {
		return
	}
	switch {
	case i == 0:
		c.slot.unset()
	case i == c.codeIndex():
		_ = c.slot.set(int64(math.Round(c.code.Value())))
	default:
		_ = c.slot.set(c.names[i-1])
	}
	c.refresh()
}

func (c *namedOrWhole) refresh() {
	selected := 0
	c.syncing = true
	defer func() { c.syncing = false }()
	switch v := c.slot.get().(type) {
	case int64:
		selected = c.codeIndex()
		c.code.SetValue(float64(v))
	case string:
		selected = slices.Index(c.names, v) + 1
	}
	c.choice.SetSelected(selected)
	c.code.SetVisible(selected == c.codeIndex())
}

var namedOrWholeRow = withEditor(func(k *kit, s slot, meta config.FieldMeta) control { return newNamedOrWhole(k, s, meta) })
