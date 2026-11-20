package settings

import (
	"slices"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

type namedValue struct{ value, key string }

type valueSelect struct {
	*widget.Dropdown
	k       *kit
	slot    slot
	choices []namedValue
	shown   []string
	syncing bool
}

func valueSelectRow(choices ...namedValue) rowOpt {
	return withEditor(func(k *kit, s slot, _ config.FieldMeta) control { return newValueSelect(k, s, choices) })
}

func newValueSelect(k *kit, s slot, choices []namedValue) *valueSelect {
	c := &valueSelect{Dropdown: widget.NewDropdown(k.face, k.textPx(), nil, 0), k: k, slot: s, choices: choices}
	c.OnSelect = func(i int) {
		if !c.syncing && i >= 0 && i < len(c.shown) {
			_ = c.slot.set(c.shown[i])
		}
	}
	c.refresh()
	return c
}

func (c *valueSelect) refresh() {
	t := locale.T()
	current := c.slot.text()
	c.shown = c.shown[:0]
	labels := make([]string, 0, len(c.choices)+1)
	if !slices.ContainsFunc(c.choices, func(n namedValue) bool { return n.value == current }) {
		c.shown = append(c.shown, current)
		labels = append(labels, t.Get("settings-choice-kept", i18n.Str("value", current)))
	}
	for _, n := range c.choices {
		c.shown = append(c.shown, n.value)
		labels = append(labels, t.Get(n.key))
	}
	c.syncing = true
	c.SetItems(labels, slices.Index(c.shown, current))
	c.syncing = false
}
