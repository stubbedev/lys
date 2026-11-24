package nightlight

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/stubbedev/lys/lib/atomicfile"
)

// overrideFile is the record's name inside the lys-shell state dir.
const overrideFile = "nightlight-override"

// overrideMaxAge discards records old enough to span a whole day/night
// cycle (persist.rs MAX_AGE).
const overrideMaxAge = 12 * time.Hour

// Override is a manual toggle remembered across restarts: the filter
// state it chose and, when the auto-schedule ran, the solar phase it
// was made in (zero Phase for none).
type Override struct {
	Phase   Phase
	Enabled bool
}

// SaveOverride records a toggle made at now (persist.rs save), atomically.
func SaveOverride(dir string, o Override, now time.Time) error {
	enabled := "off"
	if o.Enabled {
		enabled = "on"
	}
	body := o.Phase.String() + " " + enabled + " " + now.UTC().Format(time.RFC3339) + "\n"
	return saveState(dir, overrideFile, body)
}

// saveState replaces a record in the state dir, creating the dir first.
func saveState(dir, name, body string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("nightlight: create the state dir %s: %w", dir, err)
	}
	if err := atomicfile.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		return fmt.Errorf("nightlight: save %s: %w", name, err)
	}
	return nil
}

// ClearOverride drops the record (a phase change ends it); having none
// to drop is not a failure.
func ClearOverride(dir string) error {
	err := os.Remove(filepath.Join(dir, overrideFile))
	if err == nil || errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return fmt.Errorf("nightlight: clear %s: %w", overrideFile, err)
}

// LoadOverride reads the record; false when absent, malformed, or
// stale.
func LoadOverride(dir string, now time.Time) (Override, bool) {
	body, err := os.ReadFile(filepath.Join(dir, overrideFile)) //nolint:gosec // lys-shell's own state dir
	if err != nil {
		return Override{}, false
	}
	return parseOverride(string(body), now)
}

// parseOverride is persist.rs parse: "<phase> <on|off> <rfc3339>".
func parseOverride(body string, now time.Time) (Override, bool) {
	fields := strings.Fields(body)
	if len(fields) < 3 {
		return Override{}, false
	}
	saved, err := time.Parse(time.RFC3339, fields[2])
	if err != nil {
		return Override{}, false
	}
	if now.Sub(saved) > overrideMaxAge {
		return Override{}, false
	}
	var phase Phase
	switch fields[0] {
	case "day":
		phase = PhaseDay
	case "night":
		phase = PhaseNight
	case "none":
	default:
		return Override{}, false
	}
	return Override{Phase: phase, Enabled: fields[1] == "on"}, true
}
