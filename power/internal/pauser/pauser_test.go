package pauser_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/dbusx"
	"github.com/stubbedev/lys/lib/intconv"
	"github.com/stubbedev/lys/lib/polkit"
	"github.com/stubbedev/lys/lib/polkit/polkittest"
	"github.com/stubbedev/lys/lib/systemd1"
	"github.com/stubbedev/lys/lib/testhome"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"

	"github.com/stubbedev/lys/power/internal/pauser"
	"github.com/stubbedev/lys/power/pause"
)

func TestMain(m *testing.M) { testhome.Run(m) }

func TestTheTestsReachNoHostSession(t *testing.T) { testhome.RequireNoHostSession(t) }

type fakeUnit struct {
	active, freezer string
}

type fakeSystemd struct {
	mu    sync.Mutex
	units map[string]*fakeUnit
	paths map[string]dbus.ObjectPath
	fail  map[string]string
	calls []string
}

func startSystemd(t *testing.T, conn *dbus.Conn, units map[string]string) *fakeSystemd {
	t.Helper()
	f := &fakeSystemd{units: map[string]*fakeUnit{}, paths: map[string]dbus.ObjectPath{}, fail: map[string]string{}}
	names := make([]string, 0, len(units))
	for name := range units {
		names = append(names, name)
	}
	slices.Sort(names)
	for i, name := range names {
		f.units[name] = &fakeUnit{active: units[name], freezer: "running"}
		path := dbus.ObjectPath("/org/freedesktop/systemd1/unit/u" + strconv.Itoa(i))
		f.paths[name] = path
		unexport, err := dbusx.Export(conn, path, dbusx.Interface{Name: systemd1.UnitInterface, Properties: dbusx.Getters{
			"ActiveState":  func() any { return f.state(name).active },
			"FreezerState": func() any { return f.state(name).freezer },
		}})
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(unexport)
	}
	release, err := dbusx.Serve(conn, dbusx.Service{Name: systemd1.BusName, Path: systemd1.Path, Interface: systemd1.ManagerInterface, Methods: manager{f}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	return f
}

func (f *fakeSystemd) state(name string) fakeUnit {
	f.mu.Lock()
	defer f.mu.Unlock()
	return *f.units[name]
}

func (f *fakeSystemd) restart(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.units[name] = &fakeUnit{active: "active", freezer: "running"}
}

func (f *fakeSystemd) failNext(method, name, msg string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fail[method+" "+name] = msg
}

func (f *fakeSystemd) took() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.calls
	f.calls = nil
	return out
}

type manager struct{ f *fakeSystemd }

func (m manager) GetUnit(name string) (dbus.ObjectPath, *dbus.Error) {
	m.f.mu.Lock()
	defer m.f.mu.Unlock()
	path, ok := m.f.paths[name]
	if !ok {
		return "", dbus.NewError(systemd1.ErrNoSuchUnit, []any{"Unit " + name + " not loaded."})
	}
	return path, nil
}

func (m manager) act(method, name string, change func(*fakeUnit)) *dbus.Error {
	m.f.mu.Lock()
	defer m.f.mu.Unlock()
	m.f.calls = append(m.f.calls, method+" "+name)
	if msg, ok := m.f.fail[method+" "+name]; ok {
		delete(m.f.fail, method+" "+name)
		return dbus.NewError("org.freedesktop.DBus.Error.Failed", []any{msg})
	}
	u, ok := m.f.units[name]
	if !ok {
		return dbus.NewError(systemd1.ErrNoSuchUnit, []any{"Unit " + name + " not loaded."})
	}
	change(u)
	return nil
}

func (m manager) FreezeUnit(name string) *dbus.Error {
	return m.act("FreezeUnit", name, func(u *fakeUnit) { u.freezer = "frozen" })
}

func (m manager) ThawUnit(name string) *dbus.Error {
	return m.act("ThawUnit", name, func(u *fakeUnit) { u.freezer = "running" })
}

func (m manager) StartUnit(name, _ string) (dbus.ObjectPath, *dbus.Error) {
	return "/org/freedesktop/systemd1/job/1", m.act("StartUnit", name, func(u *fakeUnit) { u.active = "active" })
}

func (m manager) StopUnit(name, _ string) (dbus.ObjectPath, *dbus.Error) {
	return "/org/freedesktop/systemd1/job/2", m.act("StopUnit", name, func(u *fakeUnit) { u.active = "inactive" })
}

const user = 1000

type managers struct {
	system *dbus.Conn
	users  map[uint32]*dbus.Conn
}

func (m managers) System() pauser.Manager { return pauser.Systemd{Conn: m.system} }

func (m managers) User(_ context.Context, uid uint32) (pauser.Manager, func() error, error) {
	conn, ok := m.users[uid]
	if !ok {
		return nil, nil, pauser.ErrNoManager
	}
	return pauser.Systemd{Conn: conn}, func() error { return nil }, nil
}

type machine struct {
	system, user *fakeSystemd
	managers     managers
	state        string
}

func newMachine(t *testing.T) machine {
	t.Helper()
	systemBus, userBus := dbustest.Start(t), dbustest.Start(t)
	m := machine{
		system: startSystemd(t, systemBus.Conn(t), map[string]string{
			"docker.service": "active", "mysqld.service": "active", "nix-gc.timer": "active", "qdrant.service": "inactive",
		}),
		user:  startSystemd(t, userBus.Conn(t), map[string]string{"syncthing.service": "active"}),
		state: filepath.Join(t.TempDir(), "pause"),
	}
	m.managers = managers{system: systemBus.Conn(t), users: map[uint32]*dbus.Conn{user: userBus.Conn(t)}}
	return m
}

func quiet() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func refuseAll(context.Context, pause.Caller) error { return pause.Refusal{Reason: "no"} }

func (m machine) load(t *testing.T) *pauser.Pauser {
	t.Helper()
	p, err := pauser.Load(m.state, m.managers, refuseAll, quiet())
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func root() pause.Caller { return pause.Caller{UID: 0} }

func units(t *testing.T, parse func(string) (pause.Unit, error), names ...string) []pause.Unit {
	t.Helper()
	out := make([]pause.Unit, 0, len(names))
	for _, n := range names {
		u, err := parse(n)
		if err != nil {
			t.Fatal(err)
		}
		out = append(out, u)
	}
	return pause.Sorted(out)
}

func userUnit(name string) (pause.Unit, error) { return pause.User(user, name) }

func pick(t *testing.T, p *pauser.Pauser) {
	t.Helper()
	ctx := context.Background()
	if err := p.PickSystem(ctx, root(), units(t, pause.System, "docker.service", "nix-gc.timer", "qdrant.service")); err != nil {
		t.Fatal(err)
	}
	if err := p.PickUser(ctx, pause.Caller{UID: user}, units(t, userUnit, "syncthing.service")); err != nil {
		t.Fatal(err)
	}
}

func apply(t *testing.T, p *pauser.Pauser, onBattery, wantFailing bool) {
	t.Helper()
	failing, err := p.Apply(context.Background(), onBattery)
	if err != nil {
		t.Fatal(err)
	}
	if failing != wantFailing {
		t.Errorf("failing %v, want %v", failing, wantFailing)
	}
}

func expectCalls(t *testing.T, f *fakeSystemd, want ...string) {
	t.Helper()
	if got := f.took(); !slices.Equal(got, want) {
		t.Errorf("calls %q, want %q", got, want)
	}
}

func served(t *testing.T, p *pauser.Pauser) *pause.Client {
	t.Helper()
	bus := dbustest.Start(t)
	release, err := p.Serve(bus.Conn(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	return pause.NewClient(bus.Conn(t))
}

func TestPickedUnitsFreezeOnBatteryAndThawOnACAndUnpickedOnesAreUntouched(t *testing.T) {
	m := newMachine(t)
	p := m.load(t)
	client := served(t, p)
	pick(t, p)

	apply(t, p, false, false)
	expectCalls(t, m.system)
	expectCalls(t, m.user)

	apply(t, p, true, false)
	expectCalls(t, m.system, "FreezeUnit docker.service", "StopUnit nix-gc.timer")
	expectCalls(t, m.user, "FreezeUnit syncthing.service")
	if m.system.state("mysqld.service").freezer != "running" {
		t.Error("an unpicked unit froze")
	}
	st, err := client.State(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var paused []string
	for _, e := range st.Entries {
		if e.Status != pause.StatusPaused || e.Err != "" {
			t.Errorf("entry %+v", e)
		}
		paused = append(paused, e.Unit.String())
	}
	if want := []string{"system/docker.service", "system/nix-gc.timer", "user/1000/syncthing.service"}; !slices.Equal(paused, want) {
		t.Errorf("paused %q, want %q: an inactive pick is not paused", paused, want)
	}

	apply(t, p, true, false)
	expectCalls(t, m.system)
	expectCalls(t, m.user)

	apply(t, p, false, false)
	expectCalls(t, m.system, "ThawUnit docker.service", "StartUnit nix-gc.timer")
	expectCalls(t, m.user, "ThawUnit syncthing.service")
	if st, _ := client.State(context.Background()); len(st.Entries) != 0 {
		t.Errorf("on AC still %+v", st.Entries)
	}
	for _, name := range []string{"docker.service", "mysqld.service"} {
		if s := m.system.state(name); s.freezer != "running" || s.active != "active" {
			t.Errorf("%s ends %+v", name, s)
		}
	}
}

func TestAFailedFreezeIsReportedWithTheUnitAndSystemdsError(t *testing.T) {
	m := newMachine(t)
	p := m.load(t)
	client := served(t, p)
	pick(t, p)
	m.system.failNext("FreezeUnit", "docker.service", "Unit docker.service does not support freezing.")

	apply(t, p, true, true)
	st, err := client.State(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(st.Entries, func(e pause.Entry) bool { return e.Unit.Name() == "docker.service" })
	if i < 0 {
		t.Fatalf("no entry for docker.service in %+v", st.Entries)
	}
	e := st.Entries[i]
	if e.Status != pause.StatusPauseFailed || !strings.Contains(e.Err, "docker.service") || !strings.Contains(e.Err, "does not support freezing") {
		t.Errorf("%+v", e)
	}
	if slices.Contains(st.Paused(), e.Unit) {
		t.Error("a unit that did not freeze counts as paused")
	}

	m.system.took()
	apply(t, p, true, false)
	expectCalls(t, m.system, "FreezeUnit docker.service")
}

func TestRestartingOnBatteryNeitherFreezesTwiceNorLeavesUnitsFrozenAfterAC(t *testing.T) {
	m := newMachine(t)
	first := m.load(t)
	pick(t, first)
	apply(t, first, true, false)
	m.system.took()
	m.user.took()

	second := m.load(t)
	apply(t, second, true, false)
	expectCalls(t, m.system)
	expectCalls(t, m.user)

	apply(t, second, false, false)
	expectCalls(t, m.system, "ThawUnit docker.service", "StartUnit nix-gc.timer")
	expectCalls(t, m.user, "ThawUnit syncthing.service")
}

func TestAFailedThawStaysShownAndIsRetried(t *testing.T) {
	m := newMachine(t)
	p := m.load(t)
	client := served(t, p)
	pick(t, p)
	apply(t, p, true, false)
	m.system.took()
	m.system.failNext("ThawUnit", "docker.service", "Connection timed out")

	apply(t, p, false, true)
	st, _ := client.State(context.Background())
	if len(st.Entries) != 1 || st.Entries[0].Unit.Name() != "docker.service" || st.Entries[0].Status != pause.StatusResumeFailed {
		t.Fatalf("%+v", st.Entries)
	}
	if m.system.state("docker.service").freezer != "frozen" {
		t.Fatal("the fake thawed anyway")
	}

	m.system.took()
	reloaded := m.load(t)
	apply(t, reloaded, false, false)
	expectCalls(t, m.system, "ThawUnit docker.service")
	if m.system.state("docker.service").freezer != "running" {
		t.Error("the retried thaw left it frozen")
	}
}

func TestResumingHoldsOffUntilTheNextUnplug(t *testing.T) {
	m := newMachine(t)
	p := m.load(t)
	pick(t, p)
	apply(t, p, true, false)
	m.system.took()
	m.user.took()

	if err := p.Resume(context.Background(), pause.Caller{UID: user}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-p.Wakes():
	default:
		t.Error("resuming did not wake the daemon")
	}
	apply(t, p, true, false)
	expectCalls(t, m.system, "ThawUnit docker.service", "StartUnit nix-gc.timer")
	expectCalls(t, m.user, "ThawUnit syncthing.service")
	apply(t, m.load(t), true, false)
	expectCalls(t, m.system)

	apply(t, p, false, false)
	apply(t, p, true, false)
	expectCalls(t, m.system, "FreezeUnit docker.service", "StopUnit nix-gc.timer")
}

func TestUnpickingOnBatteryResumesAtOnce(t *testing.T) {
	m := newMachine(t)
	p := m.load(t)
	pick(t, p)
	apply(t, p, true, false)
	m.system.took()
	if err := p.PickSystem(context.Background(), root(), units(t, pause.System, "nix-gc.timer")); err != nil {
		t.Fatal(err)
	}
	apply(t, p, true, false)
	expectCalls(t, m.system, "ThawUnit docker.service")
}

func TestOnlyAnAuthorizedCallerPicksSystemUnitsAndAUserOnlyTheirOwn(t *testing.T) {
	m := newMachine(t)
	p := m.load(t)
	ctx := context.Background()
	docker := units(t, pause.System, "docker.service")
	err := p.PickSystem(ctx, pause.Caller{UID: user}, docker)
	if _, ok := errors.AsType[pause.Refusal](err); !ok {
		t.Fatalf("an unauthorized pick: %v", err)
	}
	if err := p.PickUser(ctx, pause.Caller{UID: user}, units(t, userUnit, "syncthing.service")); err != nil {
		t.Fatal(err)
	}
	other, _ := pause.User(1001, "syncthing.service")
	if err := p.PickUser(ctx, pause.Caller{UID: 1001}, []pause.Unit{other}); err != nil {
		t.Fatal(err)
	}
	if err := p.PickUser(ctx, pause.Caller{UID: 1001}, nil); err != nil {
		t.Fatal(err)
	}
	apply(t, p, true, false)
	expectCalls(t, m.system)
	expectCalls(t, m.user, "FreezeUnit syncthing.service")
}

func TestAUserWithoutAManagerHasNothingToPauseAndTheirFreezeEndsWithIt(t *testing.T) {
	m := newMachine(t)
	p := m.load(t)
	client := served(t, p)
	pick(t, p)
	apply(t, p, true, false)
	m.user.took()
	userConn := m.managers.users[user]
	delete(m.managers.users, user)

	apply(t, p, false, false)
	if st, _ := client.State(context.Background()); len(st.Entries) != 0 {
		t.Errorf("a unit whose manager is gone is still %+v", st.Entries)
	}
	apply(t, p, true, false)
	expectCalls(t, m.user)

	m.user.restart("syncthing.service")
	m.managers.users[user] = userConn
	apply(t, p, true, false)
	expectCalls(t, m.user, "FreezeUnit syncthing.service")
}

func TestNothingIsPausedUntilSomethingIsPicked(t *testing.T) {
	m := newMachine(t)
	p := m.load(t)
	apply(t, p, true, false)
	expectCalls(t, m.system)
	expectCalls(t, m.user)
	if _, err := os.Stat(m.state); err == nil {
		t.Error("an untouched state was written")
	}
}

func TestACorruptRecordIsAnErrorNotAFreshStart(t *testing.T) {
	m := newMachine(t)
	if err := os.WriteFile(m.state, []byte(`{"picks":["system/dbus.service"]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := pauser.Load(m.state, m.managers, refuseAll, quiet()); err == nil || !strings.Contains(err.Error(), m.state) {
		t.Errorf("loading an essential pick: %v", err)
	}
}

func TestAMissingUserManagerIsErrNoManager(t *testing.T) {
	_, _, err := pauser.Bus{UserRuntimeDir: t.TempDir()}.User(context.Background(), user)
	if !errors.Is(err, pauser.ErrNoManager) {
		t.Errorf("%v", err)
	}
}

func TestPolkitDecidesForSystemPicks(t *testing.T) {
	a := polkittest.Start(t, polkit.AuthorizationResult{IsAuthorized: true})
	conn, err := polkit.Dial(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	caller := pause.Caller{UID: intconv.Must[uint32](os.Getuid()), PID: intconv.Must[uint32](os.Getpid())}
	authorize := pauser.Polkit(conn)
	if err := authorize(context.Background(), caller); err != nil {
		t.Errorf("authorized: %v", err)
	}
	a.Answer(polkit.AuthorizationResult{})
	if _, ok := errors.AsType[pause.Refusal](authorize(context.Background(), caller)); !ok {
		t.Error("a denial is not a refusal")
	}
}

func TestPlans(t *testing.T) {
	a, _ := pause.System("a.service")
	b, _ := pause.System("b.service")
	c, _ := pause.System("c.service")
	running := map[pause.Unit]bool{a: true, b: true}
	for _, tc := range []struct {
		name          string
		picks, paused []pause.Unit
		holding       bool
		want          pauser.Plan
	}{
		{"holding pauses the running picks", []pause.Unit{a, c}, nil, true, pauser.Plan{Pause: []pause.Unit{a}}},
		{"holding keeps what is paused", []pause.Unit{a}, []pause.Unit{a}, true, pauser.Plan{}},
		{"holding resumes what is no longer picked", []pause.Unit{a}, []pause.Unit{a, b}, true, pauser.Plan{Resume: []pause.Unit{b}}},
		{"not holding resumes everything", []pause.Unit{a, b}, []pause.Unit{a}, false, pauser.Plan{Resume: []pause.Unit{a}}},
		{"nothing picked, nothing done", nil, nil, true, pauser.Plan{}},
	} {
		if got := pauser.Decide(tc.picks, tc.paused, tc.holding, running); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: %+v, want %+v", tc.name, got, tc.want)
		}
	}
	if pauser.Holding(true, true) || pauser.Holding(false, false) || !pauser.Holding(true, false) {
		t.Error("holding is on battery and not resumed")
	}
}
