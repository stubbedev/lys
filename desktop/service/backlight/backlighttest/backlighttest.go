// Package backlighttest fakes the sysfs backlight class directory for
// tests of code that reads or writes backlights.
package backlighttest

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stubbedev/lys/desktop/service/backlight"
)

// Device is one fake backlight: its sysfs type attribute, level and
// maximum.
type Device struct {
	Type       string
	Brightness uint32
	Max        uint32
}

// Fake points backlight.Dir at a temporary tree holding devices for the
// test's lifetime, and returns the tree. Tests that use it must not run
// in parallel.
func Fake(t testing.TB, devices map[string]Device) string {
	t.Helper()
	dir := t.TempDir()
	orig := backlight.Dir
	backlight.Dir = dir
	t.Cleanup(func() { backlight.Dir = orig })
	for name, d := range devices {
		Add(t, name, d)
	}
	return dir
}

// Add puts one more device into the faked tree.
func Add(t testing.TB, name string, d Device) {
	t.Helper()
	base := filepath.Join(backlight.Dir, name)
	if err := os.MkdirAll(base, 0o750); err != nil {
		t.Fatal(err)
	}
	for attr, value := range map[string]string{
		"type":           d.Type + "\n",
		"brightness":     strconv.FormatUint(uint64(d.Brightness), 10),
		"max_brightness": strconv.FormatUint(uint64(d.Max), 10),
	} {
		if err := os.WriteFile(filepath.Join(base, attr), []byte(value), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// SetLevel writes a device's brightness attribute as the kernel would
// after a change.
func SetLevel(t testing.TB, name string, level uint32) {
	t.Helper()
	path := filepath.Join(backlight.Dir, name, "brightness")
	if err := os.WriteFile(path, []byte(strconv.FormatUint(uint64(level), 10)), 0o600); err != nil {
		t.Fatal(err)
	}
}
