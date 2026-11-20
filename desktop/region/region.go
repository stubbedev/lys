// Package region reads what the session's locale says about units and
// the clock, each from the locale glibc picks for its category.
package region

import (
	"cmp"
	"fmt"

	"github.com/stubbedev/lys/lib/glibclocale"
	"github.com/stubbedev/lys/lib/xdg"
)

// Prefs are the session region's units and clock.
type Prefs struct {
	// Imperial is a region measuring in US customary units.
	Imperial bool
	// TwelveHour is a region writing the time on a 12-hour clock.
	TwelveHour bool
}

func categoryLocale(env xdg.Env, category string) string {
	return cmp.Or(env("LC_ALL"), env(category), env("LANG"), "C")
}

// Read is the region of the locale env names, its data read from src.
func Read(src glibclocale.Source, env xdg.Env) (Prefs, error) {
	measurement := categoryLocale(env, "LC_MEASUREMENT")
	m, err := src.Measurement(measurement)
	if err != nil {
		return Prefs{}, fmt.Errorf("region: the measurement of %s: %w", measurement, err)
	}
	clock := categoryLocale(env, "LC_TIME")
	t, err := src.Time(clock)
	if err != nil {
		return Prefs{}, fmt.Errorf("region: the time of %s: %w", clock, err)
	}
	return Prefs{Imperial: m == glibclocale.USCustomary, TwelveHour: t.TwelveHour()}, nil
}
