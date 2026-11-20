package bar

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/barview"

	"github.com/stubbedev/lys/shell/service/weather"
)

// osWrite writes one config variant for the load tests.
func osWrite(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}

func TestTheWorldClockButtonShowsAFailedFormatAndClearsIt(t *testing.T) {
	m := &worldClockModule{btn: newBarButton(newTestContext(t, config.Defaults()), nil, "")}
	failure := i18n.T("bar-world-clock-invalid-zone", i18n.Str("zone", "Mars/Olympus"))
	m.show(worldClockShown{failure: failure})
	if got, want := m.btn.Label().Text(), i18n.T("bar-module-failed"); got != want || !m.btn.HasClass(barview.FailedClass) {
		t.Errorf("a failed format shows %q (failed %v), want %q marked failed", got, m.btn.HasClass(barview.FailedClass), want)
	}
	if tip := m.btn.TooltipText(); !strings.Contains(tip, failure) {
		t.Errorf("the tooltip %q does not say %q", tip, failure)
	}
	m.show(worldClockShown{text: "14:05"})
	if got := m.btn.Label().Text(); got != "14:05" || m.btn.HasClass(barview.FailedClass) || strings.Contains(m.btn.TooltipText(), failure) {
		t.Errorf("a render after the failure shows %q, failed %v, tooltip %q", got, m.btn.HasClass(barview.FailedClass), m.btn.TooltipText())
	}
}

func TestWorldClockDefaultsMatchSchema(t *testing.T) {
	cfg := config.Defaults()
	if cfg.WorldClock.Format != "{{ tz('UTC', '%H:%M %Z') }}" {
		t.Errorf("format = %q", cfg.WorldClock.Format)
	}
	if cfg.Weather.Format != "{{ temp }}{{ temp_unit }}" || cfg.Weather.Location.Place().Name() != "San Francisco" || cfg.Weather.RefreshIntervalSeconds != 1800 {
		t.Errorf("weather = %+v", cfg.Weather)
	}
	if cfg.Weather.Units != config.TemperatureUnitRegion || cfg.Weather.TimeFormat != config.TimeFormatRegion {
		t.Errorf("units = %q, time format %q, want both to follow the region", cfg.Weather.Units, cfg.Weather.TimeFormat)
	}
	// The default world-clock left-click is empty; weather's opens its
	// dropdown.
	if cfg.WorldClock.Clicks().LeftClick.Kind != config.ClickNone {
		t.Errorf("world-clock left-click = %+v", cfg.WorldClock.Clicks().LeftClick)
	}
	if got := cfg.Weather.Clicks().LeftClick.String(); got != "dropdown:weather" {
		t.Errorf("weather left-click = %q", got)
	}
}

func TestWeatherFormatLabel(t *testing.T) {
	w := &weather.Weather{
		Zone: time.Local,
		Current: weather.Current{
			Temperature:   18.4,
			FeelsLike:     17.9,
			Condition:     weather.CondLightRain,
			Humidity:      62,
			WindSpeed:     12.3,
			WindDirection: 90,
		},
		Daily: []weather.Daily{{TempHigh: 21.6, TempLow: 12.25}},
	}
	// The default format: rounded temp plus the unit symbol.
	if got := weatherFormatLabel("{{ temp }}{{ temp_unit }}", w, false); got != "18°C" {
		t.Errorf("= %q, want 18°C", got)
	}
	// The full placeholder set.
	got := weatherFormatLabel("{{ condition }} {{ feels_like }}° {{ humidity }} {{ wind_speed }} {{ wind_dir }} {{ high }}/{{ low }}", w, false)
	want := i18n.T("weather-light-rain") + " 18° 62% 12 km/h E 22/12"
	if got != want {
		t.Errorf("= %q, want %q", got, want)
	}
	// Imperial converts temps and wind.
	if got := weatherFormatLabel("{{ temp }}{{ temp_unit }} {{ wind_speed }}", w, true); got != "65°F 8 mph" {
		t.Errorf("imperial = %q", got)
	}
	// Without a daily forecast the high and low render empty.
	w.Daily = nil
	if got := weatherFormatLabel("[{{ high }}]", w, false); got != "[]" {
		t.Errorf("no high = %q", got)
	}
}

func TestLoadFileAppliesWeatherAndWorldClock(t *testing.T) {
	path := t.TempDir() + "/config.yaml"
	good := "modules:\n  weather:\n    location:\n      name: Oslo\n      latitude: 59.91\n      longitude: 10.75\n    units: imperial\n    refresh-interval-seconds: 60\n  world-clock:\n    format: \"{{ tz('Europe/Oslo', '%H:%M') }}\"\n"
	if err := osWrite(path, good); err != nil {
		t.Fatal(err)
	}
	c, err := config.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if c.Weather.Location.Place().Name() != "Oslo" || c.Weather.Units != config.TemperatureUnitImperial || c.Weather.RefreshIntervalSeconds != 60 {
		t.Errorf("weather = %+v", c.Weather)
	}
	if c.WorldClock.Format != "{{ tz('Europe/Oslo', '%H:%M') }}" {
		t.Errorf("world-clock = %+v", c.WorldClock)
	}

	for _, bad := range []string{
		"modules:\n  weather:\n    units: kelvin\n",
		"modules:\n  weather:\n    refresh-interval-seconds: -1\n",
		"modules:\n  weather:\n    location: Oslo\n",
	} {
		if err := osWrite(path, bad); err != nil {
			t.Fatal(err)
		}
		if _, err := config.LoadFile(path); err == nil {
			t.Errorf("%q: want a load error", bad)
		}
	}
}

// Every condition maps to its own _weather.ftl message; a value
// outside the enum falls back to the unknown label, never a missing-id
// marker.
func TestWeatherConditionLabels(t *testing.T) {
	seen := map[string]weather.Condition{}
	for c := weather.CondClear; c <= weather.CondUnknown; c++ {
		id, ok := weatherConditionIDs[c]
		if !ok {
			t.Fatalf("condition %d has no message id", c)
		}
		if prev, dup := seen[id]; dup {
			t.Errorf("conditions %d and %d share %s", prev, c, id)
		}
		seen[id] = c
		if got := weatherConditionLabel(c); got != i18n.T(id) || strings.HasPrefix(got, "No localization") {
			t.Errorf("condition %d = %q", c, got)
		}
	}
	if got := weatherConditionLabel(weather.CondUnknown + 1); got != i18n.T("weather-unknown") {
		t.Errorf("out-of-range condition = %q", got)
	}
}
