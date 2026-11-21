package energy_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/stubbedev/lys/power/energy"
)

func withPanels(s energy.Sample, panels ...energy.PanelState) energy.Sample {
	s.Panels = panels
	return s
}

func TestResidencyIsTheShareOfReadingsSelfRefreshing(t *testing.T) {
	edp := func(refreshing bool) energy.PanelState {
		return energy.PanelState{Device: "0000:00:02.0", Mode: energy.SelfRefreshPSR2, Refreshing: refreshing}
	}
	off := energy.PanelState{Device: "0000:03:00.0", Mode: energy.SelfRefreshOff}
	samples := []energy.Sample{
		withPanels(sample(0), edp(false), off),
		withPanels(sample(time.Second), edp(true), off),
		withPanels(sample(2*time.Second), edp(true), off),
		withPanels(sample(3*time.Second), edp(false), off),
		withPanels(sample(4*time.Second), edp(true), off),
	}
	var intervals []energy.Interval
	for i := 1; i < len(samples); i++ {
		intervals = append(intervals, measure(t, samples[i-1], samples[i]))
	}
	want := []energy.Panel{
		{Device: "0000:00:02.0", Mode: energy.SelfRefreshPSR2, Readings: 4, Residency: 0.75},
		{Device: "0000:03:00.0", Mode: energy.SelfRefreshOff, Readings: 4, Residency: 0},
	}
	if got := energy.Attribute(intervals).Panels; !reflect.DeepEqual(got, want) {
		t.Errorf("panels = %+v, want %+v", got, want)
	}
}

func TestAPanelTakesItsLastMode(t *testing.T) {
	ivs := []energy.Interval{
		measure(t, sample(0), withPanels(sample(time.Second), energy.PanelState{Device: "0", Mode: energy.SelfRefreshPSR2, Refreshing: true})),
		measure(t, sample(time.Second), withPanels(sample(2*time.Second), energy.PanelState{Device: "0", Mode: energy.SelfRefreshOff})),
	}
	want := []energy.Panel{{Device: "0", Mode: energy.SelfRefreshOff, Readings: 2, Residency: 0.5}}
	if got := energy.Attribute(ivs).Panels; !reflect.DeepEqual(got, want) {
		t.Errorf("panels = %+v, want %+v", got, want)
	}
}

func TestWithoutPanelsTheReportHasNone(t *testing.T) {
	if got := energy.Attribute([]energy.Interval{measure(t, sample(0), sample(time.Second))}).Panels; got != nil {
		t.Errorf("panels = %+v, want none", got)
	}
}

func TestSelfRefreshModesRoundTrip(t *testing.T) {
	for s := energy.SelfRefreshUnsupported; s <= energy.SelfRefreshPanelReplaySU; s++ {
		got, err := energy.ParseSelfRefresh(s.String())
		if err != nil || got != s {
			t.Errorf("%s parsed as %s, %v", s, got, err)
		}
	}
	for _, bad := range []string{"", "PSR2", "psr3"} {
		if _, err := energy.ParseSelfRefresh(bad); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
	if _, err := energy.SelfRefresh(0).MarshalText(); err == nil {
		t.Error("the zero mode marshalled")
	}
}
