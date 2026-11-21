package energy_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/waittest"

	"github.com/stubbedev/lys/power/energy"
)

func serve(t *testing.T, bus *dbustest.Bus, watch func()) *energy.Server {
	t.Helper()
	s, release, err := energy.Serve(bus.Conn(t), watch)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	return s
}

func signals(t *testing.T, conn *dbus.Conn) <-chan *dbus.Signal {
	t.Helper()
	if err := conn.AddMatchSignal(
		dbus.WithMatchObjectPath(energy.ServicePath),
		dbus.WithMatchInterface(dbusx.PropertiesInterface),
		dbus.WithMatchMember("PropertiesChanged"),
	); err != nil {
		t.Fatal(err)
	}
	ch := make(chan *dbus.Signal, 8)
	conn.Signal(ch)
	return ch
}

func TestTheReportIsReadAndEveryNewOneAnnounced(t *testing.T) {
	bus := dbustest.Start(t)
	s := serve(t, bus, func() {})
	conn := bus.Conn(t)
	client := energy.NewClient(conn)
	ch := signals(t, conn)
	ctx := context.Background()

	got, err := client.Report(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Measured() || got.Source != energy.SourceNone || len(got.Apps) != 0 {
		t.Errorf("before any sample the report is %+v, want unmeasured", got)
	}

	want := energy.Report{Covered: 90 * time.Second, Source: energy.SourcePackage, Watts: 12.5, Apps: []energy.App{
		{Cgroup: firefox, Command: "firefox", PIDs: []uint32{10, 11}, CPU: 0.4, Wakeups: 300, GPU: 0.1, Impact: 0.56, Watts: 4},
		{Cgroup: tmux, Command: "tmux: server", PIDs: []uint32{20}, CPU: 0.1, Impact: 0.1, Watts: 1},
	}, Panels: []energy.Panel{
		{Device: "0000:00:02.0", Mode: energy.SelfRefreshPanelReplaySU, Readings: 45, Residency: 0.9},
		{Device: "0000:03:00.0", Mode: energy.SelfRefreshUnsupported, Readings: 45},
	}}
	if err := s.Publish(want); err != nil {
		t.Fatal(err)
	}
	for {
		r, ok, err := energy.Changed(waittest.For(t, ch, "a Report change"))
		if err != nil {
			t.Fatal(err)
		}
		if !ok {
			continue
		}
		if !reflect.DeepEqual(r, want) {
			t.Errorf("announced %+v, want %+v", r, want)
		}
		break
	}
	if got, err := client.Report(ctx); err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("Report = %+v, %v, want %+v", got, err, want)
	}
}

func TestThreadsStayOffTheBus(t *testing.T) {
	bus := dbustest.Start(t)
	s := serve(t, bus, func() {})
	r := energy.Report{Covered: time.Second, Source: energy.SourceNone, Apps: []energy.App{
		{Cgroup: firefox, PIDs: []uint32{}, Impact: 1, Threads: []energy.ThreadRate{{PID: 1, TID: 1, Wakeups: 3}}},
	}}
	if err := s.Publish(r); err != nil {
		t.Fatal(err)
	}
	got, err := energy.NewClient(bus.Conn(t)).Report(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Apps) != 1 || got.Apps[0].Threads != nil {
		t.Errorf("read %+v, want the app without its threads", got.Apps)
	}
}

func TestPublishRefusesAnInvalidSource(t *testing.T) {
	bus := dbustest.Start(t)
	s := serve(t, bus, func() {})
	if err := s.Publish(energy.Report{}); err == nil {
		t.Error("a report without a source published")
	}
}

func TestPublishRefusesAPanelWithoutAMode(t *testing.T) {
	bus := dbustest.Start(t)
	s := serve(t, bus, func() {})
	if err := s.Publish(energy.Report{Source: energy.SourceNone, Panels: []energy.Panel{{Device: "0"}}}); err == nil {
		t.Error("a panel without a self-refresh mode published")
	}
}

func TestWatchReachesTheService(t *testing.T) {
	bus := dbustest.Start(t)
	watched := make(chan struct{}, 1)
	serve(t, bus, func() { watched <- struct{}{} })
	if err := energy.NewClient(bus.Conn(t)).Watch(context.Background()); err != nil {
		t.Fatal(err)
	}
	waittest.For(t, watched, "the watch")
}

func TestASecondServiceFindsTheNameTaken(t *testing.T) {
	bus := dbustest.Start(t)
	serve(t, bus, func() {})
	if _, _, err := energy.Serve(bus.Conn(t), func() {}); !errors.Is(err, dbusx.ErrNameTaken) {
		t.Errorf("a second service: %v, want ErrNameTaken", err)
	}
}

func TestWithoutTheServiceReadingFails(t *testing.T) {
	bus := dbustest.Start(t)
	if _, err := energy.NewClient(bus.Conn(t)).Report(context.Background()); err == nil {
		t.Error("a report read with nobody serving it")
	}
}
