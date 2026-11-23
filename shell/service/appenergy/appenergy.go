// Package appenergy follows lys-power's per-app energy report on the
// system bus for the shell, keeps lys-power sampling while a viewer is
// open, and quits an app: the one job in its unit that uses the energy,
// or else the unit itself through the user's systemd.
package appenergy

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"syscall"
	"time"

	"github.com/stubbedev/lys/lib/dbusx"
	"github.com/stubbedev/lys/lib/procstat"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/power/energy"

	"github.com/stubbedev/lys/shell/service/powerprop"
)

const callTimeout = 5 * time.Second

const (
	systemdName  = "org.freedesktop.systemd1"
	systemdPath  = dbus.ObjectPath("/org/freedesktop/systemd1")
	stopUnit     = "org.freedesktop.systemd1.Manager.StopUnit"
	renewalShare = 2
)

// Monitor is lys-power's energy report as last read or announced.
type Monitor struct {
	*powerprop.Follower[energy.Report]
	client *energy.Client
}

// Follow starts following the report on the system bus conn; failed
// is told of each report that could not be read.
func Follow(conn *dbus.Conn, failed func(error)) *Monitor {
	client := energy.NewClient(conn)
	f := powerprop.Follow(conn, powerprop.Source[energy.Report]{
		Name: energy.ServiceName, Path: energy.ServicePath, Interface: energy.Interface,
		Read: client.Report, Changed: energy.Changed,
	}, failed)
	return &Monitor{Follower: f, client: client}
}

// Report is the last report, and whether lys-power serves one.
func (m *Monitor) Report() (energy.Report, bool) { return m.Value() }

// Watch keeps lys-power sampling quickly until life ends.
func (m *Monitor) Watch(life context.Context, failed func(error)) {
	renew := time.NewTicker(energy.WatchLease / renewalShare)
	defer renew.Stop()
	for {
		ctx, cancel := context.WithTimeout(life, callTimeout)
		err := m.client.Watch(ctx)
		cancel()
		if err != nil && life.Err() == nil && !dbusx.Unowned(err) {
			failed(err)
		}
		select {
		case <-life.Done():
			return
		case <-renew.C:
		}
	}
}

// Target is what quitting an app ends: one job in its unit, or the
// whole unit.
type Target struct {
	unit   string
	job    energy.ProcessID
	hasJob bool
}

// Quittable is what to end to quit app, when user uid owns its unit.
func Quittable(app energy.App, uid uint32) (Target, bool) {
	owner, unit, ok := app.UserUnit()
	if !ok || owner != uid {
		return Target{}, false
	}
	if app.Job == nil {
		return Target{unit: unit}, true
	}
	return Target{unit: unit, job: *app.Job, hasJob: true}, true
}

// Quit ends t: it terminates its job, or stops its unit through the
// systemd user manager on session.
func Quit(ctx context.Context, session *dbus.Conn, t Target) error {
	if t.hasJob {
		return terminate(t.job)
	}
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	var job dbus.ObjectPath
	if err := session.Object(systemdName, systemdPath).CallWithContext(ctx, stopUnit, 0, t.unit, "replace").Store(&job); err != nil {
		return fmt.Errorf("stop %s: %w", t.unit, err)
	}
	return nil
}

func terminate(id energy.ProcessID) error {
	pid := int(id.PID)
	p, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("find process %d: %w", pid, err)
	}
	defer func() { _ = p.Release() }()
	st, err := procstat.Of(pid)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("terminate %d: %w", pid, err)
	}
	if st.StartTime != id.Start {
		return nil
	}
	if err := p.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return fmt.Errorf("terminate %d: %w", pid, err)
	}
	return nil
}
