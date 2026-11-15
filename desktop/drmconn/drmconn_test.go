package drmconn

import "testing"

func TestNameParsesDRMSysnames(t *testing.T) {
	for sysname, want := range map[string]string{
		"card1-eDP-1":    "eDP-1",
		"card0-HDMI-A-1": "HDMI-A-1",
		"card12-DP-3":    "DP-3",
	} {
		if got, ok := Name(sysname); !ok || got != want {
			t.Errorf("Name(%q) = %q %v, want %q", sysname, got, ok, want)
		}
	}
	for _, sysname := range []string{"card1", "renderD128", "cardX-DP-1", "card-DP-1", "card1-", "i2c-6", "version"} {
		if got, ok := Name(sysname); ok {
			t.Errorf("Name(%q) = %q, want no connector", sysname, got)
		}
	}
}

func TestAncestorIsTheNearestConnectorAbove(t *testing.T) {
	for path, want := range map[string]string{
		"/sys/devices/pci0000:00/0000:00:02.0/drm/card1/card1-eDP-1/intel_backlight": "eDP-1",
		"/sys/devices/pci0000:00/0000:00:02.0/drm/card1/card1-DP-2/i2c-7":            "DP-2",
		"/sys/devices/pci0000:00/0000:00:02.0/drm/card1/card1-eDP-1":                 "eDP-1",
	} {
		if got := Ancestor(path); got != want {
			t.Errorf("Ancestor(%q) = %q, want %q", path, got, want)
		}
	}
	for _, path := range []string{
		"/sys/devices/platform/thinkpad_acpi/backlight/acpi_video0",
		"/sys/devices/pci0000:00/0000:00:02.0/drm/card1",
		"/",
	} {
		if got := Ancestor(path); got != "" {
			t.Errorf("Ancestor(%q) = %q, want none", path, got)
		}
	}
}
