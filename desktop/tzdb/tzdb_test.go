package tzdb

import (
	"errors"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const (
	testZone1970 = "# comment\nDE,DK,NO\t+5230+01322\tEurope/Berlin\tmost of Germany\nJP\t+353916+1394441\tAsia/Tokyo\n"
	testZoneTab  = "# comment\nDE\t+5230+01322\tEurope/Berlin\tmost of Germany\nDK\t+5540+01235\tEurope/Copenhagen\nUA\t+5026+03031\tEurope/Kyiv\nJP\t+353916+1394441\tAsia/Tokyo\n"
	testLinks    = "# version\nZ Etc/UTC 0 - UTC\nL Europe/Kyiv Europe/Kiev\nL Etc/UTC UTC\nL Europe/Berlin Europe/Copenhagen\n"
)

func put(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func testHost(t *testing.T, env map[string]string) Host {
	t.Helper()
	return Host{Root: t.TempDir(), LookupEnv: func(name string) (string, bool) {
		v, ok := env[name]
		return v, ok
	}}
}

func withTables(t *testing.T, env map[string]string) Host {
	t.Helper()
	h := testHost(t, env)
	dir := h.path("usr/share/zoneinfo")
	put(t, filepath.Join(dir, ZoneTableName), testZone1970)
	put(t, filepath.Join(dir, countryZoneTableName), testZoneTab)
	put(t, filepath.Join(dir, linkTableName), testLinks)
	put(t, filepath.Join(dir, CountryTableName), "DE\tGermany\nJP\tJapan\n")
	return h
}

func link(t *testing.T, h Host, target string) {
	t.Helper()
	if err := os.MkdirAll(h.path("etc"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, h.path(localtimePath)); err != nil {
		t.Fatal(err)
	}
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestTheZonesAreTheZoneTablesRows(t *testing.T) {
	db, err := withTables(t, nil).Database()
	if err != nil {
		t.Fatal(err)
	}
	zones := db.Zones()
	if len(zones) != 2 || zones[0].Name() != "Europe/Berlin" || zones[1].Name() != "Asia/Tokyo" {
		t.Fatalf("zones = %+v", zones)
	}
	if !slices.Equal(zones[0].Countries(), []string{"DE", "DK", "NO"}) || !near(zones[0].Coordinates().Latitude(), 52.5) {
		t.Errorf("Berlin = %v at %v", zones[0].Countries(), zones[0].Coordinates())
	}
	names, err := db.CountryNames()
	if err != nil || names["JP"] != "Japan" {
		t.Errorf("country names = %v, %v", names, err)
	}
}

func TestTheDatabaseIsTheFirstDirectoryWithAZoneTable(t *testing.T) {
	h := testHost(t, map[string]string{zoneinfoEnv: "/opt/tz"})
	put(t, h.path(filepath.Join("opt/tz", ZoneTableName)), "JP\t+353916+1394441\tAsia/Tokyo\n")
	put(t, h.path(filepath.Join("etc/zoneinfo", ZoneTableName)), testZone1970)
	db, err := h.Database()
	if err != nil || len(db.Zones()) != 1 {
		t.Errorf("$ZONEINFO's table was not first: %+v, %v", db.Zones(), err)
	}
	if _, err := testHost(t, nil).Database(); !errors.Is(err, ErrNoZoneTable) {
		t.Errorf("without a table: %v", err)
	}
	bad := testHost(t, nil)
	put(t, bad.path(filepath.Join("etc/zoneinfo", ZoneTableName)), "XX\t+9999+99999\tNowhere/Atlantis\n")
	if _, err := bad.Database(); err == nil || !strings.Contains(err.Error(), "Nowhere/Atlantis") {
		t.Errorf("a bad position: %v, want an error naming its zone", err)
	}
	short := testHost(t, nil)
	put(t, short.path(filepath.Join("etc/zoneinfo", ZoneTableName)), "XX\t+0000+00000\n")
	if _, err := short.Database(); err == nil || !strings.Contains(err.Error(), "fewer than 3 columns") {
		t.Errorf("a line without a zone: %v", err)
	}
}

func TestAZoneIsWhereTheZoneTablesPlaceIt(t *testing.T) {
	db, err := withTables(t, nil).Database()
	if err != nil {
		t.Fatal(err)
	}
	for zone, want := range map[string][2]float64{
		"Europe/Copenhagen": {55 + 40.0/60, 12 + 35.0/60},
		"Europe/Berlin":     {52.5, 13 + 22.0/60},
		"Europe/Kiev":       {50 + 26.0/60, 30 + 31.0/60},
		"Asia/Tokyo":        {35 + 39.0/60 + 16.0/3600, 139 + 44.0/60 + 41.0/3600},
	} {
		at, err := db.Coordinates(zone)
		if err != nil || !near(at.Latitude(), want[0]) || !near(at.Longitude(), want[1]) {
			t.Errorf("%s = %v, %v, want %v", zone, at, err, want)
		}
	}
}

func TestAZoneWithoutAPlaceIsErrNoCoordinates(t *testing.T) {
	db, err := withTables(t, nil).Database()
	if err != nil {
		t.Fatal(err)
	}
	for _, zone := range []string{"UTC", "Etc/UTC", "Etc/GMT+5", "CET-1CEST", "Mars/Olympus"} {
		if _, err := db.Coordinates(zone); !errors.Is(err, ErrNoCoordinates) || !strings.Contains(err.Error(), zone) {
			t.Errorf("%s = %v, want ErrNoCoordinates naming it", zone, err)
		}
	}
	h := testHost(t, nil)
	dir := h.path("usr/share/zoneinfo")
	put(t, filepath.Join(dir, ZoneTableName), testZone1970)
	put(t, filepath.Join(dir, countryZoneTableName), testZoneTab)
	nolinks, err := h.Database()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := nolinks.Coordinates("Europe/Kiev"); !errors.Is(err, ErrNoCoordinates) {
		t.Errorf("a link without %s = %v, want ErrNoCoordinates", linkTableName, err)
	}
	if at, err := nolinks.Coordinates("Europe/Copenhagen"); err != nil || !near(at.Latitude(), 55+40.0/60) {
		t.Errorf("Copenhagen without %s = %v, %v", linkTableName, at, err)
	}
	put(t, filepath.Join(dir, countryZoneTableName), "DK\t+5540\tEurope/Copenhagen\n")
	if _, err := nolinks.Coordinates("Europe/Copenhagen"); err == nil || errors.Is(err, ErrNoCoordinates) || !strings.Contains(err.Error(), countryZoneTableName) {
		t.Errorf("a torn %s = %v, want an error naming it", countryZoneTableName, err)
	}
}

func TestTheMachinesZoneIsTZThenTheLocaltimeLink(t *testing.T) {
	for tz, want := range map[string]string{
		"Europe/Copenhagen":                    "Europe/Copenhagen",
		":Asia/Tokyo":                          "Asia/Tokyo",
		"":                                     "UTC",
		":/usr/share/zoneinfo/America/Chicago": "America/Chicago",
	} {
		if got, err := testHost(t, map[string]string{tzEnv: tz}).Zone(); err != nil || got != want {
			t.Errorf("TZ=%q = %q, %v, want %q", tz, got, err, want)
		}
	}
	for target, want := range map[string]string{
		"/usr/share/zoneinfo/Europe/Copenhagen":           "Europe/Copenhagen",
		"../usr/share/zoneinfo/America/Argentina/Cordoba": "America/Argentina/Cordoba",
		"/etc/zoneinfo/Europe/Berlin":                     "Europe/Berlin",
		"/usr/share/zoneinfo/posix/Asia/Tokyo":            "Asia/Tokyo",
		"/nix/store/x-tzdata/share/zoneinfo/right/UTC":    "UTC",
	} {
		h := testHost(t, nil)
		link(t, h, target)
		if got, err := h.Zone(); err != nil || got != want {
			t.Errorf("/etc/localtime -> %s = %q, %v, want %q", target, got, err, want)
		}
	}
}

func TestTheMachinesZoneFailsWithoutAZoneName(t *testing.T) {
	for _, target := range []string{"/usr/share/zoneinfo", "/usr/share/zoneinfo/posix", "/opt/clock/Berlin"} {
		h := testHost(t, nil)
		link(t, h, target)
		if zone, err := h.Zone(); err == nil || !strings.Contains(err.Error(), target) {
			t.Errorf("/etc/localtime -> %s = %q, %v, want an error naming it", target, zone, err)
		}
	}
	if zone, err := testHost(t, nil).Zone(); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("no /etc/localtime = %q, %v", zone, err)
	}
	h := testHost(t, nil)
	put(t, h.path(localtimePath), "TZif")
	if zone, err := h.Zone(); err == nil || !strings.Contains(err.Error(), "not a link") {
		t.Errorf("a copied /etc/localtime = %q, %v", zone, err)
	}
}

func TestTheMachineIsWhereItsZoneIs(t *testing.T) {
	h := withTables(t, nil)
	link(t, h, "/usr/share/zoneinfo/Europe/Copenhagen")
	if at, err := h.Coordinates(); err != nil || !near(at.Latitude(), 55+40.0/60) || !near(at.Longitude(), 12+35.0/60) {
		t.Errorf("Copenhagen = %v, %v", at, err)
	}
	utc := withTables(t, map[string]string{tzEnv: "UTC"})
	if _, err := utc.Coordinates(); !errors.Is(err, ErrNoCoordinates) {
		t.Errorf("TZ=UTC = %v, want ErrNoCoordinates", err)
	}
	notables := testHost(t, map[string]string{tzEnv: "Europe/Copenhagen"})
	if _, err := notables.Coordinates(); !errors.Is(err, ErrNoZoneTable) {
		t.Errorf("without tables = %v, want ErrNoZoneTable", err)
	}
}
