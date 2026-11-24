package settings

import (
	"errors"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/shacrypt"

	"github.com/stubbedev/lys/settings/internal/accounts"
	"github.com/stubbedev/lys/settings/internal/locale"
)

func TestAnAccountEditAsksOnlyWhatDiffers(t *testing.T) {
	bobAccount := accounts.Account{Kind: accounts.Standard}
	bobAccount.Name, bobAccount.FullName = "bob", "Bob"
	if e, err := parseAccountEdit(bobAccount, " Bob ", accounts.Standard, nil, nil); err != nil || e != nil {
		t.Fatalf("an unchanged sheet asks %#v, %v", e, err)
	}
	e, err := parseAccountEdit(bobAccount, "  Bob Builder ", accounts.Admin, nil, nil)
	if err != nil || e.Login != "bob" || *e.RealName != "Bob Builder" || *e.Kind != accounts.Admin || e.Hash != nil {
		t.Fatalf("no password reads %#v, %v; want the trimmed name and kind and no hash", e, err)
	}
	e, err = parseAccountEdit(bobAccount, "Bob", accounts.Standard, []byte("hunter2"), []byte("hunter2"))
	if err != nil || e.RealName != nil || e.Kind != nil {
		t.Fatalf("a password alone asks %#v, %v", e, err)
	}
	if ok, verr := shacrypt.Verify([]byte("hunter2"), string(*e.Hash)); !ok || verr != nil || strings.Contains(string(*e.Hash), "hunter2") {
		t.Fatalf("a new password reads %v, %v", ok, verr)
	}
	if _, err := parseAccountEdit(bobAccount, "Bob", accounts.Standard, []byte("a"), []byte("b")); !errors.Is(err, errPasswordDiffs) {
		t.Errorf("differing passwords: %v, want %v", err, errPasswordDiffs)
	}
	if _, err := parseAccountEdit(bobAccount, "Bob", accounts.Standard, nil, []byte("b")); !errors.Is(err, errPasswordDiffs) {
		t.Errorf("a repetition alone: %v, want %v", err, errPasswordDiffs)
	}
	if _, err := parseAccountEdit(bobAccount, "Bob", 7, nil, nil); err == nil {
		t.Error("an account kind lys does not define was taken")
	}
	if _, err := parseAccountEdit(bobAccount, "Bob:0", accounts.Standard, nil, nil); !errors.Is(err, accounts.ErrRealName) {
		t.Errorf("a name with a colon: %v", err)
	}
}

func userSheetFixture(t *testing.T) (*usersPanel, *loop, *fakePickers, *userSheet, *fakeAccounts) {
	t.Helper()
	admin := alice
	admin.kind = accounts.Admin
	bus := dbustest.Start(t)
	p, l, f := usersFixture(t, bus, admin, bob)
	l.until("the admin section", func() bool { return len(buttonsTitled(p.admin, locale.T().Get("settings-users-edit"))) == 1 })
	pickers := &fakePickers{}
	p.k.pickers = pickers
	buttonsTitled(p.admin, locale.T().Get("settings-users-edit"))[0].OnClick()
	if len(pickers.dialogs) != 1 {
		t.Fatalf("the details button opened %d dialogs", len(pickers.dialogs))
	}
	cfg := pickers.dialogs[0]
	sheet := cfg.Content.(*userSheet)
	if cfg.Title != "Bob" || !cfg.Modal || cfg.CancelResponse != responseCancel || cfg.DefaultResponse != responseConfirm || cfg.Focus != sheet.full {
		t.Fatalf("dialog %q modal %v cancel %q default %q", cfg.Title, cfg.Modal, cfg.CancelResponse, cfg.DefaultResponse)
	}
	if kind, err := chosenAccountType(sheet.kinds); sheet.full.Text() != "Bob" || err != nil || kind != accounts.Standard || sheet.password.Text() != "" {
		t.Fatalf("the sheet starts at %q, kind %s (%v), password %q", sheet.full.Text(), kind, err, sheet.password.Text())
	}
	return p, l, pickers, sheet, f
}

