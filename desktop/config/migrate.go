package config

import "log"

func migrate(tree any) any { return migrateNightlightCoordinates(migrateLogin(tree)) }

const nightlightPath = "modules.nightlight"

// migrateNightlightCoordinates moves the night light's former latitude
// and longitude keys into its location, warning once. A location already
// set wins, and 0, 0, which older default configs spell out, is dropped
// as the unset value it was. A tree without the old keys is returned as
// it is; one with them is copied, never changed in place.
func migrateNightlightCoordinates(tree any) any {
	modules, _ := lookup(tree, "modules")
	section, ok := lookup(modules, "nightlight")
	if !ok {
		return tree
	}
	latitude, hasLatitude := lookup(section, "latitude")
	longitude, hasLongitude := lookup(section, "longitude")
	if !hasLatitude && !hasLongitude {
		return tree
	}
	moved := plainCopy(section)
	delete(moved, "latitude")
	delete(moved, "longitude")
	lat, latNumber := asFloat(latitude)
	lon, lonNumber := asFloat(longitude)
	switch {
	case hasPath(section, "location"):
		log.Printf("config: %s.latitude and longitude are ignored: %s.location is set", nightlightPath, nightlightPath)
	case latNumber && lonNumber && lat == 0 && lon == 0:
		log.Printf("config: %s.latitude and longitude of 0, 0 are dropped; the location follows the time zone", nightlightPath)
	default:
		place := map[string]any{}
		if hasLatitude {
			place["latitude"] = latitude
		}
		if hasLongitude {
			place["longitude"] = longitude
		}
		moved["location"] = place
		log.Printf("config: %s.latitude and longitude moved to %s.location; rename them in the config", nightlightPath, nightlightPath)
	}
	out := plainCopy(tree)
	setPlain(out, nightlightPath, moved)
	return out
}
