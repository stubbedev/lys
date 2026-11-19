package settings

import (
	"maps"
	"slices"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	launcherScriptsPath = "launcher.scripts"
	everyMonitor        = "*"
)

type choiceItem struct {
	*widget.Box
	value string
}

func (c choiceItem) itemValue() any { return c.value }

func listedChoice(items []choice, value string) choice {
	if i := slices.IndexFunc(items, func(o choice) bool { return o.value == value }); i >= 0 {
		return items[i]
	}
	return choice{value: value, label: value, group: locale.T().Get("settings-choice-unknown")}
}

type choiceListEditor struct {
	*listEditor
	picker *searchPicker
}

func newChoiceListEditor(k *kit, s slot, list lister) *choiceListEditor {
	var items []choice
	loaded := false
	c := &choiceListEditor{}
	c.listEditor = newListEditor(k, s, listSpec{
		addKey: "settings-list-add", rowClass: "string-list-row", reorder: true,
		item: func(k *kit, v any, _ func()) listItem {
			value, _ := v.(string)
			return choiceItem{choiceRow(k, listedChoice(items, value)), value}
		},
		pick: func(anchor widget.Widget, choose func(any)) {
			if k.pickers == nil {
				return
			}
			listed := c.values()
			open := slices.DeleteFunc(slices.Clone(items), func(o choice) bool { return slices.Contains(listed, any(o.value)) })
			c.picker = openChoicePicker(k, anchor, open, func(v string) { choose(v) })
		},
		full: func(int) bool { return !loaded || allChosen(items, c.values()) },
	})
	async(k, list, func(cs []choice) {
		items, loaded = cs, true
		c.rebuild(c.values())
	})
	return c
}

func allChosen(items []choice, listed []any) bool {
	return !slices.ContainsFunc(items, func(o choice) bool { return !slices.Contains(listed, any(o.value)) })
}

func choiceListRow(list func(k *kit) lister) rowOpt {
	return func(r *rowSpec) {
		r.fullWidth = true
		r.editor = func(k *kit, s slot, _ config.FieldMeta) control { return newChoiceListEditor(k, s, list(k)) }
	}
}

func launcherModes(k *kit) lister {
	scripts, _ := k.store.value(launcherScriptsPath).(map[string]any)
	names := slices.Sorted(maps.Keys(scripts))
	return func() []choice {
		t := locale.T()
		out := make([]choice, 0, len(config.LauncherBuiltinModes)+len(names))
		for _, m := range config.LauncherBuiltinModes {
			out = append(out, choice{value: m, label: t.Get("settings-launcher-mode-" + m), group: m})
		}
		for _, name := range names {
			out = append(out, choice{value: name, label: t.Get("settings-launcher-mode-script", i18n.Str("name", name)), group: name})
		}
		return out
	}
}

func monitorsOrEvery(*kit) lister {
	return func() []choice {
		every := choice{value: everyMonitor, label: locale.T().Get("settings-monitors-every"), group: everyMonitor}
		return append([]choice{every}, systemLists["Monitor"]()...)
	}
}

var (
	launcherModesRow = choiceListRow(launcherModes)
	monitorsRow      = choiceListRow(monitorsOrEvery)
	scriptsRow       = listRow(func(config.FieldMeta) listSpec {
		return pairMapSpec("settings-map-mode-name-placeholder", true, func(k *kit, value *string, changed func()) widget.Widget {
			return newFileEditor(k, stringSlot(value, changed))
		})
	})
)
