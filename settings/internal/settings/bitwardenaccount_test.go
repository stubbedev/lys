package settings

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/vaultlogin"
	"github.com/stubbedev/lys/secrets/vault"

	"github.com/stubbedev/lys/settings/internal/locale"
)

type fakeVault struct {
	st        vault.Status
	folderSet []string
	lockErr   error
	locks     int
	logouts   int
	closed    int
	logins    []vault.LoginRequest
	loginErrs []error
	emailed   []string
	unlocks   []string
	unlockErr error
}

func (f *fakeVault) login(_ context.Context, req vault.LoginRequest) error {
	f.logins = append(f.logins, req)
	if len(f.loginErrs) > 0 {
		err := f.loginErrs[0]
		f.loginErrs = f.loginErrs[1:]
		if err != nil {
			return err
		}
	}
	f.st = vault.Status{State: vault.StateUnlocked, Email: req.Email}
	return nil
}

func (f *fakeVault) sendLoginEmail(_ context.Context, req vault.LoginRequest) error {
	f.emailed = append(f.emailed, req.Email)
	return nil
}

func (f *fakeVault) unlock(_ context.Context, password string) error {
	f.unlocks = append(f.unlocks, password)
	if f.unlockErr != nil {
		return f.unlockErr
	}
	f.st.State = vault.StateUnlocked
	return nil
}

func (f *fakeVault) status(context.Context) (vault.Status, error) { return f.st, nil }

func (f *fakeVault) folders(context.Context) ([]string, error) {
	if f.st.State != vault.StateUnlocked {
		return nil, vault.ErrLocked
	}
	return f.folderSet, nil
}

func (f *fakeVault) lock(context.Context) error {
	f.locks++
	if f.lockErr != nil {
		return f.lockErr
	}
	f.st.State = vault.StateLocked
	return nil
}

func (f *fakeVault) logout(context.Context) error {
	f.logouts++
	f.st = vault.Status{State: vault.StateLoggedOut}
	return nil
}

func (f *fakeVault) changes(context.Context) (<-chan struct{}, error) {
	return make(chan struct{}), nil
}

func (f *fakeVault) close() error {
	f.closed++
	return nil
}

func accountOf(t *testing.T, f *fakeVault) (*vaultAccount, *fakePickers) {
	t.Helper()
	k := testKit(t, "modules:\n  bitwarden:\n    email: ada@example.com\n")
	k.vault = func() (vaultControl, error) { return f, nil }
	pickers := &fakePickers{}
	k.pickers = pickers
	a := newVaultAccount(k).(*vaultAccount)
	t.Cleanup(a.close)
	return a, pickers
}

func TestTheAccountCardSaysWhoIsSignedInAndLocks(t *testing.T) {
	f := &fakeVault{st: vault.Status{State: vault.StateUnlocked, Email: "me@example.org"}}
	a, _ := accountOf(t, f)
	if want := locale.T().Get("settings-vault-unlocked", i18n.Str("email", "me@example.org")); a.state.Text() != want {
		t.Fatalf("an open vault reads %q, want %q", a.state.Text(), want)
	}
	if !a.lock.Enabled() || !a.logout.Enabled() {
		t.Fatalf("an open vault: lock %v, sign out %v; want both", a.lock.Enabled(), a.logout.Enabled())
	}
	a.lock.OnClick()
	if f.locks != 1 || a.state.Text() != locale.T().Get("settings-vault-locked", i18n.Str("email", "me@example.org")) || a.lock.Enabled() {
		t.Errorf("Lock now locked %d times; reads %q with lock enabled %v", f.locks, a.state.Text(), a.lock.Enabled())
	}
}

func TestSigningOutAsksFirst(t *testing.T) {
	f := &fakeVault{st: vault.Status{State: vault.StateLocked, Email: "me@example.org"}}
	a, pickers := accountOf(t, f)
	a.logout.OnClick()
	if len(pickers.dialogs) != 1 || f.logouts != 0 {
		t.Fatalf("Sign out opened %d dialogs and signed out %d times before an answer", len(pickers.dialogs), f.logouts)
	}
	pickers.dialogs[0].OnResponse(responseCancel)
	if f.logouts != 0 {
		t.Fatal("cancelling signed out")
	}
	pickers.dialogs[0].OnResponse(responseConfirm)
	if f.logouts != 1 || a.state.Text() != locale.T().Get("settings-vault-logged-out") || a.logout.Enabled() || a.lock.Enabled() {
		t.Errorf("confirming signed out %d times; reads %q, sign out %v, lock %v", f.logouts, a.state.Text(), a.logout.Enabled(), a.lock.Enabled())
	}
}

