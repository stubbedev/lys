package settings

import (
	"errors"
	"maps"
	"reflect"
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/desktopentry"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
	"github.com/stubbedev/lys/settings/internal/wmconfig"
)

const (
	submapAction = "submap"
	// leaveModeName is the argument of lys-wm's submap action that goes
	// back to the binds outside every mode.
	leaveModeName = "reset"
	leaveModeKey  = "Escape"
)

func enterModeAction(name string) string { return submapAction + " " + name }

// shortcuts are lys-wm's binds and its modes (submaps), each a list of binds
// by name, as one value the mode edits change together. below are the modes
// the config under the overrides defines: an override can only cover one, so
// one of theirs that is dropped is written as null, which lys-wm reads as a
// mode with no binds and the table as no mode.
type shortcuts struct {
	binds []any
	modes map[string]any
	below []string
}

func (s shortcuts) names() []string {
	var out []string
	for _, name := range slices.Sorted(maps.Keys(s.modes)) {
		if s.modes[name] != nil {
			out = append(out, name)
		}
	}
	return out
}

func (s shortcuts) drop(name string) {
	if slices.Contains(s.below, name) {
		s.modes[name] = nil
		return
	}
	delete(s.modes, name)
}

// parseModeName is text as the name of a new mode beside taken: neither the
// reset lys-wm's submap action leaves every mode with nor a name already
// taken; lys-wm's own check says what else it refuses.
func parseModeName(text string, taken []string) (string, error) {
	t := locale.T()
	name := strings.TrimSpace(text)
	switch {
	case name == "":
		return "", errors.New(t.Get("wm-mode-name-missing"))
	case name == leaveModeName:
		return "", errors.New(t.Get("wm-mode-name-reset", i18n.Str("name", name)))
	case slices.Contains(taken, name):
		return "", errors.New(t.Get("wm-mode-name-taken", i18n.Str("name", name)))
	}
	return name, nil
}

// entersMode reports whether item is a bind entering the mode name.
func entersMode(item any, name string) bool {
	a := splitAction(bindAction(item))
	return a.name == submapAction && len(a.args) == 1 && a.args[0] == name
}

// eachList is every list of binds: the binds, then each mode's in name order.
func (s shortcuts) eachList(edit func(items []any) []any) shortcuts {
	out := shortcuts{binds: edit(s.binds), modes: make(map[string]any, len(s.modes)), below: s.below}
	for name, items := range s.modes {
		out.modes[name] = nil
		if list, ok := items.([]any); ok {
			out.modes[name] = edit(list)
		}
	}
	return out
}

// withMode is s with a mode name, left by Escape, entered by chord.
func (s shortcuts) withMode(name, chord string) shortcuts {
	out := s.eachList(slices.Clone)
	out.modes[name] = []any{map[string]any{chordKey: leaveModeKey, actionKey: enterModeAction(leaveModeName)}}
	out.binds = append(out.binds, map[string]any{chordKey: chord, actionKey: enterModeAction(name)})
	return out
}

// renamed is s with the mode from named to, every bind entering it entering
// it under that name, each bind otherwise as it was.
func (s shortcuts) renamed(from, to string) shortcuts {
	out := s.eachList(func(items []any) []any {
		next := slices.Clone(items)
		for i, it := range next {
			if !entersMode(it, from) {
				continue
			}
			m, _ := it.(map[string]any)
			m = maps.Clone(m)
			m[actionKey] = enterModeAction(to)
			next[i] = m
		}
		return next
	})
	out.modes[to] = out.modes[from]
	out.drop(from)
	return out
}

// without is s without the mode name and the binds that enter it.
func (s shortcuts) without(name string) shortcuts {
	out := s.eachList(func(items []any) []any {
		return slices.DeleteFunc(slices.Clone(items), func(it any) bool { return entersMode(it, name) })
	})
	out.drop(name)
	return out
}

// edits are the writes taking was to s: one per key that changed.
func (s shortcuts) edits(was shortcuts) []wmconfig.Edit {
	var out []wmconfig.Edit
	if !reflect.DeepEqual(s.binds, was.binds) {
		out = append(out, wmconfig.Edit{Path: []string{bindsKey}, Value: s.binds})
	}
	if !reflect.DeepEqual(s.modes, was.modes) {
		out = append(out, wmconfig.Edit{Path: []string{submapsKey}, Value: s.modes})
	}
	return out
}

