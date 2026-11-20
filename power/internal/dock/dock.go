// Package dock tells whether the laptop sits at a desk: an external
// display is connected. It reads the DRM connectors in sysfs, and
// recognizes the kernel uevents that announce a connector change.
package dock

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/stubbedev/lys/power/charge"
)

var connectorName = regexp.MustCompile(`^card[0-9]+-(.+)$`)

var notExternal = []string{"eDP-", "LVDS-", "DSI-", "Writeback-", "Virtual-"}

// Read is the dock state the connectors under sysDir/class/drm show:
// docked while an external connector is connected, unknown without any
// connector to tell by.
func Read(sysDir string) (charge.Dock, error) {
	dir := filepath.Join(sysDir, "class", "drm")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return charge.DockUnknown, nil
	}
	if err != nil {
		return 0, fmt.Errorf("reading the display connectors in %s: %w", dir, err)
	}
	connectors := 0
	for _, entry := range entries {
		match := connectorName.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}
		connectors++
		if !external(match[1]) {
			continue
		}
		path := filepath.Join(dir, entry.Name(), "status")
		status, err := os.ReadFile(path)
		if err != nil {
			return 0, fmt.Errorf("reading %s: %w", path, err)
		}
		if strings.TrimSpace(string(status)) == "connected" {
			return charge.DockDocked, nil
		}
	}
	if connectors == 0 {
		return charge.DockUnknown, nil
	}
	return charge.DockUndocked, nil
}

func external(connector string) bool {
	for _, prefix := range notExternal {
		if strings.HasPrefix(connector, prefix) {
			return false
		}
	}
	return true
}

// IsDRM reports whether a kernel uevent, NUL-separated "action@devpath"
// and KEY=value fields, comes from the DRM subsystem, where a connected
// or disconnected display is announced.
func IsDRM(uevent []byte) bool {
	for field := range bytes.SplitSeq(uevent, []byte{0}) {
		if string(field) == "SUBSYSTEM=drm" {
			return true
		}
	}
	return false
}
