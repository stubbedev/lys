package settings

import (
	"errors"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/shacrypt"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func TestALoginNameIsWhatUseraddTakes(t *testing.T) {
	for _, in := range []string{"zoe", "_build", "a-b_c9", strings.Repeat("a", maxLoginName)} {
		if got, err := parseLoginName(in, []string{"bob"}); err != nil || string(got) != in {
			t.Errorf("%q: %q %v", in, got, err)
		}
	}
	for in, want := range map[string]error{
		"":                                  errLoginEmpty,
		strings.Repeat("a", maxLoginName+1): errLoginLong,
		"9lives":                            errLoginStart,
		"-x":                                errLoginStart,
		"Zoe":                               errLoginStart,
		"zo e":                              errLoginChars,
		"zoë":                               errLoginChars,
		"zoe.smith":                         errLoginChars,
		"bob":                               errLoginTaken,
	} {
		if _, err := parseLoginName(in, []string{"bob"}); !errors.Is(err, want) {
			t.Errorf("%q: %v, want %v", in, err, want)
		}
	}
}

func TestALoginNameIsMadeFromTheFullName(t *testing.T) {
	for in, want := range map[string]string{
		"Zoë Smith":              "zoesmith",
		"  Ana-María  López":     "ana-marialopez",
		"42 Douglas":             "douglas",
		"Ølaf":                   "laf",
		"日本":                     "",
		strings.Repeat("Ab", 40): strings.Repeat("ab", maxLoginName/2),
	} {
		got := deriveLogin(in)
		if got != want {
			t.Errorf("%q made %q, want %q", in, got, want)
		}
		if got != "" {
			if _, err := parseLoginName(got, nil); err != nil {
				t.Errorf("%q made %q, which is no login name: %v", in, got, err)
			}
		}
	}
}

