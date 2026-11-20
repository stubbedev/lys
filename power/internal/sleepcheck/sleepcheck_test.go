package sleepcheck

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/power/internal/supply"
	"github.com/stubbedev/lys/power/sleep"
)

var slept = time.Date(2026, 11, 20, 23, 0, 0, 0, time.UTC)

func snap(at time.Time, residency uint64, energy uint64) Snapshot {
	return Snapshot{At: at, OnBattery: true, Residency: &Residency{Counter: CounterLPIT, Micros: residency}, Energy: &energy, Wakeups: map[string]uint64{}}
}

func TestADeepSleepIsDeepWithItsDrainAndNothingToReport(t *testing.T) {
	woke := slept.Add(8 * time.Hour)
	before := snap(slept, 1_000_000, 50_000_000)
	after := snap(woke, 1_000_000+uint64((7*time.Hour+50*time.Minute).Microseconds()), 49_200_000)
	r := Assess(before, after)
	if r.Depth != sleep.DepthDeep {
		t.Fatalf("a sleep 98%% in the deepest state is %s", r.Depth)
	}
	if mw, ok := r.Drain.Milliwatts(); !ok || mw != 100 {
		t.Errorf("800 mWh over 8 h drew %d mW (known %v), want 100", mw, ok)
	}
	if mw, ok := r.Drained(); ok {
		t.Errorf("a deep sleep is worth a report of %d mW", mw)
	}
	if !r.Slept.Equal(slept) || !r.Woke.Equal(woke) || r.Asleep() != 8*time.Hour {
		t.Errorf("slept %s to %s", r.Slept, r.Woke)
	}
}

func TestAShallowSleepOnBatteryIsReportedWithItsDrainRate(t *testing.T) {
	before := snap(slept, 5_000_000, 50_000_000)
	before.Wakeups = map[string]uint64{"PNP0C0D:00": 3, "0000:00:14.0": 10, "ACPI0003:00": 1}
	after := snap(slept.Add(4*time.Hour), 5_000_000+uint64(time.Hour.Microseconds()), 44_000_000)
	after.Wakeups = map[string]uint64{"PNP0C0D:00": 4, "0000:00:14.0": 13, "ACPI0003:00": 1, "rtc0": 1}
	after.Blockers = []string{"USB2PLL_OFF_STS"}
	r := Assess(before, after)
	if r.Depth != sleep.DepthShallow || r.DeepShare != 0.25 {
		t.Fatalf("a sleep a quarter in the deepest state is %s at %v", r.Depth, r.DeepShare)
	}
	if mw, ok := r.Drained(); !ok || mw != 1500 {
		t.Errorf("Drained = %d, %v; want 6 Wh over 4 h, 1500 mW", mw, ok)
	}
	if want := []string{"0000:00:14.0", "PNP0C0D:00", "rtc0"}; !slices.Equal(r.WokenBy, want) {
		t.Errorf("woken by %v, want %v, the most often first", r.WokenBy, want)
	}
	if !slices.Equal(r.Blockers, after.Blockers) {
		t.Errorf("blockers %v", r.Blockers)
	}
}

func TestWhatCannotBeJudgedIsNeverReported(t *testing.T) {
	long := slept.Add(4 * time.Hour)
	shallow := uint64(time.Hour.Microseconds())
	for _, tc := range []struct {
		name          string
		before, after Snapshot
		depth         sleep.Depth
		drain         bool
	}{
		{"too short to judge", snap(slept, 0, 50_000_000), snap(slept.Add(sleep.MinSleep-time.Second), 0, 49_000_000), sleep.DepthUnmeasured, false},
		{"no residency counter", Snapshot{At: slept, OnBattery: true, Energy: new(uint64(50_000_000))}, Snapshot{At: long, OnBattery: true, Energy: new(uint64(40_000_000))}, sleep.DepthUnmeasured, true},
		{"the counter started over, as after hibernation", snap(slept, 9_000_000_000, 50_000_000), snap(long, 1_000, 49_000_000), sleep.DepthUnmeasured, true},
		{"counters from two sources", snap(slept, 0, 50_000_000), func() Snapshot {
			s := snap(long, shallow, 40_000_000)
			s.Residency.Counter = CounterSLPS0
			return s
		}(), sleep.DepthUnmeasured, true},
		{"slept on AC", func() Snapshot { s := snap(slept, 0, 50_000_000); s.OnBattery = false; return s }(), snap(long, shallow, 40_000_000), sleep.DepthShallow, false},
		{"charged while asleep", snap(slept, 0, 40_000_000), snap(long, shallow, 50_000_000), sleep.DepthShallow, false},
		{"no energy counter", Snapshot{At: slept, OnBattery: true, Residency: &Residency{CounterLPIT, 0}}, Snapshot{At: long, OnBattery: true, Residency: &Residency{CounterLPIT, shallow}}, sleep.DepthShallow, false},
		{"woke before it slept", snap(slept, 0, 50_000_000), snap(slept.Add(-time.Hour), shallow, 40_000_000), sleep.DepthUnmeasured, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := Assess(tc.before, tc.after)
			if r.Depth != tc.depth {
				t.Errorf("depth %s, want %s", r.Depth, tc.depth)
			}
			if _, ok := r.Drain.Milliwatts(); ok != tc.drain {
				t.Errorf("drain known %v, want %v", ok, tc.drain)
			}
			if mw, ok := r.Drained(); ok {
				t.Errorf("reported a drain of %d mW", mw)
			}
		})
	}
}

