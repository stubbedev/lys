package charge_test

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/dbusx"
	"github.com/stubbedev/lys/lib/intconv"
	"github.com/stubbedev/lys/lib/testhome"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"

	"github.com/stubbedev/lys/power/charge"
)

func TestMain(m *testing.M) { testhome.Run(m) }

func TestTheTestsReachNoHostSession(t *testing.T) { testhome.RequireNoHostSession(t) }

type handler struct {
	mu         sync.Mutex
	refuse     bool
	limits     []charge.Limit
	full       int
	departures []time.Time
	callers    []charge.Caller
}

func (h *handler) heard(c charge.Caller) error {
	h.callers = append(h.callers, c)
	if h.refuse {
		return charge.Refusal{Reason: "polkit says no"}
	}
	return nil
}

func (h *handler) SetLimit(_ context.Context, c charge.Caller, limit charge.Limit) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.heard(c); err != nil {
		return err
	}
	h.limits = append(h.limits, limit)
	return nil
}

func (h *handler) ChargeFull(_ context.Context, c charge.Caller) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.heard(c); err != nil {
		return err
	}
	h.full++
	return nil
}

func (h *handler) SetDeparture(_ context.Context, c charge.Caller, at time.Time) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if err := h.heard(c); err != nil {
		return err
	}
	h.departures = append(h.departures, at)
	return nil
}

func (h *handler) snapshot() handler {
	h.mu.Lock()
	defer h.mu.Unlock()
	return handler{limits: h.limits, full: h.full, departures: h.departures, callers: h.callers}
}

func dbusErrorName(err error) string {
	reply, ok := errors.AsType[dbus.Error](err)
	if !ok {
		return ""
	}
	return reply.Name
}

func serve(t *testing.T, h charge.Handler, initial charge.State) (*dbustest.Bus, *charge.Server) {
	t.Helper()
	bus := dbustest.Start(t)
	server, release, err := charge.Serve(bus.Conn(t), h, initial)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	return bus, server
}

func TestMethodsReachTheHandlerWithTheirCaller(t *testing.T) {
	h := &handler{}
	bus, _ := serve(t, h, charge.State{Limit: charge.DefaultLimit, Reason: charge.ReasonUnmanaged, Dock: charge.DockUnknown})
	client := charge.NewClient(bus.Conn(t))
	ctx := context.Background()

	if err := client.SetLimit(ctx, charge.Limit90); err != nil {
		t.Fatal(err)
	}
	if err := client.ChargeFull(ctx); err != nil {
		t.Fatal(err)
	}
	leave := time.Date(2026, 11, 23, 9, 0, 0, 0, time.UTC)
	if err := client.SetDeparture(ctx, leave); err != nil {
		t.Fatal(err)
	}
	if err := client.SetDeparture(ctx, time.Time{}); err != nil {
		t.Fatal(err)
	}

	got := h.snapshot()
	if len(got.limits) != 1 || got.limits[0] != charge.Limit90 || got.full != 1 {
		t.Errorf("limits %v, full %d", got.limits, got.full)
	}
	if len(got.departures) != 2 || !got.departures[0].Equal(leave) || !got.departures[1].IsZero() {
		t.Errorf("departures %v, want %v and none", got.departures, leave)
	}
	uid, pid := intconv.Must[uint32](os.Getuid()), intconv.Must[uint32](os.Getpid())
	for _, c := range got.callers {
		if c.UID != uid || c.PID != pid || c.Sender == "" {
			t.Errorf("caller %+v", c)
		}
	}
}

