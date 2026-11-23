package settings

import (
	"cmp"
	"log"
	"maps"
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/portal/permissions"
	"github.com/stubbedev/lys/shell/service/shellipc"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/keys"
	"github.com/stubbedev/lys/settings/internal/locale"
	"github.com/stubbedev/lys/settings/internal/wmconfig"
)

const (
	wmChordFormat  = "lys-wm-chord"
	wmActionFormat = "lys-wm-action"
	chordKey       = "chord"
)

// chordIdentity is what two lys-wm chords pressing the same keys share: the
// chord they parse to, else their text trimmed and in lower case.
type chordIdentity struct {
	chord keys.Chord
	text  string
}

func identifyChord(text string) chordIdentity {
	if c, err := keys.WM.Parse(text); err == nil {
		return chordIdentity{chord: c}
	}
	return chordIdentity{text: strings.ToLower(strings.TrimSpace(text))}
}

// recorderValue is what a recorder holds for a lys-wm chord: its
// accelerator or its tap, nothing for an empty chord; false for a chord
// no press records, a pointer one or one lys-wm does not read.
func recorderValue(chord string) (accel string, tap widget.Tap, ok bool) {
	if chord == "" {
		return "", widget.Tap{}, true
	}
	c, err := keys.WM.Parse(chord)
	if err != nil {
		return "", widget.Tap{}, false
	}
	if accel, ok := c.Accel(); ok {
		return accel, widget.Tap{}, true
	}
	tap, ok = c.Tap()
	return "", tap, ok
}

// showChord shows the lys-wm chord on r, leaving r as it is when it
// shows the chord already; false, r unassigned, for a chord no press
// records.
func showChord(r *widget.ShortcutRecorder, chord string) bool {
	accel, tap, ok := recorderValue(chord)
	shown, _ := r.Tap()
	switch {
	case tap != widget.Tap{}:
		if shown != tap {
			r.SetTap(tap)
		}
	case r.Accel() != accel || shown != widget.Tap{}:
		r.SetAccel(accel)
	}
	return ok
}

// recorderChord is what r holds in lys-wm's form, empty for nothing.
func recorderChord(r *widget.ShortcutRecorder) string {
	if tap, ok := r.Tap(); ok {
		return keys.FromTap(tap).WM()
	}
	accel := r.Accel()
	if accel == "" {
		return ""
	}
	c, err := keys.Accel.Parse(accel)
	if err != nil {
		log.Panicf("settings: the recorder recorded %v", err)
	}
	return c.WM()
}

// recorderUnset is what emptying a recorder does.
type recorderUnset uint8

const (
	// unsetClears empties the value: a clear button beside a value, and
	// Backspace while it records.
	unsetClears recorderUnset = iota
	// unsetResets puts the row back to its default through the row's own
	// reset, so the recorder has no clear button of its own.
	unsetResets
)

// shortcutRecorder is the window's key recorder, named and explained by
// name.
func (k *kit) shortcutRecorder(name string, unset recorderUnset) *widget.ShortcutRecorder {
	r := widget.NewShortcutRecorder(k.face, "")
	r.SetAccessibleName(name)
	r.SetTooltip(name)
	r.SetClearable(unset == unsetClears)
	return r
}

// wmChordRecorder is a recorder of lys-wm chords, named and explained by
// name, which records taps as lys-wm binds them.
func (k *kit) wmChordRecorder(name string) *widget.ShortcutRecorder {
	r := k.shortcutRecorder(name, unsetClears)
	r.SetRecordTaps(true)
	return r
}

// onChordRecorded hands record every chord the user records on r, a
// key or a tap, in lys-wm's form, empty once cleared.
func onChordRecorded(r *widget.ShortcutRecorder, record func(chord string)) {
	r.OnChanged = func(string) { record(recorderChord(r)) }
	r.OnTap = func(widget.Tap) { record(recorderChord(r)) }
}

// keycaps is a chord drawn as lys-ui's keycaps: one picture, its keys
// set apart by the keycaps' own spacing.
type keycaps struct{ *widget.Box }

// keycaps draws labels, a chord's keys as they read, as keycaps.
func (k *kit) keycaps(labels []string) keycaps {
	return keycaps{widget.NewKbdGroup(k.face, labels...)}
}

// chordClashes pairs each chord with another that normal says is the
// same, -1 for none and for an empty one, normalising each chord once.
func chordClashes[K comparable](chords []string, normal func(string) K) []int {
	partner := make([]int, len(chords))
	first := make(map[K]int, len(chords))
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
	e.recorder = k.wmChordRecorder(locale.T().Get("wm-binds-chord"))
	onChordRecorded(e.recorder, func(chord string) {
		if chord == "" {
			e.slot.unset()
			return
		}
		_ = e.slot.set(chord)
	})
	e.text = newText(k, s, false)
	e.AppendAligned(e.recorder, true, widget.AlignCenter)
	e.AppendAligned(e.text, true, widget.AlignCenter)
	e.refresh()
	return e
}

