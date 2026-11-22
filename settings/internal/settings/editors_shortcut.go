package settings

import (
	"cmp"
	"maps"
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/portal/permissions"
	"github.com/stubbedev/lys/shell/service/shellipc"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
	"github.com/stubbedev/lys/settings/internal/wmconfig"
)

const (
	wmChordFormat  = "lys-wm-chord"
	wmActionFormat = "lys-wm-action"
	chordJoin      = " + "
	chordKey       = "chord"
)

type chordModifier struct {
	accel, chord string
	aliases      []string
}

var chordModifiers = []chordModifier{
	{accel: "Ctrl", chord: "CTRL", aliases: []string{"CTRL", "CONTROL"}},
	{accel: "Shift", chord: "SHIFT", aliases: []string{"SHIFT"}},
	{accel: "Alt", chord: "ALT", aliases: []string{"ALT", "MOD1"}},
	{accel: "Super", chord: "SUPER", aliases: []string{"SUPER", "MOD4", "WIN", "LOGO"}},
}

func modifierByAccel(name string) (chordModifier, bool) {
	i := slices.IndexFunc(chordModifiers, func(m chordModifier) bool { return m.accel == name })
	if i < 0 {
		return chordModifier{}, false
	}
	return chordModifiers[i], true
}

func modifierByChord(name string) (int, bool) {
	upper := strings.ToUpper(name)
	i := slices.IndexFunc(chordModifiers, func(m chordModifier) bool { return slices.Contains(m.aliases, upper) })
	return i, i >= 0
}

var chordOrder = []string{"Super", "Ctrl", "Alt", "Shift"}

func chordFromAccel(accel string) string {
	if accel == "" {
		return ""
	}
	parts := strings.Split(accel, "+")
	held := parts[:len(parts)-1]
	out := make([]string, 0, len(parts))
	for _, name := range chordOrder {
		if m, ok := modifierByAccel(name); ok && slices.Contains(held, name) {
			out = append(out, m.chord)
		}
	}
	return strings.Join(append(out, parts[len(parts)-1]), chordJoin)
}

type parsedChord struct {
	mods uint8
	key  string
}

func parseChord(chord string) (parsedChord, bool) {
	parts := strings.Split(chord, "+")
	var c parsedChord
	for _, p := range parts[:len(parts)-1] {
		i, ok := modifierByChord(strings.TrimSpace(p))
		if !ok {
			return parsedChord{}, false
		}
		c.mods |= 1 << i
	}
	c.key = strings.TrimSpace(parts[len(parts)-1])
	return c, c.key != ""
}

func (c parsedChord) mouse() bool {
	lower := strings.ToLower(c.key)
	return strings.HasPrefix(lower, "mouse:") || lower == "mouse_up" || lower == "mouse_down"
}

func (c parsedChord) accel() string {
	parts := make([]string, 0, len(chordModifiers)+1)
	for i, m := range chordModifiers {
		if c.mods&(1<<i) != 0 {
			parts = append(parts, m.accel)
		}
	}
	key := c.key
	if len([]rune(key)) == 1 {
		key = strings.ToUpper(key)
	}
	return strings.Join(append(parts, key), "+")
}

func normalChord(chord string) string {
	c, ok := parseChord(chord)
	if !ok {
		return strings.ToLower(strings.TrimSpace(chord))
	}
	if len([]rune(c.key)) == 1 {
		c.key = strings.ToLower(c.key)
	}
	return string(rune('0'+c.mods)) + c.key
}

func chordClashes(chords []string, normal func(string) string) []int {
	partner := make([]int, len(chords))
	first := make(map[string]int, len(chords))
	for i, chord := range chords {
		partner[i] = -1
		if chord == "" {
			continue
		}
		n := normal(chord)
		j, seen := first[n]
		if !seen {
			first[n] = i
			continue
		}
		partner[i] = j
		if partner[j] < 0 {
			partner[j] = i
		}
	}
	return partner
}

type chordEditor struct {
	*widget.Box
	k        *kit
	slot     slot
	recorder *widget.ShortcutRecorder
	text     *text
}

func newChordEditor(k *kit, s slot) *chordEditor {
	e := &chordEditor{Box: gapBox(widget.Row), k: k, slot: s}
	e.recorder = widget.NewShortcutRecorder(k.face, "")
	e.recorder.SetAccessibleName(locale.T().Get("wm-binds-chord"))
	e.recorder.OnChanged = func(accel string) {
		if accel == "" {
			e.slot.unset()
			return
		}
		_ = e.slot.set(chordFromAccel(accel))
	}
	e.text = newText(k, s, false)
	e.AppendAligned(e.recorder, true, widget.AlignCenter)
	e.AppendAligned(e.text, true, widget.AlignCenter)
	e.refresh()
	return e
}

