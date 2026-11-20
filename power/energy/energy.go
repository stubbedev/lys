// Package energy is lys-power's per-app energy attribution: the pure
// arithmetic that turns two samples of per-app counters (cgroup CPU time,
// per-thread context switches, DRM client GPU time) and the machine's
// measured power into a per-app energy estimate, and the system-bus
// contract that publishes it as the Report property of
// dev.stubbe.lys.Energy1, with its PropertiesChanged and the Watch method
// that asks for fast sampling while someone looks.
package energy

import (
	"fmt"
	"math"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/stubbedev/lys/lib/intconv"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

const (
	ServiceName = "dev.stubbe.lys.Energy1"
	ServicePath = dbus.ObjectPath("/dev/stubbe/lys/Energy")
	Interface   = "dev.stubbe.lys.Energy1"
	Property    = "Report"
	WatchMethod = Interface + ".Watch"
)

const (
	WatchLease        = 30 * time.Second
	WatchedInterval   = 2 * time.Second
	BatteryInterval   = time.Minute
	Span              = 5 * time.Minute
	WakeupCost        = 50 * time.Microsecond
	SignificantImpact = 0.05
	SignificantApps   = 5
)

type Source uint8

const (
	SourceNone Source = iota + 1
	SourcePackage
	SourceBattery
)

func (s Source) String() string {
	switch s {
	case SourceNone:
		return "none"
	case SourcePackage:
		return "package"
	case SourceBattery:
		return "battery"
	}
	return fmt.Sprintf("Source(%d)", uint8(s))
}

func (s Source) MarshalText() ([]byte, error) {
	if _, err := ParseSource(s.String()); err != nil {
		return nil, err
	}
	return []byte(s.String()), nil
}

func ParseSource(wire string) (Source, error) {
	switch wire {
	case "none":
		return SourceNone, nil
	case "package":
		return SourcePackage, nil
	case "battery":
		return SourceBattery, nil
	}
	return 0, fmt.Errorf("power source %q: want none, package or battery", wire)
}

type App struct {
	Cgroup  string
	Command string
	PIDs    []uint32
	CPU     float64
	Wakeups float64
	GPU     float64
	Impact  float64
	Watts   float64
	Threads []ThreadRate
}

func (a App) Unit() string { return path.Base(a.Cgroup) }

func (a App) UserUnit() (uid uint32, unit string, ok bool) {
	parts := strings.Split(a.Cgroup, "/")
	for i, p := range parts[:len(parts)-1] {
		id, found := strings.CutPrefix(p, "user@")
		if !found {
			continue
		}
		id, found = strings.CutSuffix(id, ".service")
		if !found || i+2 >= len(parts) {
			return 0, "", false
		}
		n, err := strconv.ParseUint(id, 10, 32)
		if err != nil {
			return 0, "", false
		}
		return uint32(n), parts[len(parts)-1], true
	}
	return 0, "", false
}

type ThreadRate struct {
	PID, TID      uint32
	Process, Name string
	Wakeups       float64
}

type Report struct {
	Covered time.Duration
	Source  Source
	Watts   float64
	Apps    []App
}

func (r Report) Measured() bool { return r.Covered > 0 }

func (r Report) Significant(n int) []App {
	out := make([]App, 0, n)
	for _, a := range r.Apps {
		if len(out) == n || a.Impact < SignificantImpact {
			break
		}
		out = append(out, a)
	}
	return out
}

type wireApp struct {
	Cgroup  string
	Command string
	PIDs    []uint32
	CPU     float64
	Wakeups float64
	GPU     float64
	Impact  float64
	Watts   float64
}

type wireReport struct {
	Covered uint64
	Source  string
	Watts   float64
	Apps    []wireApp
}

func toWire(r Report) wireReport {
	apps := make([]wireApp, len(r.Apps))
	for i, a := range r.Apps {
		apps[i] = wireApp{a.Cgroup, a.Command, a.PIDs, a.CPU, a.Wakeups, a.GPU, a.Impact, a.Watts}
	}
	return wireReport{Covered: intconv.Must[uint64](r.Covered.Microseconds()), Source: r.Source.String(), Watts: r.Watts, Apps: apps}
}

func fromWire(w wireReport) (Report, error) {
	source, err := ParseSource(w.Source)
	if err != nil {
		return Report{}, err
	}
	micros, err := intconv.Exact[int64](w.Covered)
	if err != nil || micros > math.MaxInt64/int64(time.Microsecond) {
		return Report{}, fmt.Errorf("report covering %d µs: longer than a duration holds", w.Covered)
	}
	apps := make([]App, len(w.Apps))
	for i, a := range w.Apps {
		apps[i] = App{Cgroup: a.Cgroup, Command: a.Command, PIDs: a.PIDs, CPU: a.CPU, Wakeups: a.Wakeups, GPU: a.GPU, Impact: a.Impact, Watts: a.Watts}
	}
	return Report{Covered: time.Duration(micros) * time.Microsecond, Source: source, Watts: w.Watts, Apps: apps}, nil
}
