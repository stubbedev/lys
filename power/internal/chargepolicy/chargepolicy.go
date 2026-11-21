// Package chargepolicy decides what the battery charges to: the user's
// limit, or full for a request, a calendar event away from the desk, or
// the usual unplug. It is pure; the daemon gathers its
// input and writes its decision.
package chargepolicy

import (
	"time"

	"github.com/stubbedev/lys/power/charge"
	"github.com/stubbedev/lys/power/internal/config"
	"github.com/stubbedev/lys/power/internal/history"
	"github.com/stubbedev/lys/power/internal/predict"
	"github.com/stubbedev/lys/power/internal/supply"
)

// Window is how long before a departure the battery charges to full.
const Window = time.Duration(config.TopupWindowMin) * time.Minute

// Input is everything the decision depends on. Departure is the start of
// the next calendar event away from the desk, zero when none is known.
type Input struct {
	Now       time.Time
	Limit     charge.Limit
	OnAC      bool
	Requested bool
	Dock      charge.Dock
	History   []history.Entry
	Departure time.Time
}

// Decision is the thresholds to write, the state to publish, and when the
// decision next changes with time alone, zero when only an event can
// change it.
type Decision struct {
	Thresholds supply.Thresholds
	State      charge.State
	Recheck    time.Time
}

// Thresholds holds the battery at limit, resuming Gap under it.
func Thresholds(limit charge.Limit) supply.Thresholds {
	return supply.Thresholds{Start: limit.Resume(), End: limit.Percent()}
}

// Decide is the charge policy. With the hold off, on battery or with a
// full charge asked for, nothing else matters. On AC an event away from
// the desk opens a top-up window before it; otherwise the battery holds
// unless the unplugs recorded predict a departure within the window,
// whatever the charger. Docked, only unplugs from the dock count.
func Decide(in Input) Decision {
	state := charge.State{Limit: in.Limit, Dock: in.Dock}
	decide := func(reason charge.Reason, fullBy, recheck time.Time) Decision {
		state.Reason, state.FullBy = reason, fullBy
		thresholds := Thresholds(in.Limit)
		if reason.Full() {
			thresholds = Thresholds(charge.Limit100)
		}
		return Decision{Thresholds: thresholds, State: state, Recheck: recheck}
	}
	switch {
	case !in.Limit.Holds():
		return decide(charge.ReasonUnlimited, time.Time{}, time.Time{})
	case !in.OnAC:
		return decide(charge.ReasonHolding, time.Time{}, time.Time{})
	case in.Requested:
		return decide(charge.ReasonRequested, time.Time{}, time.Time{})
	}

	year, month, day := in.Now.Date()
	today := func(minute int) time.Time {
		return time.Date(year, month, day, minute/60, minute%60, 0, 0, in.Now.Location())
	}
	recheck := time.Date(year, month, day+1, 0, 0, 0, 0, in.Now.Location())

	event := in.Departure.After(in.Now)
	if event {
		opens := in.Departure.Add(-Window)
		if !opens.After(in.Now) {
			return decide(charge.ReasonEvent, in.Departure, in.Departure)
		}
		recheck = earliest(recheck, opens)
	}
	pool := in.History
	if in.Dock == charge.DockDocked {
		pool = docked(in.History)
	}
	at := history.At(in.Now)
	if minute, ok := predict.NextChange(pool, at, config.MinSamples, config.TopupWindowMin); ok {
		recheck = earliest(recheck, today(minute))
	}
	var fullBy time.Time
	if predicted, ok := predict.Next(pool, at, config.MinSamples); ok {
		fullBy = today(predicted)
	}
	if predict.TopUp(pool, at, config.MinSamples, config.TopupWindowMin) {
		return decide(charge.ReasonUnplug, fullBy, recheck)
	}
	if event && (fullBy.IsZero() || in.Departure.Before(fullBy)) {
		fullBy = in.Departure
	}
	return decide(charge.ReasonHolding, fullBy, recheck)
}

func docked(entries []history.Entry) []history.Entry {
	out := make([]history.Entry, 0, len(entries))
	for _, e := range entries {
		if e.Docked {
			out = append(out, e)
		}
	}
	return out
}

func earliest(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}
