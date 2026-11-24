package settings

import (
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/geo"
	"github.com/stubbedev/lys/desktop/tzdb"
)

type zonePlace struct {
	zone  string
	place geo.Place
}

func (h host) tz() tzdb.Host {
	return tzdb.Host{Root: h.root, LookupEnv: func(name string) (string, bool) {
		v := h.env(name)
		return v, v != ""
	}}
}

func (h host) zonePlaces() ([]zonePlace, error) {
	db, err := h.tz().Database()
	if err != nil {
		return nil, err
	}
	zones := db.Zones()
	out := make([]zonePlace, 0, len(zones))
	for _, z := range zones {
		out = append(out, zonePlace{zone: z.Name(), place: geo.NewPlace(zoneCity(z.Name()), z.Coordinates())})
	}
	slices.SortFunc(out, func(a, b zonePlace) int {
		return strings.Compare(a.place.Name()+"\x00"+a.zone, b.place.Name()+"\x00"+b.zone)
	})
	return out, nil
}

func zoneCity(zone string) string {
	return strings.ReplaceAll(zone[strings.LastIndex(zone, "/")+1:], "_", " ")
}
