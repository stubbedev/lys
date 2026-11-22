package bar

import (
	"slices"
	"sync"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/feed"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/power/pause"
)

type fakePaused struct {
	changes feed.Feed[struct{}]

	mu      sync.Mutex
	state   pause.State
	running bool
}

func (f *fakePaused) set(st pause.State, running bool) {
	f.mu.Lock()
	f.state, f.running = st, running
	f.mu.Unlock()
	f.changes.Publish(struct{}{})
}

func (f *fakePaused) Value() (pause.State, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.state, f.running
}

func (f *fakePaused) Subscribe() (<-chan struct{}, func()) { return f.changes.Subscribe() }

func TestThePausedSectionListsWhatIsPausedAndWhatFailedAndHidesWhenNothingIs(t *testing.T) {
	src := &fakePaused{}
	ctx := newTestContext(t, config.Defaults())
	ctx.PausedUnits = src
	v := openView(t, ctx, batteryDropdown).(*batteryView)
	shown := func() bool { return v.paused.box.Visible() }
	if onHeadlessLoop(shown) {
		t.Error("the section shows while lys-power does not serve it")
	}

	docker, _ := pause.System("docker.service")
	timer, _ := pause.System("nix-gc.timer")
	syncthing, _ := pause.User(1000, "syncthing.service")
	src.set(pause.State{Picks: []pause.Unit{docker}}, true)
	waitHeadless(t, "lys-power with nothing paused", func() bool { return !shown() })

	src.set(pause.State{Picks: []pause.Unit{docker, timer, syncthing}, Entries: []pause.Entry{
		{Unit: docker, Status: pause.StatusPaused},
		{Unit: timer, Status: pause.StatusPauseFailed, Err: "Unit nix-gc.timer not loaded."},
		{Unit: syncthing, Status: pause.StatusResumeFailed, Err: "Connection timed out"},
	}}, true)
	waitHeadless(t, "the paused units", shown)
	labels := onHeadlessLoop(func() []string { return treetest.Labels(v.paused.box) })
	for _, want := range []string{
		"docker.service", i18n.T("dropdown-battery-paused-now"),
		"nix-gc.timer", i18n.T("dropdown-battery-paused-failed", i18n.Str("error", "Unit nix-gc.timer not loaded.")),
		"syncthing.service", i18n.T("dropdown-battery-paused-resume-failed", i18n.Str("error", "Connection timed out")),
	} {
		if !slices.Contains(labels, want) {
			t.Errorf("labels %q lack %q", labels, want)
		}
	}
	if rows := onHeadlessLoop(func() int { return len(treetest.WithClass(v, "battery-paused-unit")) }); rows != 3 {
		t.Errorf("%d rows, want 3", rows)
	}

	src.set(pause.State{}, false)
	waitHeadless(t, "the section gone with lys-power", func() bool { return !shown() })
}

func TestPausedUnitsNeedTheSystemBus(t *testing.T) {
	ctx := newTestContext(t, config.Defaults())
	stop := startPausedUnits(&ctx, nil, nil)
	defer stop()
	if ctx.PausedUnits != nil {
		t.Error("paused units followed without a system bus")
	}
}
