// Package wakesource lists the devices allowed to wake the machine, the
// ACPI devices of /proc/acpi/wakeup and the USB devices whose power/wakeup
// is enabled, and, where the user opts in, keeps removable USB devices
// from waking it on battery, since a mouse bumped in a bag wakes a closed
// laptop. Plan and Restore are pure; reading and writing sysfs happen at
// the edges.
package wakesource

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/stubbedev/lys/lib/intconv"
)

// Mode is whether removable USB devices may wake the machine on battery.
type Mode uint8

// The modes. ModeKeep, the default, leaves every device's wakeup alone;
// ModeBattery disables wakeup on removable USB devices on battery and gives
// it back on AC.
const (
	ModeKeep Mode = iota
	ModeBattery
)

var modeNames = [...]string{ModeKeep: "keep", ModeBattery: "battery"}

func (m Mode) String() string { return modeNames[m] }

// ParseMode reads a mode as String names it.
func ParseMode(s string) (Mode, error) {
	i := slices.Index(modeNames[:], s)
	if i < 0 {
		return 0, fmt.Errorf("USB wake policy %q is none of %s", s, strings.Join(modeNames[:], ", "))
	}
	return intconv.Must[Mode](i), nil
}

// ACPIDevice is one row of /proc/acpi/wakeup: the device's ACPI name,
// the deepest sleep state it can wake from, whether it may, and the
// devices it stands for, as bus:name.
type ACPIDevice struct {
	Name    string
	State   string
	Enabled bool
	Nodes   []string
}

// ParseACPI reads /proc/acpi/wakeup. A row whose ACPI device stands for
// more than one device continues on lines that start with tabs.
func ParseACPI(table string) ([]ACPIDevice, error) {
	lines := strings.Split(strings.TrimRight(table, "\n"), "\n")
	if len(lines) == 0 || !strings.HasPrefix(lines[0], "Device") {
		return nil, fmt.Errorf("header %q is not Device, S-state, Status, Sysfs node", lines[0])
	}
	var out []ACPIDevice
	for _, line := range lines[1:] {
		fields := strings.Fields(line)
		if strings.HasPrefix(line, "\t") {
			if len(out) == 0 || len(fields) != 2 {
				return nil, fmt.Errorf("continuation %q follows no device", line)
			}
			out[len(out)-1].Nodes = append(out[len(out)-1].Nodes, fields[1])
			continue
		}
		if len(fields) < 3 || len(fields) > 4 {
			return nil, fmt.Errorf("row %q is not device, state, status and node", line)
		}
		status := strings.TrimPrefix(fields[2], "*")
		if status != "enabled" && status != "disabled" {
			return nil, fmt.Errorf("row %q has status %q, not enabled or disabled", line, fields[2])
		}
		d := ACPIDevice{Name: fields[0], State: fields[1], Enabled: status == "enabled"}
		if len(fields) == 4 {
			d.Nodes = []string{fields[3]}
		}
		out = append(out, d)
	}
	return out, nil
}

// Removable is how the kernel places a USB device's port.
type Removable uint8

// The placements of the kernel's removable attribute. RemovableUnknown is
// a port the firmware does not describe.
const (
	RemovableUnknown Removable = iota
	RemovableFixed
	RemovableYes
)

var removableNames = [...]string{RemovableUnknown: "unknown", RemovableFixed: "fixed", RemovableYes: "removable"}

func (r Removable) String() string { return removableNames[r] }

func parseRemovable(s string) (Removable, error) {
	i := slices.Index(removableNames[:], s)
	if i < 0 {
		return 0, fmt.Errorf("removable %q is none of %s", s, strings.Join(removableNames[:], ", "))
	}
	return intconv.Must[Removable](i), nil
}

// USBDevice is one USB device that has a wakeup setting.
type USBDevice struct {
	Name      string
	Product   string
	Hub       bool
	Removable Removable
	Wakeup    string
	Path      string
}

// Enabled reports whether the device may wake the machine.
func (d USBDevice) Enabled() bool { return d.Wakeup == wakeEnabled }