// wmShortcutsRow is the shortcut table: lys-wm's binds filed by what they do,
// then each mode as a group of its own binds under its name and the chord
// that enters it.
func wmShortcutsRow() rowSpec {
	return wmField(bindsKey, func(r *rowSpec) {
		r.fullWidth = true
		r.also = []string{submapsKey}
		r.editor = func(k *kit, s slot, _ config.FieldMeta) control { return newShortcutTable(k, s) }
	})
}

type shortcutTable struct {
	*widget.Box
	k      *kit
	binds  slot
	modes  slot
	names  *bindNames
	inMode *bindNames
	filter *widget.SearchEntry
	main   *cardList
	groups *widget.Box
	shown  []string
	group  []*modeGroup
	add    *widget.Button
	adding *modeForm
	// problem says why the last mode deletion did not land.
	problem *widget.Label
}

func newShortcutTable(k *kit, binds slot) *shortcutTable {
	t := locale.T()
	actions := func(list string) []wmconfig.Action {
		f, _ := k.wm.fieldOf(list + "." + wmconfig.Each + "." + actionKey)
		return f.Actions
	}
	modeActions, _ := k.wm.schema.Field([]string{submapsKey, wmconfig.Each, wmconfig.Each, actionKey})
	tb := &shortcutTable{
		Box: gapBox(widget.Column), k: k, binds: binds, modes: k.wm.slot(submapsKey),
		names: &bindNames{actions: actions(bindsKey)}, inMode: &bindNames{actions: modeActions.Actions},
	}
	tb.filter = widget.NewSearchEntry(k.face, k.textPx(), t.Get("wm-binds-search"))
	tb.filter.SetDelay(0)
	tb.filter.OnSearchChanged = func(string) { tb.applyFilter() }
	tb.Append(tb.filter, false)
	check := k.wm.checker(bindsKey)
	main := tb.bindCards(tb.names, []string{bindsKey, wmconfig.Each}, func(items []any) func() error { return check(items) }, func() []clashOwner { return submapOwners(k.wm, tb.inMode) })
	main.group = tb.names.group
	main.resetKey = "wm-binds-reset"
	main.reset = func(items []map[string]any, i int) (map[string]any, bool) {
		builtIn, _ := k.wm.defaultOf(bindsKey)
		defaults, _ := builtIn.([]any)
		return bindDefault(items, i, defaults)
	}
	tb.main = newCardList(k, binds, main)
	tb.Append(tb.main, false)
	tb.groups = gapBox(widget.Column)
	tb.Append(tb.groups, false)
	tb.adding = newModeForm(k, "wm-mode-add-confirm", true, tb.addMode, func() { tb.showAdding(false) })
	tb.Append(tb.adding, false)
	tb.add = k.addButton("wm-mode-add", func() { tb.showAdding(true) })
	tb.AppendAligned(tb.add, false, widget.AlignStart)
	tb.problem = k.errorText("")
	tb.problem.SetVisible(false)
	tb.Append(tb.problem, false)
	tb.showAdding(false)
	tb.buildModes()
	dirs := desktopentry.ApplicationDirs()
	async(k, func() installedApps { return scanInstalled(dirs) }, func(ix installedApps) {
		launches := launchNames(ix)
		tb.names.launches, tb.inMode.launches = launches, launches
		tb.main.rebuild()
		for _, g := range tb.group {
			g.list.rebuild()
		}
		tb.applyFilter()
	})
	return tb
}

// bindCards is the card list of a list of binds whose items' schema is at
// item: titled by what each does, found by the table's search, its chords
// checked against others too.
func (tb *shortcutTable) bindCards(names *bindNames, item []string, check func([]any) func() error, others func() []clashOwner) cardSpec {
	return cardSpec{
		check:       check,
		title:       names.bindLabel,
		fields:      wmItemFields(tb.k.wm, item, nil, bindsKey),
		blank:       func() map[string]any { return map[string]any{} },
		search:      names.searchText,
		filter:      tb.filter,
		collapsed:   true,
		aside:       func(k *kit) cardAside { return newBindAside(k, names) },
		clashKey:    chordKey,
		clashOthers: others,
		keep:        func(m map[string]any) bool { chord, _ := m[chordKey].(string); return chord != "" },
	}
}

func (tb *shortcutTable) current() shortcuts {
	binds, _ := tb.binds.get().([]any)
	modes, _ := tb.modes.get().(map[string]any)
	below, _ := tb.k.wm.baseValue(submapsKey).(map[string]any)
	return shortcuts{binds: binds, modes: modes, below: slices.Sorted(maps.Keys(below))}
}

