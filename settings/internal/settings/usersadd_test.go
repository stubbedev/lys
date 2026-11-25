package settings

import (
	"errors"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/shacrypt"

	"github.com/stubbedev/lys/settings/internal/accounts"
	"github.com/stubbedev/lys/settings/internal/locale"
)

func TestALoginNameMustBeFree(t *testing.T) {
	if got, err := parseLoginName("zoe", []string{"bob"}); err != nil || got != "zoe" {
		t.Errorf("a free name: %q, %v", got, err)
	}
	if _, err := parseLoginName("bob", []string{"bob"}); !errors.Is(err, errLoginTaken) {
		t.Errorf("a taken name: %v", err)
	}
	if _, err := parseLoginName("Bob", nil); !errors.Is(err, accounts.ErrLoginStart) {
		t.Errorf("an invalid name: %v", err)
	}
}

func TestANewAccountHoldsAHashOfItsPasswordAndNeverThePassword(t *testing.T) {
	a, err := parseNewAccount("  Zoë Smith ", "zoe", accounts.Admin, []byte("pw-9"), []byte("pw-9"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if a.Login != "zoe" || a.RealName != "Zoë Smith" || a.Kind != accounts.Admin {
		t.Fatalf("account %+v", a)
	}
	if ok, err := shacrypt.Verify([]byte("pw-9"), string(a.Hash)); !ok || err != nil || !strings.HasPrefix(string(a.Hash), "$6$") {
		t.Fatalf("the hash does not verify: %v", err)
	}
	for _, c := range []struct {
		login       string
		kind        accounts.Kind
		pw, confirm string
		want        error
	}{
		{"zoe", accounts.Standard, "", "", errPasswordEmpty},
		{"zoe", accounts.Standard, "a", "b", errPasswordDiffs},
		{"Zoe", accounts.Standard, "a", "a", accounts.ErrLoginStart},
		{"bob", accounts.Standard, "a", "a", errLoginTaken},
	} {
		if _, err := parseNewAccount("", c.login, c.kind, []byte(c.pw), []byte(c.confirm), []string{"bob"}); !errors.Is(err, c.want) {
			t.Errorf("%+v: %v", c, err)
		}
	}
	if _, err := parseNewAccount("", "zoe", 7, []byte("a"), []byte("a"), nil); err == nil {
		t.Error("an account kind lys does not define was taken")
	}
}

func addUserDialog(t *testing.T, refuse error) (*usersPanel, *loop, *fakePickers, *addUserForm, *fakeAccounts) {
	t.Helper()
	admin := alice
	admin.kind = accounts.Admin
	bus := dbustest.Start(t)
	p, l, f := usersFixture(t, bus, admin, bob)
	l.until("the admin section", func() bool { return p.admin.Visible() && len(p.others) == 2 })
	f.refuseWith(refuse)
	pickers := &fakePickers{}
	p.k.pickers = pickers
	treetest.Button(t, p.admin, locale.T().Get("settings-users-add-open")).OnClick()
	if len(pickers.dialogs) != 1 {
		t.Fatalf("Add User opened %d dialogs", len(pickers.dialogs))
	}
	cfg := pickers.dialogs[0]
	form := cfg.Content.(*addUserForm)
	if !cfg.Modal || cfg.CancelResponse != responseCancel || cfg.DefaultResponse != responseConfirm || cfg.Focus != form.full {
		t.Fatalf("dialog modal %v cancel %q default %q", cfg.Modal, cfg.CancelResponse, cfg.DefaultResponse)
	}
	return p, l, pickers, form, f
}

func fill(f *addUserForm, full, login, pw, confirm string) {
	f.full.SetText(full)
	f.full.OnChanged(full)
	if login != "" {
		f.login.SetText(login)
		f.login.OnChanged(login)
	}
	f.password.SetText(pw)
	f.repeat.SetText(confirm)
}

func TestAddingAUserCreatesItWithAHashOfItsPassword(t *testing.T) {
	p, l, pickers, form, f := addUserDialog(t, nil)
	pickChoice(form.kinds, 1)
	fill(form, "Zoë Smith", "", "correct-Horse-9", "correct-Horse-9")
	if form.login.Text() != "zoesmith" {
		t.Fatalf("the login name reads %q", form.login.Text())
	}
	for _, taken := range []string{"bob", "svc"} {
		form.login.SetText(taken)
		form.login.OnChanged(taken)
		if want := locale.T().Get("settings-users-add-name-taken", i18n.Str("name", taken)); form.messages.Text() != want || !form.messages.Visible() {
			t.Fatalf("a taken login reads %q, want %q", form.messages.Text(), want)
		}
	}
	form.login.SetText("zoe")
	form.login.OnChanged("zoe")
	form.full.SetText("Zoë Smith-Jones")
	form.full.OnChanged("Zoë Smith-Jones")
	if form.login.Text() != "zoe" || form.messages.Visible() {
		t.Fatalf("an edited login name became %q, message %q", form.login.Text(), form.messages.Text())
	}
	if pickers.respond(responseConfirm) {
		t.Fatal("the dialog closed before the user was added")
	}
	l.until("the dialog closed", func() bool { return pickers.dialogsClosed == 1 })
	made := f.made()
	c, ok := made[0].(accounts.Create)
	if len(made) != 1 || !ok || c.Login != "zoe" || c.RealName != "Zoë Smith-Jones" || c.Kind != accounts.Admin {
		t.Fatalf("asked %#v", made)
	}
	if ok, err := shacrypt.Verify([]byte("correct-Horse-9"), string(c.Hash)); !ok || err != nil {
		t.Fatalf("the hash does not verify: %v", err)
	}
	if p.problem.Visible() || f.named("zoe").name != "zoe" {
		t.Fatalf("the page shows %q, zoe is %+v", p.problem.Text(), f.named("zoe"))
	}
	l.until("zoe listed", func() bool { return len(p.others) == 3 })
}

func TestAnAccountThatCannotBeAddedIsSaidAndNotAsked(t *testing.T) {
	for _, c := range []struct {
		login, pw, confirm, want string
	}{
		{"zoe", "a", "b", locale.T().Get("settings-users-password-mismatch")},
		{"zoe", "", "", locale.T().Get("settings-users-add-password-empty")},
		{"9zoe", "a", "a", locale.T().Get("settings-users-add-name-start")},
		{"alice", "a", "a", locale.T().Get("settings-users-add-name-taken", i18n.Str("name", "alice"))},
	} {
		_, l, pickers, form, f := addUserDialog(t, nil)
		fill(form, "Zoë", c.login, c.pw, c.confirm)
		if pickers.respond(responseConfirm) || form.messages.Text() != c.want {
			t.Errorf("%+v: the dialog reads %q, want %q", c, form.messages.Text(), c.want)
		}
		l.drain()
		if len(f.made()) != 0 {
			t.Errorf("%+v asked %#v", c, f.made())
		}
		if !pickers.respond(responseCancel) || form.password.Text() != "" || form.repeat.Text() != "" {
			t.Errorf("%+v: cancelling left a password in the dialog", c)
		}
	}
}

func TestARefusedCreateKeepsTheDialogOpenAndAddsNobody(t *testing.T) {
	p, l, pickers, form, f := addUserDialog(t, accounts.Errorf(accounts.ErrFailed, "chpasswd refused; the account was removed again"))
	fill(form, "Zoë", "zoe", "pw", "pw")
	pickers.respond(responseConfirm)
	l.until("the refusal", func() bool { return form.messages.Visible() })
	if form.messages.Text() != locale.T().Get("settings-users-add-failed") || !strings.Contains(form.messages.detail.Text(), "chpasswd refused") || pickers.dialogsClosed != 0 {
		t.Fatalf("the dialog reads %q over %q, closed %d", form.messages.Text(), form.messages.detail.Text(), pickers.dialogsClosed)
	}
	l.drain()
	if f.named("zoe").name != "" || len(p.others) != 2 {
		t.Fatalf("a refused create added %+v", f.named("zoe"))
	}
}

func TestWithoutAWindowAddingAUserIsRefused(t *testing.T) {
	p := usersFooter(testKit(t, "")).(*usersPanel)
	t.Cleanup(p.close)
	p.k.pickers = nil
	p.openAddUserDialog()
	if !p.problem.Visible() {
		t.Fatal("adding a user without a window said nothing")
	}
}
