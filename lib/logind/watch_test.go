package logind_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/logind"
	"github.com/stubbedev/lys/lib/logind/logindtest"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/waittest"
)

type decoded struct {
	event logind.Event
	err   error
}

type received chan decoded

func watch(t *testing.T, s logind.Session) (*logind.Watch, received) {
	t.Helper()
	r := make(received, 64)
	w, err := s.Watch(t.Context(), func(e logind.Event, err error) { r <- decoded{e, err} })
	if err != nil {
		t.Fatalf("Watch: %v", err)
	}
	t.Cleanup(func() { _ = w.Stop() })
	return w, r
}

func (r received) next(t *testing.T) logind.Event {
	t.Helper()
	d := waittest.For(t, r, "an event of the watch")
	if d.err != nil {
		t.Fatalf("the watch failed to decode a signal: %v", d.err)
	}
	return d.event
}

func TestWatchDecodesEverySignalOfTheSession(t *testing.T) {
	t.Parallel()
	fake, m := manager(t, logindtest.Config{Own: own, Display: display})
	s := session(t, m, own)
	_, r := watch(t, s)
	dev := logind.Device{Major: 13, Minor: 64}

	fake.Lock(t, display)
	fake.Lock(t, own)
	fake.Unlock(t, own)
	fake.PauseDevice(t, own, dev, logind.PausePause)
	fake.ResumeDevice(t, own, dev)
	fake.SetActive(t, own, false)
	fake.SetLockedHint(t, own, true)
	fake.PrepareForSleep(t, true)

	if e := r.next(t); e != (logind.Locked{}) {
		t.Fatalf("first event %#v, want the own session's Lock (another session's leaked?)", e)
	}
	if e := r.next(t); e != (logind.Unlocked{}) {
		t.Fatalf("event %#v, want Unlock", e)
	}
	if e := r.next(t); e != (logind.DevicePaused{Major: 13, Minor: 64, Kind: logind.PausePause}) {
		t.Fatalf("event %#v, want PauseDevice", e)
	}
	resumed, ok := r.next(t).(logind.DeviceResumed)
	if !ok || resumed.Device() != dev || resumed.FD < 0 {
		t.Fatalf("event %#v, want ResumeDevice of %v with a descriptor", resumed, dev)
	}
	if err := unix.Close(int(resumed.FD)); err != nil {
		t.Errorf("ResumeDevice's descriptor: %v", err)
	}
	if e := r.next(t); e != (logind.ActiveChanged{Active: false}) {
		t.Fatalf("event %#v, want Active turning false", e)
	}
	if e := r.next(t); e != (logind.LockedHintChanged{Locked: true}) {
		t.Fatalf("event %#v, want LockedHint turning true", e)
	}
	if e := r.next(t); e != (logind.Sleep{Start: true}) {
		t.Fatalf("event %#v, want PrepareForSleep(true)", e)
	}
}

func TestWatchIgnoresSignalsNotFromLogind(t *testing.T) {
	t.Parallel()
	fake, _ := manager(t, logindtest.Config{Own: own})
	conn := fake.Bus.Conn(t)
	s, err := logind.NewManager(conn).GetSession(t.Context(), own)
	if err != nil {
		t.Fatal(err)
	}
	broad := []dbus.MatchOption{dbus.WithMatchObjectPath(s.Path())}
	if err := conn.AddMatchSignal(broad...); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.RemoveMatchSignal(broad...) })
	_, r := watch(t, s)

	forger := fake.Bus.Conn(t)
	fds := make([]int, 2)
	if err := unix.Pipe2(fds, unix.O_CLOEXEC|unix.O_NONBLOCK); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = unix.Close(fds[0]) })
	if err := errors.Join(
		logind.Lock.Emit(forger, s.Path(), logind.Locked{}),
		logind.ResumeDevice.Emit(forger, s.Path(), logind.DeviceResumed{Major: 1, Minor: 3, FD: dbus.UnixFD(fds[1])}),
		unix.Close(fds[1]),
	); err != nil {
		t.Fatal(err)
	}
	fake.Unlock(t, own)
	if e := r.next(t); e != (logind.Unlocked{}) {
		t.Fatalf("event %#v, want logind's Unlock and nothing forged before it", e)
	}
	waittest.Until(t, func() bool {
		n, err := unix.Read(fds[0], make([]byte, 1))
		if err != nil && !errors.Is(err, unix.EAGAIN) {
			t.Fatalf("read the forged ResumeDevice's pipe = %d, %v; want end of file, its descriptor closed", n, err)
		}
		return n == 0 && err == nil
	}, "the forged ResumeDevice's descriptor to close")
	select {
	case d := <-r:
		t.Errorf("a forged signal was reported: %#v, %v", d.event, d.err)
	default:
	}
}

