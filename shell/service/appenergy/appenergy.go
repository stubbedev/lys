// Package appenergy follows lys-power's per-app energy report on the
// system bus for the shell, keeps lys-power sampling while a viewer is
// open, and stops an app's unit through the user's systemd.
package appenergy

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/stubbedev/lys/desktop/feed"
	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/power/energy"
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
	client  *energy.Client
	cancel  context.CancelFunc
	watch   *dbusx.Watch
	changes feed.Feed[struct{}]

	serial sync.Mutex

	mu      sync.Mutex
	report  energy.Report
	running bool
}

// Follow starts following the report on the system bus conn; failed
// is told of each report that could not be read.
func Follow(conn *dbus.Conn, failed func(error)) (*Monitor, error) {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Monitor{client: energy.NewClient(conn), cancel: cancel}
	m.serial.Lock()
	defer m.serial.Unlock()
	watch, err := dbusx.WatchSignals(ctx, conn, dbusx.FromOwner(energy.ServiceName), failed,
		dbusx.OnOthers(dbusx.PropertiesInterface, energy.ServicePath, m.changed).WithArg0(energy.Interface),
		dbusx.OnOwnerChange(energy.ServiceName, func(_, owner string, _ dbus.Sequence) error {
			m.serial.Lock()
			defer m.serial.Unlock()
			if owner == "" {
				m.set(energy.Report{}, false)
				return nil
			}
			return m.read(ctx)
		}))
	if err != nil {
		cancel()
		return nil, fmt.Errorf("follow %s: %w", energy.ServiceName, err)
	}
	m.watch = watch
	if err := m.read(ctx); err != nil {
		failed(err)
	}
	return m, nil
}

func (m *Monitor) changed(sig *dbus.Signal) error {
	m.serial.Lock()
	defer m.serial.Unlock()
	r, ok, err := energy.Changed(sig)
	err = errors.Join(err, sig.CloseFDs())
	if err != nil || !ok {
		return err
	}
	m.set(r, true)
	return nil
}

func (m *Monitor) read(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	r, err := m.client.Report(ctx)
	if dbusx.Unowned(err) {
		m.set(energy.Report{}, false)
		return nil
	}
	if err != nil {
		return err
	}
	m.set(r, true)
	return nil
}

func (m *Monitor) set(r energy.Report, running bool) {
	m.mu.Lock()
	m.report, m.running = r, running
	m.mu.Unlock()
	m.changes.Publish(struct{}{})
}

// Report is the last report, and whether lys-power serves one.
func (m *Monitor) Report() (energy.Report, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.report, m.running
}

// Subscribe ticks whenever the report changed.
func (m *Monitor) Subscribe() (<-chan struct{}, func()) { return m.changes.Subscribe() }

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

// Close stops following.
func (m *Monitor) Close() error {
	m.cancel()
	defer m.changes.Close()
	if err := m.watch.Stop(); err != nil && !errors.Is(err, dbusx.ErrDisconnected) {
		return fmt.Errorf("stop following %s: %w", energy.ServiceName, err)
	}
	return nil
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