const tigerLakeSubstates = `                         Element |    S0i2.0 |    S0i3.0 |    S0i2.1 |    S0i3.1 |    S0i3.2 |    Status |
                 USB2PLL_OFF_STS |  Required |  Required |  Required |  Required |  Required |       Yes |
     PCIe/USB3.1_Gen2PLL_OFF_STS |  Required |  Required |  Required |  Required |  Required |           |
          PCIe_Gen3PLL_OFF_STS |  Required |  Required |  Required |  Required |  Required |       Yes |
                  OPIOPLL_OFF_STS |  Required |  Required |  Required |  Required |  Required |       Yes |
                      MAIN_PG_STS |           |           |           |           |           |           |
                   ISH_VNNAON_REQ |           |  Required |           |  Required |  Required |           |
`

func TestTheBlockersAreTheRequiredElementsLeftUnmet(t *testing.T) {
	got, err := ParseSubstateRequirements(tigerLakeSubstates)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"PCIe/USB3.1_Gen2PLL_OFF_STS", "ISH_VNNAON_REQ"}; !slices.Equal(got, want) {
		t.Errorf("blockers %v, want %v", got, want)
	}
	for _, bad := range []string{"", "Name | S0i3.0 |\n", "Element | S0i3.0 | Status |\nX | Required |\n"} {
		if got, err := ParseSubstateRequirements(bad); err == nil {
			t.Errorf("ParseSubstateRequirements(%q) = %v, want an error", bad, got)
		}
	}
}

func write(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, data := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestProbeReadsTheCountersTheMachineHas(t *testing.T) {
	sys, supplies := t.TempDir(), t.TempDir()
	write(t, sys, map[string]string{
		lpitPath:                            "123456\n",
		slpS0Path:                           "999\n",
		substatesPath:                       tigerLakeSubstates,
		"class/wakeup/wakeup0/name":         "PNP0C0D:00\n",
		"class/wakeup/wakeup0/wakeup_count": "2\n",
		"class/wakeup/wakeup1/name":         "0000:00:14.0\n",
		"class/wakeup/wakeup1/wakeup_count": "7\n",
	})
	write(t, supplies, map[string]string{"BAT0/type": "Battery", "BAT0/energy_now": "41000000\n", "AC/type": "Mains", "AC/online": "0\n"})
	got, err := Probe{Sys: sys, Supply: supply.Supply{Dir: supplies}}.Snapshot(slept)
	if err != nil {
		t.Fatal(err)
	}
	want := Snapshot{
		At: slept, OnBattery: true, Residency: &Residency{CounterLPIT, 123456}, Energy: new(uint64(41000000)),
		Wakeups:  map[string]uint64{"PNP0C0D:00": 2, "0000:00:14.0": 7},
		Blockers: []string{"PCIe/USB3.1_Gen2PLL_OFF_STS", "ISH_VNNAON_REQ"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Snapshot = %+v, want %+v", got, want)
	}

	if err := os.Remove(filepath.Join(sys, lpitPath)); err != nil {
		t.Fatal(err)
	}
	if got, err := (Probe{Sys: sys, Supply: supply.Supply{Dir: supplies}}).Snapshot(slept); err != nil || *got.Residency != (Residency{CounterSLPS0, 999}) {
		t.Errorf("without LPIT the residency is %+v, %v; want pmc_core's SLP_S0", got.Residency, err)
	}
}

func TestProbeOnAMachineWithoutCountersMeasuresNothing(t *testing.T) {
	supplies := t.TempDir()
	write(t, supplies, map[string]string{"AC/type": "Mains", "AC/online": "1\n"})
	got, err := Probe{Sys: t.TempDir(), Supply: supply.Supply{Dir: supplies}}.Snapshot(slept)
	if err != nil {
		t.Fatal(err)
	}
	if got.Residency != nil || got.Energy != nil || got.OnBattery || len(got.Wakeups) != 0 || got.Blockers != nil {
		t.Errorf("a desktop without counters reads %+v", got)
	}
}

func TestProbeFailsOnACounterItCannotRead(t *testing.T) {
	sys := t.TempDir()
	write(t, sys, map[string]string{lpitPath: "lots\n"})
	if _, err := (Probe{Sys: sys, Supply: supply.Supply{Dir: t.TempDir()}}).Snapshot(slept); err == nil || !strings.Contains(err.Error(), lpitPath) {
		t.Errorf("an unreadable residency counter = %v, want an error naming it", err)
	}
}

func TestTheHistoryKeepsTheLastFewSleeps(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sleeps")
	got, err := Load(path)
	if err != nil || got != nil {
		t.Fatalf("Load without a file = %v, %v", got, err)
	}
	var kept []sleep.Report
	for i := range sleep.Kept + 3 {
		r := Assess(snap(slept.Add(time.Duration(i)*24*time.Hour), 0, 50_000_000), snap(slept.Add(time.Duration(i)*24*time.Hour+time.Hour), 1000, 49_000_000))
		if kept, err = Record(path, kept, r); err != nil {
			t.Fatal(err)
		}
	}
	if len(kept) != sleep.Kept || !kept[0].Slept.Equal(slept.Add(3*24*time.Hour)) {
		t.Fatalf("kept %d sleeps from %s, want the last %d", len(kept), kept[0].Slept, sleep.Kept)
	}
	loaded, err := Load(path)
	if err != nil || !reflect.DeepEqual(loaded, kept) {
		t.Errorf("Load = %+v, %v; want %+v", loaded, err, kept)
	}
	if err := os.WriteFile(path, []byte(`[{"Depth":"sound"}]`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || !strings.Contains(err.Error(), `"sound"`) {
		t.Errorf("a report of no known depth = %v", err)
	}
}