func TestANewAccountHoldsAHashOfItsPasswordAndNeverThePassword(t *testing.T) {
	a, err := parseNewAccount("  Zoë Smith ", "zoe", accountAdmin, []byte("pw-9"), []byte("pw-9"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if a.login != "zoe" || a.real != "Zoë Smith" || a.accountType != accountAdmin {
		t.Fatalf("account %+v", a)
	}
	if ok, err := shacrypt.Verify([]byte("pw-9"), a.hash); !ok || err != nil || !strings.HasPrefix(a.hash, "$6$") {
		t.Fatalf("hash %q does not verify: %v", a.hash, err)
	}
	for _, c := range []struct {
		login       string
		kind        int32
		pw, confirm string
		want        error
	}{
		{"zoe", accountStandard, "", "", errPasswordEmpty},
		{"zoe", accountStandard, "a", "b", errPasswordDiffs},
		{"Zoe", accountStandard, "a", "a", errLoginStart},
		{"bob", accountStandard, "a", "a", errLoginTaken},
	} {
		if _, err := parseNewAccount("", c.login, c.kind, []byte(c.pw), []byte(c.confirm), []string{"bob"}); !errors.Is(err, c.want) {
			t.Errorf("%+v: %v", c, err)
		}
	}
	if _, err := parseNewAccount("", "zoe", 7, []byte("a"), []byte("a"), nil); err == nil {
		t.Error("an account type AccountsService does not define was taken")
	}
}

func addUserDialog(t *testing.T, refuseCreate, refusePassword string) (*usersPanel, *loop, *fakePickers, *addUserForm, *fakeSys, *fakeSys) {
	t.Helper()
	admin := alice
	admin.accountType = accountAdmin
	bus := dbustest.Start(t)
	s := serveAllAccounts(t, bus, admin, bob)
	manager, created := s.manager, s.created
	p, l := usersFixture(t, bus)
	l.until("the admin section", func() bool { return p.admin.Visible() && len(p.others) == 2 })
	manager.refuseWith(refuseCreate)
	created.refuseWith(refusePassword)
	pickers := &fakePickers{}
	p.k.pickers = pickers
	buttonsTitled(p.admin, locale.T().Get("settings-users-add-open"))[0].OnClick()
	if len(pickers.dialogs) != 1 {
		t.Fatalf("Add User opened %d dialogs", len(pickers.dialogs))
	}
	cfg := pickers.dialogs[0]
	f := cfg.Content.(*addUserForm)
	if !cfg.Modal || cfg.CancelResponse != responseCancel || cfg.DefaultResponse != responseConfirm || cfg.Focus != f.full {
		t.Fatalf("dialog modal %v cancel %q default %q", cfg.Modal, cfg.CancelResponse, cfg.DefaultResponse)
	}
	return p, l, pickers, f, manager, created
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

func TestAddingAUserCreatesItAndSetsAHashOfItsPassword(t *testing.T) {
	p, l, pickers, f, manager, created := addUserDialog(t, "", "")
	pickChoice(f.kinds, 1)
	fill(f, "Zoë Smith", "", "correct-Horse-9", "correct-Horse-9")
	if f.login.Text() != "zoesmith" {
		t.Fatalf("the login name reads %q", f.login.Text())
	}
	f.login.SetText("bob")
	f.login.OnChanged("bob")
	if want := locale.T().Get("settings-users-add-name-taken", i18n.Str("name", "bob")); f.messages.Text() != want || !f.messages.Visible() {
		t.Fatalf("a taken login reads %q, want %q", f.messages.Text(), want)
	}
	f.login.SetText("zoe")
	f.login.OnChanged("zoe")
	f.full.SetText("Zoë Smith-Jones")
	f.full.OnChanged("Zoë Smith-Jones")
	if f.login.Text() != "zoe" || f.messages.Visible() {
		t.Fatalf("an edited login name became %q, message %q", f.login.Text(), f.messages.Text())
	}
	if pickers.respond(responseConfirm) {
		t.Fatal("the dialog closed before the user was added")
	}
	l.until("SetPassword", func() bool { c, _ := created.snapshot(); return len(c) == 1 })
	l.until("the dialog closed", func() bool { return pickers.dialogsClosed == 1 })
	calls, _ := manager.snapshot()
	if last := calls[len(calls)-1]; last.method != "CreateUser" || len(last.args) != 3 || last.args[0] != "zoe" || last.args[1] != "Zoë Smith-Jones" || last.args[2] != accountAdmin {
		t.Fatalf("CreateUser got %+v", last)
	}
	set, _ := created.snapshot()
	hash, _ := set[0].args[0].(string)
	if ok, err := shacrypt.Verify([]byte("correct-Horse-9"), hash); set[0].method != "SetPassword" || !ok || err != nil || set[0].args[1] != "" {
		t.Fatalf("SetPassword got %+v: %v", set[0], err)
	}
	if p.problem.Visible() {
		t.Fatalf("the page shows %q", p.problem.Text())
	}
}

func TestAnAccountThatCannotBeAddedIsSaidAndNotCreated(t *testing.T) {
	for _, c := range []struct {
		login, pw, confirm, want string
	}{
		{"zoe", "a", "b", locale.T().Get("settings-users-password-mismatch")},
		{"zoe", "", "", locale.T().Get("settings-users-add-password-empty")},
		{"9zoe", "a", "a", locale.T().Get("settings-users-add-name-start")},
		{"alice", "a", "a", locale.T().Get("settings-users-add-name-taken", i18n.Str("name", "alice"))},
	} {
		_, l, pickers, f, manager, created := addUserDialog(t, "", "")
		fill(f, "Zoë", c.login, c.pw, c.confirm)
		if pickers.respond(responseConfirm) || f.messages.Text() != c.want {
			t.Errorf("%+v: the dialog reads %q, want %q", c, f.messages.Text(), c.want)
		}
		l.drain()
		if calls, _ := manager.snapshot(); len(calls) != 1 {
			t.Errorf("%+v called %+v", c, calls)
		}
		if calls, _ := created.snapshot(); len(calls) != 0 {
			t.Errorf("%+v set a password: %+v", c, calls)
		}
		if !pickers.respond(responseCancel) || f.password.Text() != "" || f.repeat.Text() != "" {
			t.Errorf("%+v: cancelling left a password in the dialog", c)
		}
	}
}

func TestARefusedCreateUserKeepsTheDialogOpen(t *testing.T) {
	_, l, pickers, f, _, created := addUserDialog(t, "not authorized", "")
	fill(f, "Zoë", "zoe", "pw", "pw")
	pickers.respond(responseConfirm)
	l.until("the refusal", func() bool { return f.messages.Visible() })
	if f.messages.Text() != locale.T().Get("settings-users-add-failed") || !strings.Contains(f.messages.detail.Text(), "not authorized") || pickers.dialogsClosed != 0 {
		t.Fatalf("the dialog reads %q over %q, closed %d", f.messages.Text(), f.messages.detail.Text(), pickers.dialogsClosed)
	}
	if calls, _ := created.snapshot(); len(calls) != 0 {
		t.Fatalf("a password was set without a user: %+v", calls)
	}
}

func TestAUserAddedWithoutItsPasswordIsSaidOnThePage(t *testing.T) {
	p, l, pickers, f, _, _ := addUserDialog(t, "", "not authorized")
	fill(f, "Zoë", "zoe", "pw", "pw")
	pickers.respond(responseConfirm)
	l.until("the dialog closed", func() bool { return pickers.dialogsClosed == 1 })
	want := locale.T().Get("settings-users-add-no-password", i18n.Str("name", "zoe"))
	if !p.problem.Visible() || p.problem.Text() != want || !strings.Contains(p.problem.detail.Text(), "not authorized") {
		t.Fatalf("the page reads %q over %q, want %q over the refusal", p.problem.Text(), p.problem.detail.Text(), want)
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
