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
	TID      uint32
	Name     string
	Switches uint64
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

type ProcessID struct {
	PID   uint32
	Start uint64
}

type ProcessInfo struct {
	ID      ProcessID
	Command string
}

type Process struct {
	ProcessInfo
	CPU     time.Duration
	Threads []Thread
	GPU     map[DRMClient]map[string]Engine
}

type Group struct {
	Cgroup    string
	CPU       time.Duration
	Processes []Process
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

type Usage struct {
	CPU      time.Duration
	Switches uint64
	GPU      time.Duration
}

func (u Usage) busy() time.Duration {
	return u.CPU + time.Duration(float64(u.Switches)*float64(WakeupCost)) + u.GPU
}

func (u Usage) plus(o Usage) Usage {
	return Usage{CPU: u.CPU + o.CPU, Switches: u.Switches + o.Switches, GPU: u.GPU + o.GPU}
}

type ProcessInterval struct {
	ProcessInfo
	Usage   Usage
	Threads []Thread
}

type AppInterval struct {
	Cgroup    string
	CPU       time.Duration
	Processes []ProcessInterval
}

func (a AppInterval) usage() Usage {
	u := Usage{CPU: a.CPU}
	for _, p := range a.Processes {
		u.Switches += p.Usage.Switches
		u.GPU += p.Usage.GPU
	}
	return u
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
	cpu       map[string]time.Duration
	processes map[ProcessID]time.Duration
	threads   map[uint32]uint64
	engines   map[DRMClient]map[string]Engine
}

func index(s Sample) previous {
	p := previous{
		cpu:       make(map[string]time.Duration, len(s.Groups)),
		processes: make(map[ProcessID]time.Duration),
		threads:   make(map[uint32]uint64),
		engines:   make(map[DRMClient]map[string]Engine),
	}
	for _, g := range s.Groups {
		p.cpu[g.Cgroup] = g.CPU
		for _, proc := range g.Processes {
			p.processes[proc.ID] = proc.CPU
			for _, t := range proc.Threads {
				p.threads[t.TID] = t.Switches
			}
			maps.Copy(p.engines, proc.GPU)
		}
	}
	return p
}

func (p previous) since(g Group, span time.Duration) AppInterval {
	a := AppInterval{Cgroup: g.Cgroup, CPU: counterDelta(p.cpu[g.Cgroup], g.CPU), Processes: make([]ProcessInterval, 0, len(g.Processes))}
	for _, proc := range g.Processes {
		a.Processes = append(a.Processes, p.process(proc, span))
	}
	return a
}

func (p previous) process(proc Process, span time.Duration) ProcessInterval {
	pi := ProcessInterval{ProcessInfo: proc.ProcessInfo, Usage: Usage{CPU: counterDelta(p.processes[proc.ID], proc.CPU)}, Threads: make([]Thread, 0, len(proc.Threads))}
	for _, t := range proc.Threads {
		t.Switches = counterDelta(p.threads[t.TID], t.Switches)
		pi.Usage.Switches += t.Switches
		pi.Threads = append(pi.Threads, t)
	}
	for client, engines := range proc.GPU {
		before := p.engines[client]
		for name, e := range engines {
			pi.Usage.GPU += engineBusy(before[name], e, span)
		}
	}
	return pi
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

type processTotal struct {
	command string
	usage   Usage
	threads map[uint32]Thread
}

type appTotal struct {
	usage     Usage
	latest    []ProcessInterval
	processes map[ProcessID]*processTotal
	joules    float64
}

func (t *appTotal) add(a AppInterval, u Usage) {
	t.usage = t.usage.plus(u)
	t.latest = a.Processes
	for _, pi := range a.Processes {
		pt, ok := t.processes[pi.ID]
		if !ok {
			pt = &processTotal{threads: map[uint32]Thread{}}
			t.processes[pi.ID] = pt
		}
		pt.command = pi.Command
		pt.usage = pt.usage.plus(pi.Usage)
		for _, th := range pi.Threads {
			th.Switches += pt.threads[th.TID].Switches
			pt.threads[th.TID] = th
		}
	}
}

func (t *appTotal) busiest() (ProcessID, bool) {
	if len(t.processes) == 0 {
		return ProcessID{}, false
	}
	return slices.MinFunc(slices.Collect(maps.Keys(t.processes)), func(a, b ProcessID) int {
		return cmp.Or(
			cmp.Compare(t.processes[b].usage.busy(), t.processes[a].usage.busy()),
			cmp.Compare(a.PID, b.PID),
			cmp.Compare(a.Start, b.Start),
		)
	}), true
}

func (t *appTotal) pids() []uint32 {
	out := make([]uint32, len(t.latest))
	for i, p := range t.latest {
		out[i] = p.ID.PID
	}
	return out
}

func Attribute(intervals []Interval) Report {
	r := Report{Source: SourceNone}
	if len(intervals) == 0 {
		return r
	}
	r.Panels = panels(intervals)
	totals := map[string]*appTotal{}
	source := intervals[0].Source
	var joules float64
	for _, iv := range intervals {
		r.Covered += iv.Span
		joules += iv.Joules
		if iv.Source != source {
			source = SourceNone
		}
		usages := make([]Usage, len(iv.Apps))
		busy := iv.OtherCPU
		for i, a := range iv.Apps {
			usages[i] = a.usage()
			busy += usages[i].busy()
		}
		for i, a := range iv.Apps {
			t, ok := totals[a.Cgroup]
			if !ok {
				t = &appTotal{processes: map[ProcessID]*processTotal{}}
				totals[a.Cgroup] = t
			}
			t.add(a, usages[i])
			if busy > 0 {
				t.joules += iv.Joules * float64(usages[i].busy()) / float64(busy)
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
			PIDs:    t.pids(),
			CPU:     t.usage.CPU.Seconds() / seconds,
			Wakeups: float64(t.usage.Switches) / seconds,
			GPU:     t.usage.GPU.Seconds() / seconds,
			Impact:  t.usage.busy().Seconds() / seconds,
			Threads: threadRates(t.processes, seconds),
		}
		if app.Impact == 0 {
			continue
		}
		if busiest, ok := t.busiest(); ok {
			app.Command = t.processes[busiest].command
		}
		if source != SourceNone {
			app.Watts = t.joules / seconds
		}
		r.Apps = append(r.Apps, app)
	}
	slices.SortStableFunc(r.Apps, func(a, b App) int { return cmp.Compare(b.Impact, a.Impact) })
	return r
}

func threadRates(processes map[ProcessID]*processTotal, seconds float64) []ThreadRate {
	out := []ThreadRate{}
	for id, p := range processes {
		for _, t := range p.threads {
			if t.Switches == 0 {
				continue
			}
			out = append(out, ThreadRate{PID: id.PID, TID: t.TID, Process: p.command, Name: t.Name, Wakeups: float64(t.Switches) / seconds})
		}
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
