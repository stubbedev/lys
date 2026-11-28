package filebrowser

import (
	"context"
	"crypto/x509"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/desktop/vfs"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"
)

// connectTimeout bounds one attempt to connect, the login included.
const connectTimeout = time.Minute

// connectPanel is the band that connects to a server: the address with
// the servers connected to lately, then what the connection asks (the
// login, an unknown host key or certificate), or why it failed.
type connectPanel struct {
	box, body *widget.Box
	note      *widget.Label
	address   *widget.Entry
	user      *widget.Entry
	domain    *widget.Entry
	password  *widget.Entry
	remember  *widget.ChoiceChips
	choices   []Remember
	// recent are the servers connected to lately, newest first.
	recent []string
	// gen counts the attempts: a result for an earlier one is dropped,
	// and cancel ends the one running.
	gen    uint64
	cancel context.CancelFunc
	// loc is the location being connected to; onAddress is set while
	// the band asks for the address.
	loc       vfs.Location
	onAddress bool
}

// newConnectPanel builds the band, hidden.
func (c *Browser) newConnectPanel() *connectPanel {
	th := widget.Current()
	p := &connectPanel{box: widget.NewBox(widget.Column, 0, 0)}
	inner := Section(widget.Column, "file-chooser-connect")
	title := c.Label(i18n.T("file-chooser-connect-title"), "file-chooser-connect-title", css.Heading)
	inner.Append(title, false)
	p.note = c.Label("", "file-chooser-connect-note")
	p.note.SetWrap(true)
	inner.Append(p.note, false)
	p.body = widget.NewBox(widget.Column, th.Spacing, 0)
	inner.Append(p.body, false)
	p.box.Append(inner, false)
	p.box.Append(widget.NewSeparator(widget.Horizontal), false)
	p.box.SetVisible(false)

	p.address = c.entry("file-chooser-connect-address", i18n.T("file-chooser-placeholder-server"))
	p.address.OnActivate = func(text string) { c.connectTyped(text) }
	p.user = c.entry("file-chooser-connect-user", i18n.T("file-chooser-placeholder-user"))
	p.domain = c.entry("file-chooser-connect-domain", i18n.T("file-chooser-placeholder-domain"))
	p.password = c.entry("file-chooser-connect-password", i18n.T("file-chooser-placeholder-password"))
	p.password.SetEcho(widget.EchoPassword)
	for _, e := range []*widget.Entry{p.user, p.domain, p.password} {
		e.OnActivate = func(string) { c.logIn() }
	}
	for _, e := range []*widget.Entry{p.address, p.user, p.domain, p.password} {
		e.OnEscape = func() bool { c.CloseConnect(); return true }
	}
	p.remember = widget.NewChoiceChips(widget.ChooseOne)
	p.remember.AddClass("file-chooser-connect-remember")
	p.choices = []Remember{RememberNever, RememberSession}
	if c.d.Network.keyring != nil {
		p.choices = append(p.choices, RememberKeyring)
	}
	for _, r := range p.choices {
		p.remember.Append(c.Label(i18n.T(rememberLabels[r])))
	}
	p.remember.SetChecked(0, true)
	return p
}

// rememberLabels are the remember choices' messages, by Remember.
var rememberLabels = [...]string{
	RememberNever: "file-chooser-remember-never", RememberSession: "file-chooser-remember-session", RememberKeyring: "file-chooser-remember-keyring",
}

// entry is a text field of the band.
func (c *Browser) entry(class, placeholder string) *widget.Entry {
	e := widget.NewEntry(c.d.Font, widgets.TextPx(styling.Body, 1), 0)
	e.AddClass(class)
	e.SetPlaceholder(placeholder)
	return e
}

// buttons is a row of the band's buttons at its end.
func buttons(bs ...*widget.Button) *widget.Box {
	row := widget.NewBox(widget.Row, widget.Current().Spacing, 0)
	row.Append(widget.NewSpacer(0, 0), true)
	for _, b := range bs {
		row.Append(b, false)
	}
	return row
}

// cancelButton ends the attempt and closes the band.
func (c *Browser) cancelButton() *widget.Button {
	b, _ := c.TextButton(i18n.T("file-chooser-button-cancel"), "file-chooser-connect-cancel", css.Ghost, c.CloseConnect)
	return b
}

