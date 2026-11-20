package config

import (
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/service/nightlight/nightschedule"
)

func TestNightlightTransitionDefaultsToAQuarterHour(t *testing.T) {
	if got := DefaultsNightlight().TransitionDuration.Transition().Minutes(); got != 15 {
		t.Errorf("default = %d min, want 15", got)
	}
}

func TestNightlightTransitionHoldsItsRange(t *testing.T) {
	for _, minutes := range []int64{0, 5, nightschedule.MaxTransitionMinutes} {
		path := writeFile(t, t.TempDir(), "config.yaml", "modules:\n  nightlight:\n    transition-duration: "+strconv.FormatInt(minutes, 10)+"\n")
		cfg, err := LoadFile(path)
		if err != nil {
			t.Fatalf("%d minutes: %v", minutes, err)
		}
		if got := cfg.Nightlight.TransitionDuration.Transition().Minutes(); int64(got) != minutes {
			t.Errorf("%d minutes loaded as %d", minutes, got)
		}
	}
	for _, minutes := range []int64{nightschedule.MaxTransitionMinutes + 1, -1} {
		path := writeFile(t, t.TempDir(), "config.yaml", "modules:\n  nightlight:\n    transition-duration: "+strconv.FormatInt(minutes, 10)+"\n")
		_, err := LoadFile(path)
		if err == nil || !strings.Contains(err.Error(), "Field: modules.nightlight.transition-duration") {
			t.Errorf("%d minutes: err = %v, want the field's range diagnostic", minutes, err)
		}
	}
}

func TestNightlightTransitionIsABoundedSpin(t *testing.T) {
	m, ok := Field("modules.nightlight.transition-duration")
	if !ok || m.Kind != FieldInt || m.Min != 0 || m.Max != nightschedule.MaxTransitionMinutes {
		t.Errorf("meta = %+v, want an int field in 0-%d", m, nightschedule.MaxTransitionMinutes)
	}
}

// The section's former key still loads, with a warning naming the
// replacement.
func TestNightlightReadsTheHyprsunsetKey(t *testing.T) {
	path := writeFile(t, t.TempDir(), "config.yaml", "modules:\n  hyprsunset:\n    temperature: 3300\n")
	var cfg *Config
	out := logged(t, func() {
		var err error
		if cfg, err = LoadFile(path); err != nil {
			t.Fatal(err)
		}
	})
	if cfg.Nightlight.Temperature != 3300 {
		t.Errorf("temperature = %d, want 3300 from modules.hyprsunset", cfg.Nightlight.Temperature)
	}
	if !strings.Contains(out, "modules.hyprsunset") || !strings.Contains(out, "nightlight") {
		t.Errorf("log = %q, want a deprecation naming both keys", out)
	}

	path = writeFile(t, t.TempDir(), "config.yaml", "modules:\n  nightlight:\n    temperature: 3400\n")
	out = logged(t, func() {
		var err error
		if cfg, err = LoadFile(path); err != nil {
			t.Fatal(err)
		}
	})
	if cfg.Nightlight.Temperature != 3400 || strings.Contains(out, "deprecated") {
		t.Errorf("canonical key: temperature %d, log %q", cfg.Nightlight.Temperature, out)
	}
}

// A layout naming the module by its former name gets the night light
// and a warning; an unknown name still fails.
func TestBarLayoutRenamesHyprsunset(t *testing.T) {
	var m BarModule
	out := logged(t, func() {
		if err := m.UnmarshalConfig("hyprsunset"); err != nil {
			t.Fatal(err)
		}
	})
	if m != ModuleNightlight {
		t.Errorf("hyprsunset decoded to %q, want %q", m, ModuleNightlight)
	}
	if !strings.Contains(out, `"hyprsunset" was renamed to "nightlight"`) {
		t.Errorf("log = %q, want the rename", out)
	}
	if err := m.UnmarshalConfig("nightlight"); err != nil || m != ModuleNightlight {
		t.Errorf("nightlight = %q, %v", m, err)
	}
	if err := m.UnmarshalConfig("sunset"); err == nil || !strings.Contains(err.Error(), "unknown variant `sunset`") {
		t.Errorf("sunset = %v, want an unknown-variant error", err)
	}
}

func TestTheNightlightScheduleDefaultsToTheSunWithHoursAtHand(t *testing.T) {
	d := DefaultsNightlight()
	if d.Schedule != NightlightScheduleSun {
		t.Errorf("schedule = %q, want sunset-to-sunrise", d.Schedule)
	}
	if h := d.CustomHours.Hours(); h.From().String() != "21:00" || h.To().String() != "07:00" {
		t.Errorf("custom hours = %s-%s, want 21:00-07:00", h.From(), h.To())
	}
}

func TestTheNightlightReadsCustomHours(t *testing.T) {
	path := writeFile(t, t.TempDir(), "config.yaml", "modules:\n  nightlight:\n    schedule: custom\n    custom-hours: {from: \"22:30\", to: \"06:15\"}\n")
	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	h := cfg.Nightlight.CustomHours.Hours()
	if cfg.Nightlight.Schedule != NightlightScheduleCustom || h.From().String() != "22:30" || h.To().String() != "06:15" {
		t.Errorf("schedule %q, hours %s-%s", cfg.Nightlight.Schedule, h.From(), h.To())
	}
	if got := cfg.Nightlight.CustomHours.MarshalConfig(); got.(map[string]any)["from"] != "22:30" || got.(map[string]any)["to"] != "06:15" {
		t.Errorf("custom hours write back as %v", got)
	}
}

func TestTheNightlightRefusesBrokenCustomHours(t *testing.T) {
	for body, want := range map[string]string{
		"custom-hours: {from: \"07:00\", to: \"07:00\"}": nightschedule.ErrEmptyHours.Error(),
		"custom-hours: {from: \"7pm\", to: \"07:00\"}":   "from",
		"custom-hours: {from: \"21:00\", to: \"24:00\"}": "to",
		"custom-hours: {from: \"21:00\"}":                "to",
		"schedule: sunset":                               "schedule",
	} {
		path := writeFile(t, t.TempDir(), "config.yaml", "modules:\n  nightlight:\n    "+body+"\n")
		cfg, err := LoadFile(path)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want one naming %q", body, err, want)
		}
		if cfg.Nightlight.CustomHours != DefaultsNightlight().CustomHours || cfg.Nightlight.Schedule != NightlightScheduleSun {
			t.Errorf("%s: a refused value reached the config: %+v", body, cfg.Nightlight)
		}
	}
	if _, err := parseNightlightHours(nightlightHoursFields{From: "05:00", To: "05:00"}); !errors.Is(err, nightschedule.ErrEmptyHours) {
		t.Errorf("an empty night = %v, want ErrEmptyHours", err)
	}
}
