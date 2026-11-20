package chargepolicy

import (
	"testing"
	"time"

	"github.com/stubbedev/lys/power/charge"
	"github.com/stubbedev/lys/power/internal/history"
	"github.com/stubbedev/lys/power/internal/supply"
)

var monday = time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)

func at(hour, minute int) time.Time {
	return monday.Add(time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute)
}

func unplugs(minute int, docked bool) []history.Entry {
	return []history.Entry{{Dow: 1, Minute: minute, Docked: docked}, {Dow: 1, Minute: minute, Docked: docked}, {Dow: 1, Minute: minute, Docked: docked}}
}

func onAC(now time.Time) Input {
	return Input{Now: now, Limit: charge.DefaultLimit, OnAC: true, Dock: charge.DockUnknown}
}

var full = supply.Thresholds{Start: 95, End: 100}

func TestEachLimitHoldsGapUnderIt(t *testing.T) {
	want := map[charge.Limit]supply.Thresholds{
		charge.Limit80:  {Start: 75, End: 80},
		charge.Limit85:  {Start: 80, End: 85},
		charge.Limit90:  {Start: 85, End: 90},
		charge.Limit95:  {Start: 90, End: 95},
		charge.Limit100: {Start: 95, End: 100},
	}
	for _, limit := range charge.Limits() {
		got := Decide(Input{Now: at(12, 0), Limit: limit, Dock: charge.DockUnknown})
		if got.Thresholds != want[limit] {
			t.Errorf("limit %d on battery: %+v, want %+v", limit, got.Thresholds, want[limit])
		}
		in := onAC(at(12, 0))
		in.Limit = limit
		if got := Decide(in); got.Thresholds != want[limit] {
			t.Errorf("limit %d on AC: %+v, want %+v", limit, got.Thresholds, want[limit])
		}
	}
}

func TestEachLimitChangeWritesInTheOrderTheControllerTakes(t *testing.T) {
	for _, from := range charge.Limits() {
		for _, to := range charge.Limits() {
			order := supply.Order(Thresholds(from).End, Thresholds(to).End)
			first := order[0]
			want := "charge_control_start_threshold"
			if to > from {
				want = "charge_control_end_threshold"
			}
			if first != want {
				t.Errorf("%d -> %d writes %s first, want %s", from, to, first, want)
			}
		}
	}
}

func TestAHundredTurnsTheHoldOff(t *testing.T) {
	in := onAC(at(12, 0))
	in.Limit = charge.Limit100
	got := Decide(in)
	if got.Thresholds != full || got.State.Reason != charge.ReasonUnlimited || !got.Recheck.IsZero() {
		t.Errorf("%+v", got)
	}
}

func TestOnBatteryTheLimitHoldsWhateverIsAhead(t *testing.T) {
	in := onAC(at(8, 0))
	in.OnAC, in.Requested, in.Departure, in.History = false, true, at(9, 0), unplugs(9*60, false)
	got := Decide(in)
	if got.Thresholds != Thresholds(charge.DefaultLimit) || got.State.Reason != charge.ReasonHolding || !got.Recheck.IsZero() {
		t.Errorf("%+v", got)
	}
}

func TestARequestChargesToFull(t *testing.T) {
	in := onAC(at(12, 0))
	in.Requested, in.Dock = true, charge.DockDocked
	if got := Decide(in); got.Thresholds != full || got.State.Reason != charge.ReasonRequested {
		t.Errorf("%+v", got)
	}
}

func TestACalendarEventOpensATopUpWindowWithoutAnyHistory(t *testing.T) {
	in := onAC(at(7, 30))
	in.Departure = at(9, 0)
	got := Decide(in)
	if got.Thresholds != full || got.State.Reason != charge.ReasonEvent || !got.State.FullBy.Equal(at(9, 0)) {
		t.Errorf("inside the window: %+v", got)
	}
	if !got.Recheck.Equal(at(9, 0)) {
		t.Errorf("recheck %v, want the event's start", got.Recheck)
	}

	in.Dock = charge.DockDocked
	if got := Decide(in); got.State.Reason != charge.ReasonEvent {
		t.Errorf("docked: %+v", got)
	}
}

