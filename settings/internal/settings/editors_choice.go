package settings

import (
	"cmp"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/pango"
	"github.com/stubbedev/lys/desktop/shell/barview"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	choiceListPx    = 320
	checkMark       = "✓ "
	wmColorFormat   = "lys-wm-color"
	wmFontFormat    = "lys-wm-font-file"
	wmLayoutFormat  = "lys-wm-xkb-layout"
	wmVariantFormat = "lys-wm-xkb-variant"
	wmOptionsFormat = "lys-wm-xkb-options"
	wmModelFormat   = "lys-wm-xkb-model"
	wmRulesFormat   = "lys-wm-xkb-rules"
	wmCommandFormat = "lys-wm-command"
	listBrowsers    = "browser"
	listFileManager = "file-manager"
	listSSHClients  = "ssh-client"
	listSoundNames  = "sound-name"
	listTimeZones   = "time-zone"
)

func hostLists(h host) map[string]lister {
	x := sync.OnceValue(h.xkb)
	return map[string]lister{
		"Monitor":           h.outputs,
		"OsdMonitor":        h.outputs,
		"PopupMonitor":      h.outputs,
		"ReplayOutput":      h.outputs,
		"IconTheme":         h.iconThemes,
		"lys-wm-icon-theme": h.iconThemes,
		"CursorTheme":       h.cursorThemes,
		"Program":           h.programs,
		listBrowsers:        h.browsers,
		listFileManager:     h.fileManagers,
		listSSHClients:      h.sshClients,
		listSoundNames:      h.soundNames,
		listTimeZones:       h.timeZones,
		listMailTags:        mailTags,
		listMediaPlayers:    mediaPlayers,
		listMailFolders:     mailFolders,
		"NetInterface":      h.interfaces,
		"TempSensor":        h.sensors,
		"AudioSource":       audioSources,
		"PamService":        h.pamServices,
		"lys-wm-drm-device": h.drmDevices,
		wmLayoutFormat:      func() []choice { return x().layouts },
		wmVariantFormat:     func() []choice { return x().variants },
		wmOptionsFormat:     func() []choice { return x().options },
		wmModelFormat:       func() []choice { return x().models },
		wmRulesFormat:       func() []choice { return x().rules },
		listLayoutNames:     func() []choice { return layoutNames(x()) },
		listXKBSource:       func() []choice { return xkbSources(x()) },
	}
}

var systemLists = hostLists(systemHost)

var commandPresets = []struct{ keyword, command, key string }{
	{"lock", "loginctl lock-session", "settings-command-lock-session"},
	{"logout", "loginctl terminate-session self", "settings-command-end-session"},
	{"logout", "loginctl terminate-user $USER", "settings-command-end-user"},
	{"suspend", "systemctl suspend", "settings-command-suspend"},
	{"suspend", "systemctl hybrid-sleep", "settings-command-hybrid-sleep"},
	{"reboot", "systemctl reboot", "settings-command-reboot"},
	{"shutdown", "systemctl poweroff", "settings-command-poweroff"},
	{"poweroff", "systemctl poweroff", "settings-command-poweroff"},
}

var bitwardenServers = []string{"bitwarden.com", "bitwarden.eu"}

var strftimePresets = []string{"%H:%M", "%H:%M:%S", "%I:%M %p", "%a %d %b", "%A, %B %d", "%Y-%m-%d", "%d/%m/%Y", "%Y-%m-%d_%H-%M-%S"}

func fixed(values ...string) lister {
	return func() []choice {
		out := make([]choice, 0, len(values))
		for _, v := range values {
			out = append(out, choice{value: v, label: v})
		}
		return out
	}
}

func shellDefault(path string) string {
	v, err := config.DefaultValue(path)
	if err != nil {
		return ""
	}
	s, _ := config.Plain(v).(string)
	return s
}