const (
	wakeEnabled  = "enabled"
	wakeDisabled = "disabled"
)

// Change is one wakeup setting to write.
type Change struct {
	Device USBDevice
	Want   string
}

// Plan is the changes on battery under mode: wakeup disabled on every
// removable USB device that has it, never on a hub, whose wakeup carries
// that of the built-in devices behind it, nor on a device the firmware
// places as fixed or does not place at all.
func Plan(devices []USBDevice, mode Mode, onBattery bool) []Change {
	if mode != ModeBattery || !onBattery {
		return nil
	}
	var out []Change
	for _, d := range devices {
		if d.Hub || d.Removable != RemovableYes || !d.Enabled() {
			continue
		}
		out = append(out, Change{Device: d, Want: wakeDisabled})
	}
	return out
}

// Restore is the changes that give wakeup back to the devices, by sysfs
// path, Plan took it from and that are still there without it.
func Restore(devices []USBDevice, taken map[string]bool) []Change {
	var out []Change
	for _, d := range devices {
		if taken[d.Path] && !d.Enabled() {
			out = append(out, Change{Device: d, Want: wakeEnabled})
		}
	}
	return out
}

// Sysfs is the sysfs mounted at Root and the procfs at Proc.
type Sysfs struct {
	Root string
	Proc string
}

// ACPI reads /proc/acpi/wakeup; none on a machine without ACPI.
func (s Sysfs) ACPI() ([]ACPIDevice, error) {
	path := filepath.Join(s.Proc, "acpi/wakeup")
	raw, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	devices, err := ParseACPI(string(raw))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return devices, nil
}

// USB reads every USB device that has a wakeup setting.
func (s Sysfs) USB() ([]USBDevice, error) {
	dir := filepath.Join(s.Root, "bus/usb/devices")
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("listing %s: %w", dir, err)
	}
	var out []USBDevice
	for _, e := range entries {
		if strings.Contains(e.Name(), ":") {
			continue
		}
		d, ok, err := readUSB(filepath.Join(dir, e.Name()), e.Name())
		if err != nil {
			return nil, err
		}
		if ok {
			out = append(out, d)
		}
	}
	return out, nil
}

func readUSB(dev, name string) (USBDevice, bool, error) {
	wakeup, err := attr(dev, "power/wakeup")
	if errors.Is(err, fs.ErrNotExist) || err == nil && wakeup == "" {
		return USBDevice{}, false, nil
	}
	if err != nil {
		return USBDevice{}, false, err
	}
	class, err := attr(dev, "bDeviceClass")
	if err != nil {
		return USBDevice{}, false, err
	}
	removable, err := attr(dev, "removable")
	if errors.Is(err, fs.ErrNotExist) {
		removable, err = RemovableUnknown.String(), nil
	}
	if err != nil {
		return USBDevice{}, false, err
	}
	place, err := parseRemovable(removable)
	if err != nil {
		return USBDevice{}, false, fmt.Errorf("%s: %w", dev, err)
	}
	product, err := attr(dev, "product")
	if errors.Is(err, fs.ErrNotExist) {
		product, err = "", nil
	}
	if err != nil {
		return USBDevice{}, false, err
	}
	return USBDevice{Name: name, Product: product, Hub: class == "09", Removable: place, Wakeup: wakeup, Path: filepath.Join(dev, "power/wakeup")}, true, nil
}

// Apply writes changes, each named in the error of one that fails while
// the others are still written.
func Apply(changes []Change) error {
	var errs []error
	for _, c := range changes {
		if err := os.WriteFile(c.Device.Path, []byte(c.Want), 0o644); err != nil { //nolint:gosec // a sysfs attribute, whose mode the kernel sets
			errs = append(errs, fmt.Errorf("setting %s wakeup to %s: %w", c.Device.Name, c.Want, err))
		}
	}
	return errors.Join(errs...)
}

func attr(dev, name string) (string, error) {
	path := filepath.Join(dev, name)
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", path, err)
	}
	return strings.TrimSpace(string(raw)), nil
}
