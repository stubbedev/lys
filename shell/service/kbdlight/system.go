package kbdlight

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/stubbedev/lys/lib/logind"
)

// SysfsDir is where the kernel lists the LEDs.
const SysfsDir = "/sys/class/leds"

// keyboardSuffix names the LEDs that light a keyboard, tpacpi::kbd_backlight
// or dell::kbd_backlight among them.
const keyboardSuffix = "::kbd_backlight"

// System is the keyboard backlights under Dir, read from sysfs and set
// through the logind session, which may set an LED without write access
// to sysfs.
type System struct {
	Dir     string
	Session logind.Session
}

// Levels reads every keyboard backlight under Dir, in name order.
func (s System) Levels(context.Context) ([]Level, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return nil, fmt.Errorf("list the LEDs: %w", err)
	}
	var levels []Level
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), keyboardSuffix) {
			continue
		}
		path := filepath.Join(s.Dir, e.Name(), "brightness")
		raw, err := os.ReadFile(path) //nolint:gosec // the path is Dir plus a directory entry
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		v, err := strconv.ParseUint(strings.TrimSpace(string(raw)), 10, 32)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		levels = append(levels, Level{Name: e.Name(), Raw: uint32(v)})
	}
	return levels, nil
}

// Set sets one keyboard backlight through logind.
func (s System) Set(ctx context.Context, name string, raw uint32) error {
	return s.Session.SetBrightness(ctx, logind.LEDs, name, raw)
}
