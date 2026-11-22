package settings

import (
	"maps"
	"slices"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	launcherModesPath        = "launcher.modes"
	launcherDisplayNamesPath = "launcher.display-names"
)

// launcherModeOptions are the modes with options of their own, each with
// the modes those options also serve and its rows.
var launcherModeOptions = []struct {
	modes []string
	rows  []rowSpec
}{
	{[]string{"drun"}, []rowSpec{
		field("launcher.drun.categories", desktopCategoriesRow),
		field("launcher.drun.exclude-categories", desktopCategoriesRow),
		field("launcher.drun.match-fields", enumList),
		field("launcher.drun.display-format"),
		field("launcher.drun.show-actions"),
		field("launcher.drun.url-launcher", browserRow),
	}},
	{[]string{"run"}, fields("launcher.run.run-command", "launcher.run.shell-command", "launcher.run.list-command")},
	{[]string{"window", "windowcd"}, []rowSpec{
		field("launcher.window.format"),
		field("launcher.window.match-fields", enumList),
		field("launcher.window.hide-active"),
		field("launcher.window.close-on-delete"),
	}},
	{[]string{"ssh"}, []rowSpec{
		field("launcher.ssh.client", sshClientRow),
		field("launcher.ssh.command"),
		field("launcher.ssh.parse-hosts"),
		field("launcher.ssh.parse-known-hosts"),
	}},
	{[]string{"filebrowser", "recursivebrowser"}, []rowSpec{
		field("launcher.filebrowser.directory"),
		field("launcher.filebrowser.sorting-method"),
		field("launcher.filebrowser.directories-first"),
		field("launcher.filebrowser.show-hidden"),
		field("launcher.filebrowser.command", fileManagerRow),
	}},
	{[]string{"combi"}, []rowSpec{
		field("launcher.combi.modes", launcherModesRow),
		field("launcher.combi.display-format"),
	}},
}

// launcherModeListRow is every mode the launcher can open, each turned on
// and ordered in launcher.modes, beside the selected one's name in
// launcher.display-names and its own options.
func launcherModeListRow() rowSpec {
	var nested []rowSpec
	for _, o := range launcherModeOptions {
		nested = append(nested, o.rows...)
	}
	return field(launcherModesPath, withKey("settings-launcher-mode-list"), func(r *rowSpec) {
		r.fullWidth = true
		r.also = []string{launcherDisplayNamesPath}
		r.nested = nested
		r.editor = func(k *kit, s slot, _ config.FieldMeta) control {
			return newLauncherModeList(k, s, k.store.slot(launcherDisplayNamesPath))
		}
	})
}

// launcherModeList is the two panes: the modes, and the selected one's
// name and options.
type launcherModeList struct {
	*widget.Box
	k        *kit
	modes    slot
	names    slot
	selected string
	list     *widget.ItemGroup
	title    *widget.Label
	name     *widget.Entry
	state    *cardState
	groups   [][]*fieldRow
	rows     []*fieldRow
	syncing  bool
}

func newLauncherModeList(k *kit, modes, names slot) *launcherModeList {
	t := locale.T()
	c := &launcherModeList{Box: gapBox(widget.Row), k: k, modes: modes, names: names}
	c.list = widget.NewItemGroup()
	left := gapBox(widget.Column)
	left.AddClass("launcher-mode-list")
	left.Append(c.list, false)

	pane := gapBox(widget.Column)
	pane.AddClass("launcher-mode-pane")
	c.title = k.label("", css.H4)
	c.name = k.entry()
	c.name.SetAccessibleName(t.Get("settings-launcher-mode-display-name"))
	c.name.OnChanged = func(text string) {
		if !c.syncing {
			c.rename(text)
		}
	}
	nameRow := gapBox(widget.Row)
	nameRow.AppendAligned(k.label(t.Get("settings-launcher-mode-display-name")), false, widget.AlignCenter)
	nameRow.AppendAligned(c.name, true, widget.AlignCenter)
	c.state = k.cardState()
	pane.Append(c.title, false)
	pane.Append(nameRow, false)
	pane.Append(c.state.lone(), false)
	for _, o := range launcherModeOptions {
		group := make([]*fieldRow, len(o.rows))
		for i, rs := range o.rows {
			row := newFieldRow(k, k.store, rs)
			group[i] = row
			pane.Append(row, false)
		}
		c.groups = append(c.groups, group)
		c.rows = append(c.rows, group...)
	}
	c.Append(left, false)
	c.Append(pane, true)
	c.refresh()
	return c
}

func (c *launcherModeList) nestedRows() []*fieldRow { return c.rows }

func (c *launcherModeList) showsNested() bool { return true }

// showNested selects the mode whose options row is.
func (c *launcherModeList) showNested(row *fieldRow) {
	for i, group := range c.groups {
		if slices.Contains(group, row) {
			c.selected = launcherModeOptions[i].modes[0]
			c.refresh()
			return
		}
	}
}

