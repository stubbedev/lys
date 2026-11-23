package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/power/energy"
	"github.com/stubbedev/lys/power/internal/meter"
)

func TestEnergyOptions(t *testing.T) {
	o, err := parseEnergy([]string{"-for", "3s", "-threads", "-json"})
	if err != nil || o != (energyOptions{over: 3 * time.Second, threads: true, json: true}) {
		t.Errorf("options = %+v, %v", o, err)
	}
	if o, err := parseEnergy(nil); err != nil || o.over != 10*time.Second || o.threads || o.json {
		t.Errorf("defaults = %+v, %v", o, err)
	}
	for _, bad := range [][]string{{"-for", "0s"}, {"-for", "-1s"}, {"-for", "soon"}, {"-loud"}, {"firefox"}} {
		if _, err := parseEnergy(bad); !errors.Is(err, errUnknownCommand) {
			t.Errorf("%q: %v, want a usage error", bad, err)
		}
	}
}

type steps struct {
	samples []energy.Sample
	calls   int
	panels  []energy.PanelState
	polls   int
}

func (s *steps) Panels() ([]energy.PanelState, bool, error) {
	s.polls++
	if s.polls > len(s.panels) {
		return nil, false, errors.New("no more panel readings")
	}
	return []energy.PanelState{s.panels[s.polls-1]}, false, nil
}

func (s *steps) Sample(func() time.Time) (energy.Sample, meter.Gaps, error) {
	if s.calls == len(s.samples) {
		return energy.Sample{}, meter.Gaps{}, errors.New("no more samples")
	}
	s.calls++
	return s.samples[s.calls-1], meter.Gaps{GPUUnreadable: s.calls}, nil
}

func TestMeasureEnergyComparesTwoSamplesTheGivenTimeApart(t *testing.T) {
	at := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	s := &steps{samples: []energy.Sample{
		{At: at, Groups: []energy.Group{{Cgroup: "app.slice/a.scope", Processes: []energy.Process{{ID: energy.ProcessID{PID: 1}, Command: "a"}}}}},
		{At: at.Add(4 * time.Second), Groups: []energy.Group{{Cgroup: "app.slice/a.scope", CPU: 2 * time.Second, Processes: []energy.Process{{ID: energy.ProcessID{PID: 1}, Command: "a", CPU: 2 * time.Second}}}}},
	}, panels: make([]energy.PanelState, 15)}
	var slept time.Duration
	r, gaps, err := measureEnergy(s, 4*time.Second, func(d time.Duration) { slept += d })
	if err != nil {
		t.Fatal(err)
	}
	if slept != 4*time.Second || r.Covered != 4*time.Second || len(r.Apps) != 1 || r.Apps[0].CPU != 0.5 {
		t.Errorf("slept %s, report %+v", slept, r)
	}
	if gaps.GPUUnreadable != 2 {
		t.Errorf("gaps = %+v, want the second sample's", gaps)
	}
	if _, _, err := measureEnergy(&steps{samples: s.samples[:1], panels: make([]energy.PanelState, 3)}, time.Second, func(time.Duration) {}); err == nil {
		t.Error("a failed second sample measured")
	}
}

func TestMeasureEnergyReadsThePanelsThroughoutTheWindow(t *testing.T) {
	at := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	edp := func(refreshing bool) energy.PanelState {
		return energy.PanelState{Device: "0000:00:02.0", Mode: energy.SelfRefreshPSR2, Refreshing: refreshing}
	}
	s := &steps{
		samples: []energy.Sample{
			{At: at, Panels: []energy.PanelState{edp(false)}},
			{At: at.Add(time.Second), Panels: []energy.PanelState{edp(true)}},
		},
		panels: []energy.PanelState{edp(true), edp(true), edp(false)},
	}
	var sleeps []time.Duration
	r, _, err := measureEnergy(s, time.Second, func(d time.Duration) { sleeps = append(sleeps, d) })
	if err != nil {
		t.Fatal(err)
	}
	if want := []time.Duration{energy.PanelPoll, energy.PanelPoll, energy.PanelPoll, energy.PanelPoll}; !reflect.DeepEqual(sleeps, want) || s.polls != 3 {
		t.Errorf("slept %v and read the panels %d times, want %v and 3", sleeps, s.polls, want)
	}
	want := []energy.Panel{{Device: "0000:00:02.0", Mode: energy.SelfRefreshPSR2, Readings: 5, Residency: 0.6}}
	if !reflect.DeepEqual(r.Panels, want) {
		t.Errorf("panels = %+v, want %+v", r.Panels, want)
	}
	if _, _, err := measureEnergy(&steps{samples: s.samples}, time.Second, func(time.Duration) {}); err == nil {
		t.Error("a failed panel reading measured")
	}
}

var shown = energy.Report{Covered: 10 * time.Second, Source: energy.SourcePackage, Watts: 16.5, Apps: []energy.App{{
	Cgroup: "user.slice/user-1000.slice/user@1000.service/app.slice/tmux-spawn-1.scope", Command: "tmux: server",
	CPU: 10.6, Wakeups: 2300, GPU: 0, Impact: 11.06, Watts: 12.25,
	Threads: []energy.ThreadRate{{PID: 10, TID: 12, Process: "go", Name: "go", Wakeups: 760}},
}}, Panels: []energy.Panel{{Device: "0000:00:02.0", Mode: energy.SelfRefreshPSR2, Readings: 40, Residency: 0.925}}}

func TestEnergyTable(t *testing.T) {
	var out bytes.Buffer
	if err := writeEnergy(&out, shown, meter.Gaps{GPUUnreadable: 3}, energyOptions{threads: true}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"power:  16.50 W from the package over 10s",
		"GPU time of 3 processes is unreadable without root",
		"tmux: server  tmux-spawn-1.scope  12.25 W  1060.0%  2300.0     0.0%  1106.0%",
		"  go/go       10/12                                 760.0\n",
		"760.0",
		"panel:  0000:00:02.0 psr2, self-refreshing in 92.5% of 40 readings",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("the table lacks %q:\n%s", want, out.String())
		}
	}

	out.Reset()
	unmeasured := energy.Report{Covered: time.Second, Source: energy.SourceNone, Apps: shown.Apps}
	if err := writeEnergy(&out, unmeasured, meter.Gaps{EnergyUnreadable: true, PanelsUnreadable: true}, energyOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "not measured over 1s") || !strings.Contains(out.String(), "RAPL energy counters are root's to read") ||
		!strings.Contains(out.String(), "panel self refresh in the DRM debugfs is root's to read") || strings.Contains(out.String(), "panel:  ") || strings.Contains(out.String(), " W ") || strings.Contains(out.String(), "go/go") {
		t.Errorf("without power or threads:\n%s", out.String())
	}

	out.Reset()
	if err := writeEnergy(&out, energy.Report{Covered: time.Second, Source: energy.SourceNone}, meter.Gaps{}, energyOptions{}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "no app was busy") {
		t.Errorf("the empty table:\n%s", out.String())
	}
}

func TestEnergyJSON(t *testing.T) {
	var out bytes.Buffer
	if err := writeEnergy(&out, shown, meter.Gaps{}, energyOptions{json: true}); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Source string
		Panels []struct{ Mode string }
		Apps   []struct {
			Command string
			Threads []struct {
				TID     uint32
				Wakeups float64
			}
		}
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Source != "package" || len(got.Panels) != 1 || got.Panels[0].Mode != "psr2" || len(got.Apps) != 1 || got.Apps[0].Threads[0].TID != 12 || got.Apps[0].Threads[0].Wakeups != 760 {
		t.Errorf("json = %s", out.String())
	}
}