// ShowConnect opens the band on its address, seeded with text, the
// servers connected to lately offered under it; a browser without a
// network does nothing.
func (c *Browser) ShowConnect(text string) {
	if !c.active || c.d.Network == nil {
		return
	}
	p := c.ui.connect
	p.address.SetText(text)
	c.showAddress("")
	p.box.SetVisible(true)
	c.Focus(p.address)
	n := c.d.Network
	OffLoop(c, n.recentServers, func(recent []string) {
		p.recent = recent
		if p.box.Visible() && p.onAddress {
			c.showAddress(p.note.Text())
		}
	})
}

// Connecting reports the band open.
func (c *Browser) Connecting() bool { return c.ui.connect != nil && c.ui.connect.box.Visible() }

// CloseConnect closes the band, dropping the attempt running.
func (c *Browser) CloseConnect() {
	p := c.ui.connect
	if p == nil {
		return
	}
	c.stopAttempt()
	p.password.SetText("")
	p.box.SetVisible(false)
	c.Focus(c.focusView())
}

// stopAttempt ends the attempt running: its result is dropped.
func (c *Browser) stopAttempt() {
	p := c.ui.connect
	p.gen++
	if p.cancel != nil {
		p.cancel()
		p.cancel = nil
	}
}

// show lays the band's body out with rows, saying note.
func (c *Browser) show(note string, warn bool, rows ...widget.Widget) {
	p := c.ui.connect
	p.note.SetText(note)
	p.note.SetVisible(note != "")
	p.note.RemoveClass(css.Warning)
	if warn {
		p.note.AddClass(css.Warning)
	}
	p.body.Clear()
	for _, r := range rows {
		p.body.Append(r, false)
	}
	p.onAddress = false
}

// showAddress asks for the address, warning with note when it is set.
func (c *Browser) showAddress(note string) {
	p := c.ui.connect
	connect, _ := c.TextButton(i18n.T("file-chooser-button-connect"), "file-chooser-connect-go", css.SuggestedAction, func() { c.connectTyped(p.address.Text()) })
	rows := []widget.Widget{p.address}
	if len(p.recent) > 0 {
		recent := widget.NewBox(widget.Row, widget.Current().Spacing/2, 0)
		recent.AddClass("file-chooser-connect-recent")
		for _, loc := range p.recent {
			b, l := c.TextButton(loc, "file-chooser-connect-recent-server", css.Ghost, func() {
				p.address.SetText(loc)
				c.connectTyped(loc)
			})
			b.AddClass(css.Small)
			l.SetEllipsize(widget.EllipsizeMiddle)
			recent.Append(b, false)
		}
		rows = append(rows, recent)
	}
	c.show(note, note != "", append(rows, buttons(c.cancelButton(), connect))...)
	p.onAddress = true
}

// connectTyped connects to the location typed; what is no network
// location says so.
func (c *Browser) connectTyped(text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	l, err := vfs.Parse(text)
	if err != nil || l.IsLocal() || l.Server.Scheme == vfs.MTP {
		log.Printf("file browser: connect to %q: %v", text, err)
		c.showAddress(i18n.T("file-chooser-location-invalid", i18n.Str("location", text)))
		return
	}
	c.connect(l, nil, RememberNever)
}

// showLogin asks for the user, the domain where the scheme has one,
// and the password, saying the last ones were refused when wrong.
func (c *Browser) showLogin(cred vfs.Credentials, wrong bool) {
	p := c.ui.connect
	srv := p.loc.Server
	p.user.SetText(cmpOr(cred.User, srv.User))
	p.domain.SetText(cmpOr(cred.Domain, srv.Domain))
	p.password.SetText("")
	fields := widget.NewBox(widget.Row, widget.Current().Spacing, 0)
	fields.Append(p.user, true)
	if srv.Scheme == vfs.SMB {
		fields.Append(p.domain, true)
	}
	fields.Append(p.password, true)
	connect, _ := c.TextButton(i18n.T("file-chooser-button-connect"), "file-chooser-connect-login", css.SuggestedAction, c.logIn)
	note := i18n.T("file-chooser-login", i18n.Str("server", srv.String()))
	if wrong {
		note = i18n.T("file-chooser-login-failed", i18n.Str("server", srv.String()))
	}
	c.show(note, wrong, fields, p.remember, buttons(c.cancelButton(), connect))
	if p.user.Text() == "" {
		c.Focus(p.user)
	} else {
		c.Focus(p.password)
	}
}