func (e *chordEditor) refresh() {
	stored := e.slot.text()
	c, ok := parseChord(stored)
	byText := stored != "" && (!ok || c.mouse())
	e.recorder.SetVisible(!byText)
	e.text.SetVisible(byText)
	if byText {
		e.text.refresh()
		return
	}
	if accel := c.accel(); ok && accel != e.recorder.Accel() {
		e.recorder.SetAccel(accel)
	} else if !ok && e.recorder.Accel() != "" {
		e.recorder.SetAccel("")
	}
}

const globalAction = "global"

type appShortcut struct {
	app string
	permissions.Shortcut
}

func (a appShortcut) action() string { return globalAction + " " + a.app + ":" + a.ID }

var readAppShortcuts = func() ([]appShortcut, error) {
	path := permissions.ShortcutsPath()
	if path == "" {
		return nil, nil
	}
	all, err := permissions.ReadShortcuts(path)
	if err != nil {
		return nil, err
	}
	var out []appShortcut
	for _, app := range slices.Sorted(maps.Keys(all)) {
		for _, sc := range all[app] {
			out = append(out, appShortcut{app: app, Shortcut: sc})
		}
	}
	return out, nil
}

type appShortcutsListing struct {
	shortcuts []appShortcut
	err       error
}

type appShortcutsEditor struct {
	*widget.Box
	k         *kit
	binds     slot
	list      *widget.Box
	empty     *widget.Label
	entries   []*actionChord
	shortcuts []appShortcut
	names     *bindNames
}

func appShortcutsRow() rowSpec {
	return wmField(bindsKey, withKey("wm-app-shortcuts"), func(r *rowSpec) {
		r.fullWidth, r.action = true, true
		r.editor = func(k *kit, s slot, _ config.FieldMeta) control { return newAppShortcutsEditor(k, s) }
	})
}

func bindActionNames(k *kit) *bindNames {
	action, _ := k.wm.fieldOf(bindsKey + "." + wmconfig.Each + "." + actionKey)
	return &bindNames{actions: action.Actions}
}

func newAppShortcutsEditor(k *kit, binds slot) *appShortcutsEditor {
	e := &appShortcutsEditor{Box: gapBox(widget.Column), k: k, binds: binds, names: bindActionNames(k)}
	e.list = gapBox(widget.Column)
	e.empty = k.label(locale.T().Get("wm-app-shortcuts-none"), "setting-description", css.Caption, css.DimLabel)
	e.Append(e.empty, false)
	e.Append(e.list, false)
	reload := k.reloadButton("wm-app-shortcuts-reload", e.load)
	reload.SetHAlign(widget.AlignStart)
	e.Append(reload, false)
	e.load()
	return e
}

func (e *appShortcutsEditor) load() {
	async(e.k, func() appShortcutsListing {
		s, err := readAppShortcuts()
		return appShortcutsListing{s, err}
	}, e.show)
}

func (e *appShortcutsEditor) show(l appShortcutsListing) {
	t := locale.T()
	e.list.Clear()
	e.entries, e.shortcuts = e.entries[:0], l.shortcuts
	switch {
	case l.err != nil:
		e.empty.SetText(l.err.Error())
	case len(l.shortcuts) == 0:
		e.empty.SetText(t.Get("wm-app-shortcuts-none"))
	}
	e.empty.SetVisible(l.err != nil || len(l.shortcuts) == 0)
	for _, sc := range l.shortcuts {
		row := gapBox(widget.Row)
		row.AddClass("app-shortcut")
		text := widget.NewBox(widget.Column, widget.Current().Spacing/4, 0)
		text.Append(e.k.label(cmp.Or(sc.Description, sc.ID), "setting-label"), false)
		text.Append(e.k.label(sc.app, "setting-description", css.Caption, css.DimLabel), false)
		row.AppendAligned(text, true, widget.AlignCenter)
		var entry *actionChord
		if sc.Trigger != "" {
			entry = newHeldChord(e.k, e.names, sc.action(), chordFromAccel(sc.Trigger))
			row.AppendAligned(widget.NewShortcutLabel(e.k.face, sc.Trigger), false, widget.AlignCenter)
			row.SetTooltip(t.Get("wm-app-shortcuts-hotkey"))
		} else {
			entry = newActionChord(e.k, e.binds, e.names, sc.action(), cmp.Or(sc.Description, sc.ID))
			row.AppendAligned(entry.recorder, false, widget.AlignCenter)
		}
		e.entries = append(e.entries, entry)
		box := gapBox(widget.Column)
		box.Append(row, false)
		box.Append(entry.clash, false)
		e.list.Append(box, false)
	}
	e.refresh()
}

func (e *appShortcutsEditor) refresh() {
	for _, entry := range e.entries {
		entry.refresh(e.binds)
	}
}

