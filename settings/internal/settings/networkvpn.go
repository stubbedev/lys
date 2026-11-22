package settings

import (
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/network"
	"github.com/stubbedev/lys/desktop/service/network/vpntext"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"
)

const wireGuardPrivateKey = "private-key"

type vpnProfile struct {
	kind   string
	values map[string]string
}

type vpnEntry struct {
	key   string
	entry *widget.Entry
	field *widget.Field
}

type vpnPicker struct {
	key    string
	picker *widget.Dropdown
	values []string
}

type vpnForm struct {
	*widget.Box
	p          *networkPanel
	kinds      []network.VPNKind
	selected   int
	editing    string
	name       *widget.Entry
	nameField  *widget.Field
	kindField  *widget.Field
	kindSlot   *widget.Box
	importBtn  *widget.Button
	fields     *widget.Box
	entries    []vpnEntry
	pickers    []vpnPicker
	rawToggle  *modeToggle
	rawHint    *widget.Label
	raw        *widget.TextArea
	problem    *widget.Label
	saveButton *widget.Button
}

func newVPNForm(p *networkPanel) *vpnForm {
	k := p.k
	f := &vpnForm{Box: gapBox(widget.Column), p: p}
	f.AddClass("vpn-form")
	f.SetVisible(false)
	f.name = k.entry()
	f.name.SetAccessibleName(i18n.T("dropdown-network-vpn-name"))
	f.name.OnActivate = func(string) { f.save() }
	f.name.OnChanged = func(string) { f.nameField.SetError("") }
	f.nameField = widget.NewField(k.face, i18n.T("dropdown-network-vpn-name"), f.name)
	f.Append(f.nameField, false)
	f.kindSlot = widget.NewBox(widget.Column, 0, 0)
	f.kindField = widget.NewField(k.face, i18n.T("dropdown-network-vpn-type"), f.kindSlot)
	f.Append(f.kindField, false)
	f.importBtn = k.secondaryButton(i18n.T("dropdown-network-vpn-import"), f.importFile)
	f.AppendAligned(f.importBtn, false, widget.AlignStart)
	f.fields = gapBox(widget.Column)
	f.Append(f.fields, false)
	f.rawToggle = k.modeToggle(i18n.T("dropdown-network-vpn-advanced-show"), i18n.T("dropdown-network-vpn-advanced-hide"), func(bool) { f.syncRaw() })
	f.AppendAligned(f.rawToggle, false, widget.AlignStart)
	f.rawHint = k.note("")
	f.Append(f.rawHint, false)
	f.raw = widget.NewTextArea(k.mono, k.textPx(), 0)
	f.raw.SetVariants(k.monoVariants)
	f.raw.SetAccessibleName(i18n.T("dropdown-network-vpn-advanced-show"))
	f.raw.AddClass("vpn-raw")
	f.Append(f.raw, false)
	f.problem = k.errorText("")
	f.problem.SetVisible(false)
	f.Append(f.problem, false)
	cancel := k.cancelButton(f.close)
	f.saveButton = k.primaryButton(i18n.T("dropdown-network-vpn-save"), f.save)
	actions := gapBox(widget.Row)
	actions.Append(cancel, false)
	actions.Append(f.saveButton, false)
	f.AppendAligned(actions, false, widget.AlignEnd)
	return f
}

func (f *vpnForm) open(uuid, name string, prof vpnProfile) {
	f.kinds = network.KindsWith(f.p.ctl.vpnKinds(), prof.kind)
	f.selected = max(0, slices.IndexFunc(f.kinds, func(k network.VPNKind) bool { return k.ID == prof.kind }))
	f.editing = uuid
	f.setProblem("")
	f.name.SetText(name)
	f.nameField.SetError("")
	labels := make([]string, len(f.kinds))
	for i, k := range f.kinds {
		labels[i] = k.Label
	}
	picker := widget.NewDropdown(f.p.k.face, f.p.k.textPx(), labels, f.selected)
	picker.SetAccessibleName(i18n.T("dropdown-network-vpn-type"))
	picker.OnSelect = f.kindSelected
	f.kindSlot.Clear()
	f.kindSlot.Append(picker, false)
	f.kindField.SetVisible(uuid == "")
	f.rebuild(prof.values)
	f.SetVisible(true)
}

func (f *vpnForm) kind() network.VPNKind {
	if f.selected < 0 || f.selected >= len(f.kinds) {
		return network.VPNKind{}
	}
	return f.kinds[f.selected]
}

func (f *vpnForm) kindSelected(i int) {
	if i < 0 || i >= len(f.kinds) || i == f.selected {
		return
	}
	f.selected = i
	f.setProblem("")
	f.rebuild(nil)
}

func (f *vpnForm) rebuild(values map[string]string) {
	f.entries, f.pickers = nil, nil
	f.fields.Clear()
	kind := f.kind()
	f.importBtn.SetVisible(kind.ID == network.WireGuard)
	if !kind.IsTyped() {
		f.raw.SetText(network.RenderRawValues(values))
		f.rawToggle.set(true)
		return
	}
	extra := kind.Leftovers(values)
	f.raw.SetText(network.RenderRawValues(extra))
	f.rawToggle.set(len(extra) > 0)
	section := ""
	for _, field := range kind.Fields {
		if field.Section != "" && field.Section != section {
			f.fields.Append(f.p.k.label(vpntext.SectionHeading(field.Section), css.CaptionHeading, css.DimLabel), false)
			section = field.Section
		}
		current, has := values[field.Key]
		if len(field.Choices) > 0 {
			f.fields.Append(widget.NewField(f.p.k.face, vpntext.FieldLabel(field), f.picker(field, current, has)), false)
			continue
		}
		f.fields.Append(f.entry(kind, field, current), false)
	}
}

