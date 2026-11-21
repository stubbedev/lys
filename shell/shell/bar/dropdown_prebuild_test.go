package bar

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/service/upower"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/internal/apptest"
)

// loopQueue is the loop: what is handed to it runs when drained.
type loopQueue struct{ fns []func() }

func (q *loopQueue) add(fn func()) { q.fns = append(q.fns, fn) }

func (q *loopQueue) drain() {
	for len(q.fns) > 0 {
		fn := q.fns[0]
		q.fns = q.fns[1:]
		fn()
	}
}

func prebuildingRegistry(t *testing.T, cfg *config.Config) (*dropdownRegistry, *loopQueue) {
	t.Helper()
	return prebuildingRegistryOf(newTestContext(t, cfg))
}

func prebuildingRegistryOf(ctx ModuleContext) (*dropdownRegistry, *loopQueue) {
	r := newDropdownRegistry(nil, ctx.Config, ctx.Font, ctx.Style, ctx)
	q := &loopQueue{}
	r.invoke = q.add
	return r, q
}

func clockOnly() *config.Config {
	cfg := config.Defaults()
	cfg.Bar.Layout = []config.BarLayout{{Monitor: "*", Show: true, Center: []config.BarItem{{Module: "clock"}}}}
	return cfg
}

func TestTheBarsDropdownsAreTheOnesTheirModulesOpen(t *testing.T) {
	cfg := clockOnly()
	if got := barDropdowns(cfg, dropdownBuilders()); !slices.Equal(got, []string{"calendar", "weather"}) {
		t.Errorf("dropdowns = %q, want the clock's calendar and weather only", got)
	}
	cfg.Bar.Layout[0].Show = false
	if got := barDropdowns(cfg, dropdownBuilders()); len(got) != 0 {
		t.Errorf("a hidden bar's dropdowns = %q, want none", got)
	}
}

func TestDropdownsBuildAheadOnceABarHasShownItsFirstFrame(t *testing.T) {
	r, q := prebuildingRegistry(t, clockOnly())
	built := 0
	r.whenBuilt(func() { built++ })
	r.attachHost("DP-1", fakeHost{}, widget.Size{W: 1600, H: 1000})
	r.attachHost("HDMI-A-1", fakeHost{}, widget.Size{W: 1600, H: 1000})
	q.drain()
	if len(r.instances) != 0 || built != 0 {
		t.Fatalf("built %d dropdowns before a bar showed its first frame", len(r.instances))
	}
	r.barPresented()
	if len(r.instances) != 0 {
		t.Fatal("dropdowns built on the bar's own frame, not on the loop's next turns")
	}
	q.drain()
	if len(r.instances) != 2 || r.instances["calendar"] == nil || r.instances["weather"] == nil {
		t.Fatalf("built %d dropdowns, want the calendar and the weather", len(r.instances))
	}
	if built != 1 {
		t.Errorf("the finished build was told %d times, want once", built)
	}
	r.barPresented()
	moved := clockOnly()
	moved.Bar.Location = config.LocationBottom
	r.setConfig(moved)
	q.drain()
	if built != 1 {
		t.Errorf("a later build told the one waiting for the first again: %d", built)
	}
	built0 := r.instances["calendar"]
	allocs := testing.AllocsPerRun(20, func() {
		if r.instance("calendar", r.builders["calendar"]) != built0 {
			t.Fatal("the open built the calendar again")
		}
	})
	if allocs > 1 {
		t.Errorf("a first open after the build ahead allocates %v", allocs)
	}

	cold, _ := prebuildingRegistry(t, clockOnly())
	if coldAllocs := testing.AllocsPerRun(1, func() {
		cold.release()
		cold.instance("calendar", cold.builders["calendar"])
	}); coldAllocs <= 100 {
		t.Errorf("a cold first open allocates %v: the budget above proves nothing", coldAllocs)
	}
}

