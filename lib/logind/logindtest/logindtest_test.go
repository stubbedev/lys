package logindtest_test

import (
	"fmt"
	"runtime"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stubbedev/lys/lib/dbusx"
	"github.com/stubbedev/lys/lib/logind"
	"github.com/stubbedev/lys/lib/logind/logindtest"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/waittest"
)

const own logind.SessionID = "c2"

type reports struct {
	testing.TB
	mu     sync.Mutex
	errors []string
	fatal  []string
}

func (r *reports) Errorf(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}

func (r *reports) Fatalf(format string, args ...any) {
	r.mu.Lock()
	r.fatal = append(r.fatal, fmt.Sprintf(format, args...))
	r.mu.Unlock()
	runtime.Goexit()
}

func (r *reports) read() (errors, fatal []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.errors), slices.Clone(r.fatal)
}

func lock(t *testing.T, fake *logindtest.Logind) {
	t.Helper()
	s, err := logind.NewManager(fake.Bus.Conn(t)).GetSession(t.Context(), own)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Lock(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestAHookReportsAFailedAnnouncementAndGoesOn(t *testing.T) {
	fake := logindtest.Start(t, logindtest.Config{Own: own})
	r := &reports{TB: t}
	results := make(chan bool, 2)
	fake.OnLock.Store(func() {
		results <- fake.SetLockedHint(r, "absent", true)
		results <- fake.SetLockedHint(r, own, true)
	})
	t.Cleanup(func() {
		if t.Failed() {
			errs, fatal := r.read()
			t.Logf("the hook reported errors %q, fatal %q", errs, fatal)
		}
	})
	lock(t, fake)
	for i, want := range []bool{false, true} {
		if got := waittest.For(t, results, fmt.Sprintf("the hook to go on to announcement %d", i)); got != want {
			t.Errorf("announcement %d from the hook = %v, want %v", i, got, want)
		}
	}
	errs, fatal := r.read()
	if len(fatal) != 0 {
		t.Errorf("the hook's goroutine was told to stop the test: %q", fatal)
	}
	if len(errs) != 1 || !strings.Contains(errs[0], `no session "absent"`) {
		t.Errorf("reported %q, want the one session that does not exist", errs)
	}
	if got := fake.Events.Load(); !slices.Equal(got, []logindtest.Event{logindtest.EventLock, logindtest.EventLockedHint}) {
		t.Errorf("events %v, want the lock and the hint the hook set", got)
	}
}

func TestEveryAnnouncementReportsWithoutStoppingItsGoroutine(t *testing.T) {
	fake := logindtest.Start(t, logindtest.Config{Own: own})
	dev := logind.Device{Major: 13, Minor: 64}
	announcements := map[string]func(testing.TB, logind.SessionID) bool{
		"SetLockedHint": func(t testing.TB, id logind.SessionID) bool { return fake.SetLockedHint(t, id, true) },
		"SetActive":     func(t testing.TB, id logind.SessionID) bool { return fake.SetActive(t, id, false) },
		"Lock":          fake.Lock,
		"Unlock":        fake.Unlock,
		"PauseDevice": func(t testing.TB, id logind.SessionID) bool {
			return fake.PauseDevice(t, id, dev, logind.PauseGone)
		},
		"ResumeDevice": func(t testing.TB, id logind.SessionID) bool { return fake.ResumeDevice(t, id, dev) },
	}
	for name, announce := range announcements {
		r := &reports{TB: t}
		if !announce(r, own) {
			t.Errorf("%s of the session = false", name)
		}
		if errs, fatal := r.read(); len(errs) != 0 || len(fatal) != 0 {
			t.Errorf("%s of the session reported %q, %q", name, errs, fatal)
		}
		if announce(r, "absent") {
			t.Errorf("%s of a session that does not exist = true", name)
		}
		if errs, fatal := r.read(); len(errs) != 1 || len(fatal) != 0 {
			t.Errorf("%s of a session that does not exist reported errors %q, fatal %q; want one error", name, errs, fatal)
		}
	}

	fake.Bus.Stop(t)
	r := &reports{TB: t}
	waittest.Until(t, func() bool { return !fake.PrepareForSleep(r, true) }, "PrepareForSleep to fail on the stopped bus")
	if errs, fatal := r.read(); len(errs) != 1 || len(fatal) != 0 {
		t.Errorf("PrepareForSleep on a stopped bus reported errors %q, fatal %q; want one error", errs, fatal)
	}
}

func TestAHookRunningWhenTheTestEndsFinishesOnALiveBus(t *testing.T) {
	var announced atomic.Bool
	t.Run("ends with its hook running", func(t *testing.T) {
		fake := logindtest.Start(t, logindtest.Config{Own: own})
		ending := make(chan struct{})
		t.Cleanup(func() { close(ending) })
		fake.OnLock.Store(func() {
			<-ending
			announced.Store(fake.SetLockedHint(t, own, true))
		})
		lock(t, fake)
	})
	if !announced.Load() {
		t.Error("the fake did not wait for the hook that was running when its test ended")
	}
}

type lidWire struct {
	mu   sync.Mutex
	seen []string
}

func (w *lidWire) intercept(msg *dbus.Message) {
	if sender, _ := msg.Headers[dbus.FieldSender].Value().(string); sender == "org.freedesktop.DBus" {
		return
	}
	var seen string
	switch msg.Type {
	case dbus.TypeMethodReply:
		seen = fmt.Sprintf("reply %v", msg.Body[0])
	case dbus.TypeSignal:
		changed, _ := msg.Body[1].(map[string]dbus.Variant)
		seen = fmt.Sprintf("signal %v", changed["LidClosed"])
	default:
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.seen = append(w.seen, seen)
}

func (w *lidWire) read() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return slices.Clone(w.seen)
}

func TestAReadOverlappingASwitchIsAnsweredBeforeTheSignal(t *testing.T) {
	fake := logindtest.Start(t, logindtest.Config{Own: own})
	reading, release := make(chan struct{}), make(chan struct{})
	var held atomic.Bool
	logindtest.OnLidRead(fake, func() {
		if held.CompareAndSwap(false, true) {
			close(reading)
			<-release
		}
	})
	wire := &lidWire{}
	client, err := dbus.Connect(fake.Bus.Address, dbus.WithIncomingInterceptor(wire.intercept))
	if err != nil {
		t.Fatalf("connect %s: %v", fake.Bus.Address, err)
	}
	t.Cleanup(func() { _ = client.Close() })
	if err := client.AddMatchSignal(dbus.WithMatchObjectPath(logind.ManagerPath), dbus.WithMatchMember("PropertiesChanged")); err != nil {
		t.Fatal(err)
	}
	read := client.Object(logind.BusName, logind.ManagerPath).Go(dbusx.PropertiesInterface+".Get", 0, make(chan *dbus.Call, 1), logind.ManagerInterface, "LidClosed")
	waittest.For(t, reading, "the read to take the open lid")
	switched := make(chan bool, 1)
	go func() { switched <- fake.SwitchLid(t, true) }()
	waittest.Until(t, func() bool { return logindtest.LidClosed(fake) }, "the switch to close the lid")
	close(release)
	if done := waittest.For(t, read.Done, "the read's reply"); done.Err != nil {
		t.Fatalf("read: %v", done.Err)
	}
	if !waittest.For(t, switched, "the switch to be announced") {
		t.FailNow()
	}
	want := []string{"reply false", "signal true"}
	waittest.Until(t, func() bool { return len(wire.read()) >= len(want) }, "the reply and the signal")
	if got := wire.read(); !slices.Equal(got, want) {
		t.Errorf("the client received %q, want %q", got, want)
	}
}
