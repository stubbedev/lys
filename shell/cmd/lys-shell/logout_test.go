package main

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/service/lyswm/lyswmtest"
	"github.com/stubbedev/lys/lib/logind"
	"github.com/stubbedev/lys/lib/logind/logindtest"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/wmipc"
)

// `lys-shell logout` ends the user's display session with no
// XDG_SESSION_ID to go on.
func TestLogoutEndsTheDisplaySession(t *testing.T) {
	const display logind.SessionID = "2"
	t.Setenv(wmipc.SocketEnv, "")
	fake := logindtest.StartUnit(t, logindtest.Config{Display: display})
	stdout, stderr, code := runCaptured(t, false, "logout")
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("logout: code %d stdout %q stderr %q", code, stdout, stderr)
	}
	if got := terminated(fake); !slices.Equal(got, []dbus.ObjectPath{logind.SessionPath(display)}) {
		t.Errorf("terminated %v, want [%s]", got, logind.SessionPath(display))
	}
}

// With no session to resolve, logout fails loudly and ends nothing.
func TestLogoutWithoutASessionIsAnError(t *testing.T) {
	t.Setenv(wmipc.SocketEnv, "")
	fake := logindtest.StartUnit(t, logindtest.Config{})
	stdout, stderr, code := runCaptured(t, false, "logout")
	want := "Error: log out: logind Manager.GetSession(\"auto\") on /org/freedesktop/login1: No session 'auto' known\n"
	if code != 1 || stdout != "" || stderr != want {
		t.Errorf("logout: code %d stdout %q stderr %q, want code 1 stderr %q", code, stdout, stderr, want)
	}
	if got := terminated(fake); len(got) != 0 {
		t.Errorf("terminated %v with no session", got)
	}
}

func TestLogoutExitsLysWM(t *testing.T) {
	fake := logindtest.StartUnit(t, logindtest.Config{Display: "2"})
	wm := lyswmtest.Start(t)
	stdout, stderr, code := runCaptured(t, false, "logout")
	if code != 0 || stdout != "" || stderr != "" {
		t.Fatalf("logout: code %d stdout %q stderr %q", code, stdout, stderr)
	}
	wm.ExpectDispatch(t, "exit")
	if got := terminated(fake); len(got) != 0 {
		t.Errorf("logind terminated %v while lys-wm runs", got)
	}
}

func TestLogoutLysWMRejectionIsAnError(t *testing.T) {
	fake := logindtest.StartUnit(t, logindtest.Config{Display: "2"})
	wm := lyswmtest.Start(t)
	wm.Answer("dispatch", `{"version":1,"error":"exit refused"}`)
	stdout, stderr, code := runCaptured(t, false, "logout")
	want := "Error: log out: lys-wm rejected dispatch: exit refused\n"
	if code != 1 || stdout != "" || stderr != want {
		t.Errorf("logout: code %d stdout %q stderr %q, want code 1 stderr %q", code, stdout, stderr, want)
	}
	if got := terminated(fake); len(got) != 0 {
		t.Errorf("logind terminated %v after lys-wm refused the exit", got)
	}
}

func terminated(fake *logindtest.Logind) []dbus.ObjectPath {
	calls := fake.CallsTo("Session.Terminate")
	paths := make([]dbus.ObjectPath, len(calls))
	for i, c := range calls {
		paths[i] = c.Path
	}
	return paths
}