func commandChoices(path string) []actionChoice {
	keys := strings.Split(path, ".")
	last := keys[len(keys)-1]
	var out []actionChoice
	if d := shellDefault(path); d != "" {
		out = append(out, actionChoice{key: "settings-command-default", command: d})
	}
	for _, p := range commandPresets {
		if strings.Contains(last, p.keyword) && !slices.ContainsFunc(out, func(c actionChoice) bool { return c.command == p.command }) {
			out = append(out, actionChoice{key: p.key, command: p.command})
		}
	}
	return out
}

func commandsFor(path string) []choice {
	presets := commandChoices(path)
	out := make([]choice, len(presets))
	for i, p := range presets {
		out[i] = choice{value: p.command, label: p.label(), group: p.command}
	}
	return out
}

func semanticEditor(k *kit, s slot, path string, meta config.FieldMeta) (control, bool) {
	if meta.Kind != config.FieldText {
		return nil, false
	}
	switch meta.Type {
	case "StrftimeFormat":
		return newLabelBuilder(k, s, meta.Optional, strftimeParts()), true
	case "Secret":
		return newSecretEditor(k, s, meta.Optional), true
	case "Directory":
		return newFolderEditor(k, s), true
	case "Command":
		if len(meta.Placeholders) > 0 {
			return newLabelBuilder(k, s, meta.Optional, placeholderFormat(meta.Placeholders)), true
		}
		return newChoiceEditor(k, s, choiceSpec{list: fixedChoices(commandsFor(path)), free: true, unset: optionalUnset(meta.Optional), browse: (*kit).appCommandButton}), true
	case "BitwardenServer":
		return newChoiceEditor(k, s, choiceSpec{list: fixed(bitwardenServers...), free: true}), true
	case "IdleTimeout":
		return newIdleChips(k, s), true
	case wmColorFormat:
		return wmColorEditor(k, s, meta.Optional), true
	case wmFontFormat:
		return newWMFontEditor(k, s), true
	case wmChordFormat:
		return newChordEditor(k, s), true
	case wmCommandFormat:
		return newChoiceEditor(k, s, choiceSpec{list: appCommands, unset: optionalUnset(meta.Optional)}), true
	case wmActionFormat:
		f, _ := k.wm.fieldOf(path)
		return newWMActionEditor(k, s, f.Actions), true
	}
	if len(meta.Placeholders) > 0 && meta.Markup {
		f := placeholderFormat(meta.Placeholders)
		f.markup = true
		return newFormatEditor(k, s, meta.Optional, f), true
	}
	if len(meta.Placeholders) > 0 {
		return newLabelBuilder(k, s, meta.Optional, placeholderFormat(meta.Placeholders)), true
	}
	list, ok := systemLists[meta.Type]
	if !ok {
		return nil, false
	}
	spec := choiceSpec{list: list, special: meta.Variants, unset: optionalUnset(meta.Optional)}
	switch meta.Type {
	case wmLayoutFormat, wmOptionsFormat:
		spec.expand, spec.known = toggleList, allListed
	case wmVariantFormat:
		layouts := func() []string { return splitList(wmSibling(k, path, "layout")) }
		spec.expand, spec.known = variantChoices(layouts), variantsListed(layouts)
	case "Program":
		spec.known = func(current string, items []choice) bool {
			return strings.Contains(current, "/") || hasValue(items, current)
		}
	case "AudioSource":
		return audioSourceEditor(k, s, spec), true
	}
	return newChoiceEditor(k, s, spec), true
}

func wmSibling(k *kit, path, key string) string {
	if k.wm == nil {
		return ""
	}
	keys := wmPath(path)
	keys[len(keys)-1] = key
	v, _ := k.wm.value(strings.Join(keys, ".")).(string)
	return v
}

func programRow(list string) rowOpt {
	return withEditor(func(k *kit, s slot, meta config.FieldMeta) control {
		return newChoiceEditor(k, s, choiceSpec{list: systemLists[list], special: meta.Variants, free: true, unset: optionalUnset(meta.Optional), browse: (*kit).appCommandButton})
	})
}

