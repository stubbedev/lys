package settings

import (
	"slices"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

type knownVariable struct{ name, value, key string }

var knownVariables = []knownVariable{
	{"MOZ_ENABLE_WAYLAND", "1", "wm-env-moz-wayland"},
	{"QT_QPA_PLATFORM", "wayland", "wm-env-qt-wayland"},
	{"GDK_BACKEND", "wayland", "wm-env-gdk-wayland"},
	{"SDL_VIDEODRIVER", "wayland", "wm-env-sdl-wayland"},
	{"ELECTRON_OZONE_PLATFORM_HINT", "auto", "wm-env-electron-wayland"},
	{"_JAVA_AWT_WM_NONREPARENTING", "1", "wm-env-java-tiling"},
}

func knownVariableChoices() []choice {
	t := locale.T()
	out := make([]choice, len(knownVariables))
	for i, v := range knownVariables {
		out[i] = choice{value: v.name, label: t.Get(v.key), group: v.name}
	}
	return out
}

func validVariableName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		switch {
		case r == '_', r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z':
		case r >= '0' && r <= '9' && i > 0:
		default:
			return false
		}
	}
	return true
}

type envItem struct {
	*widget.Box
	name    string
	value   string
	names   *choiceEditor
	entry   *widget.Entry
	problem *widget.Label
}

func (e *envItem) itemValue() any { return [2]string{e.name, e.value} }

func (e *envItem) showProblem() {
	e.problem.SetVisible(e.name != "" && !validVariableName(e.name))
}

func newEnvItem(k *kit, pair [2]string, changed func()) *envItem {
	t := locale.T()
	e := &envItem{Box: gapBox(widget.Column), name: pair[0], value: pair[1]}
	e.entry = k.entry()
	e.entry.SetPlaceholder(t.Get("settings-map-value-placeholder"))
	e.entry.SetText(e.value)
	e.entry.OnChanged = func(s string) {
		e.value = s
		changed()
	}
	name := slot{
		get: func() any { return e.name },
		set: func(v any) error {
			e.name, _ = v.(string)
			if i := knownVariableIndex(e.name); i >= 0 && e.value == "" {
				e.entry.SetText(knownVariables[i].value)
			}
			e.showProblem()
			changed()
			return nil
		},
		unset: func() { e.name = ""; e.showProblem(); changed() },
	}
	e.names = newChoiceEditor(k, name, choiceSpec{list: knownVariableChoices, free: true})
	e.names.entry.SetPlaceholder(t.Get("wm-env-name-placeholder"))
	e.problem = k.errorText(t.Get("wm-env-name-invalid"))
	line := gapBox(widget.Row)
	line.Append(e.names, true)
	line.Append(e.entry, true)
	e.Append(line, false)
	e.Append(e.problem, false)
	e.showProblem()
	return e
}

func knownVariableIndex(name string) int {
	return slices.IndexFunc(knownVariables, func(v knownVariable) bool { return v.name == name })
}

var wmEnvRow = listRow(func(config.FieldMeta) listSpec {
	return mapSpec(func(k *kit, pair [2]string, changed func()) listItem { return newEnvItem(k, pair, changed) }, validVariableName)
})
