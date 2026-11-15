package bar

import (
	"errors"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/service/network"
)

var hiddenSecurities = []network.SecurityType{network.SecurityWPA2, network.SecurityWPA3, network.SecurityWEP, network.SecurityNone}

type netHiddenForm struct {
	*formCard
	v             *networkView
	name          *widget.Entry
	security      *widget.DropdownOf[network.SecurityType]
	passwordField *widget.Field
	password      *secretEntry
	errLabel      *widget.Label
}

func newNetHiddenForm(v *networkView) *netHiddenForm {
	f := &netHiddenForm{v: v}
	f.formCard = newFormCard(v.font, v.px, "ld-wifi-symbolic", f.cancel, "network-password-card", "network-hidden-card")
	f.head.SetTitle(i18n.T("dropdown-network-hidden-title"))
	f.name = widget.NewEntry(v.font, v.px, 0)
	f.name.AddClass("network-hidden-name")
	f.name.OnActivate = func(string) { f.join() }
	f.body.Append(widget.NewField(v.font, i18n.T("dropdown-network-hidden-name"), f.name), false)
	items := make([]widget.DropdownItem[network.SecurityType], len(hiddenSecurities))
	for i, s := range hiddenSecurities {
		items[i] = widget.DropdownItem[network.SecurityType]{Label: securityLabel(s), Value: s}
	}
	f.security = widget.NewDropdownOf(v.font, v.px, items, 0)
	f.security.OnSelect = func(int, network.SecurityType) { f.syncPassword() }
	f.body.Append(widget.NewField(v.font, i18n.T("dropdown-network-hidden-security"), f.security), false)
	f.password = newSecretEntry(v.font, v.px)
	f.password.entry.AddClass("network-password-input")
	f.password.entry.OnActivate = func(string) { f.join() }
	f.passwordField = widget.NewField(v.font, i18n.T("dropdown-network-hidden-password"), f.password.entry)
	f.body.Append(f.passwordField, false)
	f.errLabel = v.errorLabel()
	f.body.Append(f.errLabel, false)
	f.body.Append(formActions(
		v.outlineText(i18n.T("dropdown-network-cancel"), "network-hidden-cancel", f.cancel),
		v.primaryText(i18n.T("dropdown-network-connect"), "network-hidden-connect", f.join),
	), false)
	f.SetVisible(false)
	return f
}

func (f *netHiddenForm) show() {
	f.name.SetText("")
	f.security.SetSelected(0)
	f.password.reset()
	f.showError("")
	f.syncPassword()
	f.SetVisible(true)
	f.v.focus(f.name)
}

func (f *netHiddenForm) syncPassword() {
	f.passwordField.SetVisible(f.security.Value().RequiresPassword())
}

func (f *netHiddenForm) showError(text string) {
	f.errLabel.SetText(text)
	f.errLabel.SetVisible(text != "")
}

func (f *netHiddenForm) join() {
	net, err := network.NewHiddenNetwork(f.name.Text(), f.security.Value())
	if err != nil {
		f.showError(hiddenNetworkError(err))
		return
	}
	password := ""
	if net.Security().RequiresPassword() {
		password = f.password.entry.Text()
		if password == "" && !f.v.cur.known[net.Name()] {
			f.showError(i18n.T("dropdown-network-hidden-error-password"))
			return
		}
	}
	f.password.reset()
	f.v.joinHidden(net, password)
}

func (f *netHiddenForm) cancel() {
	f.password.reset()
	f.v.state = netNormal
	f.v.rebuildList()
}

func hiddenNetworkError(err error) string {
	switch {
	case errors.Is(err, network.ErrHiddenNameEmpty):
		return i18n.T("dropdown-network-hidden-error-name-empty")
	case errors.Is(err, network.ErrHiddenNameTooLong):
		return i18n.T("dropdown-network-hidden-error-name-too-long")
	}
	return err.Error()
}
