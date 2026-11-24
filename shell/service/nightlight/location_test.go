package nightlight

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/tzdb"
)

func zoneHost(t *testing.T, tz string) tzdb.Host {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "usr/share/zoneinfo")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		tzdb.ZoneTableName: "AU\t-3352+15113\tAustralia/Sydney\tNew South Wales\n",
		"zone.tab":         "AU\t-3352+15113\tAustralia/Sydney\tNew South Wales\nDK\t+5540+01235\tEurope/Copenhagen\n",
		"tzdata.zi":        "L Etc/UTC UTC\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return tzdb.Host{Root: root, LookupEnv: func(name string) (string, bool) { return tz, name == "TZ" }}
}

func TestAutomaticLocationIsTheTimeZonesPlace(t *testing.T) {
	for zone, want := range map[string][2]float64{
		"Australia/Sydney":  {-(33 + 52.0/60), 151 + 13.0/60},
		"Europe/Copenhagen": {55 + 40.0/60, 12 + 35.0/60},
	} {
		at, err := Locate(zoneHost(t, zone))
		if err != nil || math.Abs(at.Latitude()-want[0]) > 1e-9 || math.Abs(at.Longitude()-want[1]) > 1e-9 {
			t.Errorf("%s = %v, %v, want %v", zone, at, err, want)
		}
	}
}

func TestAZoneWithoutAPlaceAsksForAPinnedPlace(t *testing.T) {
	for _, zone := range []string{"UTC", "Etc/GMT-3"} {
		_, err := Locate(zoneHost(t, zone))
		if !errors.Is(err, tzdb.ErrNoCoordinates) || !strings.Contains(err.Error(), zone) || !strings.HasSuffix(err.Error(), "pin a place in "+locationKey) {
			t.Errorf("%s = %v, want ErrNoCoordinates naming the zone and asking for a pinned place", zone, err)
		}
	}
	_, err := Locate(tzdb.Host{Root: t.TempDir(), LookupEnv: func(name string) (string, bool) { return "Europe/Copenhagen", name == "TZ" }})
	if !errors.Is(err, tzdb.ErrNoZoneTable) || errors.Is(err, tzdb.ErrNoCoordinates) || strings.Contains(err.Error(), "pin a place") {
		t.Errorf("without a tz database = %v, want ErrNoZoneTable", err)
	}
}