func TestBadArgumentsAreRefusedBeforeTheHandler(t *testing.T) {
	h := &handler{}
	bus, _ := serve(t, h, charge.State{Limit: charge.DefaultLimit, Reason: charge.ReasonUnmanaged, Dock: charge.DockUnknown})
	obj := bus.Conn(t).Object(charge.ServiceName, charge.ServicePath)
	if err := obj.Call(charge.SetLimitMethod, 0, byte(87)).Err; dbusErrorName(err) != dbusx.ErrInvalidArgs {
		t.Errorf("a limit of 87: %v", err)
	}
	if err := obj.Call(charge.SetDepartureMethod, 0, int64(-1)).Err; dbusErrorName(err) != dbusx.ErrInvalidArgs {
		t.Errorf("a departure before the epoch: %v", err)
	}
	if heard := len(h.snapshot().callers); heard != 0 {
		t.Errorf("the handler heard %d refused calls", heard)
	}
}

func TestARefusalIsNotAuthorized(t *testing.T) {
	h := &handler{refuse: true}
	bus, _ := serve(t, h, charge.State{Limit: charge.DefaultLimit, Reason: charge.ReasonUnmanaged, Dock: charge.DockUnknown})
	err := charge.NewClient(bus.Conn(t)).SetLimit(context.Background(), charge.Limit95)
	if dbusErrorName(err) != charge.ErrNotAuthorized {
		t.Errorf("a refused limit: %v", err)
	}
}

func TestTheStateCrossesTheBusAndChangesAreAnnounced(t *testing.T) {
	initial := charge.State{Limit: charge.Limit85, Reason: charge.ReasonHolding, Dock: charge.DockDocked}
	bus, server := serve(t, &handler{}, initial)
	conn := bus.Conn(t)
	client := charge.NewClient(conn)
	got, err := client.State(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != initial {
		t.Errorf("state %+v, want %+v", got, initial)
	}

	if err := conn.AddMatchSignal(dbus.WithMatchObjectPath(charge.ServicePath), dbus.WithMatchInterface(dbusx.PropertiesInterface)); err != nil {
		t.Fatal(err)
	}
	signals := make(chan *dbus.Signal, 4)
	conn.Signal(signals)

	full := time.Unix(time.Date(2026, 11, 23, 8, 30, 0, 0, time.UTC).Unix(), 0)
	next := charge.State{Limit: charge.Limit85, Reason: charge.ReasonUnplug, FullBy: full, Dock: charge.DockUndocked}
	if err := server.Publish(next); err != nil {
		t.Fatal(err)
	}
	if got := nextState(t, signals); got != next {
		t.Errorf("announced %+v, want %+v", got, next)
	}
	if err := server.Publish(next); err != nil {
		t.Fatal(err)
	}
	third := charge.State{Limit: charge.Limit85, Reason: charge.ReasonRequested, Dock: charge.DockUndocked}
	if err := server.Publish(third); err != nil {
		t.Fatal(err)
	}
	if got := nextState(t, signals); got != third {
		t.Errorf("announced %+v after an unchanged state, want only %+v", got, third)
	}
}

func nextState(t *testing.T, signals <-chan *dbus.Signal) charge.State {
	t.Helper()
	for {
		select {
		case sig := <-signals:
			st, ok, err := charge.Changed(sig)
			if err != nil {
				t.Fatal(err)
			}
			if ok {
				return st
			}
		case <-time.After(10 * time.Second):
			t.Fatal("no PropertiesChanged")
		}
	}
}

func TestLimits(t *testing.T) {
	for _, l := range charge.Limits() {
		got, err := charge.ParseLimit(uint64(l))
		if err != nil || got != l {
			t.Errorf("ParseLimit(%d) = %v, %v", l, got, err)
		}
		if l.Resume() != l.Percent()-charge.Gap {
			t.Errorf("%d resumes at %d", l, l.Resume())
		}
		if l.Holds() == (l == charge.Limit100) {
			t.Errorf("%d holds: %v", l, l.Holds())
		}
	}
	for _, bad := range []uint64{0, 50, 79, 81, 101, 255} {
		if _, err := charge.ParseLimit(bad); err == nil {
			t.Errorf("ParseLimit(%d) is no error", bad)
		}
	}
}
