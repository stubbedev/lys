// Package appnap is App Nap for the lys session: it gathers what lys-wm,
// the sound server and the idle inhibitors say about each app, asks
// power/nap how hard the app naps, and applies the difference to the
// app's systemd unit and, through lys-power, to its threads' timer slack.
package appnap

import (
	"slices"
	"strings"

	"github.com/stubbedev/lys/lib/wmipc"
	"github.com/stubbedev/lys/power/nap"
)

// Hold is an idle inhibitor: the process holding it and, through the
// portal, the app ID it holds for.
type Hold struct {
	PID int32
	App string
}

// Inputs is what the session says about its apps at one moment.
type Inputs struct {
	Windows []wmipc.Window
	Units   map[int32]nap.Unit
	Streams []int32
	Holds   []Hold
	Levels  map[string]nap.Level
	Supply  nap.Supply
}

type app struct {
	activity nap.Activity
	level    nap.Level
}

var gentleness = []nap.Level{nap.LevelNever, nap.LevelDefault, nap.LevelEfficiencyCores, nap.LevelFrozen}

func gentler(a, b nap.Level) nap.Level {
	if slices.Index(gentleness, a) <= slices.Index(gentleness, b) {
		return a
	}
	return b
}

// Plan is the treatment of every app with a window whose unit is known.
func Plan(in Inputs) map[nap.Unit]nap.Treatment {
	apps := map[nap.Unit]*app{}
	for _, w := range in.Windows {
		u, ok := in.Units[w.PID]
		if !ok {
			continue
		}
		a := apps[u]
		if a == nil {
			a = &app{level: nap.LevelFrozen}
			apps[u] = a
		}
		a.activity.Windows++
		if !w.Asleep {
			a.activity.Seen++
		}
		a.activity.Focused = a.activity.Focused || w.Focused
		a.level = gentler(a.level, levelOf(in.Levels, w.AppID))
		for _, h := range in.Holds {
			if h.App != "" && strings.EqualFold(h.App, w.AppID) {
				a.activity.Inhibits = true
			}
		}
	}
	for _, pid := range in.Streams {
		if a := apps[in.Units[pid]]; a != nil {
			a.activity.Streams = true
		}
	}
	for _, h := range in.Holds {
		if a := apps[in.Units[h.PID]]; a != nil {
			a.activity.Inhibits = true
		}
	}
	out := make(map[nap.Unit]nap.Treatment, len(apps))
	for u, a := range apps {
		out[u] = nap.Decide(a.activity, a.level, in.Supply)
	}
	return out
}

func levelOf(levels map[string]nap.Level, appID string) nap.Level {
	if l, ok := levels[appID]; ok {
		return l
	}
	return nap.LevelDefault
}

// Change is one app's treatment going from one to another.
type Change struct {
	Unit     nap.Unit
	From, To nap.Treatment
}

// Changes takes the applied treatments to the next, waking apps that
// left the plan, in unit order.
func Changes(applied, next map[nap.Unit]nap.Treatment) []Change {
	var out []Change
	for u, to := range next {
		from, ok := applied[u]
		if !ok {
			from = nap.Awake
		}
		if from != to {
			out = append(out, Change{Unit: u, From: from, To: to})
		}
	}
	for u, from := range applied {
		if _, ok := next[u]; !ok && from != nap.Awake {
			out = append(out, Change{Unit: u, From: from, To: nap.Awake})
		}
	}
	slices.SortFunc(out, func(a, b Change) int { return strings.Compare(string(a.Unit), string(b.Unit)) })
	return out
}