func TestARefusedLockIsSaid(t *testing.T) {
	f := &fakeVault{st: vault.Status{State: vault.StateUnlocked, Email: "me@example.org"}, lockErr: errors.New("the vault is busy")}
	a, _ := accountOf(t, f)
	a.lock.OnClick()
	if !a.problem.Visible() || a.problem.Text() != "the vault is busy" {
		t.Errorf("a refused lock shows %q (%v)", a.problem.Text(), a.problem.Visible())
	}
}

func TestTheVaultsLastFailureIsSaidUntilAnOperationSucceeds(t *testing.T) {
	f := &fakeVault{st: vault.Status{State: vault.StateLocked, Email: "me@example.org", Err: errors.New("sync: server unreachable")}}
	a, _ := accountOf(t, f)
	if want := locale.T().Get("settings-vault-failed", i18n.Str("reason", "sync: server unreachable")); !a.failed.Visible() || a.failed.Text() != want {
		t.Fatalf("the vault's failure shows %q (%v), want %q", a.failed.Text(), a.failed.Visible(), want)
	}
	f.st.Err = nil
	a.reload()
	if a.failed.Visible() {
		t.Error("a vault with no failure still shows one")
	}
}

func TestWithoutLysSecretsTheAccountCardSaysSoAndOffersNothing(t *testing.T) {
	k := testKit(t, "")
	a := newVaultAccount(k).(*vaultAccount)
	t.Cleanup(a.close)
	want := locale.T().Get("settings-vault-unavailable", i18n.Str("reason", "no lys-secrets in the tests"))
	if a.state.Text() != want || a.lock.Enabled() || a.logout.Enabled() {
		t.Errorf("without lys-secrets: %q, lock %v, sign out %v", a.state.Text(), a.lock.Enabled(), a.logout.Enabled())
	}
}

func TestTheVaultFoldersAreOfferedOnlyWhileTheVaultIsOpen(t *testing.T) {
	f := &fakeVault{st: vault.Status{State: vault.StateUnlocked}, folderSet: []string{"Network", "Portal"}}
	k := testKit(t, "")
	k.vault = func() (vaultControl, error) { return f, nil }
	for _, path := range []string{"modules.bitwarden.network-folder", "modules.bitwarden.portal-folder"} {
		c, ok := fieldControl(k, k.store, pageRow(t, "bitwarden", path)).(*choiceEditor)
		if !ok {
			t.Fatalf("%s is not a folder picker", path)
		}
		var got []string
		for _, it := range c.items {
			got = append(got, it.value)
		}
		if !slices.Equal(got, []string{"Network", "Portal"}) || !c.spec.free {
			t.Errorf("%s offers %v (free %v), want the vault's folders and a typed one", path, got, c.spec.free)
		}
	}
	f.st.State = vault.StateLocked
	if got := vaultFolderChoices(k); len(got) != 0 {
		t.Errorf("a locked vault offers %v", got)
	}
	if f.closed == 0 {
		t.Error("reading the folders left the vault connection open")
	}
}

func TestTheVaultFoldersAreEachFolderOnceByName(t *testing.T) {
	items := []vault.Item{{Folder: "Wi-Fi"}, {Folder: ""}, {Folder: "Apps"}, {Folder: "Wi-Fi"}}
	if got := vaultFolders(items); !slices.Equal(got, []string{"Apps", "Wi-Fi"}) {
		t.Errorf("folders %v, want Apps and Wi-Fi", got)
	}
}

