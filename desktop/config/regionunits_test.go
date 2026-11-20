package config

import "testing"

func TestATemperatureUnitFollowsTheRegionOnlyWhenSetTo(t *testing.T) {
	for _, c := range []struct {
		unit           TemperatureUnit
		region, wanted bool
	}{
		{TemperatureUnitRegion, true, true},
		{TemperatureUnitRegion, false, false},
		{TemperatureUnitMetric, true, false},
		{TemperatureUnitImperial, false, true},
	} {
		if got := c.unit.Imperial(c.region); got != c.wanted {
			t.Errorf("%s in an imperial region %v reads imperial %v", c.unit, c.region, got)
		}
	}
}

func TestATimeFormatFollowsTheRegionOnlyWhenSetTo(t *testing.T) {
	for _, c := range []struct {
		format         TimeFormat
		region, wanted bool
	}{
		{TimeFormatRegion, true, true},
		{TimeFormatRegion, false, false},
		{TimeFormat24h, true, false},
		{TimeFormat12h, false, true},
	} {
		if got := c.format.TwelveHour(c.region); got != c.wanted {
			t.Errorf("%s in a twelve-hour region %v reads twelve-hour %v", c.format, c.region, got)
		}
	}
}

func TestTheWeatherFollowsTheRegionByDefault(t *testing.T) {
	w := DefaultsWeather()
	if w.Units != TemperatureUnitRegion || w.TimeFormat != TimeFormatRegion {
		t.Errorf("the weather defaults to %s and %s, want both to follow the region", w.Units, w.TimeFormat)
	}
}
