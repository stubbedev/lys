package bar

import (
	"context"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/feed"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/wmipc"
	"github.com/stubbedev/lys/power/energy"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/service/appenergy"
)

type fakeEnergy struct {
	changes feed.Feed[struct{}]

	mu      sync.Mutex
	report  energy.Report
	running bool
	watches int
	quit    []appenergy.Target
}

func (f *fakeEnergy) set(r energy.Report, running bool) {
	f.mu.Lock()
	f.report, f.running = r, running
	f.mu.Unlock()
	f.changes.Publish(struct{}{})
}

func (f *fakeEnergy) Report() (energy.Report, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.report, f.running
}

func (f *fakeEnergy) Subscribe() (<-chan struct{}, func()) { return f.changes.Subscribe() }

func (f *fakeEnergy) Watch(life context.Context, _ func(error)) {
	f.mu.Lock()
	f.watches++
	f.mu.Unlock()
	<-life.Done()
}

func (f *fakeEnergy) Quittable(a energy.App) (appenergy.Target, bool) {
	return appenergy.Quittable(a, 1000)
}

func (f *fakeEnergy) Quit(_ context.Context, target appenergy.Target) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.quit = append(f.quit, target)
	return nil
}

func (f *fakeEnergy) state() (int, []appenergy.Target) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.watches, slices.Clone(f.quit)
}

const (
	firefoxScope = "user.slice/user-1000.slice/user@1000.service/app.slice/app-firefox-1.scope"
	dockerScope  = "system.slice/docker-abc.scope"
	tmuxScope    = "user.slice/user-1000.slice/user@1000.service/app.slice/tmux-spawn-1.scope"
)

func busyReport(source energy.Source) energy.Report {
	return energy.Report{Covered: time.Minute, Source: source, Watts: 16.5, Apps: []energy.App{
		{Cgroup: firefoxScope, Command: "firefox", PIDs: []uint32{40, 41}, Impact: 0.47, Watts: 4.2},
		{Cgroup: dockerScope, Command: "mysqld", PIDs: []uint32{90}, Impact: 0.5, Watts: 4.5},
		{Cgroup: "system.slice/quiet.service", Command: "quiet", Impact: 0.01, Watts: 0.1},
	}}
}

func openBatteryEnergy(t *testing.T, src *fakeEnergy, windows ...wmipc.Window) *batteryView {
	t.Helper()
	ctx := newTestContext(t, config.Defaults())
	ctx.AppEnergy = src
	if windows != nil {
		ctx.Tasks = newTaskHub(&fakeToplevels{}, func(fn func()) { fn() })
		ctx.Tasks.geom = staticGeometry(windows)
	}
	v := openView(t, ctx, batteryDropdown).(*batteryView)
	return v
}

type pageState struct {
	shown bool
	page  string
}

func (p pageState) unpack() (bool, string) { return p.shown, p.page }

func pageOf(v *batteryView) pageState {
	shown, page := energyPage(v)
	return pageState{shown, page}
}

func energyPage(v *batteryView) (bool, string) {
	return v.energy.box.Visible(), v.energy.pages.Visible()
}

func TestTheEnergySectionFollowsTheReport(t *testing.T) {
	src := &fakeEnergy{}
	v := openBatteryEnergy(t, src)
	if shown, _ := onHeadlessLoop(func() pageState { return pageOf(v) }).unpack(); shown {
		t.Error("the energy section shows while lys-power does not serve it")
	}
	waitHeadless(t, "the open dropdown to ask for sampling", func() bool { n, _ := src.state(); return n == 1 })

	src.set(energy.Report{Source: energy.SourceNone}, true)
	waitHeadless(t, "measuring", func() bool { shown, page := energyPage(v); return shown && page == "measuring" })

	src.set(energy.Report{Covered: time.Minute, Source: energy.SourceNone, Apps: []energy.App{{Cgroup: "a", Command: "quiet", Impact: 0.01}}}, true)
	waitHeadless(t, "the empty list", func() bool { shown, page := energyPage(v); return shown && page == "none" })
	if labels := onHeadlessLoop(func() []string { return treetest.Labels(v) }); !slices.Contains(labels, i18n.T("dropdown-battery-energy-none")) || slices.Contains(labels, "quiet") {
		t.Errorf("labels %q, want the empty note and no quiet app", labels)
	}

	src.set(busyReport(energy.SourcePackage), true)
	waitHeadless(t, "the busy apps", func() bool { _, page := energyPage(v); return page == "apps" })
	rows := onHeadlessLoop(func() []widget.Widget { return treetest.WithClass(v, "battery-energy-app") })
	if len(rows) != 2 {
		t.Fatalf("%d rows, want firefox and mysqld, the quiet service left out", len(rows))
	}
	labels := onHeadlessLoop(func() []string { return treetest.Labels(v) })
	for _, want := range []string{"mysqld", "firefox", appEnergyUsage(busyReport(energy.SourcePackage), busyReport(energy.SourcePackage).Apps[0])} {
		if !slices.Contains(labels, want) {
			t.Errorf("labels %q lack %q", labels, want)
		}
	}

	src.set(energy.Report{}, false)
	waitHeadless(t, "the section gone with lys-power", func() bool { shown, _ := energyPage(v); return !shown })
}

