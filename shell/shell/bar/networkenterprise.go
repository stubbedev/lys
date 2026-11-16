package bar

import (
	"context"
	"errors"
	"log"

	"github.com/stubbedev/lys/desktop/i18n"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/internal/portal"
	"github.com/stubbedev/lys/shell/service/network"
)

var certificatePatterns = []string{"*.pem", "*.crt", "*.cer", "*.der", "*.key", "*.p12", "*.pfx"}

// eapTarget is the network an enterprise sign-in joins: a listed
// access point or a hidden network named in the hidden form.
type eapTarget interface {
	ssid() string
	icon() string
	join(w wifiControl, eap network.EAPSettings) func(context.Context, func(network.DeviceState)) error
}

type listedEAP struct {
	path     dbus.ObjectPath
	name     string
	strength uint8
}

func (t listedEAP) ssid() string { return t.name }

func (t listedEAP) icon() string { return signalStrengthIcon(t.strength) }

func (t listedEAP) join(w wifiControl, eap network.EAPSettings) func(context.Context, func(network.DeviceState)) error {
	return func(ctx context.Context, progress func(network.DeviceState)) error {
		return w.ConnectEnterpriseAndWait(ctx, t.path, eap, progress)
	}
}

type hiddenEAP struct{ net network.HiddenNetwork }

func (t hiddenEAP) ssid() string { return t.net.Name() }

func (t hiddenEAP) icon() string { return hiddenNetworkIcon }

func (t hiddenEAP) join(w wifiControl, eap network.EAPSettings) func(context.Context, func(network.DeviceState)) error {
	return func(ctx context.Context, progress func(network.DeviceState)) error {
		return w.JoinHiddenEnterpriseAndWait(ctx, t.net, eap, progress)
	}
}

type netPhase2Choice struct {
	method   network.EAPMethod
	dropdown *widget.DropdownOf[network.Phase2Auth]
	field    *widget.Field
}

// netEnterpriseForm signs in to an enterprise (802.1X) network: the EAP
// method and, for a tunneled one, its inner authentication, the
// identity and the password or, for TLS, the user certificate and key,
// and the CA certificate or server domain the server is checked
// against.
type netEnterpriseForm struct {
	*formCard
	v              *networkView
	target         eapTarget
	method         *widget.DropdownOf[network.EAPMethod]
	phase2s        []netPhase2Choice
	identity       *widget.Entry
	anonymous      *widget.Entry
	anonymousField *widget.Field
	password       *secretEntry
	passwordField  *widget.Field
	clientCert     *widget.Entry
	privateKey     *widget.Entry
	keyPassword    *secretEntry
	tlsFields      []*widget.Field
	caCert         *widget.Entry
	domain         *widget.Entry
	errLabel       *widget.Label
}

