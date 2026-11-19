package settings

import (
	"maps"
	"strconv"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	rofiControl  = "Control"
	accelControl = "Ctrl"
	customPrefix = "custom-"
)

var launcherMouseTokens = []string{"MousePrimary", "MouseDPrimary", "MouseSecondary", "MouseDSecondary", "MouseMiddle", "ScrollUp", "ScrollDown", "ScrollLeft", "ScrollRight"}

func rofiToAccel(key string) string {
	parts := strings.Split(strings.TrimSpace(key), "+")
	for i, p := range parts {
		switch {
		case p == rofiControl:
			parts[i] = accelControl
		case i == len(parts)-1 && len(p) == 1:
			parts[i] = strings.ToUpper(p)
		}
	}
	return strings.Join(parts, "+")
}

func accelToRofi(accel string) string {
	parts := strings.Split(accel, "+")
	for i, p := range parts {
		switch {
		case p == accelControl:
			parts[i] = rofiControl
		case i == len(parts)-1 && len(p) == 1:
			parts[i] = strings.ToLower(p)
		}
	}
	return strings.Join(parts, "+")
}

func launcherActionLabel(action string) string {
	t := locale.T()
	if n, ok := strings.CutPrefix(action, customPrefix); ok {
		if v, err := strconv.ParseInt(n, 10, 64); err == nil {
			return t.Get("settings-launcher-kb-custom", i18n.Int("n", v))
		}
	}
	return t.Get("settings-launcher-kb-" + action)
}

type launcherBindings struct {
	*widget.Box
	k        *kit
	slot     slot
	defaults []config.LauncherBinding
	mouse    bool
	rows     []func()
	resets   []*widget.Button
}

var (
	launcherKeysRow  = launcherBindingsRow(config.LauncherDefaultKeybindings, false)
	launcherMouseRow = launcherBindingsRow(config.LauncherDefaultMouseBindings, true)
)

func launcherBindingsRow(defaults []config.LauncherBinding, mouse bool) rowOpt {
	return func(r *rowSpec) {
		r.fullWidth = true
		r.editor = func(k *kit, s slot, _ config.FieldMeta) control {
			return newLauncherBindings(k, s, defaults, mouse)
		}
	}
}

func newLauncherBindings(k *kit, s slot, defaults []config.LauncherBinding, mouse bool) *launcherBindings {
	b := &launcherBindings{Box: gapBox(widget.Column), k: k, slot: s, defaults: defaults, mouse: mouse}
	b.AddClass("launcher-bindings")
	list := widget.NewItemGroup()
	for _, d := range defaults {
		list.Append(b.row(d))
	}
	b.Append(list, false)
	return b
}

func (b *launcherBindings) overrides() map[string]any {
	m, _ := b.slot.get().(map[string]any)
	return m
}

func (b *launcherBindings) keys(d config.LauncherBinding) (string, bool) {
	if v, ok := b.overrides()[d.Action].(string); ok {
		return v, true
	}
	return d.Keys, false
}

func (b *launcherBindings) write(action string, keys *string) {
	next := maps.Clone(b.overrides())
	if next == nil {
		next = map[string]any{}
	}
	if keys == nil {
		delete(next, action)
	} else {
		next[action] = *keys
	}
	_ = b.slot.set(next)
	b.refresh()
}

func (b *launcherBindings) row(d config.LauncherBinding) widget.Widget {
	t := locale.T()
	reset := b.k.iconButton("ld-rotate-ccw-symbolic", "settings-launcher-kb-reset", func() { b.write(d.Action, nil) }, css.Ghost)
	b.resets = append(b.resets, reset)
	var editor widget.Widget
	var sync func(keys string)
	if b.mouse {
		labels := make([]string, len(launcherMouseTokens))
		for i, tok := range launcherMouseTokens {
			labels[i] = t.Get("settings-launcher-mouse-" + strings.ToLower(tok))
		}
		pick := widget.NewDropdown(b.k.face, b.k.textPx(), labels, 0)
		pick.SetAccessibleName(launcherActionLabel(d.Action))
		syncing := false
		pick.OnSelect = func(i int) {
			if !syncing && i >= 0 && i < len(launcherMouseTokens) {
				tok := launcherMouseTokens[i]
				b.write(d.Action, &tok)
			}
		}
		editor = pick
		sync = func(keys string) {
			first, _, _ := strings.Cut(keys, ",")
			syncing = true
			for i, tok := range launcherMouseTokens {
				if tok == strings.TrimSpace(first) {
					pick.SetSelected(i)
				}
			}
			syncing = false
		}
	} else {
		rec := widget.NewShortcutRecorder(b.k.face, "")
		rec.SetAccessibleName(launcherActionLabel(d.Action))
		syncing := false
		rec.OnChanged = func(accel string) {
			if syncing || accel == "" {
				return
			}
			keys := accelToRofi(accel)
			b.write(d.Action, &keys)
		}
		editor = rec
		sync = func(keys string) {
			first, _, _ := strings.Cut(keys, ",")
			syncing = true
			rec.SetAccel(rofiToAccel(first))
			syncing = false
		}
	}
	also := detailLabel(b.k, "")
	row := panelRowWith(b.k, launcherActionLabel(d.Action), also, editor, reset)
	refresh := func() {
		keys, overridden := b.keys(d)
		_, rest, more := strings.Cut(keys, ",")
		also.SetVisible(more)
		if more {
			others := strings.Split(rest, ",")
			for i, key := range others {
				if b.mouse {
					others[i] = strings.TrimSpace(key)
				} else {
					others[i] = rofiToAccel(key)
				}
			}
			also.SetText(t.Get("settings-launcher-kb-also", i18n.Str("keys", strings.Join(others, ", "))))
		}
		reset.SetVisible(overridden)
		sync(keys)
	}
	b.rows = append(b.rows, refresh)
	refresh()
	return row
}

func (b *launcherBindings) refresh() {
	for _, r := range b.rows {
		r()
	}
}
