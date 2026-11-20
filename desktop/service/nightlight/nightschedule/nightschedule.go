package nightschedule

import (
	"fmt"
	"time"
)

// MaxTransitionMinutes bounds an ease: past two hours it stops reading
// as a transition at a solar event.
const MaxTransitionMinutes = 120

// Transition is how long an ease between day and night lasts, in whole
// minutes, 0 for an instant switch. NewTransition is the only way to a
// non-zero one, so it never exceeds MaxTransitionMinutes.
type Transition struct{ minutes uint32 }

// NewTransition validates minutes against 0..MaxTransitionMinutes.
func NewTransition(minutes int64) (Transition, error) {
	if minutes < 0 || minutes > MaxTransitionMinutes {
		return Transition{}, fmt.Errorf("nightlight: transition of %d minutes is outside 0-%d", minutes, MaxTransitionMinutes)
	}
	return Transition{minutes: uint32(minutes)}, nil
}

// Minutes is the length in minutes.
func (t Transition) Minutes() uint32 { return t.minutes }

// Duration is the length as a duration.
func (t Transition) Duration() time.Duration { return time.Duration(t.minutes) * time.Minute }
