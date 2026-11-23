package bar

import (
	"context"
	"errors"
	"log"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/network"
	"github.com/stubbedev/lys/desktop/service/network/vpntext"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/lib/render"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/shell/internal/portal"
)

// This file is the network dropdown's VPNs: the section listing NM's
// VPN profiles, each row toggling
// its tunnel and opening it in the editor, and the editor itself, the
// dropdown's second page. The form is built from the
// picked kind's fields rather than drawn per VPN type, so a plugin
// lys-shell has never heard of still gets the free-form key = value
// editor and lands in NetworkManager the same way.

// vpnControl is the VPN profiles the dropdown drives
// (*network.VPNService).
type vpnControl interface {
	Entries() []network.VPN
	Toggle(ctx context.Context, uuid string) error
	Add(ctx context.Context, kind, name string, values map[string]string) error
	Update(ctx context.Context, uuid, kind, name string, values map[string]string) error
	Remove(ctx context.Context, uuid string) error
	SettingsOf(ctx context.Context, uuid string) (network.ConnectionDict, error)
}

// fileOpener is the portal's file chooser (portal.OpenFile on the
// session bus).
type fileOpener func(ctx context.Context, title string, filters ...portal.FileFilter) (string, error)

// openFileOnSessionBus is the live fileOpener.
func openFileOnSessionBus(ctx context.Context, title string, filters ...portal.FileFilter) (string, error) {
	conn, err := dbus.SessionBus()
	if err != nil {
		return "", err
	}
	return portal.OpenFile(ctx, conn, title, filters...)
}

// vpnFieldsMaxHeight caps the form: WireGuard's nine fields run
// past the popover, so the form scrolls past this instead.
const vpnFieldsMaxHeight = 260

// The WireGuard field holding this end's private key, and the glyph of
// the button that generates one.
const (
	wireGuardPrivateKey = "private-key"
	iconGenerateKey     = "ld-refresh-cw-symbolic"
)

// vpnStateIcon is a row's glyph: the
// dropdown's own, not the bar icon the user themes. A failed attempt
// reads as not connected; the reason is the row's second line.
func vpnStateIcon(state network.VPNState) string {
	switch state {
	case network.VPNConnected:
		return "ld-lock-symbolic"
	case network.VPNConnecting:
		return "ld-refresh-cw-symbolic"
	}
	return "ld-unplug-symbolic"
}

func vpnStateLabel(state network.VPNState) string {
	switch state {
	case network.VPNConnected:
		return i18n.T("dropdown-network-vpn-connected")
	case network.VPNConnecting:
		return i18n.T("dropdown-network-vpn-connecting")
	case network.VPNFailed:
		return i18n.T("dropdown-network-vpn-failed")
	}
	return i18n.T("dropdown-network-vpn-disconnected")
}

// vpnRowCaption is the row's second line: the failure reason as a
// sentence when there is one, the state otherwise.
func vpnRowCaption(row network.VPN) string {
	if row.Detail != "" {
		return sentence(row.Detail)
	}
	return vpnStateLabel(row.State)
}

// sentence capitalises a reason for its own line: lys-shell's
// errors are written lowercase to read mid-sentence in a log; only the
// first letter is touched.
func sentence(reason string) string {
	reason = strings.TrimSpace(reason)
	first, size := utf8.DecodeRuneInString(reason)
	if size == 0 {
		return ""
	}
	return string(unicode.ToUpper(first)) + reason[size:]
}

// netVPNs is the VPN label, one row per profile, and
// the "Add VPN" row that gives a machine with none its first.
type netVPNs struct {
	*widget.Box
	v      *networkView
	rows   *widget.ItemGroup
	add    *widget.Item
	listed []network.VPN
}

func newNetVPNs(v *networkView) *netVPNs {
	s := &netVPNs{v: v}
	card, rows := itemCard()
	card.AddClass("network-list")
	s.rows = rows
	s.Box = dropdownSection(sectionLabel(v.font, i18n.T("dropdown-network-vpn")), card)
	// The add row activates as a whole.
	s.add = widget.NewItem(v.font, i18n.T("dropdown-network-vpn-add"), "")
	s.add.SetSmall(true)
	s.add.AddClass("vpn-item", "vpn-add")
	s.add.SetOnActivate(v.vpnAdd)
	s.add.SetMedia(widget.NewThemeIcon("ld-plus-symbolic", itemIconPx), false)
	s.rows.Append(s.add)
	s.SetVisible(v.vpn != nil)
	return s
}

