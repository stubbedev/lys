package config

import (
	"io/fs"
	"path"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/resources"
)

// Every Wi-Fi icon the network module shows by default draws more than
// one shape, so none of them is Lucide's lone dot, which reads as a
// 2px speck at bar size.
func TestNetworkDefaultWifiIconsAreMoreThanADot(t *testing.T) {
	d := DefaultsNetwork()
	icons := append([]string{d.WifiDisabledIcon, d.WifiAcquiringIcon, d.WifiOfflineIcon, d.WifiConnectedIcon}, d.WifiSignalIcons...)
	for _, name := range icons {
		svg, err := fs.ReadFile(resources.Icons, path.Join(resources.IconsDir, name+".svg"))
		if err != nil {
			t.Errorf("default Wi-Fi icon %s is not bundled: %v", name, err)
			continue
		}
		if shapes := strings.Count(string(svg), "gpa:fill="); shapes < 2 {
			t.Errorf("default Wi-Fi icon %s draws %d shape, want at least 2", name, shapes)
		}
	}
}

// A connected Wi-Fi's default icon climbs one arc per step, weakest first.
func TestNetworkDefaultSignalLadder(t *testing.T) {
	d := DefaultsNetwork()
	for s, want := range map[uint8]string{
		0: "ld-wifi-low-symbolic", 32: "ld-wifi-low-symbolic",
		33: "ld-wifi-high-symbolic", 65: "ld-wifi-high-symbolic",
		66: "ld-wifi-symbolic", 100: "ld-wifi-symbolic", 255: "ld-wifi-symbolic",
	} {
		if got := d.WifiSignalIcon(s); got != want {
			t.Errorf("WifiSignalIcon(%d) = %q, want %q", s, got, want)
		}
	}
}