func bindAction(item any) string {
	m, _ := item.(map[string]any)
	a, _ := m[actionKey].(string)
	return strings.Join(strings.Fields(a), " ")
}

// actionChord is the chord one bind action is pressed with, a recorder
// rebinding it or the key an app holds itself, and what else holds that
// chord.
type actionChord struct {
	k        *kit
	names    *bindNames
	action   string
	held     string
	recorder *widget.ShortcutRecorder
	clash    *widget.Label
}

func newActionChord(k *kit, binds slot, names *bindNames, action, name string) *actionChord {
	c := newHeldChord(k, names, action, "")
	c.recorder = widget.NewShortcutRecorder(k.face, "")
	c.recorder.SetAccessibleName(name)
	c.recorder.OnChanged = func(accel string) { _ = binds.set(rebound(bindItems(binds), action, chordFromAccel(accel))) }
	return c
}

// newHeldChord is the key an app holds itself for action, which a bind
// for the action replaces.
func newHeldChord(k *kit, names *bindNames, action, held string) *actionChord {
	c := &actionChord{k: k, names: names, action: action, held: held, clash: k.label("", "setting-problem", css.Caption, css.Warning)}
	c.clash.SetWrap(true)
	c.clash.SetVisible(false)
	return c
}

func bindItems(binds slot) []any {
	items, _ := binds.get().([]any)
	return slices.Clone(items)
}

// rebound is items with action bound to chord alone, or to nothing for
// an empty chord.
func rebound(items []any, action, chord string) []any {
	items = slices.DeleteFunc(items, func(it any) bool { return bindAction(it) == action })
	if chord == "" {
		return items
	}
	return append(items, map[string]any{chordKey: chord, actionKey: action})
}

// chord is what the action is pressed with: its bind's chord, else the
// key its app holds.
func (c *actionChord) chord(items []any) string {
	j := slices.IndexFunc(items, func(it any) bool { return bindAction(it) == c.action })
	if j < 0 {
		return c.held
	}
	m, _ := items[j].(map[string]any)
	chord, _ := m[chordKey].(string)
	return chord
}

func (c *actionChord) refresh(binds slot) {
	items := bindItems(binds)
	chord := c.chord(items)
	rest := slices.DeleteFunc(slices.Clone(items), func(it any) bool { return bindAction(it) == c.action })
	owner, clashes := clashWith(chord, append(bindOwners(rest, c.names, ""), submapOwners(c.k.wm, c.names)...))
	c.clash.SetVisible(clashes)
	if clashes {
		c.clash.SetText(locale.T().Get("settings-clash", i18n.Str("value", chord), i18n.Str("other", owner.owner)))
	}
	if c.recorder == nil || c.recorder.Recording() {
		return
	}
	accel := ""
	if parsed, ok := parseChord(chord); ok {
		accel = parsed.accel()
	}
	if c.recorder.Accel() != accel {
		c.recorder.SetAccel(accel)
	}
}

// launcherAction is the bind lys-shell opens the launcher on.
var launcherAction = appShortcut{app: shellipc.AppID, ID: shellipc.ShortcutLauncher}.action()

const launcherShortcutKey = "settings-launcher-shortcut"

// launcherShortcutRow is the chord lys-wm opens the launcher on, recorded
// in place, beside a way to every other shortcut.
func launcherShortcutRow() rowSpec {
	return wmField(bindsKey, withKey(launcherShortcutKey), func(r *rowSpec) {
		r.action = true
		r.editor = func(k *kit, s slot, _ config.FieldMeta) control { return newLauncherShortcut(k, s) }
	})
}

type launcherShortcut struct {
	*widget.Box
	binds slot
	chord *actionChord
}

func newLauncherShortcut(k *kit, binds slot) *launcherShortcut {
	l := &launcherShortcut{Box: gapBox(widget.Column), binds: binds}
	l.chord = newActionChord(k, binds, bindActionNames(k), launcherAction, locale.T().Get(launcherShortcutKey))
	row := gapBox(widget.Row)
	row.AppendAligned(l.chord.recorder, true, widget.AlignCenter)
	if link, ok := allShortcutsLink(k); ok {
		row.AppendAligned(link, false, widget.AlignCenter)
	}
	l.Append(row, false)
	l.Append(l.chord.clash, false)
	l.refresh()
	return l
}

func (l *launcherShortcut) refresh() { l.chord.refresh(l.binds) }

func allShortcutsLink(k *kit) (*widget.Button, bool) {
	target, ok := pageByID(k.nav, wmBindsID)
	if !ok {
		return nil, false
	}
	return k.iconButton("ld-chevron-right-symbolic", "settings-shortcuts-all", func() { k.openPage(target.id) }), true
}
