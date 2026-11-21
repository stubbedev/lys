package energy

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"
)

type Thread struct {
	PID, TID      uint32
	Process, Name string
	Switches      uint64
}

type EngineClock uint8

const (
	EngineNanoseconds EngineClock = iota + 1
	EngineCycles
)

type Engine struct {
	Clock EngineClock
	Busy  uint64
	Total uint64
}

type DRMClient struct {
	Device string
	ID     uint64
}

type Group struct {
	Cgroup  string
	Command string
	PIDs    []uint32
	CPU     time.Duration
	Threads []Thread
	GPU     map[DRMClient]map[string]Engine
}

type Counter struct {
	Microjoules uint64
	Range       uint64
}

type Sample struct {
	At       time.Time
	TotalCPU time.Duration
	Groups   []Group
	Energy   []Counter
	Battery  *uint64
	Panels   []PanelState
}

type Interval struct {
	End      time.Time
	Span     time.Duration
	Source   Source
	Joules   float64
	OtherCPU time.Duration
	Apps     []AppInterval
	Panels   []PanelState
}

type AppInterval struct {
	Cgroup   string
	Command  string
	PIDs     []uint32
	CPU      time.Duration
	Switches uint64
	GPU      time.Duration
	Threads  []Thread
}

func (a AppInterval) busy() time.Duration {
	return a.CPU + time.Duration(float64(a.Switches)*float64(WakeupCost)) + a.GPU
}

var ErrNotLater = errors.New("the sample is not later than the one before")

func Measure(prev, cur Sample) (Interval, error) {
	span := cur.At.Sub(prev.At)
	if span <= 0 {
		return Interval{}, fmt.Errorf("measure from %s to %s: %w", prev.At, cur.At, ErrNotLater)
	}
	source, joules, err := measuredEnergy(prev, cur, span)
	if err != nil {
		return Interval{}, err
	}
	before := index(prev)
	apps := make([]AppInterval, 0, len(cur.Groups))
	var appCPU time.Duration
	for _, g := range cur.Groups {
		a := before.since(g, span)
		appCPU += a.CPU
		apps = append(apps, a)
	}
	slices.SortFunc(apps, func(a, b AppInterval) int { return cmp.Compare(a.Cgroup, b.Cgroup) })
	return Interval{
		End:      cur.At,
		Span:     span,
		Source:   source,
		Joules:   joules,
		OtherCPU: max(counterDelta(prev.TotalCPU, cur.TotalCPU)-appCPU, 0),
		Apps:     apps,
		Panels:   cur.Panels,
	}, nil
}

type previous struct {
	cpu     map[string]time.Duration
	threads map[uint32]uint64
	engines map[DRMClient]map[string]Engine
}

func index(s Sample) previous {
	p := previous{
		cpu:     make(map[string]time.Duration, len(s.Groups)),
		threads: make(map[uint32]uint64),
		engines: make(map[DRMClient]map[string]Engine),
	}
	for _, g := range s.Groups {
		p.cpu[g.Cgroup] = g.CPU
		for _, t := range g.Threads {
			p.threads[t.TID] = t.Switches
		}
		maps.Copy(p.engines, g.GPU)
	}
	return p
}

func (p previous) since(g Group, span time.Duration) AppInterval {
	a := AppInterval{Cgroup: g.Cgroup, Command: g.Command, PIDs: g.PIDs, CPU: counterDelta(p.cpu[g.Cgroup], g.CPU)}
	a.Threads = make([]Thread, 0, len(g.Threads))
	for _, t := range g.Threads {
		t.Switches = counterDelta(p.threads[t.TID], t.Switches)
		a.Switches += t.Switches
		a.Threads = append(a.Threads, t)
	}
	for client, engines := range g.GPU {
		before := p.engines[client]
		for name, e := range engines {
			a.GPU += engineBusy(before[name], e, span)
		}
	}
	return a
}

func counterDelta[T time.Duration | uint64](prev, cur T) T {
	if cur < prev {
		return cur
	}
	return cur - prev
}

func engineBusy(prev, cur Engine, span time.Duration) time.Duration {
	if prev.Clock != cur.Clock {
		prev = Engine{Clock: cur.Clock}
	}
	busy := counterDelta(prev.Busy, cur.Busy)
	switch cur.Clock {
	case EngineNanoseconds:
		return time.Duration(min(float64(busy), float64(span)))
	case EngineCycles:
		total := counterDelta(prev.Total, cur.Total)
		if total == 0 {
			return 0
		}
		return time.Duration(float64(span) * min(float64(busy)/float64(total), 1))
	}
	return 0
}

