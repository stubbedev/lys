package energy_test

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/stubbedev/lys/power/energy"
)

var origin = time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)

const (
	firefox = "user.slice/user-1000.slice/user@1000.service/app.slice/app-firefox-1.scope"
	tmux    = "user.slice/user-1000.slice/user@1000.service/app.slice/tmux-spawn-2.scope"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func group(cgroup string, cpu time.Duration, threads ...energy.Thread) energy.Group {
	return energy.Group{Cgroup: cgroup, Command: "cmd", CPU: cpu, Threads: threads}
}

func thread(tid uint32, switches uint64) energy.Thread {
	return energy.Thread{PID: tid, TID: tid, Process: "p", Name: "t", Switches: switches}
}

func sample(at time.Duration, groups ...energy.Group) energy.Sample {
	return energy.Sample{At: origin.Add(at), Groups: groups}
}

func measure(t *testing.T, prev, cur energy.Sample) energy.Interval {
	t.Helper()
	iv, err := energy.Measure(prev, cur)
	if err != nil {
		t.Fatal(err)
	}
	return iv
}

func app(t *testing.T, r energy.Report, cgroup string) energy.App {
	t.Helper()
	for _, a := range r.Apps {
		if a.Cgroup == cgroup {
			return a
		}
	}
	t.Fatalf("%s is not in the report %+v", cgroup, r)
	return energy.App{}
}

func TestCgroupCPUDeltasBecomeBusyFractions(t *testing.T) {
	prev := sample(0, group(firefox, 10*time.Second), group(tmux, 100*time.Second))
	cur := sample(2*time.Second, group(firefox, 11*time.Second), group(tmux, 120*time.Second))
	r := energy.Attribute([]energy.Interval{measure(t, prev, cur)})
	if r.Covered != 2*time.Second {
		t.Errorf("the report covers %s, want 2s", r.Covered)
	}
	if got := app(t, r, firefox).CPU; !near(got, 0.5) {
		t.Errorf("firefox CPU = %v, want 0.5", got)
	}
	if got := app(t, r, tmux).CPU; !near(got, 10) {
		t.Errorf("tmux CPU = %v, want 10 cores", got)
	}
	if r.Apps[0].Cgroup != tmux {
		t.Errorf("the first app is %s, want the busiest, tmux", r.Apps[0].Cgroup)
	}
}

func TestAnAppWithNothingToShowIsLeftOut(t *testing.T) {
	prev := sample(0, group(firefox, time.Second, thread(1, 5)))
	cur := sample(time.Second, group(firefox, time.Second, thread(1, 5)))
	if r := energy.Attribute([]energy.Interval{measure(t, prev, cur)}); len(r.Apps) != 0 {
		t.Errorf("an idle app is listed: %+v", r.Apps)
	}
}

func TestWakeupsArePerThreadAndCostCPUTime(t *testing.T) {
	prev := sample(0, group(firefox, 0, thread(1, 100), thread(2, 1000)))
	cur := sample(time.Second, group(firefox, 0, thread(1, 150), thread(2, 1100), thread(3, 10)))
	a := app(t, energy.Attribute([]energy.Interval{measure(t, prev, cur)}), firefox)
	if !near(a.Wakeups, 160) {
		t.Errorf("wakeups = %v/s, want 50+100+10 from the new thread", a.Wakeups)
	}
	if want := 160 * energy.WakeupCost.Seconds(); !near(a.Impact, want) {
		t.Errorf("impact = %v, want %v from the wakeups alone", a.Impact, want)
	}
	if len(a.Threads) != 3 || a.Threads[0].TID != 2 || !near(a.Threads[0].Wakeups, 100) {
		t.Errorf("threads = %+v, want thread 2 first at 100/s", a.Threads)
	}
}

func TestAnAppThatExitsMidSampleLeavesTheReport(t *testing.T) {
	prev := sample(0, group(firefox, time.Second, thread(1, 10)), group(tmux, time.Second, thread(2, 10)))
	cur := sample(time.Second, group(tmux, 2*time.Second, thread(2, 20)))
	r := energy.Attribute([]energy.Interval{measure(t, prev, cur)})
	if len(r.Apps) != 1 || r.Apps[0].Cgroup != tmux {
		t.Errorf("apps = %+v, want tmux alone", r.Apps)
	}
}

func TestAThreadThatExitsTakesNothingAndAReusedIDStartsAgain(t *testing.T) {
	prev := sample(0, group(firefox, 0, thread(1, 10), thread(2, 500)))
	cur := sample(time.Second, group(firefox, 0, thread(1, 30), thread(2, 4)))
	a := app(t, energy.Attribute([]energy.Interval{measure(t, prev, cur)}), firefox)
	if !near(a.Wakeups, 24) {
		t.Errorf("wakeups = %v, want 20 from thread 1 and 4 from the new thread 2", a.Wakeups)
	}
}

func TestAnAppBornMidSampleCountsFromZero(t *testing.T) {
	prev := sample(0, group(tmux, 0))
	cur := sample(time.Second, group(tmux, 0), group(firefox, 300*time.Millisecond))
	a := app(t, energy.Attribute([]energy.Interval{measure(t, prev, cur)}), firefox)
	if !near(a.CPU, 0.3) {
		t.Errorf("CPU = %v, want 0.3", a.CPU)
	}
}

func TestAProcessMovedIntoAScopeKeepsItsThreadCounts(t *testing.T) {
	prev := sample(0, group(tmux, 0, thread(7, 1000)))
	cur := sample(time.Second, group(tmux, 0), group(firefox, 0, thread(7, 1005)))
	a := app(t, energy.Attribute([]energy.Interval{measure(t, prev, cur)}), firefox)
	if !near(a.Wakeups, 5) {
		t.Errorf("wakeups = %v, want the 5 since the move, not the thread's lifetime", a.Wakeups)
	}
}

func TestEnergyCounterWraps(t *testing.T) {
	const top = 262143328850
	prev := sample(0, group(firefox, 0))
	cur := sample(2*time.Second, group(firefox, time.Second))
	prev.Energy = []energy.Counter{{Microjoules: top - 4_000_000, Range: top}}
	cur.Energy = []energy.Counter{{Microjoules: 6_000_000, Range: top}}
	iv := measure(t, prev, cur)
	if iv.Source != energy.SourcePackage || !near(iv.Joules, 10) {
		t.Errorf("interval = %v J from %v, want 10 J from the package across the wrap", iv.Joules, iv.Source)
	}
	r := energy.Attribute([]energy.Interval{iv})
	if !near(r.Watts, 5) {
		t.Errorf("report = %v W, want 5", r.Watts)
	}
}

func TestEnergyCountersSumAcrossPackages(t *testing.T) {
	prev := sample(0)
	cur := sample(time.Second)
	prev.Energy = []energy.Counter{{Microjoules: 0, Range: 1e9}, {Microjoules: 0, Range: 1e9}}
	cur.Energy = []energy.Counter{{Microjoules: 1e6, Range: 1e9}, {Microjoules: 2e6, Range: 1e9}}
	if iv := measure(t, prev, cur); !near(iv.Joules, 3) {
		t.Errorf("joules = %v, want 3", iv.Joules)
	}
}

func TestAnInconsistentEnergyCounterIsAnError(t *testing.T) {
	prev := sample(0)
	cur := sample(time.Second)
	prev.Energy = []energy.Counter{{Microjoules: 5, Range: 10}}
	cur.Energy = []energy.Counter{{Microjoules: 11, Range: 10}}
	if _, err := energy.Measure(prev, cur); err == nil {
		t.Error("a counter past its range measured")
	}
	cur.Energy = []energy.Counter{{Microjoules: 5, Range: 0}}
	if _, err := energy.Measure(prev, cur); err == nil {
		t.Error("a counter without a range measured")
	}
}

func TestASampleThatIsNotLaterIsAnError(t *testing.T) {
	s := sample(time.Second)
	if _, err := energy.Measure(s, s); !errors.Is(err, energy.ErrNotLater) {
		t.Errorf("measuring a sample against itself: %v, want ErrNotLater", err)
	}
	if _, err := energy.Measure(sample(2*time.Second), sample(time.Second)); !errors.Is(err, energy.ErrNotLater) {
		t.Errorf("measuring backwards: %v, want ErrNotLater", err)
	}
}

func TestBatteryPowerIsTheMeanOfBothEnds(t *testing.T) {
	low, high := uint64(10_000_000), uint64(20_000_000)
	prev, cur := sample(0), sample(4*time.Second)
	prev.Battery, cur.Battery = &low, &high
	iv := measure(t, prev, cur)
	if iv.Source != energy.SourceBattery || !near(iv.Joules, 60) {
		t.Errorf("interval = %v J from %v, want 60 J from the battery", iv.Joules, iv.Source)
	}
	cur.Battery = nil
	if iv := measure(t, prev, cur); iv.Source != energy.SourceNone || iv.Joules != 0 {
		t.Errorf("with the charger plugged in mid-sample: %v J from %v, want none", iv.Joules, iv.Source)
	}
}

func TestPowerIsSharedByImpactWithTheKernelKeepingItsPart(t *testing.T) {
	prev := sample(0, group(firefox, 0), group(tmux, 0))
	cur := sample(time.Second, group(firefox, 250*time.Millisecond), group(tmux, 500*time.Millisecond))
	prev.Energy = []energy.Counter{{Microjoules: 0, Range: 1e12}}
	cur.Energy = []energy.Counter{{Microjoules: 10e6, Range: 1e12}}
	cur.TotalCPU = time.Second
	r := energy.Attribute([]energy.Interval{measure(t, prev, cur)})
	if !near(app(t, r, firefox).Watts, 2.5) || !near(app(t, r, tmux).Watts, 5) {
		t.Errorf("watts = %v and %v, want 2.5 and 5, the kernel's quarter left out", app(t, r, firefox).Watts, app(t, r, tmux).Watts)
	}
	if !near(r.Watts, 10) {
		t.Errorf("total = %v W, want 10", r.Watts)
	}
}

func TestWithoutPowerAppsHaveImpactButNoWatts(t *testing.T) {
	prev := sample(0, group(firefox, 0))
	cur := sample(time.Second, group(firefox, time.Second))
	r := energy.Attribute([]energy.Interval{measure(t, prev, cur)})
	if r.Source != energy.SourceNone || r.Watts != 0 || app(t, r, firefox).Watts != 0 || app(t, r, firefox).Impact != 1 {
		t.Errorf("report = %+v, want impact 1 and no watts", r)
	}
}

func TestAReportMixingSourcesClaimsNoWatts(t *testing.T) {
	watts := uint64(5_000_000)
	a, b, c := sample(0, group(firefox, 0)), sample(time.Second, group(firefox, time.Second)), sample(2*time.Second, group(firefox, 2*time.Second))
	a.Battery, b.Battery = &watts, &watts
	r := energy.Attribute([]energy.Interval{measure(t, a, b), measure(t, b, c)})
	if r.Source != energy.SourceNone || r.Watts != 0 || app(t, r, firefox).Watts != 0 {
		t.Errorf("report = %+v, want no watts over a window that was half unmeasured", r)
	}
	if r.Covered != 2*time.Second || !near(app(t, r, firefox).CPU, 1) {
		t.Errorf("report = %+v, want both seconds at one CPU", r)
	}
}

func TestGPUTimeInNanosecondsAndCycles(t *testing.T) {
	ns := energy.DRMClient{Device: "0000:03:00.0", ID: 4}
	cycles := energy.DRMClient{Device: "0000:00:02.0", ID: 9}
	prev := sample(0, energy.Group{Cgroup: firefox, GPU: map[energy.DRMClient]map[string]energy.Engine{
		ns:     {"gfx": {Clock: energy.EngineNanoseconds, Busy: 1e9}},
		cycles: {"rcs": {Clock: energy.EngineCycles, Busy: 100, Total: 1000}},
	}})
	cur := sample(2*time.Second, energy.Group{Cgroup: firefox, GPU: map[energy.DRMClient]map[string]energy.Engine{
		ns:     {"gfx": {Clock: energy.EngineNanoseconds, Busy: 1.5e9}},
		cycles: {"rcs": {Clock: energy.EngineCycles, Busy: 350, Total: 2000}},
	}})
	a := app(t, energy.Attribute([]energy.Interval{measure(t, prev, cur)}), firefox)
	if !near(a.GPU, 0.25+0.25) {
		t.Errorf("GPU = %v, want 0.25 from each engine", a.GPU)
	}
}

func TestSignificantAppsAndTheEmptyList(t *testing.T) {
	r := energy.Report{Covered: time.Minute, Source: energy.SourceNone, Apps: []energy.App{
		{Cgroup: "a", Impact: 2}, {Cgroup: "b", Impact: 0.5}, {Cgroup: "c", Impact: 0.06}, {Cgroup: "d", Impact: 0.01},
	}}
	if got := r.Significant(2); len(got) != 2 || got[0].Cgroup != "a" || got[1].Cgroup != "b" {
		t.Errorf("the top two = %+v", got)
	}
	if got := r.Significant(10); len(got) != 3 {
		t.Errorf("significant = %+v, want the three over %v", got, energy.SignificantImpact)
	}
	quiet := energy.Report{Covered: time.Minute, Source: energy.SourceNone, Apps: []energy.App{{Cgroup: "d", Impact: 0.01}}}
	if got := quiet.Significant(5); len(got) != 0 || !quiet.Measured() {
		t.Errorf("a measured quiet report lists %+v", got)
	}
	empty := energy.Attribute(nil)
	if empty.Measured() || len(empty.Significant(5)) != 0 || empty.Source != energy.SourceNone {
		t.Errorf("a report of nothing = %+v, want unmeasured and empty", empty)
	}
}

func TestTrimKeepsTheLastSpan(t *testing.T) {
	ivs := []energy.Interval{{End: origin.Add(time.Minute)}, {End: origin.Add(5 * time.Minute)}, {End: origin.Add(7 * time.Minute)}}
	if got := energy.Trim(ivs, origin.Add(7*time.Minute), energy.Span); len(got) != 2 || !got[0].End.Equal(origin.Add(5*time.Minute)) {
		t.Errorf("trimmed to %+v, want the last two", got)
	}
	if got := energy.Trim(ivs, origin.Add(time.Hour), energy.Span); len(got) != 0 {
		t.Errorf("an hour later %d intervals remain, want none", len(got))
	}
}

func TestUserUnits(t *testing.T) {
	for _, tc := range []struct {
		cgroup string
		uid    uint32
		unit   string
		ok     bool
	}{
		{firefox, 1000, "app-firefox-1.scope", true},
		{"user.slice/user-1000.slice/user@1000.service/session.slice/lys-shell.service", 1000, "lys-shell.service", true},
		{"system.slice/docker-abc.scope", 0, "", false},
		{"user.slice/user-1000.slice/session-2.scope", 0, "", false},
		{"user.slice/user-1000.slice/user@1000.service", 0, "", false},
		{"user.slice/user-1000.slice/user@x.service/app.slice/a.scope", 0, "", false},
	} {
		uid, unit, ok := energy.App{Cgroup: tc.cgroup}.UserUnit()
		if uid != tc.uid || unit != tc.unit || ok != tc.ok {
			t.Errorf("%s: %d %q %v, want %d %q %v", tc.cgroup, uid, unit, ok, tc.uid, tc.unit, tc.ok)
		}
	}
	if got := (energy.App{Cgroup: firefox}).Unit(); got != "app-firefox-1.scope" {
		t.Errorf("unit = %q", got)
	}
}

func TestSourcesRoundTrip(t *testing.T) {
	for _, s := range []energy.Source{energy.SourceNone, energy.SourcePackage, energy.SourceBattery} {
		if got, err := energy.ParseSource(s.String()); err != nil || got != s {
			t.Errorf("%v: %v, %v", s, got, err)
		}
	}
	if _, err := energy.ParseSource("mains"); err == nil {
		t.Error("an unknown source parsed")
	}
}
