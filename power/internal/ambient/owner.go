package ambient

import (
	"maps"
	"slices"

	"github.com/stubbedev/lys/power/autobrightness"
)

// Level is one backlight's raw level as sysfs reads it back.
type Level struct {
	Name string
	Raw  uint32
}

// Presence is what logind's IdleHint says of the session as a change of
// the backlight is read: the idle policy sets the hint before it dims and
// clears it after it restores.
type Presence uint8

// The presences.
const (
	Attended Presence = iota + 1
	Idle
)

// Owner decides whether the backlight is still automatic brightness's
// to set. While following, a backlight at any level but the one last
// set (or adopted) is someone else's change, and pauses. A pause by a
// change that lowered the backlight while the session was Idle is the
// idle dim, an excursion that ends when every backlight is back at a
// level the last transition set: its restore puts back the level it
// found. Any further change while paused, a lower while Attended, a
// raise, or Resume confirm or end the pause: a manual change holds until
// Resume.
type Owner struct {
	keep   int
	state  autobrightness.State
	latest map[string]uint32
	trail  map[string][]uint32
	dip    bool
	seen   map[string]uint32
}

// NewOwner follows from the start, remembering the last keep levels set
// per backlight for recognising a restore.
func NewOwner(keep int) *Owner {
	return &Owner{
		keep:   max(keep, 1),
		state:  autobrightness.Following,
		latest: map[string]uint32{},
		trail:  map[string][]uint32{},
	}
}

// State is who holds the backlight.
func (o *Owner) State() autobrightness.State { return o.state }

// Observe takes the backlights as read, with the session's presence read
// after them, and reports whether the state changed. A backlight not seen
// before is adopted at its level.
func (o *Owner) Observe(levels []Level, presence Presence) bool {
	moved, lowered := false, true
	for _, l := range levels {
		latest, known := o.latest[l.Name]
		if !known {
			o.adopt(l)
			continue
		}
		if l.Raw == latest {
			continue
		}
		moved = true
		if l.Raw > latest {
			lowered = false
		}
	}
	switch o.state {
	case autobrightness.Following:
		if !moved {
			return false
		}
		o.state, o.dip, o.seen = autobrightness.Paused, lowered && presence == Idle, snapshot(levels)
		return true
	case autobrightness.Paused:
		if o.dip && o.restored(levels) {
			o.state, o.dip, o.seen = autobrightness.Following, false, nil
			for _, l := range levels {
				o.adopt(l)
			}
			return true
		}
		if !maps.Equal(o.seen, snapshot(levels)) {
			o.dip, o.seen = false, snapshot(levels)
		}
	}
	return false
}

// restored reports whether every backlight is at a level the last
// transition set.
func (o *Owner) restored(levels []Level) bool {
	for _, l := range levels {
		if !slices.Contains(o.trail[l.Name], l.Raw) {
			return false
		}
	}
	return true
}

// Wrote records the backlights as read back after a write of ours.
func (o *Owner) Wrote(levels []Level) {
	for _, l := range levels {
		o.latest[l.Name] = l.Raw
		trail := append(o.trail[l.Name], l.Raw)
		o.trail[l.Name] = trail[max(len(trail)-o.keep, 0):]
	}
}

// Resume hands the backlight back at the levels read, and reports
// whether the state changed.
func (o *Owner) Resume(levels []Level) bool {
	changed := o.state != autobrightness.Following
	o.state, o.dip, o.seen = autobrightness.Following, false, nil
	for _, l := range levels {
		o.adopt(l)
	}
	return changed
}

func (o *Owner) adopt(l Level) {
	o.latest[l.Name] = l.Raw
	o.trail[l.Name] = []uint32{l.Raw}
}

func snapshot(levels []Level) map[string]uint32 {
	out := make(map[string]uint32, len(levels))
	for _, l := range levels {
		out[l.Name] = l.Raw
	}
	return out
}
