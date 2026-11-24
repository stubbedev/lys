// Package tzdb reads the system's tz database: where each time zone is,
// and which time zone the machine keeps.
package tzdb

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/geo"
)

const (
	// ZoneTableName is the table of the zones that differ since 1970.
	ZoneTableName = "zone1970.tab"
	// CountryTableName is the table of country names by ISO 3166 code.
	CountryTableName = "iso3166.tab"

	countryZoneTableName = "zone.tab"
	linkTableName        = "tzdata.zi"
	zoneinfoEnv          = "ZONEINFO"
	tzEnv                = "TZ"
	localtimePath        = "etc/localtime"
	zoneinfoDirName      = "zoneinfo"
)

var platformDirs = []string{"usr/share/zoneinfo", "usr/share/lib/zoneinfo", "usr/lib/locale/TZ", "etc/zoneinfo"}

// ErrNoZoneTable is a system without ZoneTableName in any zoneinfo
// directory.
var ErrNoZoneTable = errors.New("no " + ZoneTableName + " in the zoneinfo directories")

// ErrNoCoordinates is a time zone the tz database places nowhere, such
// as UTC and the Etc zones.
var ErrNoCoordinates = errors.New("the tz database places it nowhere")

// Host is a machine's tz database and time zone, under Root.
type Host struct {
	Root      string
	LookupEnv func(name string) (string, bool)
}

// System is the running machine.
var System = Host{Root: "/", LookupEnv: os.LookupEnv}

func (h Host) path(name string) string { return filepath.Join(h.Root, name) }

func (h Host) dirs() []string {
	dirs := make([]string, 0, len(platformDirs)+1)
	if dir, _ := h.LookupEnv(zoneinfoEnv); filepath.IsAbs(dir) && filepath.Ext(dir) != ".zip" {
		dirs = append(dirs, h.path(dir))
	}
	for _, dir := range platformDirs {
		dirs = append(dirs, h.path(dir))
	}
	return dirs
}

// Zone is a row of ZoneTableName: a zone, where it is, and the ISO 3166
// codes of the countries that keep it.
type Zone struct {
	name      string
	at        geo.Coordinates
	countries []string
}

// Name is the zone's name, such as Europe/Berlin.
func (z Zone) Name() string { return z.name }

// Coordinates is where the zone's principal city is.
func (z Zone) Coordinates() geo.Coordinates { return z.at }

// Countries is the ISO 3166 codes of the countries that keep the zone.
func (z Zone) Countries() []string { return slices.Clone(z.countries) }

// Database is the zoneinfo directory that holds ZoneTableName, and its
// zones.
type Database struct {
	dir   string
	zones []Zone
}

// Database reads the first zoneinfo directory holding ZoneTableName:
// $ZONEINFO, then the platform's.
func (h Host) Database() (Database, error) {
	dirs := h.dirs()
	for _, dir := range dirs {
		zones, err := readZoneTable(filepath.Join(dir, ZoneTableName))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		return Database{dir: dir, zones: zones}, err
	}
	return Database{}, fmt.Errorf("%w: %s", ErrNoZoneTable, strings.Join(dirs, ", "))
}

// Zones is ZoneTableName's rows in its order.
func (d Database) Zones() []Zone { return slices.Clone(d.zones) }

// CountryNames is CountryTableName: each country's name by its code.
func (d Database) CountryNames() (map[string]string, error) {
	names := map[string]string{}
	err := readTable(filepath.Join(d.dir, CountryTableName), 2, func(cols []string) error {
		names[cols[0]] = cols[1]
		return nil
	})
	return names, err
}

// Coordinates is where zone is: its row in zone.tab, which places every
// country's zone, or that of the zone tzdata.zi links it to. A zone in
// neither is ErrNoCoordinates.
func (d Database) Coordinates(zone string) (geo.Coordinates, error) {
	placed, err := d.countryZones()
	if err != nil {
		return geo.Coordinates{}, err
	}
	if at, ok := placed[zone]; ok {
		return at, nil
	}
	links, err := d.links()
	if err != nil {
		return geo.Coordinates{}, err
	}
	if at, ok := placed[links[zone]]; ok {
		return at, nil
	}
	return geo.Coordinates{}, fmt.Errorf("the time zone %s: %w", zone, ErrNoCoordinates)
}

