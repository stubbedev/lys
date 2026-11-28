// Package backlighttest fakes a sysfs backlight class directory for
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

// Fake is a class in a temporary tree holding devices for the test's
// lifetime.
func Fake(t testing.TB, devices map[string]Device) backlight.Class {
	t.Helper()
	class := backlight.Class(t.TempDir())
	for name, d := range devices {
		Add(t, class, name, d)
	}
	return class
}

// Add puts one more device into a faked class.
func Add(t testing.TB, class backlight.Class, name string, d Device) {
	t.Helper()
	base := filepath.Join(string(class), name)
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
func SetLevel(t testing.TB, class backlight.Class, name string, level uint32) {
	t.Helper()
	path := filepath.Join(string(class), name, "brightness")
	if err := os.WriteFile(path, []byte(strconv.FormatUint(uint64(level), 10)), 0o600); err != nil {
		t.Fatal(err)
	}
}