// launcherModeEntry is one mode the list shows.
type launcherModeEntry struct {
	choice
	on bool
}

// entries are the modes turned on, in their order, then the rest as the
// launcher lists them; a mode launcher.modes names that is neither a
// built-in nor a script is kept, marked as unknown.
func (c *launcherModeList) entries() []launcherModeEntry {
	known := launcherModes(c.k)()
	on := stringsOf(c.modes.get())
	out := make([]launcherModeEntry, 0, len(known)+len(on))
	for _, m := range on {
		out = append(out, launcherModeEntry{listedChoice(known, m), true})
	}
	for _, o := range known {
		if !slices.Contains(on, o.value) {
			out = append(out, launcherModeEntry{o, false})
		}
	}
	return out
}

func (c *launcherModeList) displayNames() map[string]any {
	names, _ := c.names.get().(map[string]any)
	return maps.Clone(names)
}

func (c *launcherModeList) refresh() {
	entries := c.refreshList()
	at := slices.IndexFunc(entries, func(e launcherModeEntry) bool { return e.value == c.selected })
	if at < 0 {
		at, c.selected = 0, entries[0].value
		entries = c.refreshList()
	}
	c.title.SetText(entries[at].label)
	c.syncing = true
	name, _ := c.displayNames()[c.selected].(string)
	if c.name.Text() != name {
		c.name.SetText(name)
	}
	c.syncing = false
	c.name.SetPlaceholder(c.selected)
	shown := false
	for i, o := range launcherModeOptions {
		mine := slices.Contains(o.modes, c.selected)
		shown = shown || mine
		for _, row := range c.groups[i] {
			row.SetVisible(mine)
			row.refresh()
		}
	}
	if shown {
		c.state.Clear()
	} else {
		c.state.Empty("ld-settings-symbolic", locale.T().Get("settings-launcher-mode-no-options"))
	}
}

func (c *launcherModeList) modeRow(e launcherModeEntry, names map[string]any, on []string) *settingRow {
	t := locale.T()
	detail := e.group
	if e.group == e.value {
		detail = ""
	}
	if name, _ := names[e.value].(string); name != "" {
		detail = t.Get("settings-launcher-mode-shown-as", i18n.Str("name", name))
	}
	row := c.k.row(e.label, detail)
	row.AddClass("launcher-mode")
	if e.value == c.selected {
		row.SetVariant(widget.ItemMuted)
	}
	row.SetOnActivate(func() {
		c.selected = e.value
		c.refresh()
	})
	if e.on {
		at := slices.Index(on, e.value)
		up, down := c.k.moveButtons(func(by int) { c.move(e.value, by) })
		up.SetEnabled(at > 0)
		down.SetEnabled(at < len(on)-1)
		row.AddAction(up)
		row.AddAction(down)
	}
	sw := widget.NewSwitch(e.on)
	sw.SetAccessibleName(e.label)
	sw.SetEnabled(!e.on || len(on) > 1)
	sw.OnChanged = func(turnOn bool) { c.turn(e.value, turnOn) }
	row.withControl(sw)
	return row
}

func (c *launcherModeList) writeModes(modes []string) {
	out := make([]any, len(modes))
	for i, m := range modes {
		out[i] = m
	}
	_ = c.modes.set(out)
	c.refresh()
}

// turn adds mode to the end of the tabs, or takes it off them; the last
// mode stays, as the launcher refuses to open with none.
func (c *launcherModeList) turn(mode string, on bool) {
	modes := slices.DeleteFunc(stringsOf(c.modes.get()), func(m string) bool { return m == mode })
	if on {
		modes = append(modes, mode)
	}
	c.selected = mode
	c.writeModes(modes)
}

func (c *launcherModeList) move(mode string, delta int) {
	modes := stringsOf(c.modes.get())
	i := slices.Index(modes, mode)
	j := i + delta
	if i < 0 || j < 0 || j >= len(modes) {
		return
	}
	modes[i], modes[j] = modes[j], modes[i]
	c.writeModes(modes)
}

// rename gives the selected mode the name the launcher shows it by; an
// empty one takes it off launcher.display-names.
func (c *launcherModeList) rename(name string) {
	names := c.displayNames()
	if names == nil {
		names = map[string]any{}
	}
	if name == "" {
		delete(names, c.selected)
	} else {
		names[c.selected] = name
	}
	if c.names.set(names) == nil {
		c.refreshList()
	}
}

// refreshList redraws the modes alone, so typing a name keeps its field,
// and returns them.
func (c *launcherModeList) refreshList() []launcherModeEntry {
	names := c.displayNames()
	on := stringsOf(c.modes.get())
	entries := c.entries()
	c.list.Clear()
	for _, e := range entries {
		c.list.Append(c.modeRow(e, names, on))
	}
	return entries
}
