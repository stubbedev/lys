package appnap_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/dbusx"
	"github.com/stubbedev/lys/lib/testhome"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/lib/wmipc"
	"github.com/stubbedev/lys/power/nap"

	"github.com/stubbedev/lys/shell/service/appnap"
)

func TestMain(m *testing.M) { testhome.Run(m) }

func TestTheTestsReachNoHostSession(t *testing.T) { testhome.RequireNoHostSession(t) }

type journal struct {
	mu    sync.Mutex
	lines []string
	units map[string]bool
}

func (j *journal) note(format string, args ...any) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.lines = append(j.lines, fmt.Sprintf(format, args...))
}

func (j *journal) take() []string {
	j.mu.Lock()
	defer j.mu.Unlock()
	out := j.lines
	j.lines = nil
	return out
}

type property struct {
	Name  string
	Value dbus.Variant
}

type manager struct{ j *journal }

func (m manager) known(name string) *dbus.Error {
	if m.j.units[name] {
		return nil
	}
	return dbus.NewError("org.freedesktop.systemd1.NoSuchUnit", []any{"Unit " + name + " not loaded."})
}

func (m manager) SetUnitProperties(name string, runtime bool, props []property) *dbus.Error {
	if err := m.known(name); err != nil {
		return err
	}
	for _, p := range props {
		m.j.note("set %s %s=%v runtime=%v", name, p.Name, p.Value.Value(), runtime)
	}
	return nil
}

func (m manager) FreezeUnit(name string) *dbus.Error {
	m.j.note("freeze %s", name)
	return m.known(name)
}

func (m manager) ThawUnit(name string) *dbus.Error {
	m.j.note("thaw %s", name)
	return m.known(name)
}

type slacker struct{ j *journal }

func (s slacker) SetTimerSlack(_ context.Context, unit nap.Unit, slack time.Duration) error {
	s.j.note("slack %s %v", unit, slack)
	return nil
}

