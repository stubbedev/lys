package config

import (
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestNightlightLocationIsAutomaticByDefault(t *testing.T) {
	if _, pinned := DefaultsNightlight().Location.Pinned(); pinned {
		t.Error("the default location is pinned")
	}
	path := writeFile(t, t.TempDir(), "config.yaml", "modules:\n  nightlight:\n    location:\n      name: Copenhagen\n      latitude: 55.6667\n      longitude: 12.5833\n")
	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	p, pinned := cfg.Nightlight.Location.Pinned()
	if !pinned || p.Name() != "Copenhagen" || p.Coordinates().Latitude() != 55.6667 || p.Coordinates().Longitude() != 12.5833 {
		t.Errorf("pinned = %v %q at %v", pinned, p.Name(), p.Coordinates())
	}
}

func TestNightlightLocationAutoReads(t *testing.T) {
	path := writeFile(t, t.TempDir(), "config.yaml", "modules:\n  nightlight:\n    location: auto\n")
	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, pinned := cfg.Nightlight.Location.Pinned(); pinned {
		t.Error("auto pinned the location")
	}
	if got := cfg.Nightlight.Location.MarshalConfig(); got != "auto" {
		t.Errorf("auto writes back as %v", got)
	}
}

func TestNightlightFormerCoordinatesMoveIntoTheLocation(t *testing.T) {
	path := writeFile(t, t.TempDir(), "config.yaml", "modules:\n  nightlight:\n    latitude: 55.6\n    longitude: 12.5\n")
	var cfg *Config
	out := logged(t, func() {
		var err error
		if cfg, err = LoadFile(path); err != nil {
			t.Fatal(err)
		}
	})
	p, pinned := cfg.Nightlight.Location.Pinned()
	if !pinned || p.Name() != "" || p.Coordinates().Latitude() != 55.6 || p.Coordinates().Longitude() != 12.5 {
		t.Errorf("former coordinates pinned = %v %q at %v", pinned, p.Name(), p.Coordinates())
	}
	if !strings.Contains(out, "moved to modules.nightlight.location") {
		t.Errorf("log = %q, want the move named", out)
	}
}

func TestNightlightFormerCoordinatesOfNullIslandStayAutomatic(t *testing.T) {
	path := writeFile(t, t.TempDir(), "config.yaml", "modules:\n  nightlight:\n    latitude: 0\n    longitude: 0.0\n")
	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, pinned := cfg.Nightlight.Location.Pinned(); pinned {
		t.Error("0, 0 pinned the location")
	}
}

func TestNightlightFormerCoordinatesFailAsTheLocation(t *testing.T) {
	for name, tc := range map[string]struct{ yaml, want string }{
		"latitude alone": {"    latitude: 55.6\n", "missing field `longitude`"},
		"off the globe":  {"    latitude: 95\n    longitude: 12.5\n", "latitude 95"},
	} {
		path := writeFile(t, t.TempDir(), "config.yaml", "modules:\n  nightlight:\n"+tc.yaml)
		cfg, err := LoadFile(path)
		if err == nil || !strings.Contains(err.Error(), "modules.nightlight.location") || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want the location's diagnostic naming %q", name, err, tc.want)
		}
		if _, pinned := cfg.Nightlight.Location.Pinned(); pinned {
			t.Errorf("%s pinned the location", name)
		}
	}
}

func TestNightlightSetLocationWinsOverFormerCoordinates(t *testing.T) {
	path := writeFile(t, t.TempDir(), "config.yaml", "modules:\n  nightlight:\n    location: auto\n    latitude: 55.6\n    longitude: 12.5\n")
	var cfg *Config
	out := logged(t, func() {
		var err error
		if cfg, err = LoadFile(path); err != nil {
			t.Fatal(err)
		}
	})
	if _, pinned := cfg.Nightlight.Location.Pinned(); pinned {
		t.Error("the former coordinates overrode location: auto")
	}
	if !strings.Contains(out, "ignored") {
		t.Errorf("log = %q, want the former keys called ignored", out)
	}
}