// apply writes next over what is stored once lys-wm would take it, else says
// in problem why it would not; done runs once it is written.
func (tb *shortcutTable) apply(next shortcuts, problem *widget.Label, done func()) {
	edits := next.edits(tb.current())
	if len(edits) == 0 {
		done()
		return
	}
	async(tb.k, tb.k.wm.checkEdits(edits...), func(err error) {
		if err != nil {
			problem.SetText(strings.TrimSpace(err.Error()))
			problem.SetVisible(true)
			return
		}
		problem.SetVisible(false)
		if err := tb.k.wm.edit(edits...); err != nil {
			problem.SetText(err.Error())
			problem.SetVisible(true)
			return
		}
		done()
		tb.refresh()
	})
}

func (tb *shortcutTable) showAdding(open bool) {
	tb.adding.reset("")
	tb.adding.SetVisible(open)
	tb.add.SetVisible(!open)
}

// owners are the chords every bind and every mode's binds hold.
func (tb *shortcutTable) owners() []clashOwner {
	return append(bindOwners(tb.binds.get(), tb.names, ""), submapOwners(tb.k.wm, tb.inMode)...)
}

func (tb *shortcutTable) addMode(text, chord string) {
	now := tb.current()
	name, err := parseModeName(text, now.names())
	if err != nil {
		tb.adding.fail(err)
		return
	}
	if chord == "" {
		tb.adding.fail(errors.New(locale.T().Get("wm-mode-chord-missing")))
		return
	}
	if owner, clashes := clashWith(chord, tb.owners()); clashes {
		tb.adding.fail(errors.New(clashText(chord, owner.owner)))
		return
	}
	tb.apply(now.withMode(name, chord), tb.adding.problem, func() { tb.showAdding(false) })
}

func (tb *shortcutTable) renameMode(g *modeGroup, text string) {
	now := tb.current()
	if strings.TrimSpace(text) == g.name {
		g.showRenaming(false)
		return
	}
	name, err := parseModeName(text, now.names())
	if err != nil {
		g.form.fail(err)
		return
	}
	tb.apply(now.renamed(g.name, name), g.form.problem, func() { g.showRenaming(false) })
}

func (tb *shortcutTable) deleteMode(name string) {
	if tb.k.pickers == nil {
		tb.problem.SetText("settings: delete mode " + name + ": no window to ask in")
		tb.problem.SetVisible(true)
		return
	}
	tb.k.pickers.dialog(confirmDialog(tb.k, "wm-mode-delete", func() {
		tb.apply(tb.current().without(name), tb.problem, func() {})
	}, i18n.Str("mode", name)))
}

// buildModes makes a group per mode stored, in name order.
func (tb *shortcutTable) buildModes() {
	tb.groups.Clear()
	tb.group = tb.group[:0]
	tb.shown = tb.current().names()
	for _, name := range tb.shown {
		g := newModeGroup(tb, name)
		tb.group = append(tb.group, g)
		tb.groups.Append(g.root, false)
	}
	tb.applyFilter()
}

func (tb *shortcutTable) refresh() {
	tb.main.refresh()
	if !slices.Equal(tb.current().names(), tb.shown) {
		tb.buildModes()
		return
	}
	for _, g := range tb.group {
		g.refresh()
	}
	tb.applyFilter()
}

// discardDraft drops the binds' draft, as the row's reset puts the binds back.
func (tb *shortcutTable) discardDraft() { tb.main.discardDraft() }

// applyFilter narrows every list to the binds the search finds, and shows a
// mode while the search is empty or finds one of its binds.
func (tb *shortcutTable) applyFilter() {
	tb.main.applyFilter()
	searching := strings.TrimSpace(tb.filter.Text()) != ""
	for _, g := range tb.group {
		g.list.applyFilter()
		g.root.SetVisible(!searching || g.list.anyShown())
	}
}

// modeGroup is one mode in the table: its name, the chord entering it, its
// rename and delete, and its binds.
type modeGroup struct {
	tb     *shortcutTable
	name   string
	root   *widget.Box
	header *widget.Box
	enter  *actionChord
	none   *widget.Label
	form   *modeForm
	list   *cardList
}