func TestStoppingAWatchLeavesTheConnectionAsItWas(t *testing.T) {
	t.Parallel()
	fake, _ := manager(t, logindtest.Config{Own: own})
	conn := fake.Bus.Conn(t)
	s, err := logind.NewManager(conn).GetSession(t.Context(), own)
	if err != nil {
		t.Fatal(err)
	}
	before := dbustest.MatchRules(t, conn)
	w, err := s.Watch(t.Context(), func(logind.Event, error) {})
	if err != nil {
		t.Fatal(err)
	}
	if during := dbustest.MatchRules(t, conn); len(during) != len(before)+4 {
		t.Errorf("match rules during the watch %v, want 4 more than %v: three for the signals and the one that follows logind's owner", during, before)
	}
	if err := w.Stop(); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if err := w.Stop(); err != nil {
		t.Errorf("Stop again: %v", err)
	}
	if after := dbustest.MatchRules(t, conn); !slices.Equal(after, before) {
		t.Errorf("match rules after the watch %v, want %v", after, before)
	}
	select {
	case <-w.Done():
	default:
		t.Error("Done is open after Stop")
	}
}

func TestAWatchEndsWithItsContext(t *testing.T) {
	t.Parallel()
	_, m := manager(t, logindtest.Config{Own: own})
	s := session(t, m, own)
	ctx, cancel := context.WithCancel(t.Context())
	w, err := s.Watch(ctx, func(logind.Event, error) {})
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	waittest.For(t, w.Done(), "the watch to end with its context")
	if err := w.Stop(); err != nil {
		t.Errorf("Stop after the context ended = %v", err)
	}
}

func TestAWatchEndsWithItsConnection(t *testing.T) {
	t.Parallel()
	fake, _ := manager(t, logindtest.Config{Own: own})
	conn := fake.Bus.Conn(t)
	s, err := logind.NewManager(conn).GetSession(t.Context(), own)
	if err != nil {
		t.Fatal(err)
	}
	w, err := s.Watch(t.Context(), func(logind.Event, error) {})
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	waittest.For(t, w.Done(), "the watch to end with its connection")
	if err := w.Stop(); !errors.Is(err, logind.ErrDisconnected) {
		t.Errorf("Stop = %v, want ErrDisconnected", err)
	}
}

func TestWaitLocked(t *testing.T) {
	t.Parallel()
	fake, m := manager(t, logindtest.Config{Display: display})
	s := session(t, m, logind.AutoSession)

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := s.WaitLocked(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("WaitLocked with its context ended = %v", err)
	}

	fake.OnLock.Store(func() { fake.SetLockedHint(t, display, true) })
	done := make(chan error, 1)
	go func() { done <- s.WaitLocked(t.Context()) }()
	if err := s.Lock(t.Context()); err != nil {
		t.Fatal(err)
	}
	if err := waittest.For(t, done, "WaitLocked to see LockedHint turn true"); err != nil {
		t.Fatalf("WaitLocked = %v", err)
	}
	if err := s.WaitLocked(t.Context()); err != nil {
		t.Errorf("WaitLocked on a locked session = %v", err)
	}
}

func activations(t *testing.T, m logind.Manager) *logind.Activations {
	t.Helper()
	a, err := m.WatchActivations(t.Context())
	if err != nil {
		t.Fatalf("WatchActivations: %v", err)
	}
	t.Cleanup(func() { _ = a.Stop() })
	return a
}