func TestAnEventBeyondTheWindowHoldsUntilItOpens(t *testing.T) {
	in := onAC(at(6, 0))
	in.Departure = at(9, 0)
	got := Decide(in)
	if got.Thresholds != Thresholds(charge.DefaultLimit) || got.State.Reason != charge.ReasonHolding {
		t.Errorf("%+v", got)
	}
	if !got.State.FullBy.Equal(at(9, 0)) || !got.Recheck.Equal(at(7, 0)) {
		t.Errorf("full by %v, recheck %v; want 09:00 and the window opening at 07:00", got.State.FullBy, got.Recheck)
	}
}

func TestAnEventInThePastOpensNothing(t *testing.T) {
	for _, departure := range []time.Time{at(9, 0), at(8, 0)} {
		in := onAC(at(9, 0))
		in.Departure = departure
		got := Decide(in)
		if got.Thresholds != Thresholds(charge.DefaultLimit) || got.State.Reason != charge.ReasonHolding || !got.State.FullBy.IsZero() {
			t.Errorf("departure %v: %+v", departure, got)
		}
	}
}

func TestABareChargerIsTravelAndChargesToFull(t *testing.T) {
	in := onAC(at(22, 0))
	in.Dock = charge.DockUndocked
	got := Decide(in)
	if got.Thresholds != full || got.State.Reason != charge.ReasonTravel || got.State.Dock != charge.DockUndocked {
		t.Errorf("%+v", got)
	}
}

func TestDockedThePredictionAloneNeedsDockedUnplugs(t *testing.T) {
	in := onAC(at(16, 0))
	in.Dock, in.History = charge.DockDocked, unplugs(17*60, false)
	got := Decide(in)
	if got.Thresholds != Thresholds(charge.DefaultLimit) || got.State.Reason != charge.ReasonHolding || !got.State.FullBy.IsZero() {
		t.Errorf("unplugs from a bare charger: %+v", got)
	}

	in.History = unplugs(17*60, true)
	got = Decide(in)
	if got.Thresholds != full || got.State.Reason != charge.ReasonUnplug || !got.State.FullBy.Equal(at(17, 0)) {
		t.Errorf("unplugs from the dock: %+v", got)
	}
}

func TestDockedVersusUndockedChangesThePolicy(t *testing.T) {
	in := onAC(at(12, 0))
	in.Dock = charge.DockDocked
	docked := Decide(in)
	in.Dock = charge.DockUndocked
	undocked := Decide(in)
	if docked.Thresholds == undocked.Thresholds {
		t.Errorf("docked and undocked both write %+v", docked.Thresholds)
	}
}

func TestAnUnknownChargerFollowsEveryUnplug(t *testing.T) {
	in := onAC(at(16, 0))
	in.History = unplugs(17*60, false)
	got := Decide(in)
	if got.Thresholds != full || got.State.Reason != charge.ReasonUnplug {
		t.Errorf("inside the window: %+v", got)
	}

	in.Now = at(14, 0)
	got = Decide(in)
	if got.State.Reason != charge.ReasonHolding || !got.State.FullBy.Equal(at(17, 0)) || !got.Recheck.Equal(at(15, 0)) {
		t.Errorf("before the window: %+v", got)
	}
}

func TestHoldingFullByIsTheEarlierOfUnplugAndEvent(t *testing.T) {
	in := onAC(at(12, 0))
	in.History, in.Departure = unplugs(17*60, false), at(16, 0)
	if got := Decide(in); !got.State.FullBy.Equal(at(16, 0)) || !got.Recheck.Equal(at(14, 0)) {
		t.Errorf("event first: %+v", got)
	}
	in.Departure = at(18, 0)
	if got := Decide(in); !got.State.FullBy.Equal(at(17, 0)) || !got.Recheck.Equal(at(15, 0)) {
		t.Errorf("unplug first: %+v", got)
	}
}

func TestWithNothingAheadItHoldsUntilMidnight(t *testing.T) {
	got := Decide(onAC(at(12, 0)))
	if got.State.Reason != charge.ReasonHolding || !got.State.FullBy.IsZero() || !got.Recheck.Equal(at(24, 0)) {
		t.Errorf("%+v", got)
	}
}
