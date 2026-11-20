package settings

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/shacrypt"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func TestAnAccountEditHashesANewPasswordAndKeepsAnEmptyOne(t *testing.T) {
	kept, err := parseAccountEdit("  Bob Builder ", accountAdmin, nil, nil)
	if err != nil || kept != (accountEdit{real: "Bob Builder", accountType: accountAdmin}) {
		t.Fatalf("no password reads %+v, %v; want the trimmed name and type and no hash", kept, err)
	}
	changed, err := parseAccountEdit("Bob", accountStandard, []byte("hunter2"), []byte("hunter2"))
	if ok, verr := shacrypt.Verify([]byte("hunter2"), changed.hash); err != nil || !ok || verr != nil || strings.Contains(changed.hash, "hunter2") {
		t.Fatalf("a new password reads %+v, %v, %v", changed, err, verr)
	}
	if _, err := parseAccountEdit("Bob", accountStandard, []byte("a"), []byte("b")); !errors.Is(err, errPasswordDiffs) {
		t.Errorf("differing passwords: %v, want %v", err, errPasswordDiffs)
	}
	if _, err := parseAccountEdit("Bob", accountStandard, nil, []byte("b")); !errors.Is(err, errPasswordDiffs) {
		t.Errorf("a repetition alone: %v, want %v", err, errPasswordDiffs)
	}
	if _, err := parseAccountEdit("Bob", 7, nil, nil); err == nil {
		t.Error("an account type AccountsService does not define was taken")
	}
}

func userSheetFixture(t *testing.T) (*usersPanel, *loop, *fakePickers, *userSheet, servedAccounts) {
	t.Helper()
	admin := alice
	admin.accountType = accountAdmin
	bus := dbustest.Start(t)
	s := serveAllAccounts(t, bus, admin, bob)
	p, l := usersFixture(t, bus)
	l.until("the admin section", func() bool { return len(treetest.WithClass(p.admin, "user-details")) == 1 })
	pickers := &fakePickers{}
	p.k.pickers = pickers
	treetest.WithClass(p.admin, "user-details")[0].(*widget.Button).OnClick()
	if len(pickers.dialogs) != 1 {
		t.Fatalf("the details button opened %d dialogs", len(pickers.dialogs))
	}
	cfg := pickers.dialogs[0]
	f := cfg.Content.(*userSheet)
	if cfg.Title != "Bob" || !cfg.Modal || cfg.CancelResponse != responseCancel || cfg.DefaultResponse != responseConfirm || cfg.Focus != f.full {
		t.Fatalf("dialog %q modal %v cancel %q default %q", cfg.Title, cfg.Modal, cfg.CancelResponse, cfg.DefaultResponse)
	}
	if f.full.Text() != "Bob" || chosenAccountType(f.kinds) != accountStandard || f.password.Text() != "" {
		t.Fatalf("the sheet starts at %q, type %d, password %q", f.full.Text(), chosenAccountType(f.kinds), f.password.Text())
	}
	return p, l, pickers, f, s
}

func TestTheUserSheetSetsTheNameTypeAndAHashOfTheNewPassword(t *testing.T) {
	p, l, pickers, f, s := userSheetFixture(t)
	f.full.SetText("Robert")
	f.kinds.SetActive(1)
	f.password.SetText("correct-Horse-9")
	f.repeat.SetText("correct-Horse-9")
	if pickers.respond(responseConfirm) {
		t.Fatal("the sheet closed before AccountsService answered")
	}
	l.until("the sheet closed", func() bool { return pickers.dialogsClosed == 1 })
	calls, _ := s.others[0].snapshot()
	if len(calls) != 3 || calls[0].method != "SetRealName" || calls[0].args[0] != "Robert" || calls[1].method != "SetAccountType" || calls[1].args[0] != accountAdmin || calls[2].method != "SetPassword" {
		t.Fatalf("bob got %+v", calls)
	}
	hash, _ := calls[2].args[0].(string)
	if ok, err := shacrypt.Verify([]byte("correct-Horse-9"), hash); !ok || err != nil || calls[2].args[1] != "" {
		t.Fatalf("SetPassword got %+v: %v", calls[2], err)
	}
	if f.password.Text() != "" || f.repeat.Text() != "" || p.problem.Visible() {
		t.Fatalf("the sheet kept the password, or the page reads %q", p.problem.Text())
	}
	if calls, _ := s.self.snapshot(); len(calls) != 0 {
		t.Fatalf("editing bob reached the session's user: %+v", calls)
	}
}

func TestAnUnchangedUserSheetCallsNothing(t *testing.T) {
	_, l, pickers, _, s := userSheetFixture(t)
	pickers.respond(responseConfirm)
	l.until("the sheet closed", func() bool { return pickers.dialogsClosed == 1 })
	if calls, _ := s.others[0].snapshot(); len(calls) != 0 {
		t.Fatalf("an unchanged sheet called %+v", calls)
	}
}

func TestAUserSheetWithDifferingPasswordsSaysSoAndCallsNothing(t *testing.T) {
	_, l, pickers, f, s := userSheetFixture(t)
	f.full.SetText("Robert")
	f.password.SetText("one")
	f.repeat.SetText("two")
	if pickers.respond(responseConfirm) || f.messages.Text() != locale.T().Get("settings-users-password-mismatch") {
		t.Fatalf("a mismatch closed the sheet or reads %q", f.messages.Text())
	}
	l.drain()
	if calls, _ := s.others[0].snapshot(); len(calls) != 0 {
		t.Fatalf("a mismatch called %+v", calls)
	}
	if !pickers.respond(responseCancel) || f.password.Text() != "" || f.repeat.Text() != "" {
		t.Fatal("cancelling left a password in the sheet")
	}
}

func TestARefusedUserEditKeepsTheSheetOpenWithTheReason(t *testing.T) {
	_, l, pickers, f, s := userSheetFixture(t)
	s.others[0].refuseWith("not authorized")
	f.full.SetText("Robert")
	pickers.respond(responseConfirm)
	l.until("the refusal", func() bool { return f.messages.Visible() })
	if !strings.Contains(f.messages.Text(), "not authorized") || pickers.dialogsClosed != 0 {
		t.Fatalf("the sheet reads %q, closed %d", f.messages.Text(), pickers.dialogsClosed)
	}
}

func TestRemovingFromTheUserSheetAsksAboutTheHomeFolder(t *testing.T) {
	_, l, pickers, _, s := userSheetFixture(t)
	if !pickers.respond(responseRemove) {
		t.Fatal("Remove User kept the sheet open")
	}
	ask := pickers.dialogs[len(pickers.dialogs)-1]
	if len(pickers.dialogs) != 2 || ask.CancelResponse != responseCancel {
		t.Fatalf("Remove User opened %d dialogs", len(pickers.dialogs))
	}
	ask.OnResponse(responseDeleteFiles)
	l.until("DeleteUser", func() bool { c, _ := s.manager.snapshot(); return len(c) == 2 })
	if calls, _ := s.manager.snapshot(); calls[1].method != "DeleteUser" || fmt.Sprint(calls[1].args) != "[2000 true]" {
		t.Fatalf("removing bob with his files: %+v", calls)
	}
	if calls, _ := s.others[0].snapshot(); len(calls) != 0 {
		t.Fatalf("removing bob edited him first: %+v", calls)
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
