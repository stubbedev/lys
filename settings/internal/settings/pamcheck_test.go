package settings

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/fakeclock"
	"github.com/stubbedev/lys/desktop/i18n"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const pamServicePath = "login.lock.pam-service"

func pamServiceRowOf(t *testing.T, k *kit) (*fieldRow, *choiceEditor) {
	t.Helper()
	row := newFieldRow(k, k.store, pageRow(t, "lock", pamServicePath))
	c, ok := row.ctl.(*choiceEditor)
	if !ok {
		t.Fatalf("the PAM service row is a %T, want a choice of services", row.ctl)
	}
	return row, c
}

func TestAPAMServiceIsSavedOnceTheLockCanUseIt(t *testing.T) {
	k := testKit(t, "")
	var checked []string
	k.checkPAM = func(_ context.Context, service string) error {
		checked = append(checked, service)
		return nil
	}
	row, c := pamServiceRowOf(t, k)
	openChoices(t, k, c).pick(t, "lys-lock")
	if !slices.Equal(checked, []string{"lys-lock"}) {
		t.Fatalf("picking lys-lock checked %q", checked)
	}
	if v := k.store.value(pamServicePath); v != "lys-lock" {
		t.Errorf("a service that passed was stored as %v", v)
	}
	if problemShown(row.settingRow) {
		t.Errorf("a service that passed reports %q", problemText(row.settingRow))
	}
}

func TestAPAMServiceTheLockCannotUseIsNotSaved(t *testing.T) {
	k := testKit(t, "")
	before := k.store.value(pamServicePath)
	k.checkPAM = func(context.Context, string) error {
		return errors.New("lys-login check-pam: PAM service login: account of alice: User account has expired")
	}
	row, c := pamServiceRowOf(t, k)
	openChoices(t, k, c).pick(t, "login")
	if v := k.store.value(pamServicePath); v != before {
		t.Errorf("a refused service was stored: %v, want %v kept", v, before)
	}
	if !refused(row, locale.T().Get("settings-pam-check-failed", i18n.Str("service", "login")), "User account has expired") {
		t.Errorf("a refused service reports %q", problemText(row.settingRow))
	}
}

// heldChecks runs the kit's background work on goroutines and queues what
// they hand the loop, so a test is the loop; every check waits for its
// service's release.
type heldChecks struct {
	loop    chan func()
	started chan string
	release map[string]chan error
}

