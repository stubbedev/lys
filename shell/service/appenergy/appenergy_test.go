package appenergy_test

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/dbusx"
	"github.com/stubbedev/lys/lib/procstat"
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
	m := appenergy.Follow(conn, func(err error) { t.Error(err) })
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
	target, ok := appenergy.Quittable(energy.App{Cgroup: firefoxScope}, 1000)
	if !ok {
		t.Fatal("the user's own app is not quittable")
	}
	if err := appenergy.Quit(context.Background(), bus.Conn(t), target); err != nil {
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
	target, ok := appenergy.Quittable(energy.App{Cgroup: firefoxScope}, 1000)
	if !ok {
		t.Fatal("the user's own app is not quittable")
	}
	if err := appenergy.Quit(context.Background(), bus.Conn(t), target); err == nil {
		t.Error("quit without a user manager")
	}
}

func TestOnlyTheUsersOwnUnitsAreQuittable(t *testing.T) {
	mine := energy.App{Cgroup: firefoxScope}
	if _, ok := appenergy.Quittable(mine, 1000); !ok {
		t.Error("the user's own app is not quittable")
	}
	if _, ok := appenergy.Quittable(mine, 1001); ok {
		t.Error("another user's app is quittable")
	}
	if _, ok := appenergy.Quittable(energy.App{Cgroup: "system.slice/docker-1.scope", Job: &energy.ProcessID{PID: 5, Start: 1}}, 0); ok {
		t.Error("a job in a system unit is quittable")
	}
}

const (
	firefoxScope = "user.slice/user-1000.slice/user@1000.service/app.slice/app-firefox-1.scope"
	tmuxScope    = "user.slice/user-1000.slice/user@1000.service/app.slice/tmux-spawn-1.scope"
	childEnv     = "APPENERGY_TEST_CHILD"
)

func TestChild(t *testing.T) {
	if os.Getenv(childEnv) == "" {
		t.Skip("the process the quit tests end")
	}
	_, _ = io.Copy(io.Discard, os.Stdin)
}

func startChild(t *testing.T) (*exec.Cmd, energy.ProcessID, io.Closer) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestChild$", "-test.count=1")
	cmd.Env = append(os.Environ(), childEnv+"=1")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		_ = stdin.Close()
	})
	st, err := procstat.Of(cmd.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	return cmd, energy.ProcessID{PID: uint32(cmd.Process.Pid), Start: st.StartTime}, stdin
}

func quitJob(t *testing.T, job energy.ProcessID) error {
	t.Helper()
	target, ok := appenergy.Quittable(energy.App{Cgroup: tmuxScope, Job: &job}, 1000)
	if !ok {
		t.Fatal("the user's own job is not quittable")
	}
	return appenergy.Quit(context.Background(), dbustest.Start(t).Conn(t), target)
}

func TestQuittingAJobTerminatesItAloneAndLeavesTheUnit(t *testing.T) {
	cmd, job, _ := startChild(t)
	if err := quitJob(t, job); err != nil {
		t.Fatal(err)
	}
	err := cmd.Wait()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.Sys().(syscall.WaitStatus).Signal() != syscall.SIGTERM {
		t.Errorf("the job ended with %v, want SIGTERM", err)
	}
}

func TestAJobWhosePIDWasReusedIsLeftAlone(t *testing.T) {
	cmd, job, stdin := startChild(t)
	job.Start++
	if err := quitJob(t, job); err != nil {
		t.Fatal(err)
	}
	if err := stdin.Close(); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Errorf("the process now holding the job's PID ended with %v, want a clean exit of its own", err)
	}
}

func TestAJobThatHasExitedIsQuitAlready(t *testing.T) {
	cmd, job, _ := startChild(t)
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	if err := quitJob(t, job); err != nil {
		t.Errorf("quitting a job that has exited: %v", err)
	}
}
