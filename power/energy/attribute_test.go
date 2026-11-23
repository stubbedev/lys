package energy_test

import (
	"errors"
	"math"
	"slices"
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
	return processes(cgroup, cpu, process(1, "p", 0, threads...))
}

func processes(cgroup string, cpu time.Duration, procs ...energy.Process) energy.Group {
	return energy.Group{Cgroup: cgroup, CPU: cpu, Processes: procs}
}

func process(pid uint32, command string, cpu time.Duration, threads ...energy.Thread) energy.Process {
	return energy.Process{ID: energy.ProcessID{PID: pid, Start: 1}, Command: command, CPU: cpu, Threads: threads}
}

func child(pid, parent uint32, start uint64, exe string, cpu time.Duration) energy.Process {
	return energy.Process{ID: energy.ProcessID{PID: pid, Start: start}, Parent: parent, Command: exe, Exe: "/bin/" + exe, CPU: cpu}
}

func jobOf(t *testing.T, before, after []energy.Process) *energy.ProcessID {
	t.Helper()
	prev := sample(0, processes(tmux, 0, before...))
	cur := sample(time.Second, processes(tmux, time.Second, after...))
	return app(t, energy.Attribute([]energy.Interval{measure(t, prev, cur)}), tmux).Job
}

func thread(tid uint32, switches uint64) energy.Thread {
	return energy.Thread{TID: tid, Name: "t", Switches: switches}
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

const session = "user.slice/user-1000.slice/session-3.scope"

func TestAnAppIsNamedAfterItsBusiestProcessNotItsFirst(t *testing.T) {
	prev := sample(0, processes(session, 0, process(10, ".lys-login-wrap", 20*time.Millisecond), process(20, "lys-wm", time.Second), process(30, "Xwayland", time.Second)))
	cur := sample(time.Second, processes(session, 150*time.Millisecond, process(10, ".lys-login-wrap", 20*time.Millisecond), process(20, "lys-wm", 1100*time.Millisecond), process(30, "Xwayland", 1050*time.Millisecond)))
	a := app(t, energy.Attribute([]energy.Interval{measure(t, prev, cur)}), session)
	if a.Command != "lys-wm" {
		t.Errorf("command = %q, want lys-wm, the busiest process, not the session worker with the lowest PID", a.Command)
	}
	if len(a.PIDs) != 3 || a.PIDs[0] != 10 {
		t.Errorf("pids = %v, want all three", a.PIDs)
	}
}

func TestTheBusiestProcessIsTheBusiestOverTheWholeWindow(t *testing.T) {
	a0 := sample(0, processes(tmux, 0, process(1, "zsh", 0), process(2, "claude", 0), process(3, "make", 0)))
	a1 := sample(time.Second, processes(tmux, time.Second, process(1, "zsh", 0), process(2, "claude", 300*time.Millisecond), process(3, "make", 700*time.Millisecond)))
	a2 := sample(2*time.Second, processes(tmux, 2*time.Second, process(1, "zsh", 0), process(2, "claude", 1200*time.Millisecond), process(3, "make", 800*time.Millisecond)))
	a := app(t, energy.Attribute([]energy.Interval{measure(t, a0, a1), measure(t, a1, a2)}), tmux)
	if a.Command != "claude" {
		t.Errorf("command = %q, want claude at 1.2s over make at 0.8s, though make was busier in the first interval", a.Command)
	}
}

func TestWakeupsAndGPUTimeCountTowardTheBusiestProcess(t *testing.T) {
	gpu := energy.DRMClient{Device: "0000:00:02.0", ID: 1}
	drawing := func(busy uint64) energy.Process {
		p := process(5, "game", 0)
		p.GPU = map[energy.DRMClient]map[string]energy.Engine{gpu: {"rcs": {Clock: energy.EngineNanoseconds, Busy: busy}}}
		return p
	}
	prev := sample(0, processes(tmux, 0, process(1, "zsh", 0), process(2, "poller", 0, thread(2, 0)), drawing(0)))
	cur := sample(time.Second, processes(tmux, 10*time.Millisecond, process(1, "zsh", 10*time.Millisecond), process(2, "poller", 0, thread(2, 1000)), drawing(0)))
	if a := app(t, energy.Attribute([]energy.Interval{measure(t, prev, cur)}), tmux); a.Command != "poller" {
		t.Errorf("command = %q, want poller, whose wakeups cost more than zsh's 10ms", a.Command)
	}
	cur = sample(time.Second, processes(tmux, 10*time.Millisecond, process(1, "zsh", 10*time.Millisecond), process(2, "poller", 0, thread(2, 1000)), drawing(5e8)))
	if a := app(t, energy.Attribute([]energy.Interval{measure(t, prev, cur)}), tmux); a.Command != "game" {
		t.Errorf("command = %q, want game, half a second on the GPU", a.Command)
	}
}

func TestAnIdleAppIsNamedAfterItsLowestPID(t *testing.T) {
	prev := sample(0, processes(tmux, 0, process(7, "b", 0), process(3, "a", 0)))
	cur := sample(time.Second, processes(tmux, 50*time.Millisecond, process(7, "b", 0), process(3, "a", 0)))
	if a := app(t, energy.Attribute([]energy.Interval{measure(t, prev, cur)}), tmux); a.Command != "a" {
		t.Errorf("command = %q, want a: with no process busier than another, the lowest PID", a.Command)
	}
}

func TestAReusedPIDIsANewProcessCountedFromZero(t *testing.T) {
	old := process(5, "exited", time.Second)
	reused := process(5, "reused", 3*time.Second)
	reused.ID.Start = 2
	prev := sample(0, processes(tmux, 0, old, process(6, "steady", 0)))
	cur := sample(time.Second, processes(tmux, time.Second, reused, process(6, "steady", 2500*time.Millisecond)))
	if a := app(t, energy.Attribute([]energy.Interval{measure(t, prev, cur)}), tmux); a.Command != "reused" {
		t.Errorf("command = %q, want reused: its 3s are its own, not 2s past the exited process it replaced", a.Command)
	}
}

func TestThreadRatesNameTheirProcess(t *testing.T) {
	prev := sample(0, processes(tmux, 0, process(1, "zsh", 0, thread(1, 0)), process(2, "claude", 0, thread(2, 0), thread(9, 0))))
	cur := sample(time.Second, processes(tmux, 0, process(1, "zsh", 0, thread(1, 3)), process(2, "claude", 0, thread(2, 40), thread(9, 7))))
	a := app(t, energy.Attribute([]energy.Interval{measure(t, prev, cur)}), tmux)
	want := []energy.ThreadRate{
		{PID: 2, TID: 2, Process: "claude", Name: "t", Wakeups: 40},
		{PID: 2, TID: 9, Process: "claude", Name: "t", Wakeups: 7},
		{PID: 1, TID: 1, Process: "zsh", Name: "t", Wakeups: 3},
	}
	if !slices.Equal(a.Threads, want) {
		t.Errorf("threads = %+v, want %+v", a.Threads, want)
	}
}

func TestAProgramStartedInATerminalIsTheJobToQuit(t *testing.T) {
	zsh := child(1, 0, 10, "zsh", 0)
	before := []energy.Process{zsh, child(2, 1, 20, "claude", 0), child(3, 2, 30, "bash", 0)}
	after := []energy.Process{zsh, child(2, 1, 20, "claude", 800*time.Millisecond), child(3, 2, 30, "bash", 100*time.Millisecond)}
	if job := jobOf(t, before, after); job == nil || *job != (energy.ProcessID{PID: 2, Start: 20}) {
		t.Errorf("job = %v, want claude, not the terminal's whole scope", job)
	}
}

func TestAToolAProgramRanIsItsOwnJob(t *testing.T) {
	zsh := child(1, 0, 10, "zsh", 0)
	before := []energy.Process{zsh, child(2, 1, 20, "claude", 0), child(3, 2, 30, "bash", 0), child(4, 3, 40, "php", 0)}
	after := []energy.Process{zsh, child(2, 1, 20, "claude", 100*time.Millisecond), child(3, 2, 30, "bash", 0), child(4, 3, 40, "php", 900*time.Millisecond)}
	if job := jobOf(t, before, after); job == nil || job.PID != 4 {
		t.Errorf("job = %v, want php, the program the row is named for, not claude which ran it", job)
	}
}

func TestAJobTakesInTheHelpersItsProgramForked(t *testing.T) {
	foot := child(1, 0, 10, "foot", 0)
	main := child(2, 1, 20, "chrome", 0)
	zygote := child(3, 2, 30, "chrome", 0)
	before := []energy.Process{foot, main, zygote, child(4, 3, 40, "chrome", 0)}
	after := []energy.Process{foot, main, zygote, child(4, 3, 40, "chrome", 900*time.Millisecond)}
	if job := jobOf(t, before, after); job == nil || job.PID != 2 {
		t.Errorf("job = %v, want chrome's first process, which its renderer and zygote descend from", job)
	}
}

func TestAnAppsOwnHelperIsNoJobOfItsOwn(t *testing.T) {
	firefox := child(1, 0, 10, "firefox", 0)
	before := []energy.Process{firefox, child(2, 1, 20, "firefox", 0)}
	after := []energy.Process{firefox, child(2, 1, 20, "firefox", 900*time.Millisecond)}
	if job := jobOf(t, before, after); job != nil {
		t.Errorf("job = %v, want none: a content process runs the app's own program, so quitting stops the app", job)
	}
}

func TestTheLeaderItselfIsNoJob(t *testing.T) {
	before := []energy.Process{child(1, 0, 10, "foot", 0), child(2, 1, 20, "zsh", 0)}
	after := []energy.Process{child(1, 0, 10, "foot", 900*time.Millisecond), child(2, 1, 20, "zsh", 0)}
	if job := jobOf(t, before, after); job != nil {
		t.Errorf("job = %v, want none: the busiest process is the one the unit was started for", job)
	}
}

func TestAnUnreadableProgramIsNoJob(t *testing.T) {
	zsh := child(1, 0, 10, "zsh", 0)
	hidden := child(2, 1, 20, "claude", 900*time.Millisecond)
	hidden.Exe = ""
	if job := jobOf(t, []energy.Process{zsh, child(2, 1, 20, "claude", 0)}, []energy.Process{zsh, hidden}); job != nil {
		t.Errorf("job = %v, want none without knowing which program it runs", job)
	}
}

func TestABusiestProcessThatHasExitedIsNoJob(t *testing.T) {
	zsh := child(1, 0, 10, "zsh", 0)
	a0 := sample(0, processes(tmux, 0, zsh, child(2, 1, 20, "claude", 0)))
	a1 := sample(time.Second, processes(tmux, time.Second, zsh, child(2, 1, 20, "claude", 900*time.Millisecond)))
	a2 := sample(2*time.Second, processes(tmux, 1100*time.Millisecond, zsh))
	a := app(t, energy.Attribute([]energy.Interval{measure(t, a0, a1), measure(t, a1, a2)}), tmux)
	if a.Command != "claude" || a.Job != nil {
		t.Errorf("command %q, job %v; want claude named and no job left to quit", a.Command, a.Job)
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
	gpu := func(engines map[energy.DRMClient]map[string]energy.Engine) energy.Group {
		p := process(1, "firefox", 0)
		p.GPU = engines
		return processes(firefox, 0, p)
	}
	prev := sample(0, gpu(map[energy.DRMClient]map[string]energy.Engine{
		ns:     {"gfx": {Clock: energy.EngineNanoseconds, Busy: 1e9}},
		cycles: {"rcs": {Clock: energy.EngineCycles, Busy: 100, Total: 1000}},
	}))
	cur := sample(2*time.Second, gpu(map[energy.DRMClient]map[string]energy.Engine{
		ns:     {"gfx": {Clock: energy.EngineNanoseconds, Busy: 1.5e9}},
		cycles: {"rcs": {Clock: energy.EngineCycles, Busy: 350, Total: 2000}},
	}))
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
