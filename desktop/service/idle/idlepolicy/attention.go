package idlepolicy

import (
	"fmt"
	"time"
)

// Presence is what a camera check before the dim saw.
type Presence uint8

// The presences: a face turned to the screen, nobody turned to it, or
// no answer this time (the camera busy, the session locked or in the
// background, every frame too dark), which dims as if no check ran.
const (
	Present Presence = iota + 1
	Absent
	Unseen
)

var presenceNames = map[Presence]string{Present: "present", Absent: "absent", Unseen: "unseen"}

func (p Presence) String() string {
	if name, ok := presenceNames[p]; ok {
		return name
	}
	return fmt.Sprintf("Presence(%d)", uint8(p))
}

// ParsePresence reads a presence as String spells it.
func ParsePresence(wire string) (Presence, error) {
	for p, name := range presenceNames {
		if name == wire {
			return p, nil
		}
	}
	return 0, fmt.Errorf("presence %q: want present, absent or unseen", wire)
}

// Attention is whether a camera check runs before the dim, and what an
// absent user changes.
type Attention uint8

// The attention modes: no check; a face turned to the screen postpones
// the dim; and, beside that, nobody there brings the lock and the
// screen-off stages within AbsentGrace of the dim.
const (
	AttentionOff Attention = iota
	AttentionPostpone
	AttentionSooner
)

// MaxPostponements is how many dim intervals in a row a face holds the
// dim off without input, so a face-shaped poster cannot hold it forever.
const MaxPostponements = 3

// AbsentGrace is the longest the lock and screen-off stages follow the
// dim after an absent check with AttentionSooner.
const AbsentGrace = 30 * time.Second

// Attending reports whether the dim on source runs after a camera check:
// attention is on and the dim is armed.
func (p Policy) Attending(source PowerSource) bool {
	_, armed := p.Timeouts[Dim].On(source).After()
	return p.Attention != AttentionOff && armed
}

// Follow is a stage that runs After the dim, once the dim ran.
type Follow struct {
	Stage Stage
	After time.Duration
}

// Watched are the stages that run on their own idle watch on source:
// every armed stage while not attending; while attending, the dim, the
// keyboard backlight and any stage the policy runs before the dim.
func (p Policy) Watched(source PowerSource) []Arm {
	arms := p.Arms(source)
	if !p.Attending(source) {
		return arms
	}
	watched := arms[:0:0]
	for _, a := range arms {
		if !p.follows(source, a) {
			watched = append(watched, a)
		}
	}
	return watched
}

// Followers are the stages that wait for the dim while attending on
// source, each at its own distance after the dim, kept when the dim is
// postponed. Hurried, the lock and screen-off stages come within
// AbsentGrace of the dim.
func (p Policy) Followers(source PowerSource, hurried bool) []Follow {
	if !p.Attending(source) {
		return nil
	}
	dim, _ := p.Timeouts[Dim].On(source).After()
	var out []Follow
	for _, a := range p.Arms(source) {
		if !p.follows(source, a) {
			continue
		}
		after := a.After - dim
		if hurried && (a.Stage == Lock || a.Stage == ScreenOff) {
			after = min(after, AbsentGrace)
		}
		out = append(out, Follow{Stage: a.Stage, After: after})
	}
	return out
}

func (p Policy) follows(source PowerSource, a Arm) bool {
	dim, _ := p.Timeouts[Dim].On(source).After()
	return a.Stage > Dim && a.After >= dim
}

// DimStep is what the dim does after a check: wait Postpone and check
// again, or, when Postpone is zero, dim now, Hurried bringing the
// following stages closer.
type DimStep struct {
	Postpone time.Duration
	Hurried  bool
}

// AtDim is the dim's step on source after a check saw seen, the dim
// having been postponed postponed times since the last input: a face
// postpones it one dim interval up to MaxPostponements times, nobody
// dims at once (hurried with AttentionSooner), and no answer dims as if
// no check ran.
func (p Policy) AtDim(source PowerSource, seen Presence, postponed int) DimStep {
	dim, _ := p.Timeouts[Dim].On(source).After()
	switch {
	case seen == Present && postponed < MaxPostponements:
		return DimStep{Postpone: dim}
	case seen == Absent:
		return DimStep{Hurried: p.Attention == AttentionSooner}
	}
	return DimStep{}
}
