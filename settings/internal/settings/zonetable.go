package settings

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
	zoneTableName    = "zone1970.tab"
	countryTableName = "iso3166.tab"
	zoneinfoEnv      = "ZONEINFO"
)

var platformZoneinfo = []string{"usr/share/zoneinfo", "usr/share/lib/zoneinfo", "usr/lib/locale/TZ", "etc/zoneinfo"}

var errNoZoneTable = errors.New("no " + zoneTableName + " in the zoneinfo directories")

type zoneRow struct {
	zone      string
	at        geo.Coordinates
	countries []string
}

type zoneTable struct {
	dir  string
	rows []zoneRow
}

type zonePlace struct {
	zone  string
	place geo.Place
}

func (h host) zoneinfoDirs() []string {
	dirs := make([]string, 0, len(platformZoneinfo)+1)
	if dir := h.env(zoneinfoEnv); filepath.IsAbs(dir) && filepath.Ext(dir) != ".zip" {
		dirs = append(dirs, h.path(dir))
	}
	for _, dir := range platformZoneinfo {
		dirs = append(dirs, h.path(dir))
	}
	return dirs
}

func (h host) zoneTable() (zoneTable, error) {
	dirs := h.zoneinfoDirs()
	for _, dir := range dirs {
		rows, err := readZoneTable(filepath.Join(dir, zoneTableName))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		return zoneTable{dir: dir, rows: rows}, err
	}
	return zoneTable{}, fmt.Errorf("%w: %s", errNoZoneTable, strings.Join(dirs, ", "))
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

func readZoneTable(path string) ([]zoneRow, error) {
	var rows []zoneRow
	err := readTable(path, 3, func(cols []string) error {
		at, err := geo.ParseISO6709(cols[1])
		if err != nil {
			return fmt.Errorf("the zone %s: %w", cols[2], err)
		}
		rows = append(rows, zoneRow{zone: cols[2], at: at, countries: strings.Split(cols[0], ",")})
		return nil
	})
	return rows, err
}

func (t zoneTable) countryNames() (map[string]string, error) {
	names := map[string]string{}
	err := readTable(filepath.Join(t.dir, countryTableName), 2, func(cols []string) error {
		names[cols[0]] = cols[1]
		return nil
	})
	return names, err
}

func (h host) zonePlaces() ([]zonePlace, error) {
	table, err := h.zoneTable()
	if err != nil {
		return nil, err
	}
	out := make([]zonePlace, 0, len(table.rows))
	for _, r := range table.rows {
		out = append(out, zonePlace{zone: r.zone, place: geo.NewPlace(zoneCity(r.zone), r.at)})
	}
	slices.SortFunc(out, func(a, b zonePlace) int {
		return strings.Compare(a.place.Name()+"\x00"+a.zone, b.place.Name()+"\x00"+b.zone)
	})
	return out, nil
}

func zoneCity(zone string) string {
	return strings.ReplaceAll(zone[strings.LastIndex(zone, "/")+1:], "_", " ")
}