func (e *chordEditor) refresh() {
	byText := !showChord(e.recorder, e.slot.text())
	e.recorder.SetVisible(!byText)
	e.text.SetVisible(byText)
	if byText {
		e.text.refresh()
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
	rows      *widget.ItemGroup
	state     *cardState
	reload    *widget.Button
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
	e.state = k.cardState()
	e.rows = widget.NewItemGroup()
	e.Append(e.rows, false)
	e.reload = k.reloadButton("wm-app-shortcuts-reload", e.load)
	e.reload.SetHAlign(widget.AlignStart)
	e.Append(e.reload, false)
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
	e.rows.Clear()
	e.rows.Append(e.state)
	e.entries, e.shortcuts = e.entries[:0], l.shortcuts
	e.reload.SetVisible(l.err == nil)
	switch {
	case l.err != nil:
		e.state.Error(t.Get("wm-app-shortcuts-unreadable"), l.err, e.load)
	case len(l.shortcuts) == 0:
		e.state.Empty("ld-keyboard-symbolic", t.Get("wm-app-shortcuts-none"))
	default:
		e.state.Clear()
	}
	for _, sc := range l.shortcuts {
		name := cmp.Or(sc.Description, sc.ID)
		row := e.k.row(name, sc.app)
		var entry *actionChord
		if sc.Trigger != "" {
			entry = newHeldChord(e.k, e.names, sc.action(), heldChord(sc))
			row.withControl(e.k.keycaps(keys.Accel.Labels(sc.Trigger)))
			row.SetTooltip(t.Get("wm-app-shortcuts-hotkey"))
		} else {
			entry = newActionChord(e.k, e.binds, e.names, sc.action(), name)
			row.withControl(entry.recorder)
		}
		row.AddDetail(entry.clash)
		e.entries = append(e.entries, entry)
		e.rows.Append(row)
	}
	e.refresh()
}

func (e *appShortcutsEditor) refresh() {
	for _, entry := range e.entries {
		entry.refresh(e.binds)
	}
}

// heldChord is the key an app holds itself in lys-wm's form, for the
// clash check; empty, and logged, for a trigger that is no accelerator.
func heldChord(sc appShortcut) string {
	c, err := keys.Accel.Parse(sc.Trigger)
	if err != nil {
		log.Printf("settings: %s's shortcut %s: %v", sc.app, sc.ID, err)
		return ""
	}
	return c.WM()
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
	c.recorder = k.wmChordRecorder(name)
	onChordRecorded(c.recorder, func(chord string) { _ = binds.set(rebound(bindItems(binds), action, chord)) })
	return c
}

// newHeldChord is the key an app holds itself for action, which a bind
// for the action replaces.
func newHeldChord(k *kit, names *bindNames, action, held string) *actionChord {
	c := &actionChord{k: k, names: names, action: action, held: held, clash: k.label("", css.Caption, css.Warning)}
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

// clashText says the lys-wm chord is also bound by other, the chord as
// it reads.
func clashText(chord, other string) string {
	return locale.T().Get("settings-clash", i18n.Str("value", keys.WM.Text(chord)), i18n.Str("other", other))
}

func (c *actionChord) refresh(binds slot) {
	items := bindItems(binds)
	chord := c.chord(items)
	rest := slices.DeleteFunc(slices.Clone(items), func(it any) bool { return bindAction(it) == c.action })
	owner, clashes := clashWith(chord, append(bindOwners(rest, c.names, ""), submapOwners(c.k.wm, c.names)...))
	c.clash.SetVisible(clashes)
	if clashes {
		c.clash.SetText(clashText(chord, owner.owner))
	}
	if c.recorder == nil || c.recorder.Recording() {
		return
	}
	showChord(c.recorder, chord)
}

// launcherAction is the bind lys-shell opens the launcher on.
var launcherAction = appShortcut{app: shellipc.AppID, ID: shellipc.ShortcutLauncher}.action()

const launcherShortcutKey = "settings-launcher-shortcut"

// launcherShortcutRow is the chord lys-wm opens the launcher on, recorded
// in place, above the way to every other shortcut.
func launcherShortcutRow() rowSpec {
	return wmField(bindsKey, withKey(launcherShortcutKey), func(r *rowSpec) {
		r.action, r.fullWidth = true, true
		r.editor = func(k *kit, s slot, _ config.FieldMeta) control {
			return newBindRecorders(k, s, []bindRecorder{{action: launcherAction, key: "settings-action-launcher"}})
		}
	})
}

// bindRecorder is one bind action a recorder rebinds, the message key
// titling its row.
type bindRecorder struct{ action, key string }

// bindRecorders is a row with a recorder for each bind action, then the
// row leading to every other shortcut.
type bindRecorders struct {
	*widget.Box
	binds  slot
	chords []*actionChord
}

// allShortcutsLink ends every list of shortcut recorders.
var allShortcutsLink = pageLink{wmBindsID, "settings-shortcuts-all-detail"}

func newBindRecorders(k *kit, binds slot, recorders []bindRecorder) *bindRecorders {
	t := locale.T()
	names := bindActionNames(k)
	b := &bindRecorders{Box: gapBox(widget.Column), binds: binds}
	rows := widget.NewItemGroup()
	for _, r := range recorders {
		chord := newActionChord(k, binds, names, r.action, t.Get(r.key))
		b.chords = append(b.chords, chord)
		row := k.row(t.Get(r.key), "").withControl(chord.recorder)
		row.AddDetail(chord.clash)
		rows.Append(row)
	}
	if all, ok := k.pageLinkRow(allShortcutsLink); ok {
		rows.Append(all)
	}
	b.Append(rows, false)
	b.refresh()
	return b
}

func (b *bindRecorders) refresh() {
	for _, c := range b.chords {
		c.refresh(b.binds)
	}
}
