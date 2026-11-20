// Package appenergy follows lys-power's per-app energy report on the
// system bus for the shell, keeps lys-power sampling while a viewer is
// open, and stops an app's unit through the user's systemd.
package appenergy

import (
	"context"
	"fmt"
	"time"

	"github.com/stubbedev/lys/lib/dbusx"
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
func Follow(conn *dbus.Conn, failed func(error)) (*Monitor, error) {
	client := energy.NewClient(conn)
	f, err := powerprop.Follow(conn, powerprop.Source[energy.Report]{
		Name: energy.ServiceName, Path: energy.ServicePath, Interface: energy.Interface,
		Read: client.Report, Changed: energy.Changed,
	}, failed)
	if err != nil {
		return nil, err
	}
	return &Monitor{Follower: f, client: client}, nil
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

// Quittable is the unit to stop to quit app, when user uid owns it.
func Quittable(app energy.App, uid uint32) (string, bool) {
	owner, unit, ok := app.UserUnit()
	return unit, ok && owner == uid
}

// Quit stops unit through the systemd user manager on session.
func Quit(ctx context.Context, session *dbus.Conn, unit string) error {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	var job dbus.ObjectPath
	if err := session.Object(systemdName, systemdPath).CallWithContext(ctx, stopUnit, 0, unit, "replace").Store(&job); err != nil {
		return fmt.Errorf("stop %s: %w", unit, err)
	}
	return nil
}
