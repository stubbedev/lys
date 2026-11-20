package nightschedule

import (
	"errors"
	"fmt"
	"time"

	"github.com/stubbedev/lys/desktop/daytime"
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

// ErrEmptyHours is custom hours that start when they end.
var ErrEmptyHours = errors.New("nightlight: the custom hours must end at another time than they start")

// Hours is the stretch of each day the custom schedule keeps the filter
// on: from From to To, past midnight when To is not after From. NewHours
// is the only way to one, so it never starts when it ends.
type Hours struct{ from, to daytime.Time }

// NewHours validates the night from from to to.
func NewHours(from, to daytime.Time) (Hours, error) {
	if from == to {
		return Hours{}, fmt.Errorf("%s-%s: %w", from, to, ErrEmptyHours)
	}
	return Hours{from: from, to: to}, nil
}

// From is when the night starts.
func (h Hours) From() daytime.Time { return h.from }

// To is when the night ends.
func (h Hours) To() daytime.Time { return h.to }