func TestTheUserSheetSetsTheNameTypeAndAHashOfTheNewPassword(t *testing.T) {
	p, l, pickers, sheet, f := userSheetFixture(t)
	sheet.full.SetText("Robert")
	pickChoice(sheet.kinds, 1)
	sheet.password.SetText("correct-Horse-9")
	sheet.repeat.SetText("correct-Horse-9")
	if pickers.respond(responseConfirm) {
		t.Fatal("the sheet closed before the helper answered")
	}
	l.until("the sheet closed", func() bool { return pickers.dialogsClosed == 1 })
	made := f.made()
	e, ok := made[0].(accounts.Edit)
	if len(made) != 1 || !ok || e.Login != "bob" || *e.RealName != "Robert" || *e.Kind != accounts.Admin || e.Hash == nil {
		t.Fatalf("bob got %#v", made)
	}
	if ok, err := shacrypt.Verify([]byte("correct-Horse-9"), string(*e.Hash)); !ok || err != nil {
		t.Fatalf("the hash does not verify: %v", err)
	}
	if b := f.named("bob"); b.full != "Robert" || b.kind != accounts.Admin {
		t.Fatalf("bob is %+v", b)
	}
	if sheet.password.Text() != "" || sheet.repeat.Text() != "" || p.problem.Visible() {
		t.Fatalf("the sheet kept the password, or the page reads %q", p.problem.Text())
	}
}

func TestAnUnchangedUserSheetAsksNothing(t *testing.T) {
	_, l, pickers, _, f := userSheetFixture(t)
	pickers.respond(responseConfirm)
	l.until("the sheet closed", func() bool { return pickers.dialogsClosed == 1 })
	if len(f.made()) != 0 {
		t.Fatalf("an unchanged sheet asked %#v", f.made())
	}
}

func TestAUserSheetWithDifferingPasswordsSaysSoAndAsksNothing(t *testing.T) {
	_, l, pickers, sheet, f := userSheetFixture(t)
	sheet.full.SetText("Robert")
	sheet.password.SetText("one")
	sheet.repeat.SetText("two")
	if pickers.respond(responseConfirm) || sheet.messages.Text() != locale.T().Get("settings-users-password-mismatch") {
		t.Fatalf("a mismatch closed the sheet or reads %q", sheet.messages.Text())
	}
	l.drain()
	if len(f.made()) != 0 {
		t.Fatalf("a mismatch asked %#v", f.made())
	}
	if !pickers.respond(responseCancel) || sheet.password.Text() != "" || sheet.repeat.Text() != "" {
		t.Fatal("cancelling left a password in the sheet")
	}
}

func TestARefusedUserEditKeepsTheSheetOpenWithTheReasonAndTheAccountAsItWas(t *testing.T) {
	_, l, pickers, sheet, f := userSheetFixture(t)
	f.refuseWith(errNotAuthorized)
	sheet.full.SetText("Robert")
	pickers.respond(responseConfirm)
	l.until("the refusal", func() bool { return sheet.messages.Visible() })
	if sheet.messages.Text() != locale.T().Get("settings-users-edit-failed") || !strings.Contains(sheet.messages.detail.Text(), "Not authorized") || pickers.dialogsClosed != 0 {
		t.Fatalf("the sheet reads %q over %q, closed %d", sheet.messages.Text(), sheet.messages.detail.Text(), pickers.dialogsClosed)
	}
	if b := f.named("bob"); b.full != "Bob" {
		t.Fatalf("a refused edit changed bob: %+v", b)
	}
}

func TestRemovingFromTheUserSheetAsksAboutTheHomeFolder(t *testing.T) {
	_, l, pickers, _, f := userSheetFixture(t)
	if !pickers.respond(responseRemove) {
		t.Fatal("Remove User kept the sheet open")
	}
	ask := pickers.dialogs[len(pickers.dialogs)-1]
	if len(pickers.dialogs) != 2 || ask.CancelResponse != responseCancel {
		t.Fatalf("Remove User opened %d dialogs", len(pickers.dialogs))
	}
	ask.OnResponse(responseDeleteFiles)
	l.until("the removal", func() bool { return len(f.made()) == 1 })
	if d, ok := f.made()[0].(accounts.Delete); !ok || d != (accounts.Delete{Login: "bob", RemoveHome: true}) {
		t.Fatalf("removing bob with his files: %#v", f.made())
	}
}

func TestWithoutAWindowTheUserSheetIsRefused(t *testing.T) {
	p, l, _, _, _ := userSheetFixture(t)
	p.k.pickers = nil
	p.openUserSheet(p.others[1])
	l.drain()
	if !p.problem.Visible() {
		t.Fatal("opening a user's details without a window said nothing")
	}
}
