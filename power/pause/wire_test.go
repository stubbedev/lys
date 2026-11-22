package pause_test

import (
	"context"
	"errors"
	"os"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/dbusx"
	"github.com/stubbedev/lys/lib/intconv"
	"github.com/stubbedev/lys/lib/testhome"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"

	"github.com/stubbedev/lys/power/pause"
)

func TestMain(m *testing.M) { testhome.Run(m) }

func TestTheTestsReachNoHostSession(t *testing.T) { testhome.RequireNoHostSession(t) }

type handler struct {
	mu      sync.Mutex
	refuse  bool
	system  []pause.Unit
	user    []pause.Unit
	callers []pause.Caller
	resumed int
}

func (h *handler) PickSystem(_ context.Context, c pause.Caller, units []pause.Unit) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.callers = append(h.callers, c)
	if h.refuse {
		return pause.Refusal{Reason: "polkit says no"}
	}
	h.system = units
	return nil
}

func (h *handler) PickUser(_ context.Context, c pause.Caller, units []pause.Unit) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.callers = append(h.callers, c)
	h.user = units
	return nil
}

func (h *handler) Resume(_ context.Context, c pause.Caller) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.callers = append(h.callers, c)
	h.resumed++
	return nil
}

func (h *handler) seen() handler {
	h.mu.Lock()
	defer h.mu.Unlock()
	return handler{system: h.system, user: h.user, callers: slices.Clone(h.callers), resumed: h.resumed}
}

func dbusErrorName(err error) string {
	reply, ok := errors.AsType[dbus.Error](err)
	if !ok {
		return ""
	}
	return reply.Name
}

func TestPicksArriveAsUnitsOfTheirCallerAndBadOnesAreRefusedBeforeTheHandler(t *testing.T) {
	bus := dbustest.Start(t)
	h := &handler{}
	_, release, err := pause.Serve(bus.Conn(t), h, pause.State{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	client := pause.NewClient(bus.Conn(t))
	ctx := context.Background()
	uid := intconv.Must[uint32](os.Getuid())

	if err := client.PickUser(ctx, []string{"syncthing.service", "backup.timer", "syncthing.service"}); err != nil {
		t.Fatal(err)
	}
	backup, _ := pause.User(uid, "backup.timer")
	syncthing, _ := pause.User(uid, "syncthing.service")
	if got := h.seen().user; !reflect.DeepEqual(got, []pause.Unit{backup, syncthing}) {
		t.Errorf("user picks %v", got)
	}
	if c := h.seen().callers[0]; c.UID != uid || c.PID != intconv.Must[uint32](os.Getpid()) || c.Sender == "" {
		t.Errorf("caller %+v", c)
	}

	if err := client.PickSystem(ctx, []string{"docker.service"}); err != nil {
		t.Fatal(err)
	}
	docker, _ := pause.System("docker.service")
	if got := h.seen().system; !reflect.DeepEqual(got, []pause.Unit{docker}) {
		t.Errorf("system picks %v", got)
	}

	calls := len(h.seen().callers)
	for _, pick := range []func(context.Context, []string) error{client.PickSystem, client.PickUser} {
		if err := pick(ctx, []string{"dbus.service"}); dbusErrorName(err) != dbusx.ErrInvalidArgs {
			t.Errorf("an essential unit: %v", err)
		}
		if err := pick(ctx, []string{"graphical.target"}); dbusErrorName(err) != dbusx.ErrInvalidArgs {
			t.Errorf("a target: %v", err)
		}
	}
	if heard := len(h.seen().callers); heard != calls {
		t.Errorf("the handler heard %d refused picks", heard-calls)
	}

	h.mu.Lock()
	h.refuse = true
	h.mu.Unlock()
	if err := client.PickSystem(ctx, nil); dbusErrorName(err) != pause.ErrNotAuthorized {
		t.Errorf("a refusal: %v", err)
	}
	if got := h.seen().system; !reflect.DeepEqual(got, []pause.Unit{docker}) {
		t.Errorf("a refused pick changed the system picks to %v", got)
	}

	if err := client.Resume(ctx); err != nil || h.seen().resumed != 1 {
		t.Errorf("resume: %v, %d", err, h.seen().resumed)
	}
}

func TestTheStateIsReadAndEveryNewOneAnnounced(t *testing.T) {
	bus := dbustest.Start(t)
	s, release, err := pause.Serve(bus.Conn(t), &handler{}, pause.State{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	conn := bus.Conn(t)
	client := pause.NewClient(conn)
	ctx := context.Background()

	if err := conn.AddMatchSignal(dbus.WithMatchObjectPath(pause.ServicePath), dbus.WithMatchInterface(dbusx.PropertiesInterface)); err != nil {
		t.Fatal(err)
	}
	signals := make(chan *dbus.Signal, 4)
	conn.Signal(signals)

	docker, _ := pause.System("docker.service")
	timer, _ := pause.User(1000, "backup.timer")
	want := pause.State{
		Picks:   []pause.Unit{docker, timer},
		Entries: []pause.Entry{{Unit: docker, Status: pause.StatusPaused}, {Unit: timer, Status: pause.StatusPauseFailed, Err: "Unit backup.timer not loaded."}},
		Resumed: true,
	}
	if err := s.Publish(want); err != nil {
		t.Fatal(err)
	}
	if got, err := client.State(ctx); err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("read %+v, %v; want %+v", got, err, want)
	}
	select {
	case sig := <-signals:
		got, ok, err := pause.Changed(sig)
		if err != nil || !ok || !reflect.DeepEqual(got, want) {
			t.Errorf("announced %+v, %v, %v", got, ok, err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("no PropertiesChanged")
	}
}

func TestAClientReadingAsSoonAsTheNameAppearsReadsTheInitialState(t *testing.T) {
	bus := dbustest.Start(t)
	conn := bus.Conn(t)
	if err := conn.AddMatchSignal(dbus.WithMatchSender("org.freedesktop.DBus"), dbus.WithMatchMember("NameOwnerChanged"), dbus.WithMatchArg(0, pause.ServiceName)); err != nil {
		t.Fatal(err)
	}
	signals := make(chan *dbus.Signal, 4)
	conn.Signal(signals)
	client := pause.NewClient(conn)
	docker, _ := pause.System("docker.service")
	initial := pause.State{Picks: []pause.Unit{docker}, Entries: []pause.Entry{{Unit: docker, Status: pause.StatusPaused}}}
	read := make(chan pause.State, 1)
	go func() {
		for sig := range signals {
			if len(sig.Body) == 3 && sig.Body[2] != "" {
				st, err := client.State(context.Background())
				if err != nil {
					t.Error(err)
				}
				read <- st
				return
			}
		}
	}()

	_, release, err := pause.Serve(bus.Conn(t), &handler{}, initial)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	select {
	case got := <-read:
		if !reflect.DeepEqual(got, initial) {
			t.Errorf("read %+v as the name appeared, want the initial %+v", got, initial)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the name never appeared")
	}
}
