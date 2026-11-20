// Package geo holds positions on the globe: validated coordinates and
// the named places the desktop's location settings store.
package geo

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrNullIsland is the coordinates 0, 0: open water no machine sits on,
// and the value an unset coordinate used to default to, so it is never
// taken for a position.
var ErrNullIsland = errors.New("the coordinates 0, 0 are not a position")

// Coordinates are a position in decimal degrees, north and east positive.
// NewCoordinates is the only way to build them, so every value is on the
// globe and not 0, 0.
type Coordinates struct {
	latitude  float64
	longitude float64
}

// NewCoordinates validates a position, refusing NaN, anything off the
// globe, and ErrNullIsland.
func NewCoordinates(latitude, longitude float64) (Coordinates, error) {
	if !(latitude >= -90 && latitude <= 90) {
		return Coordinates{}, fmt.Errorf("latitude %v is outside -90 to 90", latitude)
	}
	if !(longitude >= -180 && longitude <= 180) {
		return Coordinates{}, fmt.Errorf("longitude %v is outside -180 to 180", longitude)
	}
	if latitude == 0 && longitude == 0 {
		return Coordinates{}, ErrNullIsland
	}
	return Coordinates{latitude: latitude, longitude: longitude}, nil
}

// Latitude is the latitude in decimal degrees, north positive.
func (c Coordinates) Latitude() float64 { return c.latitude }

// Longitude is the longitude in decimal degrees, east positive.
func (c Coordinates) Longitude() float64 { return c.longitude }

// String is "latitude,longitude" in the shortest decimals that read back
// the same, the form weather services take.
func (c Coordinates) String() string {
	return strconv.FormatFloat(c.latitude, 'f', -1, 64) + "," + strconv.FormatFloat(c.longitude, 'f', -1, 64)
}

// Place is a position with the name it is known by; the name may be
// empty for coordinates set by hand.
type Place struct {
	name string
	at   Coordinates
}

// NewPlace names a position, trimming the name.
func NewPlace(name string, at Coordinates) Place {
	return Place{name: strings.TrimSpace(name), at: at}
}

// Name is the place's name, empty when it has none.
func (p Place) Name() string { return p.name }

// Coordinates is where the place is.
func (p Place) Coordinates() Coordinates { return p.at }

// ParseISO6709 reads the degrees-minutes(-seconds) form tz's zone tables
// write, such as +5540+01235 or +404251-0740023.
func ParseISO6709(s string) (Coordinates, error) {
	split := strings.LastIndexAny(s, "+-")
	if split <= 0 {
		return Coordinates{}, fmt.Errorf("ISO 6709 position %q: want a signed latitude then a signed longitude", s)
	}
	latitude, err := parseSexagesimal(s[:split], 2)
	if err != nil {
		return Coordinates{}, fmt.Errorf("ISO 6709 position %q: latitude: %w", s, err)
	}
	longitude, err := parseSexagesimal(s[split:], 3)
	if err != nil {
		return Coordinates{}, fmt.Errorf("ISO 6709 position %q: longitude: %w", s, err)
	}
	c, err := NewCoordinates(latitude, longitude)
	if err != nil {
		return Coordinates{}, fmt.Errorf("ISO 6709 position %q: %w", s, err)
	}
	return c, nil
}

func parseSexagesimal(s string, degreeDigits int) (float64, error) {
	sign := 1.0
	switch {
	case strings.HasPrefix(s, "-"):
		sign = -1
	case !strings.HasPrefix(s, "+"):
		return 0, fmt.Errorf("%q has no sign", s)
	}
	digits := s[1:]
	if n := len(digits); n != degreeDigits+2 && n != degreeDigits+4 {
		return 0, fmt.Errorf("%q is not %d degree digits then minutes and optional seconds", s, degreeDigits)
	}
	value, unit := 0.0, 1.0
	for i, part := range []string{digits[:degreeDigits], digits[degreeDigits:min(len(digits), degreeDigits+2)], digits[min(len(digits), degreeDigits+2):]} {
		if part == "" {
			continue
		}
		n, err := strconv.ParseUint(part, 10, 16)
		if err != nil {
			return 0, fmt.Errorf("%q: %w", s, err)
		}
		if i > 0 && n >= 60 {
			return 0, fmt.Errorf("%q: %d is not under 60", s, n)
		}
		value += float64(n) / unit
		unit *= 60
	}
	return sign * value, nil
}
