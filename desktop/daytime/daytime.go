// Package daytime is a time of day to the minute, written HH:MM, as the
// desktop's schedules store it.
package daytime

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

const (
	minutesPerHour = 60
	hoursPerDay    = 24
)

// Time is a time of day to the minute, 00:00 to 23:59.
type Time struct{ minute int }

// ErrInvalid is a time of day that is not HH:MM within a day.
var ErrInvalid = errors.New("not a time of day as HH:MM")

// Parse reads HH:MM, 00:00 to 23:59.
func Parse(raw string) (Time, error) {
	if len(raw) != 5 || raw[2] != ':' {
		return Time{}, fmt.Errorf("%q: %w", raw, ErrInvalid)
	}
	h, herr := strconv.Atoi(raw[:2])
	m, merr := strconv.Atoi(raw[3:])
	if herr != nil || merr != nil {
		return Time{}, fmt.Errorf("%q: %w", raw, ErrInvalid)
	}
	t, err := At(h, m)
	if err != nil {
		return Time{}, fmt.Errorf("%q: %w", raw, ErrInvalid)
	}
	return t, nil
}

// At is hour:minute, refusing one outside 00:00-23:59 with ErrInvalid.
func At(hour, minute int) (Time, error) {
	if hour < 0 || hour >= hoursPerDay || minute < 0 || minute >= minutesPerHour {
		return Time{}, fmt.Errorf("%d:%d: %w", hour, minute, ErrInvalid)
	}
	return Time{minute: hour*minutesPerHour + minute}, nil
}

// Of is the time of day clock shows, to the minute.
func Of(clock time.Time) Time {
	return Time{minute: clock.Hour()*minutesPerHour + clock.Minute()}
}

// Hour is the hour of the day, 0-23.
func (t Time) Hour() int { return t.minute / minutesPerHour }

// Minute is the minute of the hour, 0-59.
func (t Time) Minute() int { return t.minute % minutesPerHour }

// After reports whether t is later in the day than o.
func (t Time) After(o Time) bool { return t.minute > o.minute }

func (t Time) String() string {
	return fmt.Sprintf("%02d:%02d", t.Hour(), t.Minute())
}

// On is t on day's date, in day's zone.
func (t Time) On(day time.Time) time.Time {
	y, mo, d := day.Date()
	return time.Date(y, mo, d, t.Hour(), t.Minute(), 0, 0, day.Location())
}