var (
	browserRow     = programRow(listBrowsers)
	fileManagerRow = programRow(listFileManager)
	sshClientRow   = programRow(listSSHClients)
)

type choiceSpec struct {
	list    lister
	special []string
	expand  func(current string, items []choice) []choice
	known   func(current string, items []choice) bool
	browse  func(k *kit, pick func(string)) *widget.Button
	free    bool
	unset   unsetValue
}

// unsetValue is what a choice's empty value means: nothing, for a value
// that must be set, the default, or the value it inherits.
type unsetValue uint8

const (
	unsetRefused unsetValue = iota
	unsetDefault
	unsetInherited
)

// optionalUnset is the empty value of a field optional or not, which
// then reads as its default.
func optionalUnset(optional bool) unsetValue {
	if optional {
		return unsetDefault
	}
	return unsetRefused
}

// label is what the empty value reads.
func (u unsetValue) label() string {
	if u == unsetInherited {
		return locale.T().Get("settings-inherited")
	}
	return specialLabel("")
}

// choiceEditor is a text field offering choices: lys-ui's combobox
// entry, whose text is the value, typed, or picked from the searchable
// list its chevron opens, the special values first. A value no list
// holds is offered under its own heading and marked by the unknown badge
// beside the field, unless the field is free text.
type choiceEditor struct {
	*widget.Box
	k       *kit
	slot    slot
	spec    choiceSpec
	combo   *widget.ComboboxEntry
	entry   *text
	unknown *widget.Badge
	items   []choice
	listed  bool
}

func specialLabel(v string) string {
	t := locale.T()
	switch v {
	case "":
		return t.Get("settings-choice-default")
	case "auto":
		return t.Get("settings-choice-auto")
	case "primary":
		return t.Get("settings-choice-primary")
	}
	return v
}

// specialsByLabel is s as a choice's field reads and types it: a special
// value (auto, primary) by its label, which the field writes back as the
// value.
func specialsByLabel(s slot, specials []string) slot {
	return slot{
		get: func() any {
			v, _ := s.get().(string)
			if v != "" && slices.Contains(specials, v) {
				return specialLabel(v)
			}
			return s.get()
		},
		set: func(v any) error {
			text, _ := v.(string)
			for _, sp := range specials {
				if sp != "" && text == specialLabel(sp) {
					return s.set(sp)
				}
			}
			return s.set(v)
		},
		unset: s.unset,
	}
}

func newChoiceEditor(k *kit, s slot, spec choiceSpec) *choiceEditor {
	c := &choiceEditor{Box: gapBox(widget.Row), k: k, slot: s, spec: spec, combo: widget.NewComboboxEntry(k.face, k.textPx()), unknown: k.unknownBadge()}
	c.combo.Entry().SetTextWidth(widget.GTKTextWidth)
	c.combo.SetSearchPlaceholder(locale.T().Get("settings-choice-search"))
	c.combo.OnPick = c.pick
	c.entry = bindText(k, c.combo.Entry(), specialsByLabel(s, spec.special), spec.unset != unsetRefused)
	switch {
	case slices.Contains(spec.special, ""):
		c.combo.SetEmpty(spec.unset.label())
	case spec.unset != unsetRefused:
		c.combo.Entry().SetEmptyValue(spec.unset.label())
	}
	committed := c.entry.OnActivate
	c.entry.OnActivate = func(text string) {
		committed(text)
		c.refreshUnknown()
	}
	if spec.browse != nil {
		c.combo.SetAction(spec.browse(k, c.pick))
	}
	c.AppendAligned(c.unknown, false, widget.AlignCenter)
	c.AppendAligned(c.combo, true, widget.AlignCenter)
	c.refresh()
	async(k, spec.list, c.setItems)
	return c
}

