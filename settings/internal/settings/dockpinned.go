package settings

import (
	"cmp"
	"log"
	"slices"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/desktopentry"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const pinnedIconPx = 32

type pinnedStrip struct {
	*widget.Box
	k     *kit
	slot  slot
	apps  installedApps
	ids   []string
	chips *widget.ChipBox
	none  *widget.Label
	add   *widget.Button
}

var pinnedRow rowOpt = func(r *rowSpec) {
	r.fullWidth = true
	r.editor = func(k *kit, s slot, _ config.FieldMeta) control { return newPinnedStrip(k, s) }
}

func newPinnedStrip(k *kit, s slot) *pinnedStrip {
	c := &pinnedStrip{Box: gapBox(widget.Row), k: k, slot: s}
	c.AddClass("dock-pinned-strip")
	c.chips = widget.NewChipBox()
	c.chips.OnMoved = func(_ *widget.ChipBox, from, to int) { c.move(from, to) }
	c.none = k.label(locale.T().Get("settings-dock-pinned-none"), "dock-pinned-none", css.DimLabel)
	c.add = addButton(k, "settings-dock-pin", c.pick)
	c.add.SetVisible(false)
	c.Append(c.chips, true)
	c.AppendAligned(c.none, true, widget.AlignCenter)
	c.AppendAligned(c.add, false, widget.AlignCenter)
	c.ids = c.stored()
	c.rebuild()
	dirs := desktopentry.ApplicationDirs()
	async(k, func() installedApps { return scanInstalled(dirs) }, func(ix installedApps) {
		c.apps = ix
		c.add.SetVisible(true)
		c.rebuild()
	})
	return c
}

func (c *pinnedStrip) stored() []string {
	raw, _ := c.slot.get().([]any)
	ids := make([]string, len(raw))
	for i, v := range raw {
		id, ok := v.(string)
		if !ok {
			log.Panicf("settings: dock.pinned holds %T %v, not an app id", v, v)
		}
		ids[i] = id
	}
	return ids
}

func (c *pinnedStrip) commit() {
	out := make([]any, len(c.ids))
	for i, id := range c.ids {
		out[i] = id
	}
	_ = c.slot.set(out)
}

func (c *pinnedStrip) refresh() {
	if stored := c.stored(); !slices.Equal(stored, c.ids) {
		c.ids = stored
		c.rebuild()
	}
}

func (c *pinnedStrip) rebuild() {
	c.chips.Clear()
	for _, id := range c.ids {
		c.chips.Append(c.chip(id))
	}
	c.none.SetVisible(len(c.ids) == 0)
}

func (c *pinnedStrip) chip(id string) *widget.Chip {
	name, icon := id, appFallbackIcon
	if a, ok := c.apps.byID[id]; ok {
		name, icon = cmp.Or(a.DisplayName(), id), cmp.Or(a.Icon(), appFallbackIcon)
	}
	chip := widget.NewChip(c.k.face, name)
	chip.AddClass("dock-pinned-app")
	chip.SetContent(widget.NewThemeIcon(icon, pinnedIconPx))
	chip.SetTooltip(name)
	chip.SetDismiss(chipRemoveName(locale.T()), func() { c.unpin(c.chips.IndexOf(chip)) })
	return chip
}

func (c *pinnedStrip) move(from, to int) {
	id := c.ids[from]
	c.ids = slices.Insert(slices.Delete(c.ids, from, from+1), to, id)
	c.commit()
}

func (c *pinnedStrip) unpin(at int) {
	c.ids = slices.Delete(c.ids, at, at+1)
	c.commit()
	c.rebuild()
}

func (c *pinnedStrip) pick() {
	unpinned := slices.DeleteFunc(slices.Clone(c.apps.shown), func(a desktopentry.App) bool { return slices.Contains(c.ids, a.AppID()) })
	openAppPicker(c.k, c.add, unpinned, func(a desktopentry.App) {
		c.ids = append(c.ids, a.AppID())
		c.commit()
		c.rebuild()
	})
}
