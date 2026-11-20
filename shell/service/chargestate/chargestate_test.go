package chargestate_test

import (
	"context"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/fakeclock"
	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/testhome"
	"github.com/stubbedev/lys/power/charge"

	"github.com/stubbedev/lys/shell/service/chargestate"
)

func TestMain(m *testing.M) { testhome.Run(m) }

func TestTheTestsReachNoHostSession(t *testing.T) { testhome.RequireNoHostSession(t) }

type handler struct {
	departures chan time.Time
	full       chan struct{}
}

func (h *handler) SetLimit(context.Context, charge.Caller, charge.Limit) error { return nil }

func (h *handler) ChargeFull(context.Context, charge.Caller) error {
	h.full <- struct{}{}
	return nil
}

func (h *handler) SetDeparture(_ context.Context, _ charge.Caller, at time.Time) error {
	h.departures <- at
	return nil
}

var held = charge.State{Limit: charge.Limit80, Reason: charge.ReasonHolding, Dock: charge.DockDocked}

func next(t *testing.T, h *handler) time.Time {
	t.Helper()
	select {
	case at := <-h.departures:
		return at
	case <-time.After(10 * time.Second):
		t.Fatal("no departure passed")
		return time.Time{}
	}
}

func TestDeparturesArePassedOnceEachAndAgainWhenLysPowerReturns(t *testing.T) {
	bus := dbustest.Start(t)
	h := &handler{departures: make(chan time.Time, 8), full: make(chan struct{}, 1)}
	server, release, err := charge.Serve(bus.Conn(t), h, held)
	if err != nil {
		t.Fatal(err)
	}
	m := chargestate.Follow(bus.Conn(t), func(err error) { t.Errorf("following: %v", err) })
	t.Cleanup(func() { _ = m.Close() })

	start := time.Date(2026, 11, 23, 7, 0, 0, 0, time.UTC)
	meeting, flight := start.Add(time.Hour), start.Add(3*time.Hour)
	clk := fakeclock.At(start)
	changes := make(chan struct{}, 1)
	life, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		m.PassDepartures(life, clk, func(now time.Time) time.Time {
			switch {
			case now.Before(meeting):
				return meeting
			case now.Before(flight):
				return flight
			}
			return time.Time{}
		}, changes, func(err error) { t.Errorf("passing: %v", err) })
	}()
	t.Cleanup(func() { cancel(); <-done })

	if got := next(t, h); !got.Equal(meeting) {
		t.Fatalf("passed %v, want the meeting %v", got, meeting)
	}
	changes <- struct{}{}
	if err := server.Publish(charge.State{Limit: charge.Limit80, Reason: charge.ReasonEvent, FullBy: meeting, Dock: charge.DockDocked}); err != nil {
		t.Fatal(err)
	}
	clk.Advance(time.Hour)
	if got := next(t, h); !got.Equal(flight) {
		t.Fatalf("passed %v after the meeting began, want the flight %v; an unchanged departure was passed again", got, flight)
	}

	release()
	if _, release, err = charge.Serve(bus.Conn(t), h, held); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	if got := next(t, h); !got.Equal(flight) {
		t.Fatalf("passed %v when lys-power came back, want the flight %v", got, flight)
	}

	clk.Advance(2 * time.Hour)
	if got := next(t, h); !got.IsZero() {
		t.Fatalf("passed %v with nothing ahead, want none", got)
	}

	if err := m.ChargeFull(context.Background()); err != nil {
		t.Fatal(err)
	}
	<-h.full
}