func newModeGroup(tb *shortcutTable, name string) *modeGroup {
	t := locale.T()
	k := tb.k
	g := &modeGroup{tb: tb, name: name, root: gapBox(widget.Column), header: gapBox(widget.Row)}
	g.root.AddClass("card-group", "wm-mode")
	title := t.Get("wm-mode", i18n.Str("mode", name))
	heading := k.label(title, "card-group-title", css.Heading)
	heading.SetHAlign(widget.AlignStart)
	g.header.AppendAligned(heading, true, widget.AlignCenter)
	g.enter = newActionChord(k, tb.binds, tb.names, enterModeAction(name), t.Get("wm-mode-enter", i18n.Str("mode", name)))
	g.header.AppendAligned(g.enter.recorder, false, widget.AlignCenter)
	g.header.AppendAligned(k.iconButton("ld-pencil-symbolic", "wm-mode-rename", func() { g.showRenaming(true) }), false, widget.AlignCenter)
	g.header.AppendAligned(k.removeButton("wm-mode-delete-button", func() { tb.deleteMode(name) }), false, widget.AlignCenter)
	g.root.Append(g.header, false)
	g.form = newModeForm(k, "wm-mode-rename-confirm", false, func(text, _ string) { tb.renameMode(g, text) }, func() { g.showRenaming(false) })
	g.form.SetVisible(false)
	g.root.Append(g.form, false)
	g.none = wrapped(k.label(t.Get("wm-mode-unreachable"), css.Caption, css.Warning))
	g.root.Append(g.none, false)
	g.root.Append(g.enter.clash, false)
	spec := tb.bindCards(tb.inMode, []string{submapsKey, wmconfig.Each, wmconfig.Each}, func(items []any) func() error {
		modes := maps.Clone(tb.current().modes)
		if modes == nil {
			modes = map[string]any{}
		}
		modes[name] = items
		return k.wm.checker(submapsKey)(modes)
	}, func() []clashOwner { return bindOwners(tb.binds.get(), tb.names, "") })
	spec.search = func(item map[string]any) string { return tb.inMode.searchText(item) + " " + title }
	g.list = newCardList(k, nestedSlot(tb.modes, []string{name}), spec)
	g.root.Append(g.list, false)
	g.refresh()
	return g
}

func (g *modeGroup) showRenaming(open bool) {
	g.form.reset(g.name)
	g.form.SetVisible(open)
	g.header.SetVisible(!open)
}

func (g *modeGroup) refresh() {
	g.list.refresh()
	g.enter.refresh(g.tb.binds)
	g.none.SetVisible(!slices.ContainsFunc(bindItems(g.tb.binds), func(it any) bool { return entersMode(it, g.name) }))
}

// modeForm asks for a mode's name, and with a chord for the chord entering
// it, saying below why what it was given cannot be taken.
type modeForm struct {
	*widget.Box
	name    *widget.Entry
	chord   *widget.ShortcutRecorder
	problem *widget.Label
}

func newModeForm(k *kit, confirmKey string, withChord bool, onConfirm func(name, chord string), onCancel func()) *modeForm {
	t := locale.T()
	f := &modeForm{Box: gapBox(widget.Column)}
	row := gapBox(widget.Row)
	row.AppendAligned(k.label(t.Get("wm-mode-name"), css.CaptionHeading, css.DimLabel), false, widget.AlignCenter)
	f.name = k.entry()
	f.name.SetPlaceholder(t.Get("wm-mode-name-placeholder"))
	f.name.SetAccessibleName(t.Get("wm-mode-name"))
	row.AppendAligned(f.name, true, widget.AlignCenter)
	confirm := func() {
		chord := ""
		if f.chord != nil {
			chord = recordedChord(f.chord.Accel())
		}
		onConfirm(f.name.Text(), chord)
	}
	f.name.OnActivate = func(string) { confirm() }
	if withChord {
		f.chord = k.shortcutRecorder(t.Get("wm-mode-chord"), unsetClears)
		row.AppendAligned(f.chord, false, widget.AlignCenter)
	}
	row.AppendAligned(k.primaryButton(t.Get(confirmKey), confirm), false, widget.AlignCenter)
	row.AppendAligned(k.cancelButton(onCancel), false, widget.AlignCenter)
	f.Append(row, false)
	f.problem = k.errorText("")
	f.problem.SetVisible(false)
	f.Append(f.problem, false)
	return f
}

func (f *modeForm) reset(name string) {
	f.name.SetText(name)
	if f.chord != nil {
		f.chord.SetAccel("")
	}
	f.problem.SetVisible(false)
}

func (f *modeForm) fail(err error) {
	f.problem.SetText(err.Error())
	f.problem.SetVisible(true)
}
