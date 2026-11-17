package idle

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/service/idle/idlepolicy"
	"github.com/stubbedev/lys/lib/logind"
	"github.com/stubbedev/lys/lib/logind/logindtest"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/waittest"

	"github.com/stubbedev/lys/shell/service/brightness"
)

const (
	display     logind.SessionID = "c1"
	quietWindow                  = 100 * time.Millisecond
)

func startLogin(t *testing.T) (*logindtest.Logind, Login) {
	t.Helper()
	fake := logindtest.Start(t, logindtest.Config{Display: display})
	m := logind.NewManager(fake.Bus.Conn(t))
	session, err := m.GetSession(t.Context(), logind.AutoSession)
	if err != nil {
		t.Fatal(err)
	}
	return fake, Login{Manager: m, Session: session}
}

func startSleepLock(t *testing.T, lockWait time.Duration) *logindtest.Logind {
	t.Helper()
	fake, login := startLogin(t)
	stop, err := LockBeforeSleep(login, lockWait)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(stop)
	if n := fake.Inhibitors.Load(); n != 1 {
		t.Fatalf("%d sleep inhibitors held after start, want 1", n)
	}
	return fake
}

func awaitEvents(t *testing.T, fake *logindtest.Logind, want []logindtest.Event) {
	t.Helper()
	waittest.Until(t, func() bool { return len(fake.Events.Load()) >= len(want) }, fmt.Sprintf("logind to see %v", want))
	if got := fake.Events.Load(); !slices.Equal(got, want) {
		t.Fatalf("logind saw %v, want %v", got, want)
	}
}

func TestSleepWaitsForTheLockScreenBeforeLettingGo(t *testing.T) {
	fake := startSleepLock(t, 5*time.Second)
	fake.OnLock.Store(func() {
		time.Sleep(200 * time.Millisecond)
		fake.SetLockedHint(t, display, true)
	})
	fake.PrepareForSleep(t, true)
	awaitEvents(t, fake, []logindtest.Event{
		logindtest.EventInhibit, logindtest.EventLock, logindtest.EventLockedHint, logindtest.EventInhibitorReleased,
	})
	if n := fake.Inhibitors.Load(); n != 0 {
		t.Errorf("%d inhibitors held while asleep", n)
	}
	fake.SetLockedHint(t, display, false)
	fake.PrepareForSleep(t, false)
	awaitEvents(t, fake, []logindtest.Event{
		logindtest.EventInhibit, logindtest.EventLock, logindtest.EventLockedHint, logindtest.EventInhibitorReleased,
		logindtest.EventUnlockedHint, logindtest.EventInhibit,
	})
}

func TestALockScreenThatNeverConfirmsDoesNotHoldSleepForever(t *testing.T) {
	fake := startSleepLock(t, 300*time.Millisecond)
	fake.PrepareForSleep(t, true)
	awaitEvents(t, fake, []logindtest.Event{logindtest.EventInhibit, logindtest.EventLock, logindtest.EventInhibitorReleased})
	if slices.Contains(fake.Events.Load(), logindtest.EventLockedHint) {
		t.Error("LockedHint set without a lock screen")
	}
}