func TestAReleaseEndsAPrebuildAndAReloadStartsAnother(t *testing.T) {
	r, q := prebuildingRegistry(t, clockOnly())
	r.attachHost("DP-1", fakeHost{}, widget.Size{W: 1600, H: 1000})
	r.barPresented()
	r.release()
	q.drain()
	if len(r.instances) != 0 {
		t.Errorf("a released registry built %d dropdowns", len(r.instances))
	}
	moved := clockOnly()
	moved.Bar.Location = config.LocationBottom
	r.setConfig(moved)
	q.drain()
	if len(r.instances) != 2 {
		t.Errorf("after a reload %d dropdowns built, want 2", len(r.instances))
	}
}

func TestABuiltAheadDropdownReadsNothingUntilItOpens(t *testing.T) {
	cfg := config.Defaults()
	cfg.Bar.Layout = []config.BarLayout{{Monitor: "*", Show: true, Right: []config.BarItem{{Module: "battery"}}}}
	ctx := newTestContext(t, cfg)
	battery := &countedBattery{fakeBattery: newFakeBattery(upower.Device{IsPresent: true})}
	ctx.Battery = battery
	r, q := prebuildingRegistryOf(ctx)
	r.attachHost("DP-1", fakeHost{}, widget.Size{W: 1600, H: 1000})
	r.barPresented()
	q.drain()
	content := r.instances["battery"]
	if content == nil {
		t.Fatal("the battery dropdown was not built ahead")
	}
	t.Cleanup(content.(dropdownCloser).dropdownClosed)
	if got := battery.reads.Load(); got != 0 {
		t.Errorf("a dropdown built ahead read the battery %d times while nothing shows it", got)
	}
	content.(dropdownOpener).dropdownOpened()
	if battery.reads.Load() == 0 {
		t.Error("opening a dropdown built ahead read nothing")
	}
}

func TestWithoutBuildAheadNothingBuildsBeforeAnOpen(t *testing.T) {
	cfg := clockOnly()
	ctx := newTestContext(t, cfg)
	r := newDropdownRegistry(nil, cfg, ctx.Font, ctx.Style, ctx)
	r.buildAhead()
	r.attachHost("DP-1", fakeHost{}, widget.Size{W: 1600, H: 1000})
	r.barPresented()
	if r.invoke != nil || len(r.instances) != 0 {
		t.Error("a registry without an application built ahead")
	}
}

func TestAMountedBarBuildsItsDropdownsAhead(t *testing.T) {
	cfg := clockOnly()
	ctx := styledContext(t, cfg)
	a := apptest.Run(t)
	ctx.App = a
	rt := &barRuntime{ctx: ctx, font: ctx.Font, style: *ctx.Style, moduleStyle: *ctx.Style}
	host := &barHost{app: a, rt: rt, presented: func(string) { rt.ctx.Dropdowns.barPresented() }}
	outputs := a.Session().Outputs()
	if len(outputs) == 0 {
		t.Fatal("the headless sway shows no outputs")
	}
	built := func() int {
		r := rt.ctx.Dropdowns
		r.mu.Lock()
		defer r.mu.Unlock()
		return len(r.instances)
	}
	if n := apptest.On(a, func() int { rt.mount(a, cfg); return built() }); n != 0 {
		t.Fatalf("%d dropdowns built before a bar attached", n)
	}
	var bar surfaceLayer[config.BarLayout]
	if err := apptest.On(a, func() (err error) { bar, err = host.open(outputs[0], cfg.Bar.Layout[0]); return err }); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { apptest.On(a, func() bool { bar.Close(); return true }) })
	apptest.Until(t, a, "the calendar and the weather built ahead", func() bool {
		r := rt.ctx.Dropdowns
		r.mu.Lock()
		defer r.mu.Unlock()
		return r.instances["calendar"] != nil && r.instances["weather"] != nil
	})

	unmounted := &barRuntime{ctx: ctx, font: ctx.Font, style: *ctx.Style, moduleStyle: *ctx.Style}
	unmounted.mount(nil, cfg)
	unmounted.ctx.Dropdowns.attachHost("DP-1", fakeHost{}, widget.Size{W: 1600, H: 1000})
	unmounted.ctx.Dropdowns.barPresented()
	if r := unmounted.ctx.Dropdowns; r.invoke != nil || len(r.instances) != 0 {
		t.Error("a bar mounted without an application built its dropdowns ahead")
	}
}
