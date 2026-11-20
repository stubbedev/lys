// Package chargestate follows lys-power's charge policy for the shell,
// asks it for a full charge, and passes it the start of the next
// calendar event away from the desk: only the time leaves the calendar.
package chargestate

import (
	"context"
	"time"

	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/power/charge"

	"github.com/stubbedev/lys/shell/internal/clock"
	"github.com/stubbedev/lys/shell/service/powerprop"
)

const callTimeout = 5 * time.Second

// Monitor is the charge policy as last read or announced.
type Monitor struct {
	*powerprop.Follower[charge.State]
	client *charge.Client
}

// Follow starts following the policy on the system bus conn; failed is
// told of each state that could not be read.
func Follow(conn *dbus.Conn, failed func(error)) *Monitor {
	client := charge.NewClient(conn)
	f := powerprop.Follow(conn, powerprop.Source[charge.State]{
		Name: charge.ServiceName, Path: charge.ServicePath, Interface: charge.Interface,
		Read: client.State, Changed: charge.Changed,
	}, failed)
	return &Monitor{Follower: f, client: client}
}

// ChargeFull asks for a full charge until the next unplug.
func (m *Monitor) ChargeFull(ctx context.Context) error { return m.client.ChargeFull(ctx) }

// PassDepartures keeps lys-power told of next(now), the next departure or
// the zero time, until life ends: again whenever changes ticks, lys-power
// comes onto the bus, or the departure passed lys-power arrives. A
// departure is passed once while lys-power stays on the bus; failed is
// told of one it would not take.
func (m *Monitor) PassDepartures(life context.Context, clk clock.Clock, next func(now time.Time) time.Time, changes <-chan struct{}, failed func(error)) {
	ticks, stop := m.Subscribe()
	defer stop()
	arrived := make(chan struct{}, 1)
	var (
		sent     time.Time
		told     bool
		stopWait = func() bool { return false }
	)
	defer func() { stopWait() }()
	for {
		at := next(clk.Now())
		_, running := m.Value()
		switch {
		case !running:
			told = false
		case !told || !at.Equal(sent):
			ctx, cancel := context.WithTimeout(life, callTimeout)
			err := m.client.SetDeparture(ctx, at)
			cancel()
			if err != nil && life.Err() == nil {
				failed(err)
			}
			sent, told = at, true
		}
		stopWait()
		stopWait = func() bool { return false }
		if !at.IsZero() {
			stopWait = clk.AfterFunc(at.Sub(clk.Now()), func() {
				select {
				case arrived <- struct{}{}:
				default:
				}
			})
		}
		select {
		case <-life.Done():
			return
		case <-changes:
		case <-ticks:
		case <-arrived:
		}
	}
}
