package config

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/stubbedev/lys/desktop/geo"
)

// placeAutomatic is the AutoPlace that follows the system time zone's
// place.
const placeAutomatic = "auto"

// Place is a named position, refused at load off the globe or at 0, 0.
//
// A place: `name` as shown, `latitude` and `longitude` in decimal degrees.
type Place struct{ place geo.Place }

// placeFields is a Place as written.
type placeFields struct {
	// The place's name as shown, such as the city; left out for a position
	// set by hand.
	Name string `cfg:"name"`
	// Latitude in decimal degrees, north positive, from -90 to 90.
	Latitude float64 `cfg:"latitude,required"`
	// Longitude in decimal degrees, east positive, from -180 to 180.
	Longitude float64 `cfg:"longitude,required"`
}

// NewPlace is the config value of a place.
func NewPlace(p geo.Place) Place { return Place{place: p} }

func mustPlace(name string, latitude, longitude float64) Place {
	at, err := geo.NewCoordinates(latitude, longitude)
	if err != nil {
		panic(err)
	}
	return NewPlace(geo.NewPlace(name, at))
}

// Place is the validated place.
func (p Place) Place() geo.Place { return p.place }

func parsePlace(v any) (geo.Place, error) {
	if s, ok := v.(string); ok {
		return parseLegacyPlace(s)
	}
	var f placeFields
	if err := decodeValueStruct(reflect.ValueOf(&f).Elem(), v); err != nil {
		return geo.Place{}, err
	}
	at, err := geo.NewCoordinates(f.Latitude, f.Longitude)
	if err != nil {
		return geo.Place{}, err
	}
	return geo.NewPlace(f.Name, at), nil
}

var errCityWithoutCoordinates = errors.New("a city name alone is no longer looked up: set `name`, `latitude` and `longitude`, or pick the city in lys-settings")

func parseLegacyPlace(s string) (geo.Place, error) {
	lat, lon, ok := strings.Cut(s, ",")
	if !ok {
		return geo.Place{}, fmt.Errorf("location %q: %w", s, errCityWithoutCoordinates)
	}
	latitude, errLat := strconv.ParseFloat(strings.TrimSpace(lat), 64)
	longitude, errLon := strconv.ParseFloat(strings.TrimSpace(lon), 64)
	if errLat != nil || errLon != nil {
		return geo.Place{}, fmt.Errorf("location %q: %w", s, errCityWithoutCoordinates)
	}
	at, err := geo.NewCoordinates(latitude, longitude)
	if err != nil {
		return geo.Place{}, fmt.Errorf("location %q: %w", s, err)
	}
	return geo.NewPlace("", at), nil
}

func placeValue(p geo.Place) map[string]any {
	out := map[string]any{"latitude": p.Coordinates().Latitude(), "longitude": p.Coordinates().Longitude()}
	if p.Name() != "" {
		out["name"] = p.Name()
	}
	return out
}

// UnmarshalConfig implements Unmarshaler. The former "latitude,longitude"
// string still reads, as a place without a name.
func (p *Place) UnmarshalConfig(v any) error {
	parsed, err := parsePlace(v)
	if err != nil {
		return err
	}
	p.place = parsed
	return nil
}

// MarshalConfig implements Marshaler.
func (p Place) MarshalConfig() any { return placeValue(p.place) }

func (Place) configSchema(g *schemaGen) Schema {
	s := copySchema(g.structSchema(typeOf[placeFields]()))
	props, _ := s["properties"].(map[string]any)
	for key, bound := range map[string]float64{"latitude": 90, "longitude": 180} {
		prop, _ := props[key].(Schema)
		prop = copySchema(prop)
		prop["minimum"], prop["maximum"] = -bound, bound
		props[key] = prop
	}
	s["description"] = typeDoc(typeOf[Place]())
	return s
}

// AutoPlace is the system time zone's place, or a pinned Place.
//
// `"auto"` to follow the system time zone's place, or a place: `name`,
// `latitude` and `longitude`.
type AutoPlace struct {
	place  geo.Place
	pinned bool
}

// PinPlace is the AutoPlace pinned to p.
func PinPlace(p geo.Place) AutoPlace { return AutoPlace{place: p, pinned: true} }

// Pinned is the pinned place, false when the position is automatic.
func (a AutoPlace) Pinned() (geo.Place, bool) { return a.place, a.pinned }

// UnmarshalConfig implements Unmarshaler.
func (a *AutoPlace) UnmarshalConfig(v any) error {
	if v == placeAutomatic {
		*a = AutoPlace{}
		return nil
	}
	parsed, err := parsePlace(v)
	if err != nil {
		return err
	}
	*a = PinPlace(parsed)
	return nil
}

// MarshalConfig implements Marshaler.
func (a AutoPlace) MarshalConfig() any {
	if !a.pinned {
		return placeAutomatic
	}
	return placeValue(a.place)
}

func (AutoPlace) configSchema(g *schemaGen) Schema {
	return Schema{
		"description": typeDoc(typeOf[AutoPlace]()),
		"oneOf": []any{
			Schema{"const": placeAutomatic, "type": "string", "description": "Follow the place of the system time zone."},
			g.subschema(typeOf[Place]()),
		},
	}
}
