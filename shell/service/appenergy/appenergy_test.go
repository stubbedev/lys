package appenergy_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/dbusx"
	"github.com/stubbedev/lys/lib/testhome"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/power/energy"

	"github.com/stubbedev/lys/shell/service/appenergy"
)

func TestMain(m *testing.M) { testhome.Run(m) }

func TestTheTestsReachNoHostSession(t *testing.T) { testhome.RequireNoHostSession(t) }

func follow(t *testing.T, conn *dbus.Conn) *appenergy.Monitor {
	t.Helper()
	m, err := appenergy.Follow(conn, func(err error) { t.Error(err) })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := m.Close(); err != nil {
			t.Error(err)
		}
	})
	return m
}

func until(t *testing.T, m *appenergy.Monitor, what string, ok func(energy.Report, bool) bool) {
	t.Helper()
	ticks, stop := m.Subscribe()
	defer stop()
	for !ok(m.Report()) {
		waittest.For(t, ticks, what)
	}
}

func TestTheReportFollowsTheServiceThroughRestarts(t *testing.T) {
	bus := dbustest.Start(t)
	m := follow(t, bus.Conn(t))
	if _, running := m.Report(); running {
		t.Error("running before lys-power is on the bus")
	}

	serverConn := bus.Conn(t)
	s, release, err := energy.Serve(serverConn, func() {})
	if err != nil {
		t.Fatal(err)
	}
	until(t, m, "lys-power on the bus", func(r energy.Report, running bool) bool { return running && !r.Measured() })

	want := energy.Report{Covered: time.Minute, Source: energy.SourceBattery, Watts: 9, Apps: []energy.App{{Cgroup: "app.slice/a.scope", Command: "a", PIDs: []uint32{3}, Impact: 1, Watts: 9}}}
	if err := s.Publish(want); err != nil {
		t.Fatal(err)
	}
	until(t, m, "the report", func(r energy.Report, _ bool) bool { return r.Measured() && r.Apps[0].Command == "a" })

	release()
	until(t, m, "lys-power leaving the bus", func(_ energy.Report, running bool) bool { return !running })
}

func TestAnOpenViewerKeepsTheLeaseUntilItCloses(t *testing.T) {
	bus := dbustest.Start(t)
	watched := make(chan struct{}, 4)
	_, release, err := energy.Serve(bus.Conn(t), func() { watched <- struct{}{} })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	m := follow(t, bus.Conn(t))
	life, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		m.Watch(life, func(err error) { t.Error(err) })
	}()
	waittest.For(t, watched, "the first watch")
	cancel()
	waittest.For(t, done, "the viewer to stop watching")
}

func TestWatchingWithoutTheServiceIsQuiet(t *testing.T) {
	bus := dbustest.Start(t)
	m := follow(t, bus.Conn(t))
	life, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		m.Watch(life, func(err error) { t.Errorf("watching an absent service failed loudly: %v", err) })
	}()
	cancel()
	waittest.For(t, done, "the viewer to stop watching")
}

type systemd struct {
	mu      sync.Mutex
	stopped []string
}

func (s *systemd) StopUnit(name, mode string) (dbus.ObjectPath, *dbus.Error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if mode != "replace" {
		return "", dbusx.InvalidArgs("mode " + mode)
	}
	s.stopped = append(s.stopped, name)
	return "/org/freedesktop/systemd1/job/1", nil
}

func TestQuitStopsTheUnitThroughTheUsersSystemd(t *testing.T) {
	bus := dbustest.Start(t)
	sd := &systemd{}
	release, err := dbusx.Serve(bus.Conn(t), dbusx.Service{Name: "org.freedesktop.systemd1", Path: "/org/freedesktop/systemd1", Interface: "org.freedesktop.systemd1.Manager", Methods: sd})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	if err := appenergy.Quit(context.Background(), bus.Conn(t), "app-firefox-1.scope"); err != nil {
		t.Fatal(err)
	}
	sd.mu.Lock()
	defer sd.mu.Unlock()
	if len(sd.stopped) != 1 || sd.stopped[0] != "app-firefox-1.scope" {
		t.Errorf("stopped %q", sd.stopped)
	}
}

func TestQuitWithoutSystemdFails(t *testing.T) {
	bus := dbustest.Start(t)
	if err := appenergy.Quit(context.Background(), bus.Conn(t), "a.scope"); err == nil {
		t.Error("quit without a user manager")
	}
}

func TestOnlyTheUsersOwnUnitsAreQuittable(t *testing.T) {
	mine := energy.App{Cgroup: "user.slice/user-1000.slice/user@1000.service/app.slice/app-foot-3.scope"}
	if unit, ok := appenergy.Quittable(mine, 1000); !ok || unit != "app-foot-3.scope" {
		t.Errorf("own app: %q %v", unit, ok)
	}
	if _, ok := appenergy.Quittable(mine, 1001); ok {
		t.Error("another user's app is quittable")
	}
	if _, ok := appenergy.Quittable(energy.App{Cgroup: "system.slice/docker-1.scope"}, 0); ok {
		t.Error("a system unit is quittable")
	}
}