func (d Database) countryZones() (map[string]geo.Coordinates, error) {
	placed := map[string]geo.Coordinates{}
	err := readTable(filepath.Join(d.dir, countryZoneTableName), 3, func(cols []string) error {
		at, err := geo.ParseISO6709(cols[1])
		if err != nil {
			return fmt.Errorf("the zone %s: %w", cols[2], err)
		}
		placed[cols[2]] = at
		return nil
	})
	return placed, err
}

func (d Database) links() (map[string]string, error) {
	path := filepath.Join(d.dir, linkTableName)
	f, err := os.Open(path) //nolint:gosec // a table of the system's tz database
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	links := map[string]string{}
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 3 && fields[0] == "L" {
			links[fields[2]] = fields[1]
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	return links, nil
}

// Zone is the name of the time zone the machine keeps: $TZ when set,
// else the zone /etc/localtime links to.
func (h Host) Zone() (string, error) {
	if tz, ok := h.LookupEnv(tzEnv); ok {
		return ZoneOfTZ(tz)
	}
	path := h.path(localtimePath)
	target, err := os.Readlink(path)
	if err != nil {
		return "", fmt.Errorf("the system time zone: %s is not a link to a zone: %w", path, err)
	}
	return ZoneOfPath(target)
}

// Coordinates is where the time zone the machine keeps is.
func (h Host) Coordinates() (geo.Coordinates, error) {
	zone, err := h.Zone()
	if err != nil {
		return geo.Coordinates{}, err
	}
	db, err := h.Database()
	if err != nil {
		return geo.Coordinates{}, err
	}
	return db.Coordinates(zone)
}

// ZoneOfTZ is the zone a TZ value names: empty is UTC, a leading colon
// is dropped, and an absolute path is read as ZoneOfPath reads it.
func ZoneOfTZ(tz string) (string, error) {
	tz = strings.TrimPrefix(tz, ":")
	switch {
	case tz == "":
		return "UTC", nil
	case filepath.IsAbs(tz):
		return ZoneOfPath(tz)
	}
	return tz, nil
}

// ZoneOfPath is the zone a path into a zoneinfo directory names, such
// as Europe/Berlin for /usr/share/zoneinfo/Europe/Berlin; the posix and
// right trees name the same zones.
func ZoneOfPath(path string) (string, error) {
	parts := strings.Split(filepath.ToSlash(filepath.Clean(path)), "/")
	at := -1
	for i, part := range parts {
		if part == zoneinfoDirName {
			at = i
		}
	}
	if at < 0 {
		return "", fmt.Errorf("%s is not in a %s directory, so it names no time zone", path, zoneinfoDirName)
	}
	zone := parts[at+1:]
	if len(zone) > 0 && (zone[0] == "posix" || zone[0] == "right") {
		zone = zone[1:]
	}
	if len(zone) == 0 {
		return "", fmt.Errorf("%s is a %s directory, not a time zone in one", path, zoneinfoDirName)
	}
	return strings.Join(zone, "/"), nil
}

func readTable(path string, minCols int, row func(cols []string) error) error {
	f, err := os.Open(path) //nolint:gosec // a table of the system's tz database
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) < minCols {
			return fmt.Errorf("%s: %q has fewer than %d columns", path, line, minCols)
		}
		if err := row(cols); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	return nil
}

func readZoneTable(path string) ([]Zone, error) {
	var zones []Zone
	err := readTable(path, 3, func(cols []string) error {
		at, err := geo.ParseISO6709(cols[1])
		if err != nil {
			return fmt.Errorf("the zone %s: %w", cols[2], err)
		}
		zones = append(zones, Zone{name: cols[2], at: at, countries: strings.Split(cols[0], ",")})
		return nil
	})
	return zones, err
}
