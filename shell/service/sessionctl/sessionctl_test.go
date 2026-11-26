package sessionctl

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/service/lyswm/lyswmtest"
	"github.com/stubbedev/lys/lib/logind"
	"github.com/stubbedev/lys/lib/logind/logindtest"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/waittest"
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
	{config.PowerBuiltinSuspend, "Manager.Suspend", logindtest.EventSuspend, `suspend: logind Manager.Suspend(true) on /org/freedesktop/login1: access denied`},
	{config.PowerBuiltinReboot, "Manager.Reboot", logindtest.EventReboot, `reboot: logind Manager.Reboot(true) on /org/freedesktop/login1: access denied`},
	{config.PowerBuiltinPowerOff, "Manager.PowerOff", logindtest.EventPowerOff, `poweroff: logind Manager.PowerOff(true) on /org/freedesktop/login1: access denied`},
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

var powerCases = []struct {
	builtin config.PowerBuiltin
	method  string
	event   logindtest.Event
	denial  string
}{
	{config.PowerBuiltinSuspend, "Manager.Suspend", logindtest.EventSuspend, "suspend: logind Manager.Suspend(true) on /org/freedesktop/login1: Access denied"},
	{config.PowerBuiltinReboot, "Manager.Reboot", logindtest.EventReboot, "reboot: logind Manager.Reboot(true) on /org/freedesktop/login1: Access denied"},
	{config.PowerBuiltinPowerOff, "Manager.PowerOff", logindtest.EventPowerOff, "poweroff: logind Manager.PowerOff(true) on /org/freedesktop/login1: Access denied"},
}

func runAsync(ctx context.Context, conn *dbus.Conn, b config.PowerBuiltin) <-chan error {
	done := make(chan error, 1)
	go func() { done <- Run(ctx, On(conn), b) }()
	return done
}

func TestAPowerBuiltinWaitsForPolkitsPromptAndGoesOnOnceAuthorised(t *testing.T) {
	for _, c := range powerCases {
		t.Run(c.builtin.String(), func(t *testing.T) {
			fake := logindtest.Start(t, logindtest.Config{Display: display})
			prompts := fake.Challenge(c.method)
			done := runAsync(t.Context(), fake.Bus.Conn(t), c.builtin)
			prompt := waittest.For(t, prompts, "polkit's prompt for "+c.method)
			if got := fake.Events.Load(); len(got) != 0 {
				t.Errorf("%s: logind events %v before the prompt was answered", c.builtin, got)
			}
			prompt.Authorize()
			if err := waittest.For(t, done, c.builtin.String()+" once authorised"); err != nil {
				t.Fatalf("%s: %v", c.builtin, err)
			}
			if got := fake.Events.Load(); !slices.Equal(got, []logindtest.Event{c.event}) {
				t.Errorf("%s: logind events %v, want [%s]", c.builtin, got, c.event)
			}
		})
	}
}

func TestAPowerBuiltinWhosePromptIsDeniedFails(t *testing.T) {
	for _, c := range powerCases {
		t.Run(c.builtin.String(), func(t *testing.T) {
			fake := logindtest.Start(t, logindtest.Config{Display: display})
			prompts := fake.Challenge(c.method)
			done := runAsync(t.Context(), fake.Bus.Conn(t), c.builtin)
			waittest.For(t, prompts, "polkit's prompt for "+c.method).Deny()
			if err := waittest.For(t, done, "the denied "+c.builtin.String()); err == nil || err.Error() != c.denial {
				t.Fatalf("%s: err %v, want %q", c.builtin, err, c.denial)
			}
			if got := fake.Events.Load(); len(got) != 0 {
				t.Errorf("%s: logind events %v after the denial", c.builtin, got)
			}
		})
	}
}

func TestAPowerBuiltinStopsWaitingWhenItsContextEnds(t *testing.T) {
	fake := logindtest.Start(t, logindtest.Config{Display: display})
	prompts := fake.Challenge("Manager.PowerOff")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := runAsync(ctx, fake.Bus.Conn(t), config.PowerBuiltinPowerOff)
	waittest.For(t, prompts, "polkit's prompt for Manager.PowerOff")
	cancel()
	if err := waittest.For(t, done, "the cancelled poweroff"); !errors.Is(err, context.Canceled) {
		t.Fatalf("err %v, want context.Canceled", err)
	}
	if got := fake.Events.Load(); len(got) != 0 {
		t.Errorf("logind events %v after the cancel", got)
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