func holdPAMChecks(t *testing.T, k *kit, services ...string) *heldChecks {
	t.Helper()
	h := &heldChecks{loop: make(chan func(), 4), started: make(chan string, 4), release: map[string]chan error{}}
	for _, s := range services {
		h.release[s] = make(chan error, 1)
	}
	k.background = func(fn func()) { go fn() }
	k.invoke = func(fn func()) { h.loop <- fn }
	k.checkPAM = func(ctx context.Context, service string) error {
		h.started <- service
		select {
		case err := <-h.release[service]:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return h
}

func (h *heldChecks) waitStarted(t *testing.T, service string) {
	t.Helper()
	if got := <-h.started; got != service {
		t.Fatalf("checked %q, want %q", got, service)
	}
}

func (h *heldChecks) runLoop(t *testing.T) {
	t.Helper()
	select {
	case fn := <-h.loop:
		fn()
	case <-time.After(10 * time.Second):
		t.Fatal("nothing reached the loop")
	}
}

func pending(row *fieldRow, service string) bool {
	return problemShown(row.settingRow) && row.problem.kind == stateLoading && strings.Contains(row.problem.Text(), service)
}

// refused reports a row saying summary over a closed detail holding
// cause.
func refused(row *fieldRow, summary, cause string) bool {
	return failedWith(row.problem, summary, cause)
}

func TestASlowPAMCheckLeavesTheWindowResponsive(t *testing.T) {
	k := testKit(t, "")
	before := k.store.value(pamServicePath)
	row, c := pamServiceRowOf(t, k)
	h := holdPAMChecks(t, k, "lys-lock")
	openChoices(t, k, c).pick(t, "lys-lock")
	h.waitStarted(t, "lys-lock")
	if !pending(row, "lys-lock") {
		t.Fatalf("while the check runs the row shows %q", problemText(row.settingRow))
	}
	if v := k.store.value(pamServicePath); v != before {
		t.Fatalf("a service was stored before its check passed: %v", v)
	}
	if err := k.store.set("login.lock.lock-on-start", true); err != nil {
		t.Fatalf("another setting could not be written while the check ran: %v", err)
	}
	k.clock.(*fakeclock.Clock).Advance(commandTimeout - time.Second)
	if !pending(row, "lys-lock") {
		t.Fatalf("before the timeout the row shows %q", problemText(row.settingRow))
	}
	h.release["lys-lock"] <- nil
	h.runLoop(t)
	if v := k.store.value(pamServicePath); v != "lys-lock" {
		t.Errorf("a service whose slow check passed was stored as %v", v)
	}
	if problemShown(row.settingRow) {
		t.Errorf("a service that passed reports %q", problemText(row.settingRow))
	}
	k.clock.(*fakeclock.Clock).Advance(time.Minute)
	if len(h.loop) != 0 {
		t.Error("the settled check's timeout still fired")
	}
}

func TestAPAMCheckThatNeverAnswersIsRefusedAtTheTimeout(t *testing.T) {
	k := testKit(t, "")
	before := k.store.value(pamServicePath)
	row, c := pamServiceRowOf(t, k)
	h := holdPAMChecks(t, k, "lys-lock")
	openChoices(t, k, c).pick(t, "lys-lock")
	h.waitStarted(t, "lys-lock")
	k.clock.(*fakeclock.Clock).Advance(commandTimeout)
	h.runLoop(t)
	if v := k.store.value(pamServicePath); v != before {
		t.Errorf("a service whose check timed out was stored: %v", v)
	}
	timedOut := locale.T().Get("settings-check-timed-out", i18n.Int("seconds", 5))
	if !refused(row, timedOut, "lys-lock") {
		t.Errorf("a timed-out check reports %q", problemText(row.settingRow))
	}
	h.runLoop(t)
	if problemText(row.settingRow) != timedOut {
		t.Errorf("the cancelled check's late answer replaced the timeout with %q", problemText(row.settingRow))
	}
}

func TestANewerPAMServiceDropsTheOlderCheck(t *testing.T) {
	k := testKit(t, "")
	_, c := pamServiceRowOf(t, k)
	h := holdPAMChecks(t, k, "login", "lys-lock")
	openChoices(t, k, c).pick(t, "login")
	h.waitStarted(t, "login")
	openChoices(t, k, c).pick(t, "lys-lock")
	h.waitStarted(t, "lys-lock")
	h.runLoop(t)
	h.release["lys-lock"] <- nil
	h.runLoop(t)
	if v := k.store.value(pamServicePath); v != "lys-lock" {
		t.Errorf("the newer service was stored as %v", v)
	}
}

func TestAResetIsCheckedLikeAnyOtherPAMService(t *testing.T) {
	k := testKit(t, "")
	if err := k.store.set(pamServicePath, "lys-lock"); err != nil {
		t.Fatal(err)
	}
	d, _ := k.store.defaultOf(pamServicePath)
	var checked []string
	k.checkPAM = func(_ context.Context, service string) error {
		checked = append(checked, service)
		return nil
	}
	row, _ := pamServiceRowOf(t, k)
	row.reset.OnClick()
	if !slices.Equal(checked, []string{d.(string)}) {
		t.Fatalf("the reset checked %q, want the default %v", checked, d)
	}
	if v := k.store.value(pamServicePath); v != d {
		t.Errorf("a reset whose default passed left %v", v)
	}
}

func TestAResetToAPAMServiceTheLockCannotUseIsRefused(t *testing.T) {
	k := testKit(t, "")
	if err := k.store.set(pamServicePath, "lys-lock"); err != nil {
		t.Fatal(err)
	}
	k.checkPAM = func(_ context.Context, service string) error {
		return errors.New("lys-login check-pam: PAM service " + service + ": no such service")
	}
	row, _ := pamServiceRowOf(t, k)
	row.reset.OnClick()
	if v := k.store.value(pamServicePath); v != "lys-lock" {
		t.Errorf("a reset to a refused default left %v, want lys-lock kept", v)
	}
	d, _ := k.store.defaultOf(pamServicePath)
	if !refused(row, locale.T().Get("settings-pam-check-failed", i18n.Str("service", d.(string))), "no such service") {
		t.Errorf("a refused reset reports %q", problemText(row.settingRow))
	}
}
