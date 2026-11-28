// Package backlight is the kernel's backlights as the lys apps share
// them: sysfs enumeration and reads, logind's SetBrightness as the write
// path with a direct sysfs fallback, and a watch that ticks whenever a
// level may have changed.
package backlight

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/stubbedev/lys/desktop/drmconn"
)

// Class is a sysfs class directory of backlight devices, a directory
// per device.
type Class string

// System is the kernel's backlight class.
const System Class = "/sys/class/backlight"

func (c Class) device(name string, attr ...string) string {
	return filepath.Join(append([]string{string(c), name}, attr...)...)
}

// Type distinguishes firmware, platform and raw backlights, and external
// monitors driven over DDC/CI.
type Type uint8

// Backlight types.
const (
	TypeRaw Type = iota
	TypeFirmware
	TypePlatform
	TypeDDC
)

// TypeFromSysfs parses the sysfs type attribute; unknown values are Raw.
func TypeFromSysfs(value string) Type {
	switch strings.TrimSpace(value) {
	case "firmware":
		return TypeFirmware
	case "platform":
		return TypePlatform
	}
	return TypeRaw
}

// Device is one backlight snapshot: its level against its maximum in raw
// units, and the DRM connector it is wired to, "" for none.
type Device struct {
	Name       string
	Type       Type
	Brightness uint32
	Max        uint32
	Connector  string
}

// Percentage maps the raw level onto 0..100.
func (d Device) Percentage() float64 {
	if d.Max == 0 {
		return 0
	}
	return float64(d.Brightness) / float64(d.Max) * 100
}

// AveragePercentage is the mean level across devices, false when there
// is none.
func AveragePercentage(devices []Device) (float64, bool) {
	if len(devices) == 0 {
		return 0, false
	}
	var sum float64
	for _, d := range devices {
		sum += d.Percentage()
	}
	return sum / float64(len(devices)), true
}

// RawLevel maps a percentage, clamped to 0..100, onto 0..top.
func RawLevel(percent float64, top uint32) uint32 {
	return uint32(math.Round(min(max(percent, 0), 100) / 100 * float64(top)))
}

// Source enumerates, watches and sets backlights.
type Source interface {
	// Devices returns the current snapshots.
	Devices(ctx context.Context) ([]Device, error)
	// Subscribe ticks whenever any device's level may have changed.
	// The channel closes when ctx completes; stop releases the watch.
	Subscribe(ctx context.Context) (<-chan struct{}, func(), error)
	// Set applies a percentage to one device, clamped to 0..100.
	Set(ctx context.Context, name string, percent float64) error
}

func readAttr(path string) (string, error) {
	file, err := os.Open(path) //nolint:gosec // the path is a Class plus a directory entry
	if err != nil {
		return "", err
	}
	defer file.Close()
	line, err := bufio.NewReader(file).ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf("backlight: read %s: %w", path, err)
	}
	return strings.TrimSpace(line), nil
}

func readUint(path string) (uint32, error) {
	line, err := readAttr(path)
	if err != nil {
		return 0, err
	}
	value, err := strconv.ParseUint(line, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("backlight: %s: %w", path, err)
	}
	return uint32(value), nil
}

func (c Class) connector(name string) (string, error) {
	path, err := filepath.EvalSymlinks(c.device(name))
	if err != nil {
		return "", fmt.Errorf("backlight: resolve %s: %w", name, err)
	}
	return drmconn.Ancestor(path), nil
}

// ReadDevice reads one device's snapshot from sysfs.
func (c Class) ReadDevice(name string) (Device, error) {
	rawType, err := readAttr(c.device(name, "type"))
	if err != nil {
		return Device{}, err
	}
	level, err := readUint(c.device(name, "brightness"))
	if err != nil {
		return Device{}, err
	}
	top, err := readUint(c.device(name, "max_brightness"))
	if err != nil {
		return Device{}, err
	}
	wired, err := c.connector(name)
	if err != nil {
		return Device{}, err
	}
	return Device{
		Name:       name,
		Type:       TypeFromSysfs(rawType),
		Brightness: level,
		Max:        top,
		Connector:  wired,
	}, nil
}

// Enumerate lists every backlight device that reads; an unreadable class
// directory yields an empty list.
func (c Class) Enumerate() []Device {
	entries, err := os.ReadDir(string(c))
	if err != nil {
		return nil
	}
	var devices []Device
	for _, entry := range entries {
		if device, err := c.ReadDevice(entry.Name()); err == nil {
			devices = append(devices, device)
		}
	}
	return devices
}
