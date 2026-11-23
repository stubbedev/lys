package settings

import (
	"errors"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/region"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func regionRow(t *testing.T, k *kit, page, path string) *regionHint {
	t.Helper()
	h, ok := fieldControl(k, k.store, pageRow(t, page, path)).(*regionHint)
	if !ok {
		t.Fatalf("%s says nothing of the region", path)
	}
	return h
}

// A field following the region says what the region makes of it, in
// the session's own terms, and says nothing once it is set by hand.
func TestAFieldFollowingTheRegionSaysWhatItResolvesTo(t *testing.T) {
	tr := locale.T()
	k := testKit(t, "")
	k.region = func() (region.Prefs, error) {
		return region.Prefs{Imperial: true, TwelveHour: true, WeekStart: time.Sunday}, nil
	}
	units := regionRow(t, k, "weather", "modules.weather.units")
	if !units.hint.Visible() || units.hint.Text() != tr.Get("settings-region-follows-fahrenheit") {
		t.Errorf("the units read %q (%v)", units.hint.Text(), units.hint.Visible())
	}
	clock := regionRow(t, k, "weather", "modules.weather.time-format")
	if clock.hint.Text() != tr.Get("settings-region-follows-12h") {
		t.Errorf("the clock reads %q", clock.hint.Text())
	}
	week := regionRow(t, k, dateTimeID, "modules.clock.calendar-weekday-start")
	day, err := sessionTimeLocale().Format("%A", aSunday)
	if err != nil {
		t.Fatal(err)
	}
	if want := tr.Get("settings-region-follows-week", i18n.Str("day", day)); week.hint.Text() != want {
		t.Errorf("the week reads %q, want %q", week.hint.Text(), want)
	}

	k.region = func() (region.Prefs, error) { return region.Prefs{WeekStart: time.Monday}, nil }
	units.refresh()
	clock.refresh()
	if units.hint.Text() != tr.Get("settings-region-follows-celsius") || clock.hint.Text() != tr.Get("settings-region-follows-24h") {
		t.Errorf("a metric 24-hour region reads %q and %q", units.hint.Text(), clock.hint.Text())
	}
	if err := k.store.set("modules.weather.units", "imperial"); err != nil {
		t.Fatal(err)
	}
	units.refresh()
	if units.hint.Visible() {
		t.Errorf("units set by hand still say %q", units.hint.Text())
	}
}

func TestAnUnreadableRegionIsSaid(t *testing.T) {
	k := testKit(t, "")
	k.region = func() (region.Prefs, error) { return region.Prefs{}, errors.New("no locale archive") }
	h := regionRow(t, k, "weather", "modules.weather.units")
	wantClosedFailure(t, "an unreadable region", h.hint.stateParts, h.hint.Text(), locale.T().Get("settings-region-unreadable"), "no locale archive")
}
