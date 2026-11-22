package settings

import (
	"maps"
	"slices"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/desktopentry"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const appNapPath = "modules.battery.app-nap"

// appNapRow is every app the user gave a nap level, each with the level
// picked from the levels lys-shell knows.
func appNapRow() rowSpec {
	return field(appNapPath, func(r *rowSpec) {
		r.fullWidth = true
		r.editor = func(k *kit, s slot, _ config.FieldMeta) control { return newNapApps(k, s) }
	})
}

// napApps lists the apps with a nap level, an app added here showing
// at the default level until another is picked.
type napApps struct {
	*widget.Box
	k       *kit
	slot    slot
	apps    installedApps
	pending []string
	rows    []string
	list    *widget.Box
	empty   *widget.Label
	add     *widget.Button
	levels  map[string]*widget.Dropdown
	syncing bool
}

func newNapApps(k *kit, s slot) *napApps {
	t := locale.T()
	c := &napApps{Box: gapBox(widget.Column), k: k, slot: s}
	c.list = gapBox(widget.Column)
	c.empty = wrapped(k.label(t.Get("settings-app-nap-none"), "setting-description", css.Caption, css.DimLabel))
	c.add = k.addButton("settings-app-nap-add", c.pickApp)
	c.Append(c.empty, false)
	c.Append(c.list, false)
	c.Append(c.add, false)
	dirs := desktopentry.ApplicationDirs()
	async(k, func() installedApps { return scanInstalled(dirs) }, func(ix installedApps) {
		c.apps = ix
		c.rebuild()
	})
	c.rebuild()
	return c
}

func (c *napApps) stored() map[string]string {
	out := map[string]string{}
	v, _ := c.slot.get().(map[string]any)
	for app, level := range v {
		if s, ok := level.(string); ok {
			out[app] = s
		}
	}
	return out
}

func (c *napApps) named() []string {
	stored := c.stored()
	out := slices.DeleteFunc(slices.Clone(c.rows), func(app string) bool {
		_, ok := stored[app]
		return !ok && !slices.Contains(c.pending, app)
	})
	for _, app := range slices.Concat(c.pending, slices.Sorted(maps.Keys(stored))) {
		if !slices.Contains(out, app) {
			out = append(out, app)
		}
	}
	return out
}

func (c *napApps) pickApp() {
	named := c.named()
	apps := slices.DeleteFunc(slices.Clone(c.apps.shown), func(a desktopentry.App) bool { return slices.Contains(named, a.AppID()) })
	openAppPicker(c.k, c.add, apps, func(a desktopentry.App) {
		c.pending = append(c.pending, a.AppID())
		c.rebuild()
	})
}

func (c *napApps) write(stored map[string]string) {
	out := make(map[string]any, len(stored))
	for app, level := range stored {
		out[app] = level
	}
	_ = c.slot.set(out)
}

func (c *napApps) setLevel(app string, level config.NapLevel) {
	stored := c.stored()
	stored[app] = string(level)
	c.write(stored)
	c.refresh()
}

func (c *napApps) remove(app string) {
	c.pending = slices.DeleteFunc(c.pending, func(p string) bool { return p == app })
	stored := c.stored()
	delete(stored, app)
	c.write(stored)
	c.rebuild()
}

func (c *napApps) face(app string) *widget.Box {
	if a, ok := c.apps.byID[app]; ok {
		return installedFace(c.k, a)
	}
	return appFace(c.k, "", app, app)
}

func napLevelLabels() []string {
	t := locale.T()
	levels := config.NapLevels()
	labels := make([]string, len(levels))
	for i, l := range levels {
		labels[i] = t.Get(config.EnumLabelKey("NapLevel", string(l)))
	}
	return labels
}

func (c *napApps) rebuild() {
	t := locale.T()
	c.list.Clear()
	c.levels = map[string]*widget.Dropdown{}
	c.rows = c.named()
	c.empty.SetVisible(len(c.rows) == 0)
	levels := config.NapLevels()
	for _, app := range c.rows {
		row := gapBox(widget.Row)
		row.AddClass("nap-app")
		row.AppendAligned(c.face(app), true, widget.AlignCenter)
		level := widget.NewDropdown(c.k.face, c.k.textPx(), napLevelLabels(), 0)
		level.SetAccessibleName(t.Get("settings-modules-battery-app-nap"))
		level.OnSelect = func(i int) {
			if !c.syncing && i >= 0 && i < len(levels) {
				c.setLevel(app, levels[i])
			}
		}
		c.levels[app] = level
		row.AppendAligned(level, false, widget.AlignCenter)
		row.AppendAligned(c.k.removeButton("settings-list-remove", func() { c.remove(app) }), false, widget.AlignCenter)
		c.list.Append(row, false)
	}
	c.refresh()
}

func (c *napApps) refresh() {
	if !slices.Equal(c.named(), c.rows) {
		c.rebuild()
		return
	}
	c.syncing = true
	defer func() { c.syncing = false }()
	stored := c.stored()
	levels := config.NapLevels()
	for _, app := range c.rows {
		i := slices.Index(levels, config.NapLevel(stored[app]))
		c.levels[app].SetSelected(max(i, 0))
	}
}
