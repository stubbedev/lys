package settings

import (
	"context"
	"errors"
	"fmt"
	"log"
	"slices"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/vaultlogin"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/secrets/vault"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// vaultControl is lys-secrets' Bitwarden vault, the one lys-shell's
// Bitwarden module shows: its status, the folders of the open vault,
// signing in (and asking for the emailed code), unlocking, locking,
// signing out, and a tick whenever it changed.
type vaultControl interface {
	status(ctx context.Context) (vault.Status, error)
	folders(ctx context.Context) ([]string, error)
	login(ctx context.Context, req vault.LoginRequest) error
	sendLoginEmail(ctx context.Context, req vault.LoginRequest) error
	unlock(ctx context.Context, password string) error
	lock(ctx context.Context) error
	logout(ctx context.Context) error
	changes(ctx context.Context) (<-chan struct{}, error)
	close() error
}

type secretsVault struct {
	conn   *dbus.Conn
	client *vault.Client
}

func openVault(dial func() (*dbus.Conn, error)) (vaultControl, error) {
	conn, err := dial()
	if err != nil {
		return nil, fmt.Errorf("connect to the session bus: %w", err)
	}
	return &secretsVault{conn: conn, client: vault.NewClient(conn)}, nil
}

func (v *secretsVault) status(ctx context.Context) (vault.Status, error) {
	st, err := v.client.Status(ctx)
	if err != nil {
		return vault.Status{}, fmt.Errorf("read the Bitwarden vault from lys-secrets: %w", err)
	}
	return st, nil
}

func (v *secretsVault) folders(ctx context.Context) ([]string, error) {
	items, err := v.client.Items(ctx)
	if err != nil {
		return nil, fmt.Errorf("list the Bitwarden vault's folders: %w", err)
	}
	return vaultFolders(items), nil
}

// vaultFolders are the folders items sit in, each once, by name.
func vaultFolders(items []vault.Item) []string {
	var out []string
	for _, it := range items {
		if it.Folder != "" && !slices.Contains(out, it.Folder) {
			out = append(out, it.Folder)
		}
	}
	slices.Sort(out)
	return out
}

func (v *secretsVault) login(ctx context.Context, req vault.LoginRequest) error {
	if err := v.client.Login(ctx, req); err != nil {
		return fmt.Errorf("sign in to Bitwarden as %s: %w", req.Email, err)
	}
	return nil
}

func (v *secretsVault) sendLoginEmail(ctx context.Context, req vault.LoginRequest) error {
	if err := v.client.SendLoginEmail(ctx, req); err != nil {
		return fmt.Errorf("email a Bitwarden sign-in code to %s: %w", req.Email, err)
	}
	return nil
}

func (v *secretsVault) unlock(ctx context.Context, password string) error {
	if err := v.client.Unlock(ctx, password); err != nil {
		return fmt.Errorf("unlock the Bitwarden vault: %w", err)
	}
	return nil
}

func (v *secretsVault) lock(ctx context.Context) error {
	if err := v.client.Lock(ctx); err != nil {
		return fmt.Errorf("lock the Bitwarden vault: %w", err)
	}
	return nil
}

func (v *secretsVault) logout(ctx context.Context) error {
	if err := v.client.Logout(ctx); err != nil {
		return fmt.Errorf("sign out of Bitwarden: %w", err)
	}
	return nil
}

func (v *secretsVault) changes(ctx context.Context) (<-chan struct{}, error) {
	ticks := make(chan struct{}, 1)
	tick := func() {
		select {
		case ticks <- struct{}{}:
		default:
		}
	}
	err := v.client.Watch(ctx, vault.Watcher{Changed: func(vault.Status) { tick() }, SyncFailed: func(error) { tick() }})
	if err != nil {
		return nil, err
	}
	return ticks, nil
}

func (v *secretsVault) close() error { return v.conn.Close() }

// vaultAccount is the account the Bitwarden module signs in to: who is
// signed in and whether the vault is open, with Sign in, Unlock, Lock
// now and Sign out.
type vaultAccount struct {
	*widget.Box
	k       *kit
	ctl     vaultControl
	stop    context.CancelFunc
	state   *widget.Label
	login   *widget.Button
	unlock  *widget.Button
	form    *vaultForm
	lock    *widget.Button
	logout  *widget.Button
	failed  *widget.Label
	problem *widget.Label
	closed  bool
}

func newVaultAccount(k *kit) widget.Widget {
	t := locale.T()
	a := &vaultAccount{Box: gapBox(widget.Column), k: k}
	a.state = wrapped(k.label(t.Get("settings-vault-asking"), "setting-label"))
	a.lock = k.secondaryButton(t.Get("settings-vault-lock"), a.lockNow)
	a.form = newVaultForm(a)
	a.login = k.primaryButton(t.Get("settings-vault-login"), a.form.openLogin)
	a.unlock = k.primaryButton(t.Get("settings-vault-unlock"), a.form.openUnlock)
	a.logout = k.destructiveButton(t.Get("settings-vault-logout"), a.askLogout)
	a.failed = wrapped(k.label("", "setting-problem", css.Caption, css.Error))
	a.failed.SetVisible(false)
	a.problem = wrapped(k.label("", "setting-problem", css.Caption, css.Error))
	a.problem.SetVisible(false)
	actions := gapBox(widget.Row)
	actions.Append(a.login, false)
	actions.Append(a.unlock, false)
	actions.Append(a.lock, false)
	actions.Append(a.logout, false)
	a.Append(a.state, false)
	a.Append(a.failed, false)
	a.Append(actions, false)
	a.Append(a.form, false)
	a.Append(a.problem, false)
	a.show(vault.Status{}, false)
	type opened struct {
		ctl vaultControl
		st  vault.Status
		err error
	}
	async(k, func() opened {
		ctl, err := k.vault()
		if err != nil {
			return opened{err: err}
		}
		st, err := callWithin(shellCallWait, ctl.status)
		if err != nil {
			return opened{err: errors.Join(err, ctl.close())}
		}
		return opened{ctl: ctl, st: st}
	}, func(o opened) {
		if o.err != nil {
			a.state.SetText(t.Get("settings-vault-unavailable", i18n.Str("reason", o.err.Error())))
			return
		}
		if a.closed {
			_ = o.ctl.close()
			return
		}
		a.ctl = o.ctl
		a.follow()
		a.show(o.st, true)
	})
	return a
}

func (a *vaultAccount) follow() {
	ctx, cancel := context.WithCancel(context.Background())
	ticks, err := a.ctl.changes(ctx)
	if err != nil {
		cancel()
		a.fail(err)
		return
	}
	a.stop = cancel
	followTicks(a.k, ticks, a.reload)
}

func (a *vaultAccount) reload() {
	if a.closed || a.ctl == nil {
		return
	}
	ctl := a.ctl
	type result struct {
		st  vault.Status
		err error
	}
	async(a.k, func() result {
		st, err := callWithin(shellCallWait, ctl.status)
		return result{st, err}
	}, func(r result) {
		if a.closed {
			return
		}
		if r.err != nil {
			a.fail(r.err)
			return
		}
		a.show(r.st, true)
	})
}

// show says st, the buttons enabled only for what the vault can do now;
// known is false until lys-secrets answered.
func (a *vaultAccount) show(st vault.Status, known bool) {
	t := locale.T()
	if known {
		a.state.SetText(vaultWords(st))
	}
	a.login.SetVisible(known && st.State == vault.StateLoggedOut)
	a.unlock.SetVisible(known && st.State == vault.StateLocked)
	a.lock.SetEnabled(known && st.State == vault.StateUnlocked)
	a.logout.SetEnabled(known && st.State != vault.StateLoggedOut)
	if known {
		a.form.follow(st.State)
	}
	a.failed.SetVisible(st.Err != nil)
	if st.Err != nil {
		a.failed.SetText(t.Get("settings-vault-failed", i18n.Str("reason", st.Err.Error())))
	}
}

func vaultWords(st vault.Status) string {
	t := locale.T()
	switch st.State {
	case vault.StateLocked:
		return t.Get("settings-vault-locked", i18n.Str("email", st.Email))
	case vault.StateUnlocked:
		return t.Get("settings-vault-unlocked", i18n.Str("email", st.Email))
	}
	return t.Get("settings-vault-logged-out")
}

func (a *vaultAccount) lockNow() {
	a.act(func(ctx context.Context) error { return a.ctl.lock(ctx) }, nil)
}

func (a *vaultAccount) askLogout() {
	if a.k.pickers == nil {
		return
	}
	a.k.pickers.dialog(confirmDialog(a.k, "settings-vault-logout", func() {
		a.act(func(ctx context.Context) error { return a.ctl.logout(ctx) }, nil)
	}))
}

// act runs do against the vault; then, when set, sees its outcome first
// and reports whether it dealt with a failure, which is said otherwise.
func (a *vaultAccount) act(do func(ctx context.Context) error, then func(err error) (handled bool)) {
	if a.ctl == nil {
		return
	}
	a.problem.SetVisible(false)
	async(a.k, func() error { return doWithin(shellCallWait, do) }, func(err error) {
		handled := then != nil && then(err)
		if err != nil && !handled {
			a.fail(err)
		}
		a.reload()
	})
}

func (a *vaultAccount) fail(err error) {
	a.problem.SetText(vaultlogin.ErrorText(err))
	a.problem.SetVisible(true)
}

func (a *vaultAccount) close() {
	a.closed = true
	if a.stop != nil {
		a.stop()
	}
	if a.ctl != nil {
		_ = a.ctl.close()
		a.ctl = nil
	}
}

// vaultFolderRow picks a vault folder from the open vault's folders, or
// takes one typed while the vault is not open; empty is none.
var vaultFolderRow = withEditor(func(k *kit, s slot, meta config.FieldMeta) control {
	return newChoiceEditor(k, s, choiceSpec{list: func() []choice { return vaultFolderChoices(k) }, free: true, optional: meta.Optional})
})

func vaultFolderChoices(k *kit) []choice {
	ctl, err := k.vault()
	if err != nil {
		log.Printf("settings: the Bitwarden vault's folders: %v", err)
		return nil
	}
	folders, err := callWithin(shellCallWait, ctl.folders)
	err = errors.Join(err, ctl.close())
	if errors.Is(err, vault.ErrLocked) || errors.Is(err, vault.ErrLoginRequired) {
		return nil
	}
	if err != nil {
		log.Printf("settings: the Bitwarden vault's folders: %v", err)
		return nil
	}
	out := make([]choice, len(folders))
	for i, f := range folders {
		out[i] = choice{value: f, label: f}
	}
	return out
}
