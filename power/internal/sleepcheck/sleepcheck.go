// Package sleepcheck judges a sleep from two snapshots of the machine, one
// as it goes to sleep and one as it wakes: the platform's deepest-state
// residency counter, the battery's energy and the kernel's wakeup sources.
// Assess is pure; Probe reads the snapshots from sysfs and debugfs under
// an injected root.
package sleepcheck

import (
	"cmp"
	"slices"
	"time"

	"github.com/stubbedev/lys/lib/intconv"

	"github.com/stubbedev/lys/power/sleep"
)

// Counter is the residency counter a reading comes from.
type Counter uint8

// The counters, in the order Probe prefers them: the ACPI LPIT system
// residency, then Intel pmc_core's SLP_S0 residency in debugfs.
const (
	CounterLPIT Counter = iota + 1
	CounterSLPS0
)

// Residency is the time the platform spent in its deepest idle state
// since boot, as one counter reads it.
type Residency struct {
	Counter Counter
	Micros  uint64
}

// Snapshot is the machine at one moment. Residency and Energy are nil
// where the platform has no such counter; Energy is in µWh. Wakeups maps
// each wakeup source to its wakeup count. Blockers are the deepest
// state's requirements the platform reports unmet.
type Snapshot struct {
	At        time.Time
	OnBattery bool
	Residency *Residency
	Energy    *uint64
	Wakeups   map[string]uint64
	Blockers  []string
}

// Assess judges the sleep between before and after.
func Assess(before, after Snapshot) sleep.Report {
	r := sleep.Report{
		Slept:    before.At.UTC().Truncate(time.Microsecond),
		Woke:     after.At.UTC().Truncate(time.Microsecond),
		Depth:    sleep.DepthUnmeasured,
		Blockers: after.Blockers,
		WokenBy:  wokenBy(before.Wakeups, after.Wakeups),
	}
	asleep := r.Asleep()
	if asleep < sleep.MinSleep {
		return r
	}
	if share, ok := deepShare(before.Residency, after.Residency, asleep); ok {
		r.DeepShare = share
		r.Depth = sleep.DepthShallow
		if share >= sleep.DeepShare {
			r.Depth = sleep.DepthDeep
		}
	}
	if before.OnBattery && after.OnBattery && before.Energy != nil && after.Energy != nil && *after.Energy <= *before.Energy {
		r.Drain = sleep.Drew(drainMilliwatts(*before.Energy-*after.Energy, asleep))
	}
	return r
}

func deepShare(before, after *Residency, asleep time.Duration) (float64, bool) {
	if before == nil || after == nil || before.Counter != after.Counter || after.Micros < before.Micros {
		return 0, false
	}
	share := float64(after.Micros-before.Micros) / float64(asleep.Microseconds())
	return min(share, 1), true
}

func drainMilliwatts(microwattHours uint64, asleep time.Duration) uint64 {
	return microwattHours * uint64(time.Hour/time.Millisecond) / intconv.Must[uint64](asleep.Milliseconds()) / 1000
}

func wokenBy(before, after map[string]uint64) []string {
	type fired struct {
		name  string
		times uint64
	}
	var all []fired
	for name, n := range after {
		if prev := before[name]; n > prev {
			all = append(all, fired{name, n - prev})
		}
	}
	slices.SortFunc(all, func(a, b fired) int {
		return cmp.Or(cmp.Compare(b.times, a.times), cmp.Compare(a.name, b.name))
	})
	var names []string
	for _, f := range all {
		names = append(names, f.name)
	}
	return names
}