// columnField makes the editor a field of the control column: its
// combobox takes the column's width beside the unknown badge.
func (c *choiceEditor) columnField() {}

func (c *choiceEditor) dirtyBadge() *widget.Badge { return c.entry.badge }

func (c *choiceEditor) setItems(items []choice) {
	c.items, c.listed = items, true
	c.refreshUnknown()
}

func (c *choiceEditor) refresh() {
	c.entry.refresh()
	c.refreshUnknown()
}

func (c *choiceEditor) isKnown(current string) bool {
	switch {
	case c.spec.free, current == "", slices.Contains(c.spec.special, current):
		return true
	case c.spec.known != nil:
		return c.spec.known(current, c.items)
	}
	return hasValue(c.items, current)
}

func hasValue(items []choice, v string) bool {
	return slices.ContainsFunc(items, func(it choice) bool { return it.value == v })
}

// refreshUnknown marks a value no list holds and offers the choices
// the value calls for.
func (c *choiceEditor) refreshUnknown() {
	current := c.slot.text()
	unknown := c.listed && !c.isKnown(current)
	c.unknown.SetVisible(unknown)
	c.combo.SetOptions(c.options(current, unknown)...)
}

// options are the specials but the empty one, which the combobox lists
// itself, then the choices the current value calls for: the expanded
// ones, or the unknown value under its heading and the listed ones.
func (c *choiceEditor) options(current string, unknown bool) []widget.ComboboxOption[string] {
	var out []widget.ComboboxOption[string]
	add := func(group string, cs ...choice) {
		for _, o := range cs {
			out = append(out, widget.ComboboxOption[string]{Value: o.value, Label: o.label, Detail: choiceHint(o), Group: group})
		}
	}
	for _, v := range c.spec.special {
		if v != "" {
			add("", choice{value: v, label: specialLabel(v)})
		}
	}
	if c.spec.expand != nil {
		add("", c.spec.expand(current, c.items)...)
		return out
	}
	if unknown {
		add(locale.T().Get("settings-choice-unknown"), choice{value: current, label: current})
	}
	add("", c.items...)
	return out
}

func choiceHint(o choice) string {
	if o.group != "" {
		return o.group
	}
	if o.value != o.label {
		return o.value
	}
	return ""
}

func openChoicePicker(k *kit, anchor widget.Widget, opts []choice, pick func(string)) *searchPicker {
	byValue := make(map[string]choice, len(opts))
	names := make([]string, 0, len(opts))
	for _, o := range opts {
		if _, dup := byValue[o.value]; !dup {
			names, byValue[o.value] = append(names, o.value), o
		}
	}
	return openSearchPicker(k, anchor, names, pickerSpec{
		class: "choice-popover", placeholder: "settings-choice-search", maxH: choiceListPx,
		row:  func(k *kit, name string) widget.Widget { return choiceRow(k, byValue[name]) },
		text: func(name string) string { o := byValue[name]; return o.label + " " + choiceHint(o) },
	}, pick)
}

// choiceRow is a choice's row in a picker: its label, and its hint
// (a placeholder, a sample) a section's gap after it at the row's end.
func choiceRow(k *kit, o choice) *widget.Box {
	row := widget.NewBox(widget.Row, sectionGap(), 0)
	label := k.label(o.label, "choice-label")
	label.SetEllipsize(widget.EllipsizeEnd)
	row.AppendAligned(label, true, widget.AlignCenter)
	if hint := choiceHint(o); hint != "" {
		row.AppendAligned(k.label(hint, "choice-hint", css.Caption, css.DimLabel), false, widget.AlignCenter)
	}
	return row
}

// pick writes the value picked from the list or browsed for; a refused
// one stays in the field, marked unsaved.
func (c *choiceEditor) pick(v string) {
	if c.spec.unset != unsetRefused && v == "" {
		c.slot.unset()
	} else if c.slot.set(v) != nil {
		return
	}
	c.entry.badge.SetVisible(false)
	c.refresh()
}