func TestStopLetsGoOfTheInhibitor(t *testing.T) {
	fake, login := startLogin(t)
	stop, err := LockBeforeSleep(login, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	stop()
	awaitEvents(t, fake, []logindtest.Event{logindtest.EventInhibit, logindtest.EventInhibitorReleased})
	fake.PrepareForSleep(t, true)
	time.Sleep(quietWindow)
	if got := fake.Events.Load(); slices.Contains(got, logindtest.EventLock) {
		t.Errorf("a stopped lock-before-sleep still locked: %v", got)
	}
}

func TestDimIsAnnouncedAsTheSessionIdleUntilItsRestore(t *testing.T) {
	fake, login := startLogin(t)
	var hints []bool
	b := &fakeBacklights{devices: map[string]brightness.Device{
		"intel_backlight": {Name: "intel_backlight", Brightness: 400, Max: 1000},
		"ddcci7":          {Name: "ddcci7", Brightness: 37, Max: 100},
	}}
	b.onSet = func() {
		idle, err := login.Session.IdleHint(t.Context())
		if err != nil {
			t.Errorf("read IdleHint: %v", err)
		}
		hints = append(hints, idle)
	}
	share, err := idlepolicy.NewDimPercent(30)
	if err != nil {
		t.Fatal(err)
	}
	restore, err := System{Backlights: b, Login: &login}.Dim(share)
	if err != nil {
		t.Fatal(err)
	}
	if err := restore(); err != nil {
		t.Fatal(err)
	}
	if want := []bool{true, true, true, true}; !slices.Equal(hints, want) {
		t.Errorf("IdleHint as each backlight was dimmed and restored: %v, want %v", hints, want)
	}
	if idle, err := login.Session.IdleHint(t.Context()); err != nil || idle {
		t.Errorf("IdleHint after the restore = %v, %v; want false", idle, err)
	}
	if got, want := fake.Events.Load(), []logindtest.Event{logindtest.EventIdleHint, logindtest.EventActiveHint}; !slices.Equal(got, want) {
		t.Errorf("logind saw %v, want %v", got, want)
	}
}

func TestADimLogindRefusesToAnnounceStillDimsAndSaysSo(t *testing.T) {
	fake, login := startLogin(t)
	fake.Refuse("Session.SetIdleHint", dbus.NewError("org.freedesktop.DBus.Error.AccessDenied", []any{"not allowed"}))
	b := &fakeBacklights{devices: map[string]brightness.Device{
		"intel_backlight": {Name: "intel_backlight", Brightness: 400, Max: 1000},
	}}
	share, err := idlepolicy.NewDimPercent(30)
	if err != nil {
		t.Fatal(err)
	}
	restore, err := System{Backlights: b, Login: &login}.Dim(share)
	if err == nil || !strings.Contains(err.Error(), "IdleHint to true") {
		t.Errorf("Dim = %v, want logind's refusal of the hint", err)
	}
	if got := b.devices["intel_backlight"].Brightness; got != 120 {
		t.Errorf("dimmed to %d, want 120", got)
	}
	if restore == nil {
		t.Fatal("a dim without its announcement has no restore")
	}
	if err := restore(); err == nil || !strings.Contains(err.Error(), "IdleHint to false") {
		t.Errorf("restore = %v, want logind's refusal of the hint", err)
	}
	if got := b.devices["intel_backlight"].Brightness; got != 400 {
		t.Errorf("restored to %d, want 400", got)
	}
}

func TestSystemLocksAndSuspendsThroughLogind(t *testing.T) {
	fake, login := startLogin(t)
	s := System{Login: &login}
	if err := s.Lock(); err != nil {
		t.Fatal(err)
	}
	if err := s.Sleep(logind.Suspend); err != nil {
		t.Fatal(err)
	}
	want := []logindtest.Event{logindtest.EventLock, logindtest.EventSuspend}
	if got := fake.Events.Load(); !slices.Equal(got, want) {
		t.Errorf("logind saw %v, want %v", got, want)
	}
	if got := fake.CallsTo("Session.Lock"); len(got) != 1 || got[0].Path != logind.SessionPath(display) {
		t.Errorf("Session.Lock calls %v, want one on %s", got, logind.SessionPath(display))
	}
	if err := (System{}).Lock(); !errors.Is(err, ErrNoLogind) {
		t.Errorf("Lock without logind: %v, want ErrNoLogind", err)
	}
	if err := (System{}).Sleep(logind.Suspend); !errors.Is(err, ErrNoLogind) {
		t.Errorf("Sleep without logind: %v, want ErrNoLogind", err)
	}
}

func TestSystemSleepsThroughTheLogindMethodItIsGiven(t *testing.T) {
	for action, tc := range map[logind.PowerAction]struct {
		method string
		event  logindtest.Event
	}{
		logind.Suspend:              {"Manager.Suspend", logindtest.EventSuspend},
		logind.Hibernate:            {"Manager.Hibernate", logindtest.EventHibernate},
		logind.HybridSleep:          {"Manager.HybridSleep", logindtest.EventHybridSleep},
		logind.SuspendThenHibernate: {"Manager.SuspendThenHibernate", logindtest.EventSuspendThenHibernate},
	} {
		t.Run(action.String(), func(t *testing.T) {
			fake, login := startLogin(t)
			if err := (System{Login: &login}).Sleep(action); err != nil {
				t.Fatal(err)
			}
			if got := fake.Events.Load(); !slices.Equal(got, []logindtest.Event{tc.event}) {
				t.Errorf("logind saw %v, want %s", got, tc.event)
			}
			if got := fake.CallsTo(tc.method); len(got) != 1 {
				t.Errorf("%s calls %v, want one", tc.method, got)
			}
		})
	}
}