func systemd(t *testing.T, units ...string) (*appnap.Napper, *journal) {
	t.Helper()
	bus := dbustest.Start(t)
	j := &journal{units: map[string]bool{}}
	for _, u := range units {
		j.units[u] = true
	}
	release, err := dbusx.Serve(bus.Conn(t), dbusx.Service{
		Name:      "org.freedesktop.systemd1",
		Path:      "/org/freedesktop/systemd1",
		Interface: "org.freedesktop.systemd1.Manager",
		Methods:   manager{j},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	s := appnap.NewSystemd(bus.Conn(t), slacker{j}, t.TempDir())
	s.Efficiency = func() ([]byte, error) { return []byte{0xf0}, nil }
	return appnap.NewNapper(s), j
}

func TestNappingLowersWeightsAndSlackAndFocusRestoresThemAtOnce(t *testing.T) {
	n, j := systemd(t, string(chrome))
	ctx := context.Background()
	if err := n.Reconcile(ctx, appnap.Plan(hiddenChrome())); err != nil {
		t.Fatal(err)
	}
	want := []string{
		fmt.Sprintf("set %s CPUWeight=20 runtime=true", chrome),
		fmt.Sprintf("set %s IOWeight=20 runtime=true", chrome),
		fmt.Sprintf("slack %s 50ms", chrome),
	}
	if got := j.take(); !slices.Equal(got, want) {
		t.Fatalf("napping:\n got %q\nwant %q", got, want)
	}
	if err := n.Reconcile(ctx, appnap.Plan(hiddenChrome())); err != nil {
		t.Fatal(err)
	}
	if got := j.take(); len(got) != 0 {
		t.Fatalf("an unchanged plan asked systemd for %q", got)
	}
	focused := hiddenChrome()
	focused.Windows[0].Asleep, focused.Windows[0].Focused, focused.Windows[2].Focused = false, true, false
	if err := n.Reconcile(ctx, appnap.Plan(focused)); err != nil {
		t.Fatal(err)
	}
	unset := uint64(math.MaxUint64)
	want = []string{
		fmt.Sprintf("set %s CPUWeight=%d runtime=true", chrome, unset),
		fmt.Sprintf("set %s IOWeight=%d runtime=true", chrome, unset),
		fmt.Sprintf("slack %s 0s", chrome),
	}
	if got := j.take(); !slices.Equal(got, want) {
		t.Fatalf("focus:\n got %q\nwant %q", got, want)
	}
}

func TestAFrozenAppThawsBeforeAnythingElseAndFreezesLast(t *testing.T) {
	n, j := systemd(t, string(chrome))
	ctx := context.Background()
	in := hiddenChrome()
	in.Levels = map[string]nap.Level{"google-chrome": nap.LevelFrozen}
	if err := n.Reconcile(ctx, appnap.Plan(in)); err != nil {
		t.Fatal(err)
	}
	if got := j.take(); len(got) == 0 || got[len(got)-1] != "freeze "+string(chrome) {
		t.Fatalf("napping frozen: %q, want the freeze last", got)
	}
	if err := n.WakeAll(ctx); err != nil {
		t.Fatal(err)
	}
	if got := j.take(); len(got) == 0 || got[0] != "thaw "+string(chrome) {
		t.Fatalf("waking: %q, want the thaw first", got)
	}
}

func TestEfficiencyCoresAreAllowedWhileNappingAndAllCoresAfter(t *testing.T) {
	n, j := systemd(t, string(chrome))
	ctx := context.Background()
	in := hiddenChrome()
	in.Levels = map[string]nap.Level{"google-chrome": nap.LevelEfficiencyCores}
	if err := n.Reconcile(ctx, appnap.Plan(in)); err != nil {
		t.Fatal(err)
	}
	if got := j.take(); !slices.Contains(got, fmt.Sprintf("set %s AllowedCPUs=[240] runtime=true", chrome)) {
		t.Fatalf("napping on efficiency cores: %q", got)
	}
	if err := n.WakeAll(ctx); err != nil {
		t.Fatal(err)
	}
	if got := j.take(); !slices.Contains(got, fmt.Sprintf("set %s AllowedCPUs=[] runtime=true", chrome)) {
		t.Fatalf("waking from efficiency cores: %q", got)
	}
}

func TestAMachineWithoutEfficiencyCoresSaysSo(t *testing.T) {
	_, err := appnap.EfficiencyMask(t.TempDir())
	if !errors.Is(err, appnap.ErrNoEfficiencyCores) {
		t.Fatalf("no cpu_atom: %v, want %v", err, appnap.ErrNoEfficiencyCores)
	}
}

func TestTheSessionEndingWakesEveryNappingApp(t *testing.T) {
	n, j := systemd(t, string(chrome))
	proc := t.TempDir()
	for pid, cgroup := range map[string]string{"10": "app.slice/" + string(chrome), "20": "app.slice/" + string(foot)} {
		dir := filepath.Join(proc, pid)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "cgroup"), []byte("0::/user.slice/user-1000.slice/user@1000.service/"+cgroup+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	src := appnap.Sources{
		Windows:   func() []wmipc.Window { return hiddenChrome().Windows },
		Streams:   func() []int32 { return nil },
		Holds:     func(context.Context) ([]appnap.Hold, error) { return nil, nil },
		OnBattery: func(context.Context) (bool, error) { return true, nil },
		Levels:    func() (map[string]nap.Level, error) { return nil, nil },
		Proc:      proc,
	}
	ticks := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		appnap.Run(context.Background(), n, src, ticks, func(err error) { t.Error(err) })
	}()
	ticks <- struct{}{}
	close(ticks)
	waittest.For(t, done, "the napper to stop")
	got := j.take()
	if !slices.Contains(got, fmt.Sprintf("set %s CPUWeight=20 runtime=true", chrome)) || got[len(got)-1] != fmt.Sprintf("slack %s 0s", chrome) {
		t.Fatalf("a session that napped Chrome and ended: %q, want the nap and then its waking", got)
	}
}

func TestAUnitThatIsGoneNeedsNoWaking(t *testing.T) {
	n, j := systemd(t, string(chrome))
	ctx := context.Background()
	in := hiddenChrome()
	in.Units[10] = "app-gone-1.scope"
	if err := n.Reconcile(ctx, appnap.Plan(in)); err != nil {
		t.Fatalf("napping a unit that went: %v", err)
	}
	j.take()
	if err := n.WakeAll(ctx); err != nil {
		t.Fatalf("waking a unit that went: %v", err)
	}
}