func splitList(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

func toggleList(current string, items []choice) []choice {
	have := splitList(current)
	out := make([]choice, 0, len(items))
	for _, it := range items {
		next := slices.DeleteFunc(slices.Clone(have), func(v string) bool { return v == it.value })
		label := it.label
		if it.group != "" {
			label = it.group + ": " + label
		}
		if len(next) == len(have) {
			next = append(next, it.value)
		} else {
			label = checkMark + label
		}
		out = append(out, choice{value: strings.Join(next, ","), label: label, group: it.value})
	}
	return out
}

func allListed(current string, items []choice) bool {
	for _, v := range splitList(current) {
		if !hasValue(items, v) {
			return false
		}
	}
	return true
}

func joinTrimmed(items []string) string {
	for len(items) > 0 && items[len(items)-1] == "" {
		items = items[:len(items)-1]
	}
	return strings.Join(items, ",")
}

func variantChoices(layouts func() []string) func(current string, items []choice) []choice {
	return func(current string, items []choice) []choice {
		ls := layouts()
		have := splitList(current)
		var out []choice
		for i, layout := range ls {
			for _, it := range items {
				if it.group != layout {
					continue
				}
				next := make([]string, len(ls))
				copy(next, have)
				label := layout + ": " + it.label
				if next[i] == it.value {
					next[i], label = "", checkMark+label
				} else {
					next[i] = it.value
				}
				out = append(out, choice{value: joinTrimmed(next), label: label, group: layout + "(" + it.value + ")"})
			}
		}
		return out
	}
}

func variantsListed(layouts func() []string) func(current string, items []choice) bool {
	return func(current string, items []choice) bool {
		ls := layouts()
		for i, v := range splitList(current) {
			if v == "" {
				continue
			}
			if i >= len(ls) || !slices.ContainsFunc(items, func(it choice) bool { return it.group == ls[i] && it.value == v }) {
				return false
			}
		}
		return true
	}
}

type formatKind struct {
	insert     []choice
	presets    []string
	separators []string
	escapes    []string
	render     func(string) (string, error)
	// markup is a format whose rendered text is Pango markup, previewed
	// styled.
	markup bool
}

// label is s as a choice shows it: its sample, or that it is no valid
// format.
func (f formatKind) label(s string) string { return formatLabel(f.render, s) }

// formatLabel is s as render renders it, or that it is no valid format.
func formatLabel(render func(string) (string, error), s string) string {
	text, err := render(s)
	if err != nil {
		return locale.T().Get("settings-format-invalid")
	}
	return text
}

func placeholderFormat(ps []config.Placeholder) formatKind {
	t := locale.T()
	f := formatKind{separators: labelSeparators}
	for _, p := range ps {
		f.insert = append(f.insert, choice{value: p.Token, label: cmp.Or(t.Get(p.Key), p.Name), group: p.Token})
	}
	f.render = func(s string) (string, error) {
		pairs := make([]string, 0, 4*len(ps))
		for _, p := range ps {
			pairs = append(pairs, p.Token, p.Sample, strings.ReplaceAll(p.Token, " ", ""), p.Sample)
		}
		return strings.NewReplacer(pairs...).Replace(s), nil
	}
	return f
}

func strftimeFormat() formatKind {
	return formatKind{presets: strftimePresets, render: func(s string) (string, error) {
		f, err := config.ParseStrftimeFormat(s)
		if err != nil {
			return "", err
		}
		return f.Format(formatSampleTime), nil
	}}
}

var timeParts = []struct{ directive, key string }{
	{"%A", "settings-time-weekday-long"},
	{"%a", "settings-time-weekday-short"},
	{"%d", "settings-time-day"},
	{"%-d", "settings-time-day-short"},
	{"%B", "settings-time-month-long"},
	{"%b", "settings-time-month-short"},
	{"%m", "settings-time-month-number"},
	{"%Y", "settings-time-year"},
	{"%y", "settings-time-year-short"},
	{"%H", "settings-time-hour-24"},
	{"%-H", "settings-time-hour-24-short"},
	{"%I", "settings-time-hour-12"},
	{"%-I", "settings-time-hour-12-short"},
	{"%M", "settings-time-minute"},
	{"%S", "settings-time-second"},
	{"%p", "settings-time-period"},
}

var timeSeparators = []string{" ", ":", ", ", "-", "/", ".", "_", " · "}

func strftimeParts() formatKind {
	f := strftimeFormat()
	f.separators = timeSeparators
	f.escapes = []string{"%%"}
	t := locale.T()
	for _, p := range timeParts {
		f.insert = append(f.insert, choice{value: p.directive, label: t.Get(p.key), group: f.label(p.directive)})
	}
	return f
}

// formatSampleTime is the moment a format's preview renders: the bar
// preview's, so the two never show different times.
var formatSampleTime = barview.SampleTime

type formatEditor struct {
	*widget.Box
	k       *kit
	slot    slot
	kind    formatKind
	entry   *text
	open    *widget.Button
	preview *formatPreview
	picker  *searchPicker
}

func newFormatEditor(k *kit, s slot, optional bool, kind formatKind) *formatEditor {
	c := &formatEditor{Box: widget.NewBox(widget.Column, widget.Current().Spacing/2, 0), k: k, slot: s, kind: kind, entry: newText(k, s, optional)}
	icon, tip := "ld-plus-symbolic", "settings-format-insert"
	if len(kind.insert) == 0 {
		icon, tip = "ld-chevron-down-symbolic", "settings-format-presets"
	}
	c.open = k.pickerIconButton(icon, tip, c.show)
	c.preview = k.formatPreview()
	typed := c.entry.OnChanged
	c.entry.OnChanged = func(text string) {
		typed(text)
		c.showPreview(text)
	}
	line := gapBox(widget.Row)
	line.AppendAligned(c.entry, false, widget.AlignCenter)
	line.AppendAligned(c.open, false, widget.AlignCenter)
	c.Append(line, false)
	c.Append(c.preview, false)
	c.refresh()
	return c
}

func (c *formatEditor) dirtyBadge() *widget.Badge { return c.entry.badge }

func (c *formatEditor) refresh() {
	c.entry.refresh()
	c.showPreview(c.entry.Text())
}

func (c *formatEditor) showPreview(text string) {
	rendered, err := c.kind.render(text)
	if c.kind.markup {
		c.preview.showMarkup(rendered, err)
		return
	}
	c.preview.show(rendered, err)
}

// formatPreview is a format's sample under its entry: the format at the
// sample time, or, for one that does not parse, that it is no valid
// format, the error behind the disclosure.
type formatPreview struct {
	*widget.Box
	sample *widget.Label
	// styled is the sample of a format rendering Pango markup.
	styled  *widget.RichLabel
	invalid *inlineState
}

func (k *kit) formatPreview() *formatPreview {
	p := &formatPreview{Box: widget.NewBox(widget.Column, 0, 0), sample: k.label("", "format-preview", css.Caption, css.DimLabel), invalid: k.inlineState()}
	p.sample.SetEllipsize(widget.EllipsizeEnd)
	p.styled = widget.NewRichLabel(k.face, k.textPx(), "", 0)
	p.styled.AddClass("format-preview", css.Caption, css.DimLabel)
	p.styled.SetVisible(false)
	p.Append(p.sample, false)
	p.Append(p.styled, false)
	p.Append(p.invalid, false)
	return p
}

// show previews text, or the failure err when it is not nil.
func (p *formatPreview) show(text string, err error) {
	p.styled.SetVisible(false)
	p.sample.SetVisible(err == nil)
	p.invalid.Result(locale.T().Get("settings-format-invalid"), err)
	if err == nil {
		p.sample.SetText(locale.T().Get("settings-format-preview", i18n.Str("text", text)))
	}
}

// previewSlot stands for the sample in the preview's message while the
// message is split around it.
const previewSlot = "\x00"

// showMarkup previews markup, Pango markup, styled as the launcher
// renders it; markup that does not parse is the failure it previews,
// as err when it is not nil.
func (p *formatPreview) showMarkup(markup string, err error) {
	if err == nil {
		before, after, _ := strings.Cut(locale.T().Get("settings-format-preview", i18n.Str("text", previewSlot)), previewSlot)
		styled, ok := pango.Translate(pango.Escape(before) + markup + pango.Escape(after))
		if ok {
			p.sample.SetVisible(false)
			p.invalid.Clear()
			p.styled.SetMarkup(styled)
			p.styled.SetVisible(true)
			return
		}
		err = fmt.Errorf("settings: %q is no Pango markup", markup)
	}
	p.show("", err)
}

// Text is what the preview says: the sample, or the failure's summary.
func (p *formatPreview) Text() string {
	if p.sample.Visible() {
		return p.sample.Text()
	}
	return p.invalid.Text()
}

func (c *formatEditor) options() []choice {
	if len(c.kind.insert) > 0 {
		return c.kind.insert
	}
	out := make([]choice, 0, len(c.kind.presets)+1)
	if current := c.slot.text(); current != "" && !slices.Contains(c.kind.presets, current) {
		out = append(out, choice{value: current, label: c.kind.label(current), group: current})
	}
	for _, p := range c.kind.presets {
		out = append(out, choice{value: p, label: c.kind.label(p), group: p})
	}
	return out
}

func (c *formatEditor) show() {
	if c.k.pickers != nil {
		c.picker = openChoicePicker(c.k, c.open, c.options(), c.pick)
	}
}

func (c *formatEditor) pick(v string) {
	if len(c.kind.insert) > 0 {
		c.entry.Insert(v)
		v = c.entry.Text()
	}
	if c.entry.commit(v) {
		c.entry.badge.SetVisible(false)
	}
	c.refresh()
}

func newSecretEditor(k *kit, s slot, optional bool) *text {
	c := newText(k, s, optional)
	c.SetEcho(widget.EchoPassword)
	c.SetPlaceholder(locale.T().Get("settings-secret-placeholder"))
	return c
}

// wmAccent is the palette name lys-wm gives the desktop accent, which is
// lys-shell's accent (docs/settings/guide/lys-wm.md).
const wmAccent = "accent"

// wmPalette is every name a lys-wm colour takes, with its colour: lys-wm's
// built-in palette, the desktop accent, and the palette's own names over
// them.
func wmPalette(k *kit) map[string]render.Color {
	out := map[string]render.Color{}
	if k.wm == nil {
		return out
	}
	builtIn, _ := k.wm.defaultOf("palette")
	own, _ := k.wm.value("palette").(map[string]any)
	add := func(names map[string]any) {
		for name, v := range names {
			hex, _ := v.(string)
			if c, ok := styling.ParseHex(hex); ok {
				out[name] = c
			}
		}
	}
	add(asMap(builtIn))
	shell := k.palette()
	if c, ok := shell.Token(config.TokenAccent); ok {
		out[wmAccent] = c
	}
	add(own)
	return out
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

// wmColorRGB is the colour a lys-wm colour value names: a hex, or a name
// of the palette.
func wmColorRGB(k *kit, value string) (render.Color, bool) {
	if c, ok := styling.ParseHex(value); ok {
		return c, true
	}
	c, ok := wmPalette(k)[value]
	return c, ok
}

func audioSources() []choice {
	sources, ok := audioDaemonSources()
	if !ok {
		return nil
	}
	out := make([]choice, 0, len(sources))
	for _, s := range sources {
		out = append(out, choice{value: s.Name, label: cmp.Or(s.Description, s.Name)})
	}
	return out
}
