package settings

import (
	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	switchInherit = iota
	switchOff
	switchOn
)

// wmSwitch is a lys-wm key that is false or an object: Inherit leaves it to
// lys-wm, Off writes false, and On shows the object's fields, the key taking
// the object once one of them is set.
type wmSwitch struct {
	*widget.Box
	slot   slot
	choice *choicePick
	body   *widget.Box
	fields []control
	labels []string
	on     bool
}

func newWMSwitch(k *kit, s slot, fields []cardField) *wmSwitch {
	return newHeadedWMSwitch(k, s, fields, nil)
}

// newHeadedWMSwitch is a wmSwitch whose choices end the line heading
// starts, on the control edge, its fields under both; nil heading leaves
// the choices alone on their line, at its start.
func newHeadedWMSwitch(k *kit, s slot, fields []cardField, heading widget.Widget) *wmSwitch {
	t := locale.T()
	choices := []string{
		t.Get("settings-inherited"),
		t.Get(config.EnumLabelKey(wmToggleType, wmToggleOff)),
		t.Get(config.EnumLabelKey(wmToggleType, wmToggleOn)),
	}
	c := &wmSwitch{Box: gapBox(widget.Column), slot: s, choice: newChoicePick(k, choices, switchInherit), body: gapBox(widget.Column)}
	c.choice.OnSelect = c.choose
	grid := k.formGrid()
	for _, f := range fields {
		ctl := f.item(k, func(key string) slot { return nestedSlot(s, []string{key}) })
		c.body.Append(grid.row(grid.label(t.Get(f.label)), ctl, f.fill), false)
		c.fields = append(c.fields, ctl)
		c.labels = append(c.labels, f.label)
	}
	if heading == nil {
		c.AppendAligned(c.choice, false, widget.AlignStart)
	} else {
		line := gapBox(widget.Row)
		line.AppendAligned(heading, true, widget.AlignCenter)
		line.AppendAligned(c.choice, false, widget.AlignCenter)
		c.Append(line, false)
	}
	c.Append(c.body, false)
	c.refresh()
	return c
}

// stacked takes the row's width under its text (controlFit): the three
// choices are wider than a card's control column, and On opens the
// object's fields under them.
func (c *wmSwitch) stacked() {}

func (c *wmSwitch) choose(i int) {
	c.on = i == switchOn
	switch i {
	case switchInherit:
		c.slot.unset()
	case switchOff:
		_ = c.slot.set(false)
	}
	c.refresh()
}

func (c *wmSwitch) refresh() {
	v := c.slot.get()
	_, object := v.(map[string]any)
	selected := switchInherit
	switch {
	case object || c.on:
		selected = switchOn
	case v == false:
		selected = switchOff
	}
	c.choice.show(selected)
	c.body.SetVisible(selected == switchOn)
	for _, f := range c.fields {
		f.refresh()
	}
}