// logIn connects with the login typed, remembered as chosen.
func (c *Browser) logIn() {
	p := c.ui.connect
	how := RememberNever
	if checked := p.remember.Checked(); len(checked) == 1 {
		how = p.choices[checked[0]]
	}
	cred := vfs.Credentials{User: strings.TrimSpace(p.user.Text()), Password: p.password.Text()}
	if p.loc.Server.Scheme == vfs.SMB {
		cred.Domain = strings.TrimSpace(p.domain.Text())
	}
	p.password.SetText("")
	c.connect(p.loc, &cred, how)
}

// showQuestion asks text of a server the user must vouch for, trust
// running on Trust and Connect.
func (c *Browser) showQuestion(text string, trust func()) {
	ok, _ := c.TextButton(i18n.T("file-chooser-button-trust"), "file-chooser-connect-trust", css.DestructiveAction, trust)
	c.show(text, true, buttons(c.cancelButton(), ok))
}

// attempt is how one try to connect went: the connection, else why
// not, and the credentials it logged in with.
type attempt struct {
	conn vfs.Conn
	err  error
	cred vfs.Credentials
}

// connect connects to loc's server off the loop: with cred, or what is
// remembered for it when cred is nil, then mounts it and opens loc. A
// server connected already just opens.
func (c *Browser) connect(loc vfs.Location, cred *vfs.Credentials, how Remember) {
	p := c.ui.connect
	c.stopAttempt()
	p.loc = loc
	if c.mounts.Mounted(loc.Server) {
		c.connected(loc)
		return
	}
	c.show(i18n.T("file-chooser-connecting", i18n.Str("server", loc.Server.String())), false, buttons(c.cancelButton()))
	ctx, cancel := context.WithTimeout(context.Background(), connectTimeout)
	p.cancel = cancel
	gen, n, u := p.gen, c.d.Network, c.ui
	c.checking++
	c.offLoop.Go(func() {
		defer cancel()
		a := attempt{cred: vfs.Credentials{User: loc.Server.User, Domain: loc.Server.Domain}}
		if cred != nil {
			a.cred = *cred
		} else if kept, ok := n.remembered(loc.Server); ok {
			a.cred = kept
		}
		a.conn, a.err = n.dial(ctx, loc, a.cred, n.trust)
		c.d.Invoke(func() {
			c.checking--
			if c.ui != u || !c.active || p.gen != gen {
				if a.conn != nil {
					_ = a.conn.Close()
				}
				return
			}
			p.cancel = nil
			c.attempted(loc, a, how)
		})
	})
}

// attempted takes an attempt's outcome: mounted and opened, or the
// question it raises, or why it failed.
func (c *Browser) attempted(loc vfs.Location, a attempt, how Remember) {
	srv := loc.Server.String()
	var hk *vfs.HostKeyError
	var ce *vfs.CertificateError
	switch {
	case a.err == nil:
		c.mount(loc, a, how)
	case errors.Is(a.err, vfs.ErrAuth):
		log.Printf("file browser: %v", a.err)
		c.showLogin(a.cred, a.cred.Password != "")
	case errors.As(a.err, &hk) && hk.Changed:
		log.Printf("file browser: %v", a.err)
		c.showAddress(i18n.T("file-chooser-host-key-changed", i18n.Str("server", srv)))
	case errors.As(a.err, &hk):
		c.showQuestion(i18n.T("file-chooser-host-key-unknown", i18n.Str("server", srv), i18n.Str("algorithm", hk.Algorithm), i18n.Str("fingerprint", hk.Fingerprint)), func() {
			trust := c.d.Network.trust
			OffLoop(c, func() error { return trust.AcceptHostKey(hk) }, func(err error) {
				if err != nil {
					c.failed(i18n.T("file-chooser-connect-failed", i18n.Str("server", srv)), err)
					c.showAddress(i18n.T("file-chooser-connect-failed", i18n.Str("server", srv)))
					return
				}
				c.connect(loc, &a.cred, how)
			})
		})
	case errors.As(a.err, &ce):
		log.Printf("file browser: %v", a.err)
		c.showQuestion(i18n.T(certificateQuestion(ce.Err), i18n.Str("server", srv), i18n.Str("fingerprint", ce.Fingerprint())), func() {
			if ce.Cert == nil {
				c.showAddress(i18n.T("file-chooser-connect-failed", i18n.Str("server", srv)))
				return
			}
			c.d.Network.trust.AcceptCertificate(loc.Server.Host, ce.Cert)
			c.connect(loc, &a.cred, how)
		})
	case errors.Is(a.err, vfs.ErrUnreachable):
		log.Printf("file browser: %v", a.err)
		c.showAddress(i18n.T("file-chooser-unreachable", i18n.Str("server", srv)))
	case errors.Is(a.err, context.Canceled):
	default:
		log.Printf("file browser: %v", a.err)
		c.showAddress(i18n.T("file-chooser-connect-failed", i18n.Str("server", srv)))
	}
}

