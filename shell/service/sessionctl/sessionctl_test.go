package sessionctl

import (
	"errors"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/service/lyswm/lyswmtest"
	"github.com/stubbedev/lys/lib/logind"
	"github.com/stubbedev/lys/lib/logind/logindtest"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/wmipc"
)

const display logind.SessionID = "2"

var builtinCases = []struct {
	builtin config.PowerBuiltin
	method  string
	event   logindtest.Event
	refusal string
}{
	{config.PowerBuiltinLock, "Manager.LockSession", logindtest.EventLock, `lock: logind Manager.LockSession("auto") on /org/freedesktop/login1: access denied`},
	{config.PowerBuiltinLogout, "Session.Terminate", "", `log out: logind Session.Terminate() on /org/freedesktop/login1/session/_32: access denied`},
	{config.PowerBuiltinSuspend, "Manager.Suspend", logindtest.EventSuspend, `suspend: logind Manager.Suspend(false) on /org/freedesktop/login1: access denied`},
	{config.PowerBuiltinReboot, "Manager.Reboot", logindtest.EventReboot, `reboot: logind Manager.Reboot(false) on /org/freedesktop/login1: access denied`},
	{config.PowerBuiltinPowerOff, "Manager.PowerOff", logindtest.EventPowerOff, `poweroff: logind Manager.PowerOff(false) on /org/freedesktop/login1: access denied`},
}

func TestEveryBuiltinIsCovered(t *testing.T) {
	var covered []config.PowerBuiltin
	for _, c := range builtinCases {
		covered = append(covered, c.builtin)
	}
	if !slices.Equal(covered, config.PowerBuiltins()) {
		t.Fatalf("cases cover %v, want %v", covered, config.PowerBuiltins())
	}
}

func TestABuiltinAsksLogind(t *testing.T) {
	for _, c := range builtinCases {
		t.Run(c.builtin.String(), func(t *testing.T) {
			t.Setenv(wmipc.SocketEnv, "")
			fake := logindtest.Start(t, logindtest.Config{Display: display})
			if err := Run(t.Context(), On(fake.Bus.Conn(t)), c.builtin); err != nil {
				t.Fatalf("%s: %v", c.builtin, err)
			}
			if got := fake.CallsTo(c.method); len(got) != 1 {
				t.Errorf("%s: %s calls %v, want one", c.builtin, c.method, got)
			}
			if c.builtin == config.PowerBuiltinLogout {
				if got := fake.CallsTo("Session.Terminate"); got[0].Path != logind.SessionPath(display) {
					t.Errorf("logout terminated %s, want %s", got[0].Path, logind.SessionPath(display))
				}
				return
			}
			if got := fake.Events.Load(); !slices.Equal(got, []logindtest.Event{c.event}) {
				t.Errorf("%s: logind events %v, want [%s]", c.builtin, got, c.event)
			}
		})
	}
}

func TestALogindRefusalIsTheBuiltinsError(t *testing.T) {
	for _, c := range builtinCases {
		t.Run(c.builtin.String(), func(t *testing.T) {
			t.Setenv(wmipc.SocketEnv, "")
			fake := logindtest.Start(t, logindtest.Config{Display: display})
			fake.Refuse(c.method, dbus.NewError("org.freedesktop.DBus.Error.AccessDenied", []any{"access denied"}))
			err := Run(t.Context(), On(fake.Bus.Conn(t)), c.builtin)
			if err == nil || err.Error() != c.refusal {
				t.Fatalf("%s: err %v, want %q", c.builtin, err, c.refusal)
			}
			if got := fake.Events.Load(); len(got) != 0 {
				t.Errorf("%s: logind events %v after the refusal", c.builtin, got)
			}
		})
	}
}

func TestABuiltinWithoutASystemBusFails(t *testing.T) {
	t.Setenv(wmipc.SocketEnv, "")
	for _, b := range config.PowerBuiltins() {
		if err := Run(t.Context(), On(nil), b); !errors.Is(err, ErrNoSystemBus) {
			t.Errorf("%s with no system bus: %v, want ErrNoSystemBus", b, err)
		}
	}
}

func TestLogoutExitsLysWMWithoutLogind(t *testing.T) {
	wm := lyswmtest.Start(t)
	if err := Run(t.Context(), On(nil), config.PowerBuiltinLogout); err != nil {
		t.Fatalf("logout under lys-wm: %v", err)
	}
	wm.ExpectDispatch(t, "exit")
}

func TestLogoutLysWMRejectionIsTheError(t *testing.T) {
	wm := lyswmtest.Start(t)
	wm.Answer("dispatch", `{"version":1,"error":"exit refused"}`)
	err := Run(t.Context(), On(nil), config.PowerBuiltinLogout)
	if want := "log out: lys-wm rejected dispatch: exit refused"; err == nil || err.Error() != want {
		t.Fatalf("err %v, want %q", err, want)
	}
}