func TestOnlyTheUsersOwnAppsCanBeQuit(t *testing.T) {
	src := &fakeEnergy{}
	src.set(busyReport(energy.SourceNone), true)
	v := openBatteryEnergy(t, src)
	quits := onHeadlessLoop(func() []widget.Widget { return treetest.WithClass(v, "battery-energy-quit") })
	if len(quits) != 1 {
		t.Fatalf("%d quit buttons, want firefox's alone: docker's scope is the system's", len(quits))
	}
	onHeadlessLoop(func() bool { quits[0].(*widget.Button).ClickAt(widget.Point{}); return true })
	waitHeadless(t, "the quit", func() bool { _, quit := src.state(); return len(quit) == 1 })
	if want, _ := appenergy.Quittable(busyReport(energy.SourceNone).Apps[0], 1000); quit(src)[0] != want {
		t.Errorf("quit %+v, want firefox's scope", quit(src))
	}
	if shows := onHeadlessLoop(func() []widget.Widget { return treetest.WithClass(v, "battery-energy-show") }); len(shows) != 0 {
		t.Errorf("%d show buttons without any window", len(shows))
	}
}

func quit(src *fakeEnergy) []appenergy.Target {
	_, q := src.state()
	return q
}

func TestQuittingAJobInATerminalEndsThatJob(t *testing.T) {
	claude := energy.App{Cgroup: tmuxScope, Command: "claude", PIDs: []uint32{20, 21}, Job: &energy.ProcessID{PID: 21, Start: 99}, Impact: 0.6}
	src := &fakeEnergy{}
	src.set(energy.Report{Covered: time.Minute, Source: energy.SourceNone, Apps: []energy.App{claude}}, true)
	v := openBatteryEnergy(t, src)
	quits := onHeadlessLoop(func() []widget.Widget { return treetest.WithClass(v, "battery-energy-quit") })
	if len(quits) != 1 {
		t.Fatalf("%d quit buttons, want claude's", len(quits))
	}
	onHeadlessLoop(func() bool { quits[0].(*widget.Button).ClickAt(widget.Point{}); return true })
	waitHeadless(t, "the quit", func() bool { return len(quit(src)) == 1 })
	job, _ := appenergy.Quittable(claude, 1000)
	scope, _ := appenergy.Quittable(energy.App{Cgroup: tmuxScope}, 1000)
	if got := quit(src)[0]; got != job || got == scope {
		t.Errorf("quit %+v, want claude's job %+v, not the terminal's scope", got, job)
	}
}

type staticGeometry []wmipc.Window

func (g staticGeometry) Windows() []wmipc.Window { return g }

func TestAnAppWithAWindowCanBeShownAndIsNamedForIt(t *testing.T) {
	src := &fakeEnergy{}
	src.set(busyReport(energy.SourcePackage), true)
	v := openBatteryEnergy(t, src, wmipc.Window{ID: 5, AppID: "org.mozilla.firefox", PID: 41}, wmipc.Window{ID: 6, AppID: "foot", PID: 7})
	shows := onHeadlessLoop(func() []widget.Widget { return treetest.WithClass(v, "battery-energy-show") })
	if len(shows) != 1 {
		t.Errorf("%d show buttons, want firefox's alone: mysqld has no window", len(shows))
	}
	labels := onHeadlessLoop(func() []string { return treetest.Labels(v) })
	if !slices.Contains(labels, "org.mozilla.firefox") || slices.Contains(labels, "firefox") || !slices.Contains(labels, "mysqld") {
		t.Errorf("labels %q, want firefox named for its window and mysqld for its command", labels)
	}
}

func TestAppWindowMatchesAProcessOfTheApp(t *testing.T) {
	windows := []wmipc.Window{{ID: 9, PID: 41}, {ID: 3, PID: 41}, {ID: 1, PID: 7}, {ID: 2, PID: -1}}
	if w, ok := appWindow(windows, []uint32{40, 41}); !ok || w.ID != 3 {
		t.Errorf("window %+v %v, want 3, the lowest of the app's", w, ok)
	}
	if _, ok := appWindow(windows, []uint32{99}); ok {
		t.Error("a window found for an app without one")
	}
}

func TestEnergyUsageNamesWattsOnlyWhenMeasured(t *testing.T) {
	a := energy.App{Impact: 0.474, Watts: 4.25}
	if got, want := appEnergyUsage(energy.Report{Source: energy.SourceBattery}, a), i18n.T("dropdown-battery-energy-watts", i18n.Str("watts", "4.2W"), i18n.Str("percent", "47")); got != want {
		t.Errorf("measured: %q, want %q", got, want)
	}
	if got, want := appEnergyUsage(energy.Report{Source: energy.SourceNone}, a), i18n.T("dropdown-battery-energy-cpu", i18n.Str("percent", "47")); got != want {
		t.Errorf("unmeasured: %q, want %q", got, want)
	}
}

func TestAppEnergyNeedsTheSystemBus(t *testing.T) {
	ctx := newTestContext(t, config.Defaults())
	stop := startAppEnergy(&ctx, nil, nil)
	defer stop()
	if ctx.AppEnergy != nil {
		t.Error("energy followed without a system bus")
	}
}