// certificateQuestion is the question a certificate that did not
// verify for why raises: one no authority vouches for, one made out to
// another name, or one out of its dates.
func certificateQuestion(why error) string {
	var invalid x509.CertificateInvalidError
	switch {
	case errors.As(why, new(x509.HostnameError)):
		return "file-chooser-certificate-wrong-host"
	case errors.As(why, &invalid) && invalid.Reason == x509.Expired:
		return "file-chooser-certificate-expired"
	}
	return "file-chooser-certificate-untrusted"
}

// mount mounts what an attempt connected, keeps its login as how says
// and the location among the recent servers, and opens it.
func (c *Browser) mount(loc vfs.Location, a attempt, how Remember) {
	p, n := c.ui.connect, c.d.Network
	if err := c.mounts.Add(vfs.Mount{Server: loc.Server, Label: mountLabel(loc), Home: loc.String(), Conn: a.conn}); err != nil {
		_ = a.conn.Close()
	}
	if how != RememberNever && a.cred.Password != "" {
		srv, cred := loc.Server, a.cred
		c.offLoop.Go(func() {
			if err := n.remember(srv, cred, how); err != nil {
				log.Printf("file browser: %v", err)
			}
		})
	}
	p.recent = n.addRecent(p.recent, loc.String())
	c.connected(loc)
}

// connected closes the band and opens loc.
func (c *Browser) connected(loc vfs.Location) {
	c.CloseConnect()
	c.goTo(loc.String())
}

// mountLabel is what a mounted location shows as in Locations: the
// share and its server for an SMB share, else the server's user and
// host.
func mountLabel(loc vfs.Location) string {
	root := vfs.Root(loc.String())
	if loc.Server.Scheme == vfs.SMB {
		if share, _, _ := strings.Cut(strings.TrimPrefix(loc.Path, "/"), "/"); share != "" {
			return i18n.T("file-chooser-share-on-server", i18n.Str("share", share), i18n.Str("server", loc.Server.Host))
		}
	}
	return vfs.Base(root)
}

// disconnect disconnects the server mounted, off the loop, leaving its
// folders first when one is shown.
func (c *Browser) disconnect(m vfs.Mount) {
	mounts := c.mounts
	OffLoop(c, func() error { return mounts.Remove(m.Server) }, func(err error) {
		if err != nil {
			c.failed(i18n.T("file-chooser-disconnect-failed", i18n.Str("server", m.Label)), err)
		}
		if vfs.IsRemote(c.s.dir) && vfs.Root(c.s.dir) == vfs.Root(m.Home) {
			c.goTo(c.d.Home)
		}
	})
}

// networkRows are the Locations rows of the network: each location
// mounted, opening where it was connected to with a disconnect button,
// then Connect to Server.
func (c *Browser) networkRows(mounted []vfs.Mount) []widget.Widget {
	var rows []widget.Widget
	for _, m := range mounted {
		if m.Server.Scheme == vfs.MTP {
			continue
		}
		row := widget.NewBox(widget.Row, 0, 0)
		row.Append(c.placeButton(Place{Label: m.Label, Path: m.Home, Icon: "ld-server-symbolic"}, func() { c.goTo(m.Home) }), true)
		row.AppendAligned(c.iconButton("ld-eject-symbolic", i18n.T("file-chooser-disconnect", i18n.Str("server", m.Label)), func() { c.disconnect(m) }), false, widget.AlignCenter)
		rows = append(rows, row)
	}
	if c.d.Network != nil {
		rows = append(rows, c.placeButton(Place{Label: i18n.T("file-chooser-connect-server"), Icon: "ld-globe-symbolic"}, func() { c.ShowConnect("") }))
	}
	return rows
}
