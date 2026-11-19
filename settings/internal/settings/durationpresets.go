package settings

import (
	"slices"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// minutePresets is a duration in whole minutes chosen among presets,
// with Custom for any other, which a number beside it then edits.
func minutePresets(presets ...int64) rowOpt {
	return withEditor(func(k *kit, s slot, meta config.FieldMeta) control {
		return newPresetDuration(k, s, meta, presets)
	})
}

// minutesLabel reads a number of minutes the way the idle times read.
func minutesLabel(m int64) string { return durationLabel((time.Duration(m) * time.Minute).String()) }

type presetDuration struct {
	*widget.Box
	slot     slot
	presets  []int64
	menu     *widget.Dropdown
	custom   *number
	choosing bool
	syncing  bool
}

func newPresetDuration(k *kit, s slot, meta config.FieldMeta, presets []int64) *presetDuration {
	labels := make([]string, 0, len(presets)+1)
	for _, m := range presets {
		labels = append(labels, minutesLabel(m))
	}
	labels = append(labels, locale.T().Get("settings-duration-custom"))
	c := &presetDuration{Box: gapBox(widget.Row), slot: s, presets: presets}
	c.menu = widget.NewDropdown(k.face, k.textPx(), labels, 0)
	c.custom = newNumber(k, s, max(meta.Min, 1), meta.Max, 1, 0)
	c.menu.OnSelect = func(i int) {
		if c.syncing || i < 0 || i > len(c.presets) {
			return
		}
		c.choosing = i == len(c.presets)
		if !c.choosing {
			_ = c.slot.set(c.presets[i])
		}
		c.refresh()
	}
	c.AppendAligned(c.menu, false, widget.AlignCenter)
	c.AppendAligned(c.custom, false, widget.AlignCenter)
	c.refresh()
	return c
}

// refresh selects the stored preset, or Custom with its number for a
// value no preset holds and while Custom was chosen.
func (c *presetDuration) refresh() {
	v, _ := c.slot.get().(int64)
	at := slices.Index(c.presets, v)
	custom := c.choosing || at < 0
	if custom {
		at = len(c.presets)
	}
	c.syncing = true
	c.menu.SetSelected(at)
	c.syncing = false
	c.custom.SetVisible(custom)
	c.custom.refresh()
}
