package settings

import (
	"cmp"
	"slices"

	"github.com/stubbedev/lys/desktop/desktopentry"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const appStripIconPx = 32

// appStripSpec shapes an app strip: its class, which names its parts, the
// keys of its empty note and its add button, and the installed apps it
// offers to add.
type appStripSpec struct {
	class, none, add string
	offer            func(installedApps) []desktopentry.App
}

// appStrip is an ordered list of apps by id as a strip of icons named by
// their apps, an id no installed app has by itself: dragged to reorder,
// dismissed to remove, added from the installed apps it offers. Every
// change hands the whole list to write.
type appStrip struct {
	*widget.Box
	k     *kit
	spec  appStripSpec
	write func([]string)
	apps  installedApps
	ids   []string
	chips *widget.ChipBox
	none  *inlineState
	add   *widget.Button
}

func newAppStrip(k *kit, spec appStripSpec, ids []string, write func([]string)) *appStrip {
	c := &appStrip{Box: gapBox(widget.Row), k: k, spec: spec, write: write, ids: slices.Clone(ids)}
	c.AddClass(spec.class + "-strip")
	c.chips = widget.NewChipBox()
	c.chips.OnMoved = func(_ *widget.ChipBox, from, to int) { c.move(from, to) }
	c.none = k.inlineState()
	c.none.AddClass(spec.class + "-none")
	c.add = k.addButton(spec.add, c.pick)
	c.add.SetVisible(false)
	c.Append(c.chips, true)
	c.AppendAligned(c.none, true, widget.AlignCenter)
	c.AppendAligned(c.add, false, widget.AlignCenter)
	c.rebuild()
	k.withInstalledApps(func(ix installedApps) {
		c.apps = ix
		c.add.SetVisible(true)
		c.rebuild()
	})
	return c
}

// show takes ids changed elsewhere; the same ids rebuild nothing.
func (c *appStrip) show(ids []string) {
	if slices.Equal(ids, c.ids) {
		return
	}
	c.ids = slices.Clone(ids)
	c.rebuild()
}

func (c *appStrip) rebuild() {
	c.chips.Clear()
	for _, id := range c.ids {
		c.chips.Append(c.chip(id))
	}
	if len(c.ids) > 0 {
		c.none.Clear()
		return
	}
	c.none.Empty("", locale.T().Get(c.spec.none))
}

// face is what app id is called and its icon: its installed app's, else
// the id and the fallback icon.
func (c *appStrip) face(id string) (name, icon string) {
	if a, ok := c.apps.byID[id]; ok {
		return cmp.Or(a.DisplayName(), id), cmp.Or(a.Icon(), appFallbackIcon)
	}
	return id, appFallbackIcon
}

func (c *appStrip) chip(id string) *widget.Chip {
	name, icon := c.face(id)
	chip := widget.NewChip(c.k.face, name)
	chip.AddClass(c.spec.class + "-app")
	chip.SetContent(widget.NewThemeIcon(icon, appStripIconPx))
	chip.SetTooltip(name)
	chip.SetDismiss(chipRemoveName(locale.T()), func() { c.drop(c.chips.IndexOf(chip)) })
	return chip
}

func (c *appStrip) move(from, to int) {
	id := c.ids[from]
	c.ids = slices.Insert(slices.Delete(c.ids, from, from+1), to, id)
	c.write(slices.Clone(c.ids))
}

func (c *appStrip) drop(at int) {
	id := c.ids[at]
	c.ids = slices.Delete(c.ids, at, at+1)
	c.write(slices.Clone(c.ids))
	c.rebuild()
	name, _ := c.face(id)
	c.k.offerUndo(name, func() {
		if slices.Contains(c.ids, id) {
			return
		}
		c.ids = insertAt(c.ids, at, id)
		c.write(slices.Clone(c.ids))
		c.rebuild()
	})
}

func (c *appStrip) pick() {
	unlisted := slices.DeleteFunc(c.spec.offer(c.apps), func(a desktopentry.App) bool { return slices.Contains(c.ids, a.AppID()) })
	openAppPicker(c.k, c.add, unlisted, func(a desktopentry.App) {
		c.ids = append(c.ids, a.AppID())
		c.write(slices.Clone(c.ids))
		c.rebuild()
	})
}
