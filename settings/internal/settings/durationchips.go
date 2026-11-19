package settings

import (
	"slices"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

type durationUnit int

const (
	unitMinutes durationUnit = iota
	unitDays
)

func (u durationUnit) words(n int64) string {
	if n == 0 {
		return locale.T().Get("settings-duration-never")
	}
	if u == unitDays {
		return locale.T().Get("settings-duration-days", i18n.Int("n", n))
	}
	return durationWords(time.Duration(n) * time.Minute)
}

type durationChips struct {
	*widget.Box
	chips   *widget.ToggleGroup
	custom  *number
	slot    slot
	presets []int64
	chosen  bool
	syncing bool
}

func durationChoice(unit durationUnit, presets ...int64) rowOpt {
	return withEditor(func(k *kit, s slot, meta config.FieldMeta) control {
		return newDurationChips(k, s, meta, unit, presets)
	})
}

func newDurationChips(k *kit, s slot, meta config.FieldMeta, unit durationUnit, presets []int64) *durationChips {
	c := &durationChips{Box: gapBox(widget.Row), chips: widget.NewToggleGroup(), slot: s, presets: append(slices.Clone(presets), 0)}
	for _, n := range c.presets {
		c.chips.Append(k.label(unit.words(n)))
	}
	c.chips.Append(k.label(locale.T().Get("settings-duration-custom")))
	c.custom = newNumber(k, s, meta.Min, meta.Max, 1, 0)
	c.chips.OnChanged = func(i int) {
		if c.syncing {
			return
		}
		c.chosen = i == len(c.presets)
		if !c.chosen {
			_ = c.slot.set(c.presets[i])
		}
		c.refresh()
	}
	c.AppendAligned(c.chips, false, widget.AlignCenter)
	c.AppendAligned(c.custom, false, widget.AlignCenter)
	c.refresh()
	return c
}

func (c *durationChips) refresh() {
	v, _ := c.slot.get().(int64)
	at := slices.Index(c.presets, v)
	custom := c.chosen || at < 0
	if custom {
		at = len(c.presets)
	}
	c.syncing = true
	c.chips.SetActive(at)
	c.syncing = false
	c.custom.refresh()
	c.custom.SetVisible(custom)
}
