package settings

import (
	"context"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/network"
	"github.com/stubbedev/lys/desktop/service/network/eaptext"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// eapTarget is the network an enterprise sign-in joins: one in range or
// a hidden one named in the hidden form.
type eapTarget interface {
	ssid() string
	join(ctx context.Context, ctl netControl, eap network.EAPSettings) error
}

type listedEAP struct{ ap network.AccessPoint }

func (t listedEAP) ssid() string { return t.ap.Name() }

func (t listedEAP) join(ctx context.Context, ctl netControl, eap network.EAPSettings) error {
	return ctl.connectEnterprise(ctx, t.ap, eap)
}

type hiddenEAP struct{ net network.HiddenNetwork }

func (t hiddenEAP) ssid() string { return t.net.Name() }

func (t hiddenEAP) join(ctx context.Context, ctl netControl, eap network.EAPSettings) error {
	return ctl.joinHiddenEnterprise(ctx, t.net, eap)
}

type eapPhase2 struct {
	method network.EAPMethod
	choice *widget.DropdownOf[network.Phase2Auth]
	field  *widget.Field
}

// eapForm signs in to an enterprise (802.1X) network as the bar's
// network menu does: the EAP method and, for a tunneled one, its inner
// authentication, the identity and the password or, for TLS, the user
// certificate and key, and the CA certificate or server domain the
// server is checked against.
type eapForm struct {
	*widget.Box
	p              *networkPanel
	target         eapTarget
	head           *widget.Label
	method         *widget.DropdownOf[network.EAPMethod]
	phase2s        []eapPhase2
	identity       *widget.Entry
	anonymous      *widget.Entry
	anonymousField *widget.Field
	password       *widget.Entry
	passwordField  *widget.Field
	clientCert     *widget.Entry
	privateKey     *widget.Entry
	keyPassword    *widget.Entry
	tlsFields      []*widget.Field
	caCert         *widget.Entry
	domain         *widget.Entry
	problem        *widget.Label
}

func newEAPForm(p *networkPanel) *eapForm {
	k := p.k
	f := &eapForm{Box: gapBox(widget.Column), p: p}
	f.AddClass("network-enterprise")
	f.head = wrapped(k.label("", "setting-label"))
	f.Append(f.head, false)
	methods := make([]widget.DropdownItem[network.EAPMethod], len(network.EAPMethods))
	for i, m := range network.EAPMethods {
		methods[i] = widget.DropdownItem[network.EAPMethod]{Label: m.String(), Value: m}
	}
	f.method = widget.NewDropdownOf(k.face, k.textPx(), methods, 0)
	f.method.SetAccessibleName(i18n.T("dropdown-network-enterprise-method"))
	f.method.OnSelect = func(int, network.EAPMethod) { f.syncMethod() }
	f.Append(widget.NewField(k.face, i18n.T("dropdown-network-enterprise-method"), f.method), false)
	for _, m := range network.EAPMethods {
		if !m.UsesPassword() {
			continue
		}
		items := make([]widget.DropdownItem[network.Phase2Auth], len(m.Phase2s()))
		for i, a := range m.Phase2s() {
			items[i] = widget.DropdownItem[network.Phase2Auth]{Label: a.String(), Value: a}
		}
		choice := widget.NewDropdownOf(k.face, k.textPx(), items, 0)
		choice.SetAccessibleName(i18n.T("dropdown-network-enterprise-phase2"))
		field := widget.NewField(k.face, i18n.T("dropdown-network-enterprise-phase2"), choice)
		f.Append(field, false)
		f.phase2s = append(f.phase2s, eapPhase2{method: m, choice: choice, field: field})
	}
	var identityField *widget.Field
	f.identity, identityField = f.text("dropdown-network-enterprise-identity", false)
	f.Append(identityField, false)
	f.anonymous, f.anonymousField = f.text("dropdown-network-enterprise-anonymous-identity", false)
	f.anonymous.SetPlaceholder(i18n.T("dropdown-network-enterprise-optional"))
	f.Append(f.anonymousField, false)
	f.password, f.passwordField = f.text("dropdown-network-enterprise-password", true)
	f.Append(f.passwordField, false)
	var certField, keyField, keyPasswordField *widget.Field
	f.clientCert, certField = f.path("dropdown-network-enterprise-client-cert")
	f.privateKey, keyField = f.path("dropdown-network-enterprise-private-key")
	f.keyPassword, keyPasswordField = f.text("dropdown-network-enterprise-private-key-password", true)
	f.keyPassword.SetPlaceholder(i18n.T("dropdown-network-enterprise-optional"))
	f.tlsFields = []*widget.Field{certField, keyField, keyPasswordField}
	for _, field := range f.tlsFields {
		f.Append(field, false)
	}
	var caField, domainField *widget.Field
	f.caCert, caField = f.path("dropdown-network-enterprise-ca-cert")
	f.Append(caField, false)
	f.domain, domainField = f.text("dropdown-network-enterprise-domain", false)
	f.Append(domainField, false)
	f.problem = wrapped(k.label("", "setting-problem", css.Caption, css.Error))
	f.problem.SetVisible(false)
	f.Append(f.problem, false)
	f.Append(formButtons(
		k.button(k.label(i18n.T("dropdown-network-cancel")), f.cancel, css.Small, css.Flat),
		k.button(k.label(i18n.T("dropdown-network-connect")), f.connect, css.Small, css.SuggestedAction),
	), false)
	f.syncMethod()
	return f
}

func (f *eapForm) text(labelKey string, secret bool) (*widget.Entry, *widget.Field) {
	e := f.p.k.entry()
	if secret {
		e.SetEcho(widget.EchoPassword)
	}
	e.SetAccessibleName(i18n.T(labelKey))
	e.OnActivate = func(string) { f.connect() }
	return e, widget.NewField(f.p.k.face, i18n.T(labelKey), e)
}

// path is a certificate or key path box with the file chooser beside
// it.
func (f *eapForm) path(labelKey string) (*widget.Entry, *widget.Field) {
	e := f.p.k.entry()
	e.SetAccessibleName(i18n.T(labelKey))
	e.OnActivate = func(string) { f.connect() }
	row := gapBox(widget.Row)
	row.Append(e, true)
	row.AppendAligned(f.p.k.browseButton(e.SetText), false, widget.AlignCenter)
	return e, widget.NewField(f.p.k.face, i18n.T(labelKey), row)
}

// open shows the form for target, keeping what was typed for the same
// network but its secrets.
func (f *eapForm) open(target eapTarget) {
	if f.target == nil || f.target.ssid() != target.ssid() {
		f.method.SetSelected(0)
		for _, c := range f.phase2s {
			c.choice.SetSelected(0)
		}
		for _, e := range []*widget.Entry{f.identity, f.anonymous, f.clientCert, f.privateKey, f.caCert, f.domain} {
			e.SetText("")
		}
	}
	f.target = target
	f.head.SetText(locale.T().Get("settings-wifi-enterprise-title", i18n.Str("name", target.ssid())))
	f.forgetSecrets()
	f.say("")
	f.syncMethod()
}

func (f *eapForm) syncMethod() {
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

func (f *eapForm) forgetSecrets() {
	f.password.SetText("")
	f.keyPassword.SetText("")
}

func (f *eapForm) say(text string) {
	f.problem.SetText(text)
	f.problem.SetVisible(text != "")
}

// input is the sign-in as typed, only the fields the method uses.
func (f *eapForm) input() network.EAPInput {
	in := network.EAPInput{Method: f.method.Value(), Identity: f.identity.Text(), CACert: f.caCert.Text(), Domain: f.domain.Text()}
	if !in.Method.UsesPassword() {
		in.ClientCert, in.PrivateKey, in.PrivateKeyPassword = f.clientCert.Text(), f.privateKey.Text(), f.keyPassword.Text()
		return in
	}
	for _, c := range f.phase2s {
		if c.method == in.Method {
			in.Phase2 = c.choice.Value()
		}
	}
	in.AnonymousIdentity, in.Password = f.anonymous.Text(), f.password.Text()
	return in
}

func (f *eapForm) connect() {
	if f.target == nil {
		return
	}
	eap, err := network.NewEAPSettings(f.input())
	if err != nil {
		f.say(eaptext.Problem(err))
		return
	}
	f.forgetSecrets()
	f.p.joinEnterprise(f.target, eap)
}

func (f *eapForm) cancel() {
	f.forgetSecrets()
	f.target = nil
	f.p.show()
}