func newNetEnterpriseForm(v *networkView) *netEnterpriseForm {
	f := &netEnterpriseForm{v: v}
	f.formCard = newFormCard(v.font, v.px, signalStrengthIcon(0), i18n.T("dropdown-network-cancel"), f.cancel, "network-password-card", "network-enterprise-card")
	methods := make([]widget.DropdownItem[network.EAPMethod], len(network.EAPMethods))
	for i, m := range network.EAPMethods {
		methods[i] = widget.DropdownItem[network.EAPMethod]{Label: m.String(), Value: m}
	}
	f.method = widget.NewDropdownOf(v.font, v.px, methods, 0)
	f.method.AddClass("network-enterprise-method")
	f.method.OnSelect = func(int, network.EAPMethod) { f.syncMethod() }
	f.body.Append(widget.NewField(v.font, i18n.T("dropdown-network-enterprise-method"), f.method), false)
	for _, m := range network.EAPMethods {
		if !m.UsesPassword() {
			continue
		}
		items := make([]widget.DropdownItem[network.Phase2Auth], 0, len(m.Phase2s()))
		for _, p := range m.Phase2s() {
			items = append(items, widget.DropdownItem[network.Phase2Auth]{Label: p.String(), Value: p})
		}
		dd := widget.NewDropdownOf(v.font, v.px, items, 0)
		dd.AddClass("network-enterprise-phase2")
		field := widget.NewField(v.font, i18n.T("dropdown-network-enterprise-phase2"), dd)
		f.body.Append(field, false)
		f.phase2s = append(f.phase2s, netPhase2Choice{method: m, dropdown: dd, field: field})
	}
	f.identity = f.textEntry("network-enterprise-identity")
	f.body.Append(widget.NewField(v.font, i18n.T("dropdown-network-enterprise-identity"), f.identity), false)
	f.anonymous = f.textEntry("network-enterprise-anonymous-identity")
	f.anonymous.SetPlaceholder(i18n.T("dropdown-network-enterprise-optional"))
	f.anonymousField = widget.NewField(v.font, i18n.T("dropdown-network-enterprise-anonymous-identity"), f.anonymous)
	f.body.Append(f.anonymousField, false)
	f.password = f.secret("network-password-input")
	f.passwordField = widget.NewField(v.font, i18n.T("dropdown-network-enterprise-password"), f.password.entry)
	f.body.Append(f.passwordField, false)
	var certField, keyField *widget.Field
	f.clientCert, certField = f.pathField(i18n.T("dropdown-network-enterprise-client-cert"), "network-enterprise-client-cert")
	f.privateKey, keyField = f.pathField(i18n.T("dropdown-network-enterprise-private-key"), "network-enterprise-private-key")
	f.keyPassword = f.secret("network-enterprise-private-key-password")
	f.keyPassword.entry.SetPlaceholder(i18n.T("dropdown-network-enterprise-optional"))
	f.tlsFields = []*widget.Field{certField, keyField, widget.NewField(v.font, i18n.T("dropdown-network-enterprise-private-key-password"), f.keyPassword.entry)}
	for _, field := range f.tlsFields {
		f.body.Append(field, false)
	}
	var caField *widget.Field
	f.caCert, caField = f.pathField(i18n.T("dropdown-network-enterprise-ca-cert"), "network-enterprise-ca-cert")
	f.body.Append(caField, false)
	f.domain = f.textEntry("network-enterprise-domain")
	f.body.Append(widget.NewField(v.font, i18n.T("dropdown-network-enterprise-domain"), f.domain), false)
	f.errLabel = v.errorLabel()
	f.body.Append(f.errLabel, false)
	f.body.Append(formActions(
		v.outlineText(i18n.T("dropdown-network-cancel"), "network-enterprise-cancel", f.cancel),
		v.primaryText(i18n.T("dropdown-network-connect"), "network-enterprise-connect", f.connect),
	), false)
	f.SetVisible(false)
	return f
}

func (f *netEnterpriseForm) textEntry(class string) *widget.Entry {
	e := widget.NewEntry(f.v.font, f.v.px, 0)
	e.AddClass(class)
	e.OnActivate = func(string) { f.connect() }
	return e
}

func (f *netEnterpriseForm) secret(class string) *secretEntry {
	s := newSecretEntry(f.v.font, f.v.px)
	s.entry.AddClass(class)
	s.entry.OnActivate = func(string) { f.connect() }
	return s
}

// pathField is a certificate or key path box, with the file chooser
// beside it when there is one.
func (f *netEnterpriseForm) pathField(label, class string) (*widget.Entry, *widget.Field) {
	e := f.textEntry(class)
	if f.v.openFile == nil {
		return e, widget.NewField(f.v.font, label, e)
	}
	row := widget.NewBox(widget.Row, widget.Current().Spacing, 0)
	row.Append(e, true)
	row.Append(ghostIconButton("ld-folder-open-symbolic", int(f.v.px), i18n.T("dropdown-network-enterprise-browse"), func() { f.browse(e, label) }, class+"-browse"), false)
	return e, widget.NewField(f.v.font, label, row)
}

// show opens the form for target, keeping what was typed for the same
// network but its secrets, and saying errMessage when there is one.
func (f *netEnterpriseForm) show(target eapTarget, errMessage string) {
	if f.target == nil || target.ssid() != f.target.ssid() {
		f.method.SetSelected(0)
		for _, c := range f.phase2s {
			c.dropdown.SetSelected(0)
		}
		for _, e := range []*widget.Entry{f.identity, f.anonymous, f.clientCert, f.privateKey, f.caCert, f.domain} {
			e.SetText("")
		}
	}
	f.target = target
	f.resetSecrets()
	f.icon.SetThemeName(target.icon())
	f.head.SetTitle(target.ssid())
	f.head.SetDescription(securityLabel(network.SecurityEnterprise))
	f.showError(errMessage)
	f.syncMethod()
	f.SetVisible(true)
	f.v.focus(f.identity)
}

func (f *netEnterpriseForm) syncMethod() {
	m := f.method.Value()
	for _, c := range f.phase2s {
		c.field.SetVisible(c.method == m)
	}
	f.anonymousField.SetVisible(m.UsesPassword())
	f.passwordField.SetVisible(m.UsesPassword())
	for _, field := range f.tlsFields {
		field.SetVisible(!m.UsesPassword())
	}
}

