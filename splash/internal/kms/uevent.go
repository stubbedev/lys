package kms

import (
	"strings"

	"github.com/stubbedev/lys/lib/udevmonitor"
)

// subsystems are the devices whose events the display follows.
var subsystems = []string{"drm"}

// card is the card node's name under /dev/dri, such as "card0", when ev is
// about a DRM card.
func card(ev udevmonitor.Event) (string, bool) {
	if ev.Subsystem != "drm" {
		return "", false
	}
	name, ok := strings.CutPrefix(ev.Props["DEVNAME"], "/dev/dri/")
	if !ok || !isCard(name) {
		return "", false
	}
	return name, true
}

func isCard(name string) bool {
	digits, ok := strings.CutPrefix(name, "card")
	if !ok || digits == "" {
		return false
	}
	for _, c := range []byte(digits) {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
