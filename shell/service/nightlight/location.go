package nightlight

import (
	"errors"
	"fmt"

	"github.com/stubbedev/lys/desktop/geo"
	"github.com/stubbedev/lys/desktop/tzdb"
)

const locationKey = "modules.nightlight.location"

// Locate is where the schedule follows the sun without a pinned place:
// the place of the time zone h keeps. A zone the tz database places
// nowhere, such as UTC, wraps tzdb.ErrNoCoordinates and asks for a
// pinned place.
func Locate(h tzdb.Host) (geo.Coordinates, error) {
	at, err := h.Coordinates()
	switch {
	case errors.Is(err, tzdb.ErrNoCoordinates):
		return geo.Coordinates{}, fmt.Errorf("nightlight: %w: pin a place in %s", err, locationKey)
	case err != nil:
		return geo.Coordinates{}, fmt.Errorf("nightlight: the time zone's place: %w", err)
	}
	return at, nil
}