// apply rebuilds the rows from one read of the profiles, the add row
// last; a read that changes no row keeps them.
func (s *netVPNs) apply(rows []network.VPN) {
	if slices.Equal(rows, s.listed) {
		return
	}
	s.listed = slices.Clone(rows)
	s.rows.Clear()
	for _, row := range rows {
		s.rows.Append(s.row(row))
	}
	s.rows.Append(s.add)
}

// row is a VPN on lys-ui's Item: the state glyph as the media, the
// name over the state or the gateway's reason, and the edit button as
// the action. Activating the row toggles the tunnel; the edit button keeps its own click. The name and a
// reason keep to one line each, the tooltip holding the reason whole.
func (s *netVPNs) row(r network.VPN) *widget.Item {
	v := s.v
	uuid := r.UUID
	row := widget.NewItem(v.font, r.Name, vpnRowCaption(r))
	row.SetSmall(true)
	row.SetEllipsize(true)
	row.AddClass("vpn-item")
	row.SetOnActivate(func() { v.vpnToggle(uuid) })
	icon := widget.NewThemeIcon(vpnStateIcon(r.State), itemIconPx)
	icon.AddClass("vpn-state-icon")
	switch r.State {
	case network.VPNConnected:
		row.AddClass("connected")
	case network.VPNConnecting:
		row.AddClass("connecting")
	}
	row.SetMedia(icon, false)
	if r.Detail != "" {
		row.AddClass("failed")
		row.SetTooltip(r.Detail)
	}
	gear := widget.NewThemeIcon("ld-settings-symbolic", int(v.px))
	edit := iconButton(gear, i18n.T("dropdown-network-vpn-edit"), func() { v.vpnEdit(uuid) }, "network-vpn-edit", css.Ghost, css.Icon, css.Small)
	row.AddAction(edit)
	return row
}

// vpnEntry is a typed box and the key it fills.
type vpnEntry struct {
	key   string
	entry *widget.Entry
}

// vpnPicker is a fixed-choice field and the values behind its labels.
type vpnPicker struct {
	key    string
	picker *widget.Dropdown
	values []string
}

// netVPNForm is the editor page. The header goes back and, for
// WireGuard, imports a wg-quick file; the scrolled body holds the name,
// the type picker (creating only: another type is another profile),
// the kind's fields, and the raw editor; the footer holds the error and
// Delete (confirmed in a modal alert), Cancel, Save.
type netVPNForm struct {
	*widget.Box
	v          *networkView
	kinds      []network.VPNKind
	selected   int
	editing    string // the profile's UUID; "" creates one
	head       *widget.Item
	importBtn  *widget.Button
	name       *widget.Entry
	kindField  *widget.Field
	kindSlot   *widget.Box
	kindPicker *widget.Dropdown
	fields     *widget.Box
	entries    []vpnEntry
	pickers    []vpnPicker
	advanced   *widget.Button
	advLabel   *widget.Label
	rawHint    *widget.Label
	raw        *widget.TextArea
	rawTyped   bool // the kind has a typed form; the raw editor is extra
	rawOpen    bool
	errLabel   *widget.Label
	// importFailed says the file chooser could not open, the error as
	// its description.
	importFailed *widget.Alert
	deleteBtn    *widget.Button
	// confirming is set while the alert is up; a second Delete click asks
	// nothing more.
	confirming bool
}

