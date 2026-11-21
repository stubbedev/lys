package dock

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stubbedev/lys/power/charge"
)

func connectors(t *testing.T, statuses map[string]string) string {
	t.Helper()
	sys := t.TempDir()
	drm := filepath.Join(sys, "class", "drm")
	for _, dir := range []string{"card0", "renderD128"} {
		if err := os.MkdirAll(filepath.Join(drm, dir), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	for name, status := range statuses {
		if err := os.MkdirAll(filepath.Join(drm, name), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(drm, name, "status"), []byte(status+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return sys
}

func TestRead(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		statuses map[string]string
		want     charge.Dock
	}{
		{"an external display", map[string]string{"card0-eDP-1": "connected", "card1-DP-3": "connected", "card0-HDMI-A-1": "disconnected"}, charge.DockDocked},
		{"only the panel", map[string]string{"card0-eDP-1": "connected", "card1-DP-3": "disconnected", "card0-HDMI-A-1": "disconnected"}, charge.DockUndocked},
		{"internal kinds never dock", map[string]string{"card0-LVDS-1": "connected", "card0-DSI-1": "connected", "card0-Writeback-1": "connected", "card0-Virtual-1": "connected"}, charge.DockUndocked},
		{"no connectors", map[string]string{}, charge.DockUnknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got, err := Read(connectors(t, c.statuses))
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Errorf("Read() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestReadWithoutDRMIsUnknown(t *testing.T) {
	t.Parallel()
	got, err := Read(t.TempDir())
	if err != nil || got != charge.DockUnknown {
		t.Errorf("Read() = %v, %v; want unknown", got, err)
	}
}

func TestAnUnreadableConnectorIsAnError(t *testing.T) {
	t.Parallel()
	sys := connectors(t, map[string]string{})
	if err := os.MkdirAll(filepath.Join(sys, "class", "drm", "card1-DP-1"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := Read(sys); err == nil {
		t.Error("a connector without a status is no error")
	}
}
