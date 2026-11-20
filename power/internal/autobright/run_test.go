package autobright

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"

	"github.com/stubbedev/lys/desktop/service/powerprofiles"
	"github.com/stubbedev/lys/lib/logind"
	"github.com/stubbedev/lys/lib/logind/logindtest"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/waittest"
)

func TestDisplaySessionsResolveTheUsersDisplaySession(t *testing.T) {
	fake := logindtest.Start(t, logindtest.Config{Display: "2"})
	s := displaySessions{logind.NewManager(fake.Bus.Conn(t))}
	for range 2 {
		session, err := s.Display(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if session.Path() != logind.SessionPath("2") {
			t.Fatalf("the display session is %s, want %s", session.Path(), logind.SessionPath("2"))
		}
	}
	if got := fake.CallsTo("Manager.GetSession"); len(got) != 2 {
		t.Errorf("GetSession calls %v, want one per resolve", got)
	}
}

func TestDisplaySessionsReportASessionThatIsGone(t *testing.T) {
	fake := logindtest.Start(t, logindtest.Config{})
	s := displaySessions{logind.NewManager(fake.Bus.Conn(t))}
	session, err := s.Display(context.Background())
	if derr, ok := errors.AsType[dbus.Error](err); !ok || derr.Name != logind.ErrNoSuchSession {
		t.Fatalf("Display with no session = %v, %v; want logind's %s", session, err, logind.ErrNoSuchSession)
	}
	if session != nil {
		t.Errorf("Display with no session returned %v", session)
	}
}

type fakeDaemon struct {
	mu      sync.Mutex
	snap    powerprofiles.Snapshot
	readErr error
	ticks   chan struct{}
}

func (f *fakeDaemon) Read(context.Context) (powerprofiles.Snapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.snap, f.readErr
}

func (f *fakeDaemon) SetActive(context.Context, string) error { return errors.New("not in this test") }

func (f *fakeDaemon) Subscribe(context.Context) (<-chan struct{}, func() error, error) {
	return f.ticks, func() error { return nil }, nil
}

func (f *fakeDaemon) set(snap powerprofiles.Snapshot, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.snap, f.readErr = snap, err
}

func TestThePowerProfileIsReadAtStartAndOnEachChange(t *testing.T) {
	fake := &fakeDaemon{snap: powerprofiles.Snapshot{Available: true, Active: powerprofiles.ProfilePowerSaver}, ticks: make(chan struct{})}
	saver, err := ppdProfile{source: fake, log: slog.New(slog.NewTextHandler(io.Discard, nil))}.Saver(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for i, step := range []struct {
		snap powerprofiles.Snapshot
		want bool
	}{
		{powerprofiles.Snapshot{Available: true, Active: powerprofiles.ProfileBalanced}, false},
		{powerprofiles.Snapshot{Available: true, Active: powerprofiles.ProfilePowerSaver}, true},
		{powerprofiles.Snapshot{Active: powerprofiles.ProfilePowerSaver}, false},
		{powerprofiles.Snapshot{Available: true, Active: powerprofiles.ProfilePerformance}, false},
	} {
		if i == 0 {
			if got := waittest.For(t, saver, "the profile at start"); !got {
				t.Fatal("power-saver at start read as not power-saver")
			}
		}
		fake.set(step.snap, nil)
		fake.ticks <- struct{}{}
		if got := waittest.For(t, saver, "the changed profile"); got != step.want {
			t.Errorf("%+v read as power-saver %v, want %v", step.snap, got, step.want)
		}
	}
	close(fake.ticks)
	if _, open := <-saver; open {
		t.Error("the profile feed stayed open after the daemon's watch ended")
	}
}

func TestAnUnreadablePowerProfileEndsTheFeed(t *testing.T) {
	fake := &fakeDaemon{readErr: errors.New("org.freedesktop.DBus.Error.NoReply"), ticks: make(chan struct{})}
	saver, err := ppdProfile{source: fake, log: slog.New(slog.NewTextHandler(io.Discard, nil))}.Saver(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, open := <-saver; open {
		t.Error("an unreadable profile was reported as an answer")
	}
}