func newNetVPNForm(v *networkView) *netVPNForm {
	th := widget.Current()
	f := &netVPNForm{v: v, Box: widget.NewBox(widget.Column, 0, 0)}
	f.AddClass(css.Card, "network-vpn-form")

	// The header goes back, names the page, and imports. Its title is
	// always "New VPN": creating and editing
	// share it.
	f.head = widget.NewItem(v.font, i18n.T("dropdown-network-vpn-new"), "")
	f.head.SetSmall(true)
	backBtn := ghostIconButton("ld-arrow-left-symbolic", int(v.px), i18n.T("dropdown-network-vpn-back"), f.cancel, "network-vpn-back")
	f.head.SetMedia(backBtn, true)
	f.importBtn = ghostIconButton("ld-folder-open-symbolic", int(v.px), i18n.T("dropdown-network-vpn-import"), f.importFile, "network-vpn-import")
	f.head.AddAction(f.importBtn)
	f.Append(f.head, false)

	body := widget.NewBox(widget.Column, th.Spacing, 0)
	f.name = widget.NewEntry(v.font, v.px, 0)
	f.name.AddClass("network-password-input")
	body.Append(widget.NewField(v.font, i18n.T("dropdown-network-vpn-name"), f.name), false)
	f.kindSlot = widget.NewBox(widget.Column, 0, 0)
	f.kindField = widget.NewField(v.font, i18n.T("dropdown-network-vpn-type"), f.kindSlot)
	body.Append(f.kindField, false)
	f.fields = widget.NewBox(widget.Column, th.Spacing, 0)
	body.Append(f.fields, false)
	f.advLabel = widget.NewLabel(v.font, v.px, "", 0)
	f.advanced = dropdownButton(f.advLabel, f.toggleAdvanced, "network-vpn-advanced", css.Ghost, css.Small)
	f.advanced.SetHAlign(widget.AlignStart)
	body.Append(f.advanced, false)
	f.rawHint = widget.NewLabel(v.font, v.px, "", 0)
	f.rawHint.AddClass(css.Caption, css.DimLabel)
	f.rawHint.SetWrap(true)
	body.Append(f.rawHint, false)
	f.raw = widget.NewTextArea(monoFont(v.ctx, v.font, v.px), v.px, 0)
	f.raw.AddClass("network-vpn-raw")
	body.Append(f.raw, false)
	scroll := dropdownScroll(body, "network-vpn-form-scroll")
	scroll.SetMaxContentHeight(vpnFieldsMaxHeight)

	foot := itemBody()
	foot.Append(scroll, true)
	f.errLabel = v.errorLabel()
	foot.Append(f.errLabel, false)
	f.importFailed = writeAlert(v.font, "network-vpn-import-failed")
	foot.Append(f.importFailed, false)
	f.deleteBtn = v.ghostText(i18n.T("dropdown-network-vpn-delete"), "network-vpn-delete", f.askDelete)
	foot.Append(formActions(
		f.deleteBtn,
		v.outlineText(i18n.T("dropdown-network-cancel"), "network-password-cancel", f.cancel),
		v.primaryText(i18n.T("dropdown-network-vpn-save"), "network-password-connect", f.save),
	), false)
	f.Append(foot, true)
	return f
}

// monoFont is the configured monospace face for the raw editor, the
// dropdown's own when it cannot be loaded.
func monoFont(ctx ModuleContext, fallback render.Font, px float64) render.Font {
	if ctx.Config == nil || ctx.Config.General.FontMono == "" {
		return fallback
	}
	face, err := app.Font(ctx.Config.General.FontMono, px)
	if err != nil {
		return fallback
	}
	return app.FontFallback(face)
}

// kind is the picked kind.
func (f *netVPNForm) kind() (network.VPNKind, bool) {
	if f.selected < 0 || f.selected >= len(f.kinds) {
		return network.VPNKind{}, false
	}
	return f.kinds[f.selected], true
}

// open resets the form onto kinds, picking kind (the first when "").
func (f *netVPNForm) open(uuid, name, kind string, values map[string]string) {
	var kinds []network.VPNKind
	if f.v.vpnKinds != nil {
		kinds = f.v.vpnKinds()
	}
	f.kinds = network.KindsWith(kinds, kind)
	f.selected = max(0, slices.IndexFunc(f.kinds, func(k network.VPNKind) bool { return k.ID == kind }))
	f.editing = uuid
	f.setError("")
	f.importFailed.SetVisible(false)
	f.name.SetText(name)
	labels := make([]string, len(f.kinds))
	for i, k := range f.kinds {
		labels[i] = k.Label
	}
	f.kindPicker = widget.NewDropdown(f.v.font, f.v.px, labels, f.selected)
	f.kindPicker.OnSelect = f.kindSelected
	f.kindSlot.Clear()
	f.kindSlot.Append(f.kindPicker, false)
	creating := uuid == ""
	f.kindField.SetVisible(creating)
	f.deleteBtn.SetVisible(!creating)
	f.rebuild(values)
}

// showNew opens the form empty, the name box focused.
func (f *netVPNForm) showNew() {
	f.open("", "", "", nil)
	f.v.focus(f.name)
}