func (f *netEnterpriseForm) resetSecrets() {
	f.password.reset()
	f.keyPassword.reset()
}

func (f *netEnterpriseForm) showError(text string) {
	f.errLabel.SetText(text)
	f.errLabel.SetVisible(text != "")
}

// input is the sign-in as typed, only the fields the picked method
// uses.
func (f *netEnterpriseForm) input() network.EAPInput {
	in := network.EAPInput{
		Method:   f.method.Value(),
		Identity: f.identity.Text(),
		CACert:   f.caCert.Text(),
		Domain:   f.domain.Text(),
	}
	if !in.Method.UsesPassword() {
		in.ClientCert = f.clientCert.Text()
		in.PrivateKey = f.privateKey.Text()
		in.PrivateKeyPassword = f.keyPassword.entry.Text()
		return in
	}
	for _, c := range f.phase2s {
		if c.method == in.Method {
			in.Phase2 = c.dropdown.Value()
		}
	}
	in.AnonymousIdentity = f.anonymous.Text()
	in.Password = f.password.entry.Text()
	return in
}

func (f *netEnterpriseForm) connect() {
	eap, err := network.NewEAPSettings(f.input())
	if err != nil {
		f.showError(enterpriseError(err))
		return
	}
	f.resetSecrets()
	f.v.connectEnterprise(f.target, eap)
}

func (f *netEnterpriseForm) cancel() {
	f.resetSecrets()
	f.v.state, f.v.selection = netNormal, nil
	f.v.rebuildList()
}

// browse fills target from the portal's file chooser, the outcome
// handled back on the loop.
func (f *netEnterpriseForm) browse(target *widget.Entry, title string) {
	open, life := f.v.openFile, f.v.life
	f.v.ctx.Go(func() {
		path, err := open(life, title, portal.FileFilter{Name: i18n.T("dropdown-network-enterprise-browse-filter"), Patterns: certificatePatterns})
		f.v.ctx.Invoke(func() { f.browsed(target, path, err) })
	})
}

func (f *netEnterpriseForm) browsed(target *widget.Entry, path string, err error) {
	switch {
	case errors.Is(err, portal.ErrCancelled):
		return
	case err != nil:
		log.Printf("network: enterprise file chooser: %v", err)
		f.showError(i18n.T("dropdown-network-enterprise-error-browse", i18n.Str("error", err.Error())))
		return
	}
	target.SetText(path)
}

// connectEnterprise joins target with eap; a refused sign-in opens
// the form for it again.
func (v *networkView) connectEnterprise(target eapTarget, eap network.EAPSettings) {
	w := v.wifi()
	if w == nil {
		return
	}
	v.signIn = target
	v.watchConnect(target.ssid(), target.join(w, eap))
}

// askSignIn opens the enterprise sign-in for target.
func (v *networkView) askSignIn(target eapTarget, errMessage string) {
	v.state = netEnterpriseEntry
	v.enterprise.show(target, errMessage)
	v.rebuildList()
}

// authFailureMessage is what a refused sign-in says when its form opens
// again.
func authFailureMessage(security network.SecurityType) string {
	if security == network.SecurityEnterprise {
		return i18n.T("dropdown-network-enterprise-error-auth")
	}
	return i18n.T("dropdown-network-error-wrong-password")
}

func enterpriseError(err error) string {
	switch {
	case errors.Is(err, network.ErrEAPIdentityEmpty):
		return i18n.T("dropdown-network-enterprise-error-identity")
	case errors.Is(err, network.ErrEAPPasswordEmpty):
		return i18n.T("dropdown-network-enterprise-error-password")
	case errors.Is(err, network.ErrEAPServerUnchecked):
		return i18n.T("dropdown-network-enterprise-error-server")
	case errors.Is(err, network.ErrEAPDomainInvalid):
		return i18n.T("dropdown-network-enterprise-error-domain")
	case errors.Is(err, network.ErrEAPPathNotAbsolute), errors.Is(err, network.ErrEAPPathInvalid):
		return i18n.T("dropdown-network-enterprise-error-path")
	case errors.Is(err, network.ErrEAPClientCertMissing):
		return i18n.T("dropdown-network-enterprise-error-client-cert")
	case errors.Is(err, network.ErrEAPPrivateKeyMissing):
		return i18n.T("dropdown-network-enterprise-error-private-key")
	}
	return err.Error()
}
