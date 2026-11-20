package notifyview

import (
	"fmt"
	"strconv"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"
)

// AgeUnit is the unit an Age counts in.
type AgeUnit uint8

// The age units, finest first.
const (
	AgeJustNow AgeUnit = iota
	AgeMinutes
	AgeHours
	AgeDays
)

// Age is RelativeTime: under a minute is just now, under an hour is
// whole minutes, under a day whole hours, else whole days.
type Age struct {
	Unit  AgeUnit
	Count int64
}

// RelativeTime is the age of ts at now.
func RelativeTime(now, ts time.Time) Age {
	d := now.Sub(ts)
	switch {
	case d < time.Minute:
		return Age{Unit: AgeJustNow}
	case d < time.Hour:
		return Age{Unit: AgeMinutes, Count: int64(d / time.Minute)}
	case d < 24*time.Hour:
		return Age{Unit: AgeHours, Count: int64(d / time.Hour)}
	}
	return Age{Unit: AgeDays, Count: int64(d / (24 * time.Hour))}
}

// TimeLabel renders an age in a surface's own
// strings: domain is "notification-popup" or "notification-dropdown",
// whose -time-just-now, -time-minutes-ago, -time-hours-ago and
// -time-days-ago messages it reads.
func TimeLabel(age Age, domain string) string {
	count := strconv.FormatInt(age.Count, 10)
	switch age.Unit {
	case AgeJustNow:
		return i18n.T(domain + "-time-just-now")
	case AgeMinutes:
		return i18n.T(domain+"-time-minutes-ago", i18n.Str("minutes", count))
	case AgeHours:
		return i18n.T(domain+"-time-hours-ago", i18n.Str("hours", count))
	case AgeDays:
		return i18n.T(domain+"-time-days-ago", i18n.Str("days", count))
	}
	panic(fmt.Sprintf("notifyui: age unit %d has no label", age.Unit))
}
