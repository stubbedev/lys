// Package pauser runs lys-power's pausing of chosen units on battery:
// the pure decision of which units to pause and resume, the record of
// what it paused that survives a restart, and the systemd managers it
// freezes, thaws, stops and starts them through.
package pauser

import (
	"slices"

	"github.com/stubbedev/lys/power/pause"
)

type Plan struct {
	Pause  []pause.Unit
	Resume []pause.Unit
}

func Holding(onBattery, resumed bool) bool { return onBattery && !resumed }

func Decide(picks, paused []pause.Unit, holding bool, running map[pause.Unit]bool) Plan {
	var p Plan
	for _, u := range paused {
		if !holding || !slices.Contains(picks, u) {
			p.Resume = append(p.Resume, u)
		}
	}
	if !holding {
		return p
	}
	for _, u := range picks {
		if running[u] && !slices.Contains(paused, u) {
			p.Pause = append(p.Pause, u)
		}
	}
	return p
}

func Candidates(picks, paused []pause.Unit, holding bool) []pause.Unit {
	if !holding {
		return nil
	}
	var out []pause.Unit
	for _, u := range picks {
		if !slices.Contains(paused, u) {
			out = append(out, u)
		}
	}
	return out
}
