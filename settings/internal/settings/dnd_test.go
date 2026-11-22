package settings

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/fakeclock"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/dbustest"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/shell/service/notifications"

	"github.com/stubbedev/lys/settings/internal/locale"
)

type fakeDND struct {
	state  dndState
	setErr error
	sets   []bool
	closed bool
}

func (f *fakeDND) read(context.Context) (dndState, error) { return f.state, nil }

func (f *fakeDND) changes(context.Context) (<-chan struct{}, error) {
	return make(chan struct{}), nil
}

func (f *fakeDND) set(_ context.Context, on bool) error {
	f.sets = append(f.sets, on)
	if f.setErr != nil {
		return f.setErr
	}
	f.state = dndState{on: on, reason: notifications.FocusOff}
	if on {
		f.state.reason = notifications.FocusManual
	}
	return nil
}

func (f *fakeDND) close() error {
	f.closed = true
	return nil
}

func dndPanelOf(t *testing.T, f *fakeDND) *dndPanel {
	t.Helper()
	k := testKit(t, "")
	k.dnd = func() (dndControl, error) { return f, nil }
	p := newDNDPanel(k)
	t.Cleanup(p.close)
	return p
}

func dndStatusOf(status dndStatus, at string) string {
	return locale.T().Get("settings-dnd-status", i18n.Str("status", string(status)), i18n.Str("time", at))
}

func TestTheDoNotDisturbSwitchShowsAndSetsTheShellsState(t *testing.T) {
	f := &fakeDND{state: dndState{reason: notifications.FocusOff}}
	p := dndPanelOf(t, f)
	if p.toggle.On() || !p.toggle.Enabled() || p.Description() != dndStatusOf(dndOff, "") {
		t.Fatalf("off reads on %v enabled %v %q", p.toggle.On(), p.toggle.Enabled(), p.Description())
	}
	p.toggle.SetOn(true)
	if len(f.sets) != 1 || !f.sets[0] {
		t.Fatalf("switching on asked lys-shell for %v", f.sets)
	}
	if !p.toggle.On() || p.Description() != dndStatusOf(dndManual, "") || p.problem.Visible() {
		t.Fatalf("after switching on: on %v %q, problem %q", p.toggle.On(), p.Description(), p.problem.Text())
	}
	p.close()
	if !f.closed {
		t.Error("closing the page left the bus connection open")
	}
}

func TestARefusedDoNotDisturbSwitchSaysSoAndKeepsTheState(t *testing.T) {
	f := &fakeDND{state: dndState{reason: notifications.FocusOff}, setErr: errors.New("access denied")}
	p := dndPanelOf(t, f)
	p.toggle.SetOn(true)
	if !p.problem.Visible() || p.toggle.On() {
		t.Fatalf("a refusal shows %q (%v), switch on %v", p.problem.Text(), p.problem.Visible(), p.toggle.On())
	}
}

func TestWithoutLysShellTheDoNotDisturbSwitchStaysOff(t *testing.T) {
	k := testKit(t, "")
	p := newDNDPanel(k)
	want := locale.T().Get("settings-dnd-unavailable", i18n.Str("reason", "no lys-shell in the tests"))
	if p.toggle.Enabled() || p.Description() != want {
		t.Fatalf("without lys-shell the switch is enabled %v, reads %q", p.toggle.Enabled(), p.Description())
	}
	p.toggle.SetOn(true)
}

func TestTheDoNotDisturbStatusSaysWhyAndUntilWhen(t *testing.T) {
	now := time.Date(2026, time.November, 19, 14, 5, 0, 0, time.UTC)
	tonight, tomorrow := time.Date(2026, time.November, 19, 22, 0, 0, 0, time.UTC), time.Date(2026, time.November, 20, 7, 0, 0, 0, time.UTC)
	k := testKit(t, "modules:\n  clock:\n    format: \"%H:%M\"\n")
	for name, c := range map[string]struct {
		state dndState
		want  string
	}{
		"off":                   {dndState{reason: notifications.FocusOff}, dndStatusOf(dndOff, "")},
		"by hand":               {dndState{on: true, reason: notifications.FocusManual}, dndStatusOf(dndManual, "")},
		"until tonight":         {dndState{on: true, reason: notifications.FocusUntil, until: tonight}, dndStatusOf(dndUntil, "22:00")},
		"scheduled to tomorrow": {dndState{on: true, reason: notifications.FocusScheduled, until: tomorrow}, dndStatusOf(dndScheduled, "Fri 07:00")},
		"sharing":               {dndState{on: true, reason: notifications.FocusSharing}, dndStatusOf(dndSharing, "")},
		"fullscreen":            {dndState{on: true, reason: notifications.FocusFullscreen}, dndStatusOf(dndFullscreen, "")},
		"held on":               {dndState{on: true, reason: notifications.FocusOverride, until: tonight}, dndStatusOf(dndHeldOn, "22:00")},
		"held on, open":         {dndState{on: true, reason: notifications.FocusOverride}, dndStatusOf(dndHeldOnOpen, "")},
		"held off":              {dndState{reason: notifications.FocusOverride, until: tonight}, dndStatusOf(dndHeldOff, "22:00")},
		"held off, open":        {dndState{reason: notifications.FocusOverride}, dndStatusOf(dndHeldOffOpen, "")},
	} {
		if got := dndStatusText(k, c.state, now); got != c.want {
			t.Errorf("%s reads %q, want %q", name, got, c.want)
		}
	}
	if dndStatusOf(dndHeldOn, "22:00") == dndStatusOf(dndHeldOff, "22:00") || dndStatusOf(dndOff, "") == dndStatusOf(dndManual, "") {
		t.Error("on and off read the same")
	}
}

func TestTheDoNotDisturbClientFollowsLysShellOnItsBus(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	bus := dbustest.Start(t)
	svc := notifications.NewService(fakeclock.New())
	t.Cleanup(func() { svc.FlushWrites(10 * time.Second) })
	release, err := notifications.ServeLysShell(bus.Conn(t), svc)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	ctl, err := openDND(func() (*dbus.Conn, error) { return bus.Conn(t), nil })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ctl.close() })
	ctx := t.Context()
	if st, err := ctl.read(ctx); err != nil || st.on || st.reason != notifications.FocusOff {
		t.Fatalf("at the start do-not-disturb reads %+v (%v)", st, err)
	}
	ticks, err := ctl.changes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := ctl.set(ctx, true); err != nil {
		t.Fatal(err)
	}
	waittest.For(t, ticks, "the change settings made")
	if st, err := ctl.read(ctx); err != nil || !st.on || st.reason != notifications.FocusManual || !svc.DND() {
		t.Fatalf("after switching on do-not-disturb reads %+v (%v), the shell has it %v", st, err, svc.DND())
	}
	svc.SetDND(false)
	waittest.For(t, ticks, "the change the bar made")
	if st, err := ctl.read(ctx); err != nil || st.on {
		t.Fatalf("after the shell switched it off do-not-disturb reads %+v (%v)", st, err)
	}
}

func TestTheDoNotDisturbClientSaysWhenLysShellIsNotThere(t *testing.T) {
	bus := dbustest.Start(t)
	ctl, err := openDND(func() (*dbus.Conn, error) { return bus.Conn(t), nil })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ctl.close() })
	if _, err := ctl.read(t.Context()); err == nil {
		t.Error("do-not-disturb read without lys-shell on the bus")
	}
	if err := ctl.set(t.Context(), true); err == nil {
		t.Error("do-not-disturb set without lys-shell on the bus")
	}
}