// systemd-logind 258 and later answer Active true for a session as soon
// as it is the seat's foreground session, while udev has yet to grant
// it the seat's devices; only the announcement follows the grant.
func TestActivationsWaitForTheAnnouncementNotTheProperty(t *testing.T) {
	t.Parallel()
	fake, m := manager(t, logindtest.Config{Own: own})
	a := activations(t, m)
	if active, err := session(t, m, own).Active(t.Context()); err != nil || !active {
		t.Fatalf("Active = %v, %v; want the session in the foreground already", active, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := a.Wait(ctx, own); !errors.Is(err, context.Canceled) {
		t.Fatalf("Wait before logind announced the session active = %v, want its context's end", err)
	}
	done := make(chan error, 1)
	go func() { done <- a.Wait(t.Context(), own) }()
	fake.AnnounceProperty(t, own, "Active", true)
	if err := waittest.For(t, done, "Wait to see the session announced active"); err != nil {
		t.Fatalf("Wait = %v", err)
	}
}

func TestActivationsKeepAnAnnouncementMadeBeforeTheWait(t *testing.T) {
	t.Parallel()
	fake, m := manager(t, logindtest.Config{Own: own})
	a := activations(t, m)
	fake.SetActive(t, own, false)
	fake.SetActive(t, own, true)
	if err := a.Wait(t.Context(), own); err != nil {
		t.Fatalf("Wait after logind announced the session active = %v", err)
	}
}

func TestActivationsGoByEachSessionsLastAnnouncement(t *testing.T) {
	t.Parallel()
	fake, m := manager(t, logindtest.Config{Own: own, Display: display})
	a := activations(t, m)
	fake.SetActive(t, own, true)
	fake.SetActive(t, own, false)
	fake.SetActive(t, display, true)
	if err := a.Wait(t.Context(), display); err != nil {
		t.Fatalf("Wait for the session announced active = %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := a.Wait(ctx, own); !errors.Is(err, context.Canceled) {
		t.Errorf("Wait for a session announced active, then in the background = %v, want its context's end", err)
	}
}

func TestActivationsReportAnAnnouncementTheyCannotRead(t *testing.T) {
	t.Parallel()
	fake, m := manager(t, logindtest.Config{Own: own})
	a := activations(t, m)
	fake.AnnounceProperty(t, own, "Active", "yes")
	err := a.Wait(t.Context(), own)
	if err == nil || !strings.Contains(err.Error(), "Active of "+string(logind.SessionPath(own))+" changed to a s") {
		t.Errorf("Wait = %v, want the Active that changed to a string", err)
	}
}

func TestStateFollowsActive(t *testing.T) {
	t.Parallel()
	fake, m := manager(t, logindtest.Config{Own: own})
	s := session(t, m, own)
	for _, c := range []struct {
		active bool
		want   logind.SessionState
	}{{true, logind.StateActive}, {false, logind.StateOnline}} {
		fake.SetActive(t, own, c.active)
		if got, err := s.State(t.Context()); err != nil || got != c.want {
			t.Errorf("State with Active %v = %q, %v; want %q", c.active, got, err, c.want)
		}
	}
}

// A change WaitLocked cannot read ends the wait with what was wrong
// with it: it discarded the failure and waited on, for a hint that
// might have been the one announced.
func TestWaitLockedReportsAChangeItCannotRead(t *testing.T) {
	t.Parallel()
	fake, m := manager(t, logindtest.Config{Display: display})
	s := session(t, m, logind.AutoSession)
	done := make(chan error, 1)
	go func() { done <- s.WaitLocked(t.Context()) }()
	var err error
	waittest.Until(t, func() bool {
		select {
		case err = <-done:
			return true
		default:
		}
		if !fake.AnnounceProperty(t, display, "LockedHint", "yes") {
			t.FailNow()
		}
		return false
	}, "WaitLocked to end on a LockedHint it cannot read")
	if err == nil || !strings.Contains(err.Error(), "LockedHint") || !strings.Contains(err.Error(), "changed to a s") {
		t.Errorf("WaitLocked = %v, want the LockedHint that changed to a string", err)
	}
}

func TestPrepareForSleepFollowsSuspend(t *testing.T) {
	t.Parallel()
	fake, m := manager(t, logindtest.Config{Display: display})
	conn := fake.Bus.Conn(t)
	if err := conn.AddMatchSignal(logind.PrepareForSleepMatch()...); err != nil {
		t.Fatal(err)
	}
	signals := make(chan *dbus.Signal, 4)
	conn.Signal(signals)
	t.Cleanup(func() { conn.RemoveSignal(signals) })
	if err := m.Power(t.Context(), logind.Suspend); err != nil {
		t.Fatal(err)
	}
	fake.PrepareForSleep(t, false)
	for _, want := range []bool{true, false} {
		got, err := logind.PrepareForSleep.Decode(waittest.For(t, signals, "PrepareForSleep"))
		if err != nil || got.Start != want {
			t.Fatalf("PrepareForSleep = %+v, %v; want start %v", got, err, want)
		}
	}
	if err := m.Power(t.Context(), logind.Suspend); err != nil {
		t.Errorf("Suspend after waking = %v", err)
	}
}
