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

type namedChoiceItem struct {
	*widget.Box
	key  string
	name *widget.Entry
}

func (n namedChoiceItem) itemValue() any { return [2]string{n.key, n.name.Text()} }

type choiceShape struct {
	key     func(v any) string
	item    func(k *kit, o choice, v any, changed func()) listItem
	picked  func(value string) any
	reorder bool
	decode  func(v any) []any
	encode  func(items []any) any
}

var plainChoices = choiceShape{
	key: func(v any) string { s, _ := v.(string); return s },
	item: func(k *kit, o choice, _ any, _ func()) listItem {
		return choiceItem{choiceRow(k, o), o.value}
	},
	picked:  func(value string) any { return value },
	reorder: true,
}

var namedChoices = choiceShape{
	key: func(v any) string { pair, _ := v.([2]string); return pair[0] },
	item: func(k *kit, o choice, v any, changed func()) listItem {
		pair, _ := v.([2]string)
		row := namedChoiceItem{Box: gapBox(widget.Row), key: o.value, name: k.entry()}
		row.name.SetPlaceholder(locale.T().Get("settings-map-name-placeholder"))
		row.name.SetText(pair[1])
		row.name.OnChanged = func(string) { changed() }
		row.Append(choiceRow(k, o), true)
		row.Append(row.name, true)
		return row
	},
	picked: func(value string) any { return [2]string{value, ""} },
	decode: decodePairs,
	encode: encodePairs(func(pair [2]string) bool { return pair[0] != "" && pair[1] != "" }),
}

type choiceListEditor struct {
	*listEditor
	picker *searchPicker
}

func newChoiceListEditor(k *kit, s slot, list lister, shape choiceShape) *choiceListEditor {
	var items []choice
	loaded := false
	c := &choiceListEditor{}
	keys := func() []string {
		values := c.values()
		out := make([]string, len(values))
		for i, v := range values {
			out[i] = shape.key(v)
		}
		return out
	}
	c.listEditor = newListEditor(k, s, listSpec{
		addKey: "settings-list-add", rowClass: "string-list-row", reorder: shape.reorder,
		item: func(k *kit, v any, changed func()) listItem {
			return shape.item(k, listedChoice(items, shape.key(v)), v, changed)
		},
		pick: func(anchor widget.Widget, choose func(any)) {
			if k.pickers == nil {
				return
			}
			listed := keys()
			open := slices.DeleteFunc(slices.Clone(items), func(o choice) bool { return slices.Contains(listed, o.value) })
			c.picker = openChoicePicker(k, anchor, open, func(v string) { choose(shape.picked(v)) })
		},
		full:   func(int) bool { return !loaded || allChosen(items, keys()) },
		decode: shape.decode,
		encode: shape.encode,
	})
	async(k, list, func(cs []choice) {
		items, loaded = cs, true
		c.rebuild(c.values())
	})
	return c
}

func allChosen(items []choice, listed []string) bool {
	return !slices.ContainsFunc(items, func(o choice) bool { return !slices.Contains(listed, o.value) })
}

func choiceListRow(list func(k *kit) lister, shape choiceShape) rowOpt {
	return func(r *rowSpec) {
		r.fullWidth = true
		r.editor = func(k *kit, s slot, _ config.FieldMeta) control { return newChoiceListEditor(k, s, list(k), shape) }
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
	launcherModesRow = choiceListRow(launcherModes, plainChoices)
	displayNamesRow  = choiceListRow(launcherModes, namedChoices)
	monitorsRow      = choiceListRow(monitorsOrEvery, plainChoices)
	scriptsRow       = listRow(func(config.FieldMeta) listSpec {
		return pairMapSpec("settings-map-mode-name-placeholder", true, func(k *kit, value *string, changed func()) widget.Widget {
			return newFileEditor(k, stringSlot(value, changed))
		})
	})
)
