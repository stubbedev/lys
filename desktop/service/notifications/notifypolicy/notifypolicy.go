package notifypolicy

import (
	"errors"
	"fmt"
	"time"

	"github.com/stubbedev/lys/desktop/daytime"
)

// DaySet is a set of weekdays.
type DaySet uint8

// Has reports whether d is in the set.
func (s DaySet) Has(d time.Weekday) bool { return s&(1<<d) != 0 }

// With is the set with d added.
func (s DaySet) With(d time.Weekday) DaySet { return s | 1<<d }

// ErrEmptyWindow is a schedule window that starts when it ends.
var ErrEmptyWindow = errors.New("a do-not-disturb window must end at another time than it starts")

// Window is one scheduled stretch of do-not-disturb: from From to To on
// each of Days, running past midnight into the next day when To is not
// after From.
type Window struct {
	days     DaySet
	from, to daytime.Time
}

// NewWindow validates a window on days from from to to.
func NewWindow(days DaySet, from, to daytime.Time) (Window, error) {
	if from == to {
		return Window{}, fmt.Errorf("%s-%s: %w", from, to, ErrEmptyWindow)
	}
	return Window{days: days, from: from, to: to}, nil
}

// Days are the days the window starts on.
func (w Window) Days() DaySet { return w.days }

// From is when the window starts.
func (w Window) From() daytime.Time { return w.from }

// To is when the window ends.
func (w Window) To() daytime.Time { return w.to }

func (w Window) overnight() bool { return !w.to.After(w.from) }

func (w Window) activeAt(t time.Time) bool {
	m := daytime.Of(t)
	day := t.Weekday()
	started, ended := !w.from.After(m), !w.to.After(m)
	if !w.overnight() {
		return w.days.Has(day) && started && !ended
	}
	return (w.days.Has(day) && started) || (w.days.Has((day+6)%7) && !ended)
}

// boundaries calls fn with each start and end of the window on the day
// of base, day days later.
func (w Window) boundaries(base time.Time, day int, fn func(time.Time)) {
	date := base.AddDate(0, 0, day)
	if !w.days.Has(date.Weekday()) {
		return
	}
	fn(w.from.On(date))
	if w.overnight() {
		fn(w.to.On(date.AddDate(0, 0, 1)))
		return
	}
	fn(w.to.On(date))
}

// FocusRules are the do-not-disturb rules: what still pops up while it is
// on, and what turns it on by itself.
type FocusRules struct {
	// AllowCritical lets critical notifications pop up through it.
	AllowCritical bool
	// AllowApps are app-name or desktop-entry globs that pop up through it.
	AllowApps []string
	// Schedule turns it on within each window, in the local time zone.
	Schedule []Window
	// WhileSharing turns it on while the screen is shared or recorded.
	WhileSharing bool
	// WhileFullscreen turns it on while a window is fullscreen.
	WhileFullscreen bool
}

// Scheduled reports whether t falls in a schedule window.
func (r FocusRules) Scheduled(t time.Time) bool {
	for _, w := range r.Schedule {
		if w.activeAt(t) {
			return true
		}
	}
	return false
}

// NextBoundary is the first start or end of a schedule window after t;
// false without a schedule.
func (r FocusRules) NextBoundary(t time.Time) (time.Time, bool) {
	var next time.Time
	for day := -1; day <= 7; day++ {
		for _, w := range r.Schedule {
			w.boundaries(t, day, func(b time.Time) {
				if b.After(t) && (next.IsZero() || b.Before(next)) {
					next = b
				}
			})
		}
	}
	return next, !next.IsZero()
}

// ErrUntil is a --until that is neither a time of day nor a duration.
var ErrUntil = errors.New("want a time of day as HH:MM or a duration such as 30m or 1h")

// ParseUntil reads when a do-not-disturb ends, from now: a time of day
// (HH:MM, its next occurrence in now's zone) or a positive duration.
func ParseUntil(raw string, now time.Time) (time.Time, error) {
	if at, err := daytime.Parse(raw); err == nil {
		until := at.On(now)
		if !until.After(now) {
			until = at.On(now.AddDate(0, 0, 1))
		}
		return until, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return time.Time{}, fmt.Errorf("until %q: %w", raw, ErrUntil)
	}
	return now.Add(d), nil
}

// History bounds: the notifications kept in memory, and on disk. The
// least kept on disk is the most kept in memory, so the in-memory
// window is always the newest part of what the store holds.
const (
	MinHistoryMemory     = 10
	MaxHistoryMemory     = 1000
	DefaultHistoryMemory = 200
	MinHistoryStored     = MaxHistoryMemory
	MaxHistoryStored     = 1_000_000
	DefaultHistoryStored = 10_000
)

// ErrHistoryLimit is a history bound outside its range.
var ErrHistoryLimit = errors.New("notification history limit out of range")

// MemoryLimit is how many of the newest notifications stay in memory,
// validated to [MinHistoryMemory, MaxHistoryMemory].
type MemoryLimit struct{ n int }

// StoredLimit is how many notifications the store keeps, validated to
// [MinHistoryStored, MaxHistoryStored].
type StoredLimit struct{ n int }

// NewMemoryLimit validates an in-memory bound.
func NewMemoryLimit(n int64) (MemoryLimit, error) {
	if err := checkLimit("in memory", n, MinHistoryMemory, MaxHistoryMemory); err != nil {
		return MemoryLimit{}, err
	}
	return MemoryLimit{n: int(n)}, nil
}

// NewStoredLimit validates an on-disk bound.
func NewStoredLimit(n int64) (StoredLimit, error) {
	if err := checkLimit("stored", n, MinHistoryStored, MaxHistoryStored); err != nil {
		return StoredLimit{}, err
	}
	return StoredLimit{n: int(n)}, nil
}

// DefaultMemoryLimit is DefaultHistoryMemory.
func DefaultMemoryLimit() MemoryLimit { return MemoryLimit{n: DefaultHistoryMemory} }

// DefaultStoredLimit is DefaultHistoryStored.
func DefaultStoredLimit() StoredLimit { return StoredLimit{n: DefaultHistoryStored} }

// Count is the bound.
func (l MemoryLimit) Count() int { return l.n }

// Count is the bound.
func (l StoredLimit) Count() int { return l.n }

func checkLimit(what string, n, lo, hi int64) error {
	if n < lo || n > hi {
		return fmt.Errorf("%w: %d notifications %s, valid %d-%d", ErrHistoryLimit, n, what, lo, hi)
	}
	return nil
}
