package settings

import (
	"slices"
	"strconv"
	"strings"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
	"github.com/stubbedev/lys/settings/internal/wmconfig"
)

const (
	virtualOutputsPath = "virtual_outputs"
	virtualNamePrefix  = "VIRT-"
)

// virtualModes are the sizes a virtual display is offered in, the second
// the one a new display takes.
var virtualModes = []string{"1280x720@60", "1920x1080@60", "2560x1440@60", "3840x2160@60"}

// virtualNames are the virtual displays lys-wm's config defines, in its
// order.
func virtualNames(w *wmBackend) []string {
	items, _ := w.value(virtualOutputsPath).([]any)
	out := make([]string, 0, len(items))
	for _, it := range items {
		if name, ok := asMap(it)["name"].(string); ok && name != "" {
			out = append(out, name)
		}
	}
	return out
}

// nextVirtualName is the lowest VIRT-n no display, virtual or not, holds.
func nextVirtualName(taken []string) string {
	for n := 1; ; n++ {
		if name := virtualNamePrefix + strconv.Itoa(n); !slices.Contains(taken, name) {
			return name
		}
	}
}

func (p *displaysPanel) addVirtualOutput() {
	w, err := p.k.wmBackend()
	if err != nil {
		p.fail("settings-wm-unreadable", err)
		return
	}
	taken := virtualNames(w)
	for _, h := range p.heads {
		taken = append(taken, h.Name)
	}
	items, _ := w.value(virtualOutputsPath).([]any)
	items = append(slices.Clone(items), map[string]any{"name": nextVirtualName(taken), "mode": virtualModes[1]})
	if err := w.edit(wmconfig.Edit{Path: []string{virtualOutputsPath}, Value: items}); err != nil {
		p.fail("settings-displays-save-failed", err)
	}
	p.render()
}

// virtualSlot is the virtual display named name in lys-wm's config: unset
// removes it.
func virtualSlot(w *wmBackend, name string) slot {
	find := func(items []any) int {
		return slices.IndexFunc(items, func(it any) bool { return asMap(it)["name"] == name })
	}
	items := func() []any {
		v, _ := w.value(virtualOutputsPath).([]any)
		return slices.Clone(v)
	}
	return slot{
		get: func() any {
			all := items()
			if i := find(all); i >= 0 {
				return all[i]
			}
			return nil
		},
		set: func(v any) error {
			all := items()
			i := find(all)
			if i < 0 {
				return errNoVirtual(name)
			}
			all[i] = v
			return w.edit(wmconfig.Edit{Path: []string{virtualOutputsPath}, Value: all})
		},
		unset: func() {
			all := items()
			i := find(all)
			if i < 0 {
				return
			}
			all = slices.Delete(all, i, i+1)
			e := wmconfig.Edit{Path: []string{virtualOutputsPath}, Value: all}
			if base, _ := w.baseValue(virtualOutputsPath).([]any); len(all) == 0 && len(base) == 0 {
				e = wmconfig.Edit{Path: []string{virtualOutputsPath}, Unset: true}
			}
			if err := w.edit(e); err != nil {
				w.show(err.Error(), css.Error)
			}
		},
	}
}

type errNoVirtual string

func (e errNoVirtual) Error() string {
	return "settings: lys-wm's config has no virtual display " + strconv.Quote(string(e))
}

func virtualModeLabel(mode string) string {
	size, rate, _ := strings.Cut(mode, "@")
	width, height, ok := strings.Cut(size, "x")
	if !ok || rate == "" {
		return mode
	}
	return locale.T().Get("settings-displays-virtual-mode-choice", i18n.Str("width", width), i18n.Str("height", height), i18n.Str("rate", rate))
}

func virtualModeChoices() []choice {
	out := make([]choice, len(virtualModes))
	for i, m := range virtualModes {
		out[i] = choice{value: m, label: virtualModeLabel(m)}
	}
	return out
}

// showVirtualRows puts the selected display's size and a remove button
// over its other rows when it is a virtual one.
func (p *displaysPanel) showVirtualRows(w *wmBackend, name string) {
	if !slices.Contains(virtualNames(w), name) {
		return
	}
	s := virtualSlot(w, name)
	mode := newChoiceEditor(p.k, nestedSlot(s, []string{"mode"}), choiceSpec{list: fixedChoices(virtualModeChoices()), free: true})
	p.addExtra(p.k.keyRow("settings-displays-virtual-mode").withControl(mode))
	remove := p.k.removeButton("settings-displays-virtual-remove", func() {
		s.unset()
		p.render()
	})
	p.addExtra(p.k.keyRow("settings-displays-virtual").withControl(remove))
}