func (f *vpnForm) picker(field network.VPNField, current string, has bool) *widget.Dropdown {
	labels := make([]string, len(field.Choices))
	values := make([]string, len(field.Choices))
	for i, c := range field.Choices {
		labels[i], values[i] = vpntext.ChoiceLabel(c), c.Value
	}
	selected := 0
	if i := slices.Index(values, current); has && i >= 0 {
		selected = i
	}
	d := widget.NewDropdown(f.p.k.face, f.p.k.textPx(), labels, selected)
	d.SetAccessibleName(vpntext.FieldName(field))
	f.pickers = append(f.pickers, vpnPicker{key: field.Key, picker: d, values: values})
	return d
}

func (f *vpnForm) entry(kind network.VPNKind, field network.VPNField, current string) *widget.Field {
	k := f.p.k
	entry := k.entry()
	if field.Secret {
		entry.SetEcho(widget.EchoPassword)
	}
	entry.SetAccessibleName(vpntext.FieldName(field))
	entry.SetPlaceholder(field.Placeholder)
	entry.SetText(current)
	entry.OnActivate = func(string) { f.save() }
	control := widget.Widget(entry)
	var readout *widget.Label
	if kind.ID == network.WireGuard && field.Key == wireGuardPrivateKey {
		entry.SetLeadingIcon("ld-refresh-cw-symbolic", k.textPx(), func() {
			entry.SetText(network.GenerateKeyPair().Private)
		})
		readout = wrapped(k.label(vpntext.PublicKey(current), "vpn-public-key", css.Caption, css.DimLabel, css.Monospace))
		box := widget.NewBox(widget.Column, widget.Current().Spacing/2, 0)
		box.Append(entry, false)
		box.Append(readout, false)
		control = box
	}
	row := widget.NewField(k.face, vpntext.FieldLabel(field), control)
	entry.OnChanged = func(text string) {
		row.SetError("")
		if readout != nil {
			readout.SetText(vpntext.PublicKey(text))
		}
	}
	f.entries = append(f.entries, vpnEntry{key: field.Key, entry: entry, field: row})
	return row
}

func (f *vpnForm) syncRaw() {
	f.rawToggle.SetVisible(f.kind().IsTyped())
	f.rawHint.SetText(vpntext.RawHint(f.kind()))
	f.rawHint.SetVisible(f.rawToggle.isOn())
	f.raw.SetVisible(f.rawToggle.isOn())
}

func (f *vpnForm) values() map[string]string {
	typed := make([]network.TypedValue, 0, len(f.entries)+len(f.pickers))
	for _, e := range f.entries {
		typed = append(typed, network.TypedValue{Key: e.key, Value: e.entry.Text()})
	}
	for _, p := range f.pickers {
		if i := p.picker.Selected(); i >= 0 && i < len(p.values) {
			typed = append(typed, network.TypedValue{Key: p.key, Value: p.values[i]})
		}
	}
	return network.MergeVPNValues(network.ParseRawValues(f.raw.Text()), typed)
}

func (f *vpnForm) markProblems(problems []network.VPNProblem) {
	for _, e := range f.entries {
		i := slices.IndexFunc(problems, func(p network.VPNProblem) bool { return p.Field.Key == e.key })
		if i < 0 {
			e.field.SetError("")
			continue
		}
		e.field.SetError(vpntext.Problem(problems[i]))
	}
}

func (f *vpnForm) save() {
	kind := f.kind()
	if kind.ID == "" {
		return
	}
	name := strings.TrimSpace(f.name.Text())
	values := f.values()
	problems := kind.Check(values)
	f.markProblems(problems)
	if name == "" {
		f.nameField.SetError(i18n.T("dropdown-network-vpn-name-required"))
	}
	if name == "" || len(problems) > 0 {
		return
	}
	f.setProblem("")
	f.saveButton.SetEnabled(false)
	ctl, uuid := f.p.ctl, f.editing
	async(f.p.k, func() error {
		return doWithin(networkCallWait, func(ctx context.Context) error {
			if uuid == "" {
				return ctl.addVPN(ctx, kind.ID, name, values)
			}
			return ctl.updateVPN(ctx, uuid, kind.ID, name, values)
		})
	}, func(err error) {
		f.saveButton.SetEnabled(true)
		if err != nil {
			f.setProblem(err.Error())
			return
		}
		f.close()
		f.p.reload()
	})
}

func (f *vpnForm) importFile() {
	if f.p.k.pickers == nil {
		return
	}
	f.p.k.pickers.openFile(f.imported)
}

func (f *vpnForm) imported(path string) {
	values, err := network.ReadWgQuick(path)
	switch {
	case errors.Is(err, network.ErrNotWgQuick):
		f.setProblem(i18n.T("dropdown-network-vpn-import-failed"))
		return
	case err != nil:
		f.setProblem(i18n.T("dropdown-network-vpn-import-error", i18n.Str("error", err.Error())))
		return
	}
	f.setProblem("")
	if strings.TrimSpace(f.name.Text()) == "" {
		f.name.SetText(values["interface"])
	}
	f.rebuild(values)
}

func (f *vpnForm) setProblem(text string) {
	f.problem.SetText(text)
	f.problem.SetVisible(text != "")
}

func (f *vpnForm) close() {
	f.SetVisible(false)
	f.editing = ""
}
