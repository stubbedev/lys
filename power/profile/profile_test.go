package profile_test

import (
	"context"
	"errors"
	"os"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/dbusx"
	"github.com/stubbedev/lys/lib/intconv"
	"github.com/stubbedev/lys/lib/testhome"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"

	"github.com/stubbedev/lys/power/profile"
)

func TestMain(m *testing.M) { testhome.Run(m) }

func TestTheTestsReachNoHostSession(t *testing.T) { testhome.RequireNoHostSession(t) }

type handler struct {
	mu      sync.Mutex
	refuse  bool
	fail    error
	set     []profile.Profile
	callers []profile.Caller
}

func (h *handler) SetActive(_ context.Context, c profile.Caller, p profile.Profile) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.callers = append(h.callers, c)
	if h.refuse {
		return profile.Refusal{Reason: "polkit says no"}
	}
	if h.fail != nil {
		return h.fail
	}
	h.set = append(h.set, p)
	return nil
}

func (h *handler) heard() ([]profile.Profile, []profile.Caller) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.set), slices.Clone(h.callers)
}

func dbusErrorName(err error) string {
	reply, ok := errors.AsType[dbus.Error](err)
	if !ok {
		return ""
	}
	return reply.Name
}

func serve(t *testing.T, h profile.Handler, initial profile.Profile) (*dbustest.Bus, *profile.Server) {
	t.Helper()
	bus := dbustest.Start(t)
	server, release, err := profile.Serve(bus.Conn(t), h, initial)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	return bus, server
}

func TestParseTakesOnlyTheThreeProfiles(t *testing.T) {
	t.Parallel()
	for _, p := range profile.All() {
		if got, err := profile.Parse(string(p)); err != nil || got != p {
			t.Errorf("Parse(%q) = %q, %v", p, got, err)
		}
	}
	for _, name := range []string{"", "turbo", "Balanced"} {
		if _, err := profile.Parse(name); err == nil {
			t.Errorf("Parse(%q) took it", name)
		}
	}
}

func TestSetActiveReachesTheHandlerWithItsCaller(t *testing.T) {
	h := &handler{}
	bus, _ := serve(t, h, profile.Balanced)
	if err := profile.NewClient(bus.Conn(t)).SetActive(t.Context(), profile.PowerSaver); err != nil {
		t.Fatal(err)
	}
	set, callers := h.heard()
	if !slices.Equal(set, []profile.Profile{profile.PowerSaver}) {
		t.Errorf("set %v", set)
	}
	uid, pid := intconv.Must[uint32](os.Getuid()), intconv.Must[uint32](os.Getpid())
	if len(callers) != 1 || callers[0].UID != uid || callers[0].PID != pid {
		t.Errorf("callers %+v", callers)
	}
}

func TestAnUnknownProfileIsRefusedBeforeTheHandler(t *testing.T) {
	h := &handler{}
	bus, _ := serve(t, h, profile.Balanced)
	err := bus.Conn(t).Object(profile.ServiceName, profile.ServicePath).Call(profile.SetActiveMethod, 0, "turbo").Err
	if dbusErrorName(err) != dbusx.ErrInvalidArgs {
		t.Errorf("turbo: %v", err)
	}
	if _, callers := h.heard(); len(callers) != 0 {
		t.Errorf("the handler heard %d refused calls", len(callers))
	}
}

func TestARefusalIsNotAuthorizedAndAFailureFails(t *testing.T) {
	refusing := &handler{refuse: true}
	bus, _ := serve(t, refusing, profile.Balanced)
	if err := profile.NewClient(bus.Conn(t)).SetActive(t.Context(), profile.Performance); dbusErrorName(err) != profile.ErrNotAuthorized {
		t.Errorf("a refused switch: %v", err)
	}
	failing := &handler{fail: errors.New("write platform_profile: device busy")}
	bus, _ = serve(t, failing, profile.Balanced)
	err := profile.NewClient(bus.Conn(t)).SetActive(t.Context(), profile.Performance)
	if dbusErrorName(err) != dbusx.ErrFailed {
		t.Errorf("a failed switch: %v", err)
	}
}

func TestTheActiveProfileCrossesTheBusAndChangesAreAnnounced(t *testing.T) {
	bus, server := serve(t, &handler{}, profile.Performance)
	conn := bus.Conn(t)
	if got, err := profile.NewClient(conn).Active(t.Context()); err != nil || got != profile.Performance {
		t.Fatalf("Active = %q, %v", got, err)
	}
	if err := conn.AddMatchSignal(dbus.WithMatchObjectPath(profile.ServicePath), dbus.WithMatchInterface(dbusx.PropertiesInterface)); err != nil {
		t.Fatal(err)
	}
	signals := make(chan *dbus.Signal, 4)
	conn.Signal(signals)
	for _, p := range []profile.Profile{profile.PowerSaver, profile.PowerSaver, profile.Balanced} {
		if err := server.Publish(p); err != nil {
			t.Fatal(err)
		}
	}
	for _, want := range []profile.Profile{profile.PowerSaver, profile.Balanced} {
		if got := nextProfile(t, signals); got != want {
			t.Errorf("announced %q, want %q: an unchanged profile is not announced", got, want)
		}
	}
}

func nextProfile(t *testing.T, signals <-chan *dbus.Signal) profile.Profile {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case sig := <-signals:
			p, ok, err := profile.Changed(sig)
			if err != nil {
				t.Fatal(err)
			}
			if ok {
				return p
			}
		case <-timeout:
			t.Fatal("no PropertiesChanged")
		}
	}
}
