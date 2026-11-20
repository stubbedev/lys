package settings

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/geo"
)

var zoneinfoDirs = []string{"usr/share/zoneinfo", "usr/share/lib/zoneinfo", "etc/zoneinfo"}

var errNoZoneTable = fmt.Errorf("no zone1970.tab under %s", strings.Join(zoneinfoDirs, ", "))

type zoneRow struct{ zone, position string }

type zonePlace struct {
	zone  string
	place geo.Place
}

func (h host) zoneTable() ([]zoneRow, error) {
	for _, dir := range zoneinfoDirs {
		rows, err := readZoneTable(h.path(dir, "zone1970.tab"))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		return rows, err
	}
	return nil, errNoZoneTable
}

func readZoneTable(path string) ([]zoneRow, error) {
	f, err := os.Open(path) //nolint:gosec // the system's tz table
	if err != nil {
		return nil, fmt.Errorf("read the zone table: %w", err)
	}
	defer func() { _ = f.Close() }()
	var rows []zoneRow
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cols := strings.Split(line, "\t")
		if len(cols) < 3 {
			return nil, fmt.Errorf("zone table %s: %q has no zone column", path, line)
		}
		rows = append(rows, zoneRow{zone: cols[2], position: cols[1]})
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read the zone table %s: %w", path, err)
	}
	return rows, nil
}

func (h host) zonePlaces() ([]zonePlace, error) {
	rows, err := h.zoneTable()
	if err != nil {
		return nil, err
	}
	out := make([]zonePlace, 0, len(rows))
	for _, r := range rows {
		at, err := geo.ParseISO6709(r.position)
		if err != nil {
			return nil, fmt.Errorf("the zone table's %s: %w", r.zone, err)
		}
		out = append(out, zonePlace{zone: r.zone, place: geo.NewPlace(zoneCity(r.zone), at)})
	}
	slices.SortFunc(out, func(a, b zonePlace) int {
		return strings.Compare(a.place.Name()+"\x00"+a.zone, b.place.Name()+"\x00"+b.zone)
	})
	return out, nil
}

func zoneCity(zone string) string {
	return strings.ReplaceAll(zone[strings.LastIndex(zone, "/")+1:], "_", " ")
}
