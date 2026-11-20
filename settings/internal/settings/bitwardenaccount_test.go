package settings

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
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
	k := testKit(t, "")
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
