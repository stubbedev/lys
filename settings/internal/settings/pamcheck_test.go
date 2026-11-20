package settings

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

const pamServicePath = "login.lock.pam-service"

func pamServiceRowOf(t *testing.T, k *kit) (*settingRow, *choiceEditor) {
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
	k.checkPAM = func(service string) error {
		checked = append(checked, service)
		return nil
	}
	row, c := pamServiceRowOf(t, k)
	pickName(t, openChoices(t, k, c), "lys-lock")
	if !slices.Equal(checked, []string{"lys-lock"}) {
		t.Fatalf("picking lys-lock checked %q", checked)
	}
	if v := k.store.value(pamServicePath); v != "lys-lock" {
		t.Errorf("a service that passed was stored as %v", v)
	}
	if row.problem.Visible() {
		t.Errorf("a service that passed reports %q", row.problem.Text())
	}
}

func TestAPAMServiceTheLockCannotUseIsNotSaved(t *testing.T) {
	k := testKit(t, "")
	before := k.store.value(pamServicePath)
	k.checkPAM = func(string) error {
		return errors.New("lys-login check-pam: PAM service login: account of alice: User account has expired")
	}
	row, c := pamServiceRowOf(t, k)
	pickName(t, openChoices(t, k, c), "login")
	if v := k.store.value(pamServicePath); v != before {
		t.Errorf("a refused service was stored: %v, want %v kept", v, before)
	}
	if !row.problem.Visible() || !strings.Contains(row.problem.Text(), "User account has expired") || !strings.Contains(row.problem.Text(), "login") {
		t.Errorf("a refused service reports %q (visible %v)", row.problem.Text(), row.problem.Visible())
	}
}
