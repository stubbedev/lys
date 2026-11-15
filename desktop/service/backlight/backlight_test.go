package backlight_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stubbedev/lys/desktop/service/backlight"
	"github.com/stubbedev/lys/desktop/service/backlight/backlighttest"
)

func TestEnumerateReadsSysfs(t *testing.T) {
	backlighttest.Fake(t, map[string]backlighttest.Device{
		"intel_backlight": {Type: "raw", Brightness: 3000, Max: 12000},
		"acpi_video0":     {Type: "firmware", Brightness: 50, Max: 100},
	})
	devices := backlight.Enumerate()
	if len(devices) != 2 {
		t.Fatalf("devices = %+v, want 2", devices)
	}
	byName := map[string]backlight.Device{}
	for _, d := range devices {
		byName[d.Name] = d
	}
	intel := byName["intel_backlight"]
	if intel.Type != backlight.TypeRaw || intel.Brightness != 3000 || intel.Max != 12000 {
		t.Errorf("intel = %+v", intel)
	}
	if got := intel.Percentage(); got < 24.9 || got > 25.1 {
		t.Errorf("percentage = %v, want 25", got)
	}
	if byName["acpi_video0"].Type != backlight.TypeFirmware {
		t.Errorf("acpi type = %v, want firmware", byName["acpi_video0"].Type)
	}
}

func TestEnumerateLeavesOutADeviceThatDoesNotRead(t *testing.T) {
	dir := backlighttest.Fake(t, map[string]backlighttest.Device{
		"intel_backlight": {Type: "raw", Brightness: 3000, Max: 12000},
		"broken":          {Type: "raw", Brightness: 1, Max: 10},
	})
	if err := os.WriteFile(filepath.Join(dir, "broken", "max_brightness"), []byte("lots"), 0o644); err != nil {
		t.Fatal(err)
	}
	devices := backlight.Enumerate()
	if len(devices) != 1 || devices[0].Name != "intel_backlight" {
		t.Errorf("devices = %+v, want intel_backlight alone", devices)
	}
	if _, err := backlight.ReadDevice("broken"); err == nil {
		t.Error("ReadDevice on an unparsable maximum succeeded")
	}
}

func TestEnumerateWithoutTheClassDirectoryIsEmpty(t *testing.T) {
	backlighttest.Fake(t, nil)
	backlight.Dir = filepath.Join(backlight.Dir, "missing")
	if devices := backlight.Enumerate(); len(devices) != 0 {
		t.Errorf("devices = %+v, want none", devices)
	}
}

func TestTypeFromSysfs(t *testing.T) {
	for _, tc := range []struct {
		raw  string
		want backlight.Type
	}{
		{"raw\n", backlight.TypeRaw},
		{"firmware", backlight.TypeFirmware},
		{"platform", backlight.TypePlatform},
		{"虚拟", backlight.TypeRaw},
	} {
		if got := backlight.TypeFromSysfs(tc.raw); got != tc.want {
			t.Errorf("TypeFromSysfs(%q) = %v, want %v", tc.raw, got, tc.want)
		}
	}
}

func TestAveragePercentage(t *testing.T) {
	if _, ok := backlight.AveragePercentage(nil); ok {
		t.Error("no devices: want present=false")
	}
	got, ok := backlight.AveragePercentage([]backlight.Device{
		{Brightness: 2500, Max: 10000},
		{Brightness: 7500, Max: 10000},
	})
	if !ok || got < 49.9 || got > 50.1 {
		t.Errorf("average = %v/%v, want 50", got, ok)
	}
	if p := (backlight.Device{Brightness: 5}).Percentage(); p != 0 {
		t.Errorf("a device without a maximum is at %v%%, want 0", p)
	}
}

func TestRawLevelRoundsAndClamps(t *testing.T) {
	for percent, want := range map[float64]uint32{36: 4, 34: 3, 5: 1, 4: 0, -20: 0, 900: 10} {
		if got := backlight.RawLevel(percent, 10); got != want {
			t.Errorf("RawLevel(%v, 10) = %d, want %d", percent, got, want)
		}
	}
}

func TestBacklightConnectorFollowsTheSysfsLink(t *testing.T) {
	dir := backlighttest.Fake(t, map[string]backlighttest.Device{
		"intel_backlight": {Type: "raw", Brightness: 3000, Max: 12000},
		"acpi_video0":     {Type: "firmware", Brightness: 50, Max: 100},
	})
	connector := filepath.Join(t.TempDir(), "devices", "pci0000:00", "0000:00:02.0", "drm", "card1", "card1-eDP-1")
	if err := os.MkdirAll(connector, 0o755); err != nil {
		t.Fatal(err)
	}
	wired := filepath.Join(connector, "intel_backlight")
	if err := os.Rename(filepath.Join(dir, "intel_backlight"), wired); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(wired, filepath.Join(dir, "intel_backlight")); err != nil {
		t.Fatal(err)
	}
	connectors := map[string]string{}
	for _, d := range backlight.Enumerate() {
		connectors[d.Name] = d.Connector
	}
	if connectors["intel_backlight"] != "eDP-1" {
		t.Errorf("intel_backlight lights %q, want eDP-1", connectors["intel_backlight"])
	}
	if c, ok := connectors["acpi_video0"]; !ok || c != "" {
		t.Errorf("acpi_video0 lights %q (listed %v), want no connector", c, ok)
	}
}