// showEdit opens the form prefilled from what is stored.
func (f *netVPNForm) showEdit(uuid, name, kind string, values map[string]string) {
	f.open(uuid, name, kind, values)
}

// kindSelected switches the type: another type, other fields.
func (f *netVPNForm) kindSelected(i int) {
	if i < 0 || i >= len(f.kinds) || i == f.selected {
		return
	}
	f.selected = i
	f.setError("")
	f.rebuild(nil)
}

// rebuild redraws the fields for the picked kind, prefilled from
// values. A kind without a typed form is the raw editor, open; a typed
// one keeps it behind the advanced toggle, holding what the form has no
// box for, and opened when that is anything.
func (f *netVPNForm) rebuild(values map[string]string) {
	f.entries, f.pickers = nil, nil
	f.fields.Clear()
	kind, _ := f.kind()
	f.importBtn.SetVisible(kind.ID == network.WireGuard)
	f.rawTyped = kind.IsTyped()
	if !f.rawTyped {
		f.raw.SetText(network.RenderRawValues(values))
		f.rawOpen = true
		f.syncRaw(kind)
		return
	}
	extra := kind.Leftovers(values)
	f.raw.SetText(network.RenderRawValues(extra))
	f.rawOpen = len(extra) > 0
	section := ""
	for _, field := range kind.Fields {
		if field.Section != "" && field.Section != section {
			f.fields.Append(sectionLabel(f.v.font, vpntext.SectionHeading(field.Section)), false)
			section = field.Section
		}
		current, has := values[field.Key]
		var control widget.Widget
		if len(field.Choices) > 0 {
			control = f.picker(field, current, has)
		} else {
			control = f.entry(kind, field, current)
		}
		row := widget.NewField(f.v.font, vpntext.FieldLabel(field), control)
		if !field.Required {
			row.AddClass("optional")
		}
		f.fields.Append(row, false)
	}
	f.syncRaw(kind)
}

// picker draws a fixed-choice field. A stored value this build does not
// offer keeps the first choice rather than rewriting itself unseen.
func (f *netVPNForm) picker(field network.VPNField, current string, has bool) *widget.Dropdown {
	labels := make([]string, len(field.Choices))
	values := make([]string, len(field.Choices))
	for i, c := range field.Choices {
		labels[i], values[i] = vpntext.ChoiceLabel(c), c.Value
	}
	selected := 0
	if i := slices.Index(values, current); has && i >= 0 {
		selected = i
	}
	p := widget.NewDropdown(f.v.font, f.v.px, labels, selected)
	f.pickers = append(f.pickers, vpnPicker{key: field.Key, picker: p, values: values})
	return p
}

// entry draws a typed box: masked with the reveal toggle when secret,
// and for WireGuard's private key with the generate button and the
// public key it derives under it.
func (f *netVPNForm) entry(kind network.VPNKind, field network.VPNField, current string) widget.Widget {
	var entry *widget.Entry
	if field.Secret {
		entry = newSecretEntry(f.v.font, f.v.px).entry
	} else {
		entry = widget.NewEntry(f.v.font, f.v.px, 0)
	}
	entry.AddClass("network-password-input")
	input := widget.NewBox(widget.Column, widget.Current().Spacing/2, 0)
	input.Append(entry, false)
	entry.SetPlaceholder(field.Placeholder)
	entry.SetText(current)
	entry.OnActivate = func(string) { f.save() }
	var readout *widget.RichLabel
	if kind.ID == network.WireGuard && field.Key == wireGuardPrivateKey {
		// The entry's primary icon generates a keypair; the reveal
		// toggle keeps the secondary slot.
		entry.SetLeadingIcon(iconGenerateKey, f.v.px, func() {
			entry.SetText(network.GenerateKeyPair().Private)
		})
		readout = widget.NewRichLabel(f.v.font, f.v.px*styling.Caption.Ratio(), vpntext.PublicKey(current), 0)
		readout.SetVariants(app.FontVariantsOf(f.v.font))
		readout.AddClass("network-vpn-public-key", css.Caption, css.DimLabel, css.Monospace)
		readout.SetWrap(true)
		readout.SetSelectable(true)
		input.Append(readout, false)
	}
	// A box stops being wrong the moment it is edited.
	entry.OnChanged = func(text string) {
		entry.RemoveClass("error")
		if readout != nil {
			readout.SetMarkup(vpntext.PublicKey(text))
		}
	}
	f.entries = append(f.entries, vpnEntry{key: field.Key, entry: entry})
	return input
}