func measuredEnergy(prev, cur Sample, span time.Duration) (Source, float64, error) {
	if len(prev.Energy) > 0 && len(prev.Energy) == len(cur.Energy) {
		var microjoules uint64
		for i, c := range cur.Energy {
			d, err := wrapped(prev.Energy[i], c)
			if err != nil {
				return 0, 0, err
			}
			microjoules += d
		}
		return SourcePackage, float64(microjoules) / 1e6, nil
	}
	if prev.Battery != nil && cur.Battery != nil {
		microwatts := (float64(*prev.Battery) + float64(*cur.Battery)) / 2
		return SourceBattery, microwatts / 1e6 * span.Seconds(), nil
	}
	return SourceNone, 0, nil
}

func wrapped(prev, cur Counter) (uint64, error) {
	if cur.Range == 0 || prev.Range != cur.Range || prev.Microjoules > cur.Range || cur.Microjoules > cur.Range {
		return 0, fmt.Errorf("energy counter %d then %d µJ in a range of %d then %d µJ: inconsistent", prev.Microjoules, cur.Microjoules, prev.Range, cur.Range)
	}
	if cur.Microjoules >= prev.Microjoules {
		return cur.Microjoules - prev.Microjoules, nil
	}
	return cur.Range - prev.Microjoules + cur.Microjoules, nil
}

func Attribute(intervals []Interval) Report {
	r := Report{Source: SourceNone}
	if len(intervals) == 0 {
		return r
	}
	r.Panels = panels(intervals)
	type total struct {
		app     AppInterval
		threads map[uint32]Thread
		joules  float64
	}
	totals := map[string]*total{}
	source := intervals[0].Source
	var joules float64
	for _, iv := range intervals {
		r.Covered += iv.Span
		joules += iv.Joules
		if iv.Source != source {
			source = SourceNone
		}
		busy := iv.OtherCPU
		for _, a := range iv.Apps {
			busy += a.busy()
		}
		for _, a := range iv.Apps {
			t, ok := totals[a.Cgroup]
			if !ok {
				t = &total{app: AppInterval{Cgroup: a.Cgroup}, threads: map[uint32]Thread{}}
				totals[a.Cgroup] = t
			}
			t.app.Command, t.app.PIDs = a.Command, a.PIDs
			t.app.CPU += a.CPU
			t.app.Switches += a.Switches
			t.app.GPU += a.GPU
			for _, th := range a.Threads {
				th.Switches += t.threads[th.TID].Switches
				t.threads[th.TID] = th
			}
			if busy > 0 {
				t.joules += iv.Joules * float64(a.busy()) / float64(busy)
			}
		}
	}
	r.Source = source
	seconds := r.Covered.Seconds()
	if source != SourceNone {
		r.Watts = joules / seconds
	}
	for _, unit := range slices.Sorted(maps.Keys(totals)) {
		t := totals[unit]
		app := App{
			Cgroup:  unit,
			Command: t.app.Command,
			PIDs:    t.app.PIDs,
			CPU:     t.app.CPU.Seconds() / seconds,
			Wakeups: float64(t.app.Switches) / seconds,
			GPU:     t.app.GPU.Seconds() / seconds,
			Impact:  t.app.busy().Seconds() / seconds,
			Threads: threadRates(t.threads, seconds),
		}
		if app.Impact == 0 {
			continue
		}
		if source != SourceNone {
			app.Watts = t.joules / seconds
		}
		r.Apps = append(r.Apps, app)
	}
	slices.SortStableFunc(r.Apps, func(a, b App) int { return cmp.Compare(b.Impact, a.Impact) })
	return r
}

func threadRates(threads map[uint32]Thread, seconds float64) []ThreadRate {
	out := make([]ThreadRate, 0, len(threads))
	for _, t := range threads {
		if t.Switches == 0 {
			continue
		}
		out = append(out, ThreadRate{PID: t.PID, TID: t.TID, Process: t.Process, Name: t.Name, Wakeups: float64(t.Switches) / seconds})
	}
	slices.SortFunc(out, func(a, b ThreadRate) int {
		return cmp.Or(cmp.Compare(b.Wakeups, a.Wakeups), cmp.Compare(a.TID, b.TID))
	})
	return out
}

func Trim(intervals []Interval, now time.Time, span time.Duration) []Interval {
	from := now.Add(-span)
	i := slices.IndexFunc(intervals, func(iv Interval) bool { return iv.End.After(from) })
	if i < 0 {
		return intervals[:0]
	}
	return intervals[i:]
}
