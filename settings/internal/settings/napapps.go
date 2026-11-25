package settings

import (
	"maps"
	"slices"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/widget"

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
	*appList
	levels map[string]*widget.Dropdown
}

func newNapApps(k *kit, s slot) *napApps {
	c := &napApps{levels: map[string]*widget.Dropdown{}}
	c.appList = newAppList(k, appListSpec{
		addKey:   "settings-app-nap-add",
		emptyKey: "settings-app-nap-none",
		rowClass: "nap-app",
		stored:   func() []string { return slices.Sorted(maps.Keys(storedNapLevels(s))) },
		controls: func(list *appList, app string) []widget.Widget { return []widget.Widget{c.levelPick(list, s, app)} },
		show: func(rows []string) {
			stored := storedNapLevels(s)
			for _, app := range rows {
				c.levels[app].SetSelected(max(slices.Index(config.NapLevels(), config.NapLevel(stored[app])), 0))
			}
		},
		drop: func(app string) func() {
			stored := storedNapLevels(s)
			level, had := stored[app]
			delete(stored, app)
			writeNapLevels(s, stored)
			return func() {
				if !had {
					return
				}
				stored := storedNapLevels(s)
				stored[app] = level
				writeNapLevels(s, stored)
			}
		},
	})
	return c
}

// storedNapLevels is each app's stored nap level.
func storedNapLevels(s slot) map[string]string {
	out := map[string]string{}
	v, _ := s.get().(map[string]any)
	for app, level := range v {
		if l, ok := level.(string); ok {
			out[app] = l
		}
	}
	return out
}

func writeNapLevels(s slot, stored map[string]string) {
	out := make(map[string]any, len(stored))
	for app, level := range stored {
		out[app] = level
	}
	_ = s.set(out)
}

// levelPick is app's level dropdown, writing the level picked.
func (c *napApps) levelPick(list *appList, s slot, app string) *widget.Dropdown {
	levels := config.NapLevels()
	level := widget.NewDropdown(list.k.face, list.k.textPx(), napLevelLabels(), 0)
	level.SetAccessibleName(locale.T().Get("settings-modules-battery-app-nap"))
	level.OnSelect = func(i int) {
		if list.syncing || i < 0 || i >= len(levels) {
			return
		}
		stored := storedNapLevels(s)
		stored[app] = string(levels[i])
		writeNapLevels(s, stored)
		list.refresh()
	}
	c.levels[app] = level
	return level
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