func TestSigningInAsksForTheCodeTheServerWants(t *testing.T) {
	f := &fakeVault{st: vault.Status{State: vault.StateLoggedOut}, loginErrs: []error{
		&vault.TwoFactorRequiredError{Providers: []vault.TwoFactorProvider{vault.TwoFactorEmail}},
	}}
	a, _ := accountOf(t, f)
	if !a.login.Visible() || a.unlock.Visible() || a.form.Visible() {
		t.Fatalf("signed out: sign in %v, unlock %v, form %v", a.login.Visible(), a.unlock.Visible(), a.form.Visible())
	}
	a.login.OnClick()
	if !a.form.login.Visible() || a.form.email.Text() != "ada@example.com" {
		t.Fatalf("Sign in shows the form %v with the email %q", a.form.login.Visible(), a.form.email.Text())
	}
	a.form.password.SetText("hunter2")
	a.form.password.OnActivate("hunter2")
	if len(f.logins) != 1 || f.logins[0].Email != "ada@example.com" || f.logins[0].Password != "hunter2" || f.logins[0].TwoFactor != nil {
		t.Fatalf("signing in sent %+v", f.logins)
	}
	if !a.form.code.Visible() || a.form.login.Visible() || !a.form.sendEmail.Visible() || a.problem.Visible() {
		t.Fatalf("an emailed code: code step %v, login %v, send %v, problem %q", a.form.code.Visible(), a.form.login.Visible(), a.form.sendEmail.Visible(), a.problem.Text())
	}
	a.form.sendEmail.OnClick()
	if !slices.Equal(f.emailed, []string{"ada@example.com"}) {
		t.Errorf("Send code emailed %v", f.emailed)
	}
	a.form.codeIn.SetText("123456")
	a.form.verify()
	if len(f.logins) != 2 || f.logins[1].TwoFactor == nil || f.logins[1].TwoFactor.Code != "123456" || f.logins[1].Password != "hunter2" {
		t.Fatalf("the code sent %+v", f.logins)
	}
	if a.form.Visible() || a.form.password.Text() != "" || a.login.Visible() || !a.lock.Enabled() {
		t.Errorf("signed in: form %v, password kept %q, sign in %v, lock %v", a.form.Visible(), a.form.password.Text(), a.login.Visible(), a.lock.Enabled())
	}
}

func TestARefusedSignInIsSaidAndKeepsTheForm(t *testing.T) {
	f := &fakeVault{st: vault.Status{State: vault.StateLoggedOut}, loginErrs: []error{vault.ErrWrongPassword}}
	a, _ := accountOf(t, f)
	a.login.OnClick()
	a.form.password.SetText("wrong")
	a.form.signIn()
	if !a.problem.Visible() || a.problem.Text() != vaultlogin.ErrorText(vault.ErrWrongPassword) {
		t.Errorf("a wrong password shows %q (%v)", a.problem.Text(), a.problem.Visible())
	}
	if !a.form.login.Visible() || a.form.code.Visible() || a.lock.Enabled() {
		t.Errorf("after a refusal: login %v, code %v, lock %v", a.form.login.Visible(), a.form.code.Visible(), a.lock.Enabled())
	}
}

func TestALockedVaultUnlocksWithTheMasterPassword(t *testing.T) {
	f := &fakeVault{st: vault.Status{State: vault.StateLocked, Email: "ada@example.com"}, unlockErr: vault.ErrWrongPassword}
	a, _ := accountOf(t, f)
	if a.login.Visible() || !a.unlock.Visible() {
		t.Fatalf("locked: sign in %v, unlock %v", a.login.Visible(), a.unlock.Visible())
	}
	a.unlock.OnClick()
	a.form.open()
	if len(f.unlocks) != 0 {
		t.Fatal("an empty password was sent")
	}
	a.form.unlockPw.SetText("wrong")
	a.form.open()
	if !a.problem.Visible() || a.problem.Text() != vaultlogin.ErrorText(vault.ErrWrongPassword) || !a.form.unlock.Visible() {
		t.Fatalf("a wrong password shows %q (%v), form %v", a.problem.Text(), a.problem.Visible(), a.form.unlock.Visible())
	}
	f.unlockErr = nil
	a.form.unlockPw.SetText("right")
	a.form.unlockPw.OnActivate("right")
	if !slices.Equal(f.unlocks, []string{"wrong", "right"}) || a.form.Visible() || a.unlock.Visible() || !a.lock.Enabled() || a.problem.Visible() {
		t.Errorf("unlocking sent %v; form %v, unlock %v, lock %v, problem %q", f.unlocks, a.form.Visible(), a.unlock.Visible(), a.lock.Enabled(), a.problem.Text())
	}
}
