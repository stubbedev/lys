package settings

import (
	"context"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/vaultlogin"
	"github.com/stubbedev/lys/secrets/vault"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"
)

// vaultForm is the account card's sign-in and unlock: the email and
// master password, the code a login can stop at, or the master password
// opening a locked vault. One step shows at a time, none until asked.
type vaultForm struct {
	*widget.Box
	a         *vaultAccount
	login     *widget.Box
	email     *widget.Entry
	password  *widget.Entry
	code      *widget.Box
	codeTitle *widget.Label
	codeDesc  *widget.Label
	codeIn    *widget.Entry
	sendEmail *widget.Button
	unlock    *widget.Box
	unlockPw  *widget.Entry
	pending   *vaultlogin.Challenge
}

func newVaultForm(a *vaultAccount) *vaultForm {
	k := a.k
	f := &vaultForm{Box: gapBox(widget.Column), a: a}
	f.email = k.entry()
	f.email.SetPlaceholder(i18n.T("dropdown-bitwarden-email-placeholder"))
	f.email.SetAccessibleName(i18n.T("dropdown-bitwarden-email"))
	f.password = secretEntry(k, i18n.T("dropdown-bitwarden-master-password"))
	f.password.OnActivate = func(string) { f.signIn() }
	f.login = gapBox(widget.Column)
	f.login.Append(f.email, false)
	f.login.Append(f.password, false)
	f.login.Append(formButtons(
		k.button(k.label(i18n.T("dropdown-bitwarden-cancel")), f.close, css.Small, css.Flat),
		k.button(k.label(i18n.T("dropdown-bitwarden-login")), f.signIn, css.Small, css.SuggestedAction),
	), false)

	f.codeTitle = wrapped(k.label("", "setting-label"))
	f.codeDesc = wrapped(k.label("", "setting-description", css.Caption, css.DimLabel))
	f.codeIn = k.entry()
	f.codeIn.SetPlaceholder(i18n.T("dropdown-bitwarden-code-placeholder"))
	f.codeIn.SetAccessibleName(i18n.T("dropdown-bitwarden-code-placeholder"))
	f.codeIn.OnActivate = func(string) { f.verify() }
	f.sendEmail = k.button(k.label(i18n.T("dropdown-bitwarden-send-code")), f.resend, css.Small, css.Flat)
	f.sendEmail.SetTooltip(i18n.T("dropdown-bitwarden-send-code-tooltip"))
	f.code = gapBox(widget.Column)
	f.code.Append(f.codeTitle, false)
	f.code.Append(f.codeDesc, false)
	f.code.Append(f.codeIn, false)
	f.code.Append(formButtons(
		f.sendEmail,
		k.button(k.label(i18n.T("dropdown-bitwarden-back")), f.openLogin, css.Small, css.Flat),
		k.button(k.label(i18n.T("dropdown-bitwarden-verify")), f.verify, css.Small, css.SuggestedAction),
	), false)

	f.unlockPw = secretEntry(k, i18n.T("dropdown-bitwarden-master-password"))
	f.unlockPw.OnActivate = func(string) { f.open() }
	f.unlock = gapBox(widget.Column)
	f.unlock.Append(f.unlockPw, false)
	f.unlock.Append(formButtons(
		k.button(k.label(i18n.T("dropdown-bitwarden-cancel")), f.close, css.Small, css.Flat),
		k.button(k.label(i18n.T("dropdown-bitwarden-unlock")), f.open, css.Small, css.SuggestedAction),
	), false)

	f.Append(f.login, false)
	f.Append(f.code, false)
	f.Append(f.unlock, false)
	f.close()
	return f
}

func secretEntry(k *kit, name string) *widget.Entry {
	e := k.entry()
	e.SetEcho(widget.EchoPassword)
	e.SetPlaceholder(name)
	e.SetAccessibleName(name)
	return e
}

func formButtons(buttons ...widget.Widget) *widget.Box {
	row := gapBox(widget.Row)
	row.Append(widget.NewBox(widget.Row, 0, 0), true)
	for _, b := range buttons {
		row.AppendAligned(b, false, widget.AlignCenter)
	}
	return row
}

// step shows one step of the form, or none.
func (f *vaultForm) step(shown *widget.Box) {
	for _, s := range []*widget.Box{f.login, f.code, f.unlock} {
		s.SetVisible(s == shown)
	}
	f.SetVisible(shown != nil)
}

// close hides the form and forgets everything typed into it but the
// email.
func (f *vaultForm) close() {
	f.pending = nil
	f.password.SetText("")
	f.codeIn.SetText("")
	f.unlockPw.SetText("")
	f.step(nil)
}

// follow keeps the form to what the vault can do now: a step the vault
// left behind closes.
func (f *vaultForm) follow(st vault.State) {
	switch {
	case f.unlock.Visible() && st != vault.StateLocked,
		(f.login.Visible() || f.code.Visible()) && st != vault.StateLoggedOut:
		f.close()
	}
}

func (f *vaultForm) openLogin() {
	f.pending = nil
	f.codeIn.SetText("")
	if f.email.Text() == "" {
		f.email.SetText(f.a.k.store.svc.Config().Bitwarden.Email)
	}
	f.step(f.login)
}

func (f *vaultForm) openUnlock() { f.step(f.unlock) }

func (f *vaultForm) signIn() {
	req := vault.LoginRequest{Server: f.a.k.store.svc.Config().Bitwarden.Server.Server(), Email: f.email.Text(), Password: f.password.Text()}
	f.attempt(req)
}

// attempt logs in with req, turning the server's ask for a code into
// the code step.
func (f *vaultForm) attempt(req vault.LoginRequest) {
	f.a.act(func(ctx context.Context) error { return f.a.ctl.login(ctx, req) }, func(err error) bool {
		if c, ok := vaultlogin.ChallengeOf(req, err); ok {
			f.challenge(c)
			return true
		}
		if err == nil {
			f.close()
		}
		return false
	})
}

func (f *vaultForm) challenge(c vaultlogin.Challenge) {
	f.pending = &c
	f.codeIn.SetText("")
	f.codeTitle.SetText(c.Title())
	f.codeDesc.SetText(c.Description())
	f.sendEmail.SetVisible(c.Emailed())
	f.step(f.code)
}

func (f *vaultForm) verify() {
	if f.pending == nil || f.codeIn.Text() == "" {
		return
	}
	f.attempt(f.pending.Answer(f.codeIn.Text()))
}

func (f *vaultForm) resend() {
	if f.pending == nil {
		return
	}
	req := f.pending.Request
	f.a.act(func(ctx context.Context) error { return f.a.ctl.sendLoginEmail(ctx, req) }, nil)
}

func (f *vaultForm) open() {
	password := f.unlockPw.Text()
	if password == "" {
		return
	}
	f.a.act(func(ctx context.Context) error { return f.a.ctl.unlock(ctx, password) }, func(err error) bool {
		if err == nil {
			f.close()
		}
		return false
	})
}
