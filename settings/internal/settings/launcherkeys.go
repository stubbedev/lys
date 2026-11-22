package settings

import (
	"log"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/keys"
	"github.com/stubbedev/lys/settings/internal/locale"
)

const customPrefix = "custom-"

var launcherMouseTokens = []string{"MousePrimary", "MouseDPrimary", "MouseSecondary", "MouseDSecondary", "MouseMiddle", "ScrollUp", "ScrollDown", "ScrollLeft", "ScrollRight"}

// launcherKeySeparator parts the keys of one launcher action.
const launcherKeySeparator = ","

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
	resets   []resetButton
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
	reset := b.k.resetButton("settings-launcher-kb-reset", func() { b.write(d.Action, nil) })
	b.resets = append(b.resets, reset)
	var editor widget.Widget
	var sync func(first string)
	if b.mouse {
		editor, sync = b.mousePick(d)
	} else {
		editor, sync = b.keyRecorder(d)
	}
	row := b.k.row(launcherActionLabel(d.Action), "").withActions(reset.Button).withControl(editor)
	also := gapBox(widget.Row)
	row.AddDetail(also)
	refresh := func() {
		stored, overridden := b.keys(d)
		first, rest, more := strings.Cut(stored, launcherKeySeparator)
		also.Clear()
		also.SetVisible(more)
		if more {
			b.showAlso(also, strings.Split(rest, launcherKeySeparator))
		}
		reset.show(overridden)
		sync(strings.TrimSpace(first))
	}
	b.rows = append(b.rows, refresh)
	refresh()
	return row
}

// keyRecorder records an action's first key; emptying it puts the
// action back to its default keys.
func (b *launcherBindings) keyRecorder(d config.LauncherBinding) (widget.Widget, func(first string)) {
	rec := b.k.shortcutRecorder(launcherActionLabel(d.Action), unsetResets)
	syncing := false
	rec.OnChanged = func(accel string) {
		if syncing {
			return
		}
		if accel == "" {
			b.write(d.Action, nil)
			return
		}
		c, err := keys.Accel.Parse(accel)
		if err != nil {
			log.Panicf("settings: the recorder recorded %v", err)
		}
		rofi, _ := c.Rofi()
		b.write(d.Action, &rofi)
	}
	return rec, func(first string) {
		syncing = true
		defer func() { syncing = false }()
		c, err := keys.Rofi.Parse(first)
		if err != nil {
			rec.SetAccel("")
			rec.SetError(locale.T().Get("settings-launcher-kb-unreadable", i18n.Str("keys", keys.Rofi.Text(first))))
			return
		}
		accel, _ := c.Accel()
		rec.SetAccel(accel)
	}
}

// mousePick picks an action's first pointer binding from the buttons.
func (b *launcherBindings) mousePick(d config.LauncherBinding) (widget.Widget, func(first string)) {
	pick := widget.NewDropdown(b.k.face, b.k.textPx(), launcherMouseLabels(launcherMouseTokens), 0)
	pick.SetAccessibleName(launcherActionLabel(d.Action))
	syncing := false
	pick.OnSelect = func(i int) {
		if !syncing && i >= 0 && i < len(launcherMouseTokens) {
			tok := launcherMouseTokens[i]
			b.write(d.Action, &tok)
		}
	}
	return pick, func(first string) {
		syncing = true
		defer func() { syncing = false }()
		if i := slices.Index(launcherMouseTokens, first); i >= 0 {
			pick.SetSelected(i)
		}
	}
}

// showAlso draws an action's further keys after the first in also, as
// keycaps after the word saying they work too.
func (b *launcherBindings) showAlso(also *widget.Box, more []string) {
	also.AppendAligned(b.k.label(locale.T().Get("settings-launcher-kb-also"), css.Caption, css.DimLabel), false, widget.AlignCenter)
	for _, key := range more {
		labels := keys.Rofi.Labels(strings.TrimSpace(key))
		if b.mouse {
			labels = launcherMouseLabels(labels)
		}
		also.AppendAligned(b.k.keycaps(labels), false, widget.AlignCenter)
	}
}

// launcherMouseLabels are parts of a pointer binding as they read: a
// rofi mouse name by its message, any other part as it is.
func launcherMouseLabels(parts []string) []string {
	t := locale.T()
	out := make([]string, len(parts))
	for i, p := range parts {
		out[i] = p
		if slices.Contains(launcherMouseTokens, p) {
			out[i] = t.Get("settings-launcher-mouse-" + strings.ToLower(p))
		}
	}
	return out
}

func (b *launcherBindings) refresh() {
	for _, r := range b.rows {
		r()
	}
}