// syncRaw shows the raw editor's parts for the current state.
func (f *netVPNForm) syncRaw(kind network.VPNKind) {
	f.advanced.SetVisible(f.rawTyped)
	if f.rawOpen {
		f.advLabel.SetText(i18n.T("dropdown-network-vpn-advanced-hide"))
	} else {
		f.advLabel.SetText(i18n.T("dropdown-network-vpn-advanced-show"))
	}
	f.rawHint.SetText(vpntext.RawHint(kind))
	f.rawHint.SetVisible(f.rawOpen)
	f.raw.SetVisible(f.rawOpen)
}

func (f *netVPNForm) toggleAdvanced() {
	f.rawOpen = !f.rawOpen
	kind, _ := f.kind()
	f.syncRaw(kind)
}

// values is both editors: the raw keys with the typed boxes and the
// pickers over them.
func (f *netVPNForm) values() map[string]string {
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

// save validates, marks every refused box, and hands a valid profile to
// the view, which writes it off the loop.
func (f *netVPNForm) save() {
	kind, ok := f.kind()
	if !ok {
		return
	}
	name := f.name.Text()
	if strings.TrimSpace(name) == "" {
		f.setError(i18n.T("dropdown-network-vpn-name-required"))
		return
	}
	values := f.values()
	problems := kind.Check(values)
	f.markBad(problems)
	if len(problems) > 0 {
		f.setError(vpntext.Problem(problems[0]))
		return
	}
	f.setError("")
	f.v.vpnSave(f.editing, kind.ID, name, values)
}

// markBad puts the error mark on the refused boxes and takes it off the
// rest.
func (f *netVPNForm) markBad(bad []network.VPNProblem) {
	for _, e := range f.entries {
		if slices.ContainsFunc(bad, func(b network.VPNProblem) bool { return b.Field.Key == e.key }) {
			e.entry.AddClass("error")
		} else {
			e.entry.RemoveClass("error")
		}
	}
}

// fail shows that NM refused; the form stays, values
// intact, so the user can correct what it refused.
func (f *netVPNForm) fail(reason string) { f.setError(reason) }

func (f *netVPNForm) setError(reason string) {
	f.errLabel.SetText(reason)
	f.errLabel.SetVisible(reason != "")
}

func (f *netVPNForm) cancel() {
	f.v.vpnCloseForm()
}

// askDelete confirms a deletion: a profile is recoverable from nowhere
// else (a WireGuard private key lives only in NM), so Delete asks in a
// modal alert - Cancel the default - and nothing goes until it is
// accepted.
func (f *netVPNForm) askDelete() {
	if f.editing == "" || f.confirming {
		return
	}
	f.confirming = true
	uuid := f.editing
	f.v.ctx.showAlert(alertSpec{
		message: i18n.T("dropdown-network-vpn-delete-confirm", i18n.Str("name", f.name.Text())),
		detail:  i18n.T("dropdown-network-vpn-delete-confirm-detail"),
		cancel:  i18n.T("dropdown-network-cancel"),
		accept:  i18n.T("dropdown-network-vpn-delete"),
	}, func(accepted bool) {
		f.confirming = false
		if accepted {
			f.v.vpnDelete(uuid)
		}
	})
}

// importFile imports a wg-quick file through the portal's
// chooser, its outcome handled back on the loop.
func (f *netVPNForm) importFile() {
	open := f.v.openFile
	if open == nil {
		return
	}
	life := f.v.life
	go func() {
		path, err := open(life, i18n.T("dropdown-network-vpn-import"),
			portal.FileFilter{Name: i18n.T("dropdown-network-vpn-import-filter"), Patterns: []string{"*.conf"}})
		f.v.ctx.Invoke(func() { f.importDone(path, err) })
	}()
}

// importDone reads the picked file here rather than off the loop: a
// wg-quick file is a few hundred bytes on local
// disk. Dismissing the chooser is not a failure, and saying so would
// be noise; a chooser that cannot open says so in the form's alert,
// the error as its description.
func (f *netVPNForm) importDone(path string, err error) {
	if errors.Is(err, portal.ErrCancelled) {
		return
	}
	reportWrite(f.importFailed, "network", i18n.T("dropdown-network-vpn-import-error"), err)
	if err != nil {
		return
	}
	values, err := network.ReadWgQuick(path)
	if err != nil {
		log.Printf("network: VPN import: %v", err)
		f.setError(i18n.T("dropdown-network-vpn-import-failed"))
		return
	}
	f.imported(values["interface"], values)
}

// imported applies the file's values, and its interface as the
// name unless one is typed already.
func (f *netVPNForm) imported(name string, values map[string]string) {
	f.setError("")
	if strings.TrimSpace(f.name.Text()) == "" {
		f.name.SetText(name)
	}
	f.rebuild(values)
}

// The page names in the dropdown's body stack.
const (
	netPageBrowse = "browse"
	netPageEdit   = "edit"
)

// vpnToggle connects a VPN that is down and disconnects one that is up;
// the rows follow NM's state on their own.
func (v *networkView) vpnToggle(uuid string) {
	vpn := v.vpn
	if vpn == nil {
		return
	}
	name := v.vpnName(uuid)
	v.act(func(ctx context.Context) error { return vpn.Toggle(ctx, uuid) }, func(err error) string {
		return i18n.T("dropdown-network-vpn-error-toggle", i18n.Str("name", name), i18n.Str("error", err.Error()))
	}, nil)
}

// vpnRow is the profile the rows show as uuid.
func (v *networkView) vpnRow(uuid string) (network.VPN, bool) {
	i := slices.IndexFunc(v.cur.vpns, func(r network.VPN) bool { return r.UUID == uuid })
	if i < 0 {
		return network.VPN{}, false
	}
	return v.cur.vpns[i], true
}

// vpnName is the name the row of uuid shows, the uuid for a profile no
// row shows.
func (v *networkView) vpnName(uuid string) string {
	if r, ok := v.vpnRow(uuid); ok {
		return r.Name
	}
	return uuid
}

// vpnAdd opens the editor empty.
func (v *networkView) vpnAdd() {
	v.vpnForm.showNew()
	v.body.Show(netPageEdit)
}

// vpnEdit reads the saved profile (not what the row shows) and opens
// the editor on it. A profile that cannot be read stays closed: an
// editor open on stale values could save them over it.
func (v *networkView) vpnEdit(uuid string) {
	vpn := v.vpn
	if vpn == nil {
		return
	}
	row, _ := v.vpnRow(uuid)
	name, shown := row.Name, v.vpnName(uuid)
	v.showActionError("")
	go func() {
		dict, err := vpn.SettingsOf(v.life, uuid)
		v.ctx.Invoke(func() {
			if err != nil {
				log.Printf("network: cannot read VPN profile %s: %v", uuid, err)
				v.showActionError(i18n.T("dropdown-network-vpn-error-read", i18n.Str("name", shown), i18n.Str("error", err.Error())))
				return
			}
			v.vpnForm.showEdit(uuid, name, network.KindOf(dict), network.ReadProfileValues(dict))
			v.body.Show(netPageEdit)
		})
	}()
}

// vpnSave writes the form's profile: created without a UUID, rewritten
// in place with one. The page goes back at once; a refusal brings the
// form back with the reason.
func (v *networkView) vpnSave(uuid, kind, name string, values map[string]string) {
	vpn := v.vpn
	if vpn == nil {
		return
	}
	v.body.Show(netPageBrowse)
	go func() {
		var err error
		if uuid == "" {
			err = vpn.Add(context.Background(), kind, name, values)
		} else {
			err = vpn.Update(context.Background(), uuid, kind, name, values)
		}
		v.vpnWritten(err, "cannot save VPN profile")
	}()
}

// vpnDelete removes a profile.
func (v *networkView) vpnDelete(uuid string) {
	vpn := v.vpn
	if vpn == nil {
		return
	}
	v.body.Show(netPageBrowse)
	go func() { v.vpnWritten(vpn.Remove(context.Background(), uuid), "cannot delete VPN profile") }()
}

// vpnWritten reopens the form with NM's refusal; success needs nothing,
// NM announces the profile and the rows rebuild.
func (v *networkView) vpnWritten(err error, what string) {
	if err == nil {
		return
	}
	log.Printf("network: %s: %v", what, err)
	v.ctx.Invoke(func() {
		v.vpnForm.fail(err.Error())
		v.body.Show(netPageEdit)
	})
}

func (v *networkView) vpnCloseForm() { v.body.Show(netPageBrowse) }