func TestWeatherLocationIsANamedPlace(t *testing.T) {
	def := DefaultsWeather().Location.Place()
	if def.Name() != "San Francisco" || def.Coordinates().Latitude() != 37.7749 {
		t.Errorf("default = %q at %v", def.Name(), def.Coordinates())
	}
	path := writeFile(t, t.TempDir(), "config.yaml", "modules:\n  weather:\n    location:\n      name: Lisbon\n      latitude: 38.7167\n      longitude: -9.1333\n")
	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	p := cfg.Weather.Location.Place()
	if p.Name() != "Lisbon" || p.Coordinates().Longitude() != -9.1333 {
		t.Errorf("loaded %q at %v", p.Name(), p.Coordinates())
	}
	want := map[string]any{"name": "Lisbon", "latitude": 38.7167, "longitude": -9.1333}
	if got := cfg.Weather.Location.MarshalConfig(); !reflect.DeepEqual(got, want) {
		t.Errorf("writes back as %v, want %v", got, want)
	}
}

func TestWeatherLocationReadsTheFormerCoordinateString(t *testing.T) {
	path := writeFile(t, t.TempDir(), "config.yaml", "modules:\n  weather:\n    location: \" 52.52 , 13.40 \"\n")
	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	p := cfg.Weather.Location.Place()
	if p.Name() != "" || p.Coordinates().Latitude() != 52.52 || p.Coordinates().Longitude() != 13.40 {
		t.Errorf("loaded %q at %v", p.Name(), p.Coordinates())
	}
	if _, named := cfg.Weather.Location.MarshalConfig().(map[string]any)["name"]; named {
		t.Error("an unnamed place writes a name")
	}
}

func TestWeatherLocationRefusesWhatIsNoPlace(t *testing.T) {
	for name, tc := range map[string]struct{ yaml, want string }{
		"a city name":       {"location: Paris", "a city name alone is no longer looked up"},
		"words with commas": {"location: \"Paris, France\"", "a city name alone"},
		"null island":       {"location: \"0,0\"", "0, 0"},
		"off the globe":     {"location:\n      latitude: 10\n      longitude: 190", "longitude 190"},
		"no longitude":      {"location:\n      name: X\n      latitude: 10", "missing field `longitude`"},
		"unknown key":       {"location:\n      latitude: 10\n      longitude: 10\n      city: X", "city"},
		"auto":              {"location: auto", "a city name alone"},
	} {
		path := writeFile(t, t.TempDir(), "config.yaml", "modules:\n  weather:\n    "+tc.yaml+"\n")
		cfg, err := LoadFile(path)
		if err == nil || !strings.Contains(err.Error(), "modules.weather.location") || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: err = %v, want the location's diagnostic naming %q", name, err, tc.want)
		}
		if cfg.Weather.Location != DefaultsWeather().Location {
			t.Errorf("%s: the default location was not kept", name)
		}
	}
}

func TestTheLocationSchemasDescribeTheirForms(t *testing.T) {
	defs, _ := generateSchemaTree()["$defs"].(Schema)
	place, ok := defs["Place"].(Schema)
	if !ok {
		t.Fatalf("no Place schema among %v", slices.Sorted(maps.Keys(defs)))
	}
	autoPlace, _ := defs["AutoPlace"].(Schema)
	props, _ := place["properties"].(map[string]any)
	lat, _ := props["latitude"].(Schema)
	lon, _ := props["longitude"].(Schema)
	if lat["minimum"] != -90.0 || lat["maximum"] != 90.0 || lon["minimum"] != -180.0 || lon["maximum"] != 180.0 {
		t.Errorf("latitude %v, longitude %v: want their globe bounds", lat, lon)
	}
	auto, _ := autoPlace["oneOf"].([]any)
	if len(auto) != 2 || auto[0].(Schema)["const"] != "auto" {
		t.Errorf("AutoPlace is %v, want auto or a place", autoPlace)
	}
	m, ok := Field("modules.nightlight.location")
	if !ok || m.Kind != FieldOther {
		t.Errorf("the nightlight location is %+v, want a field its own editor handles", m)
	}
}
