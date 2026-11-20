// Package supply reads and writes the kernel's power supply interfaces: AC
// presence from /sys/class/power_supply and the battery charge thresholds
// the embedded controller honors.
package supply

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/stubbedev/lys/lib/intconv"
)

const (
	startAttr = "charge_control_start_threshold"
	endAttr   = "charge_control_end_threshold"
)

// Supply addresses one sysfs power supply class directory.
type Supply struct {
	Dir string
}

// Battery names one discovered battery and its sysfs directory.
type Battery struct {
	Name string
	Path string
}

// OnAC reports whether any mains or USB-C source is currently online. Both
// types matter: a USB-C-only charger may never fire a Mains event.
func (s Supply) OnAC() (bool, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return false, fmt.Errorf("reading %s: %w", s.Dir, err)
	}
	for _, entry := range entries {
		dir := filepath.Join(s.Dir, entry.Name())
		typ, err := readTrim(filepath.Join(dir, "type"))
		if err != nil {
			continue
		}
		if typ != "Mains" && typ != "USB" {
			continue
		}
		online, err := readTrim(filepath.Join(dir, "online"))
		if err != nil {
			continue
		}
		if online == "1" {
			return true, nil
		}
	}
	return false, nil
}

// Battery finds the battery to manage: BAT0 when present, else the first
// supply of type Battery. It returns nil, nil on machines without a
// battery, where only profile handling applies.
func (s Supply) Battery() (*Battery, error) {
	if b, ok := s.batteryNamed("BAT0"); ok {
		return b, nil
	}
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", s.Dir, err)
	}
	for _, entry := range entries {
		if b, ok := s.batteryNamed(entry.Name()); ok {
			return b, nil
		}
	}
	return nil, nil
}

// batteryNamed reports whether name is a battery supply. A missing or
// unreadable type file just means "not a battery".
func (s Supply) batteryNamed(name string) (*Battery, bool) {
	dir := filepath.Join(s.Dir, name)
	typ, err := readTrim(filepath.Join(dir, "type"))
	if err != nil || typ != "Battery" {
		return nil, false
	}
	return &Battery{Name: name, Path: dir}, true
}

// Thresholds is a battery's charge_control start/end pair, in percent of
// full capacity.
type Thresholds struct {
	Start, End int
}

// Thresholds reads the current charge_control start and end thresholds.
func (b Battery) Thresholds() (Thresholds, error) {
	start, err := b.threshold(startAttr)
	if err != nil {
		return Thresholds{}, err
	}
	end, err := b.threshold(endAttr)
	if err != nil {
		return Thresholds{}, err
	}
	return Thresholds{Start: start, End: end}, nil
}

// Writable reports whether the charge thresholds can be written. A
// battery whose EC exposes no thresholds is false; a threshold that is
// there but cannot be opened for writing, such as on a read-only mount,
// is an error.
func (b Battery) Writable() (bool, error) {
	path := filepath.Join(b.Path, endAttr)
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("opening %s for writing: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return false, fmt.Errorf("closing %s: %w", path, err)
	}
	return true, nil
}

// SetThresholds moves both charge thresholds to want and reports whether
// anything was written. The embedded controller rejects a start above the
// current end, so the end is written first when raising and the start
// first when lowering; values already in place are never rewritten.
func (b Battery) SetThresholds(want Thresholds) (bool, error) {
	cur, err := b.Thresholds()
	if err != nil {
		return false, fmt.Errorf("reading %s thresholds: %w", b.Name, err)
	}
	if cur == want {
		return false, nil
	}
	for _, attr := range Order(cur.End, want.End) {
		value := want.Start
		if attr == endAttr {
			value = want.End
		}
		path := filepath.Join(b.Path, attr)
		if err := os.WriteFile(path, []byte(strconv.Itoa(value)), 0o600); err != nil {
			return false, fmt.Errorf("writing %s: %w", path, err)
		}
	}
	return true, nil
}

// Order returns the two charge_control attributes in the safe write order
// for moving from curEnd to end.
func Order(curEnd, end int) [2]string {
	if end > curEnd {
		return [2]string{endAttr, startAttr}
	}
	return [2]string{startAttr, endAttr}
}

func (b Battery) DischargeMicrowatts() (uint64, bool, error) {
	status, err := readTrim(filepath.Join(b.Path, "status"))
	if err != nil {
		return 0, false, fmt.Errorf("reading %s status: %w", b.Name, err)
	}
	if status != "Discharging" {
		return 0, false, nil
	}
	power, err := b.counter("power_now")
	if err == nil {
		return power, true, nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return 0, false, err
	}
	current, err := b.counter("current_now")
	if err != nil {
		return 0, false, err
	}
	voltage, err := b.counter("voltage_now")
	if err != nil {
		return 0, false, err
	}
	return uint64(float64(current) * float64(voltage) / 1e6), true, nil
}

// CapacityUnit is what a Capacity counts: microwatt-hours from the
// energy_* attributes or microampere-hours from the charge_* ones.
type CapacityUnit uint8

const (
	UnitEnergy CapacityUnit = iota + 1
	UnitCharge
)

// Capacity is what a battery holds when full, now and as designed, in
// one unit.
type Capacity struct {
	Full, Design uint64
	Unit         CapacityUnit
}

// Percent is the capacity now against the design.
func (c Capacity) Percent() float64 {
	return float64(c.Full) * 100 / float64(c.Design)
}

var capacityAttrs = [...]struct {
	full, design string
	unit         CapacityUnit
}{
	{"energy_full", "energy_full_design", UnitEnergy},
	{"charge_full", "charge_full_design", UnitCharge},
}

// Capacity reads the battery's full and design capacity, from the
// energy_* pair or else the charge_* pair, never one of each. It is
// false when neither pair is complete or the design reads 0, so health
// is hidden instead of computed from a wrong number.
func (b Battery) Capacity() (Capacity, bool, error) {
	for _, attrs := range capacityAttrs {
		full, ok, err := b.optionalCounter(attrs.full)
		if err != nil {
			return Capacity{}, false, err
		}
		if !ok {
			continue
		}
		design, ok, err := b.optionalCounter(attrs.design)
		if err != nil {
			return Capacity{}, false, err
		}
		if !ok || design == 0 {
			continue
		}
		return Capacity{Full: full, Design: design, Unit: attrs.unit}, true, nil
	}
	return Capacity{}, false, nil
}

// Cycles reads the battery's charge cycle count. It is false when the
// battery reports none, or 0, which drivers report when they do not
// know.
func (b Battery) Cycles() (uint64, bool, error) {
	cycles, ok, err := b.optionalCounter("cycle_count")
	if err != nil || !ok {
		return 0, false, err
	}
	return cycles, cycles > 0, nil
}

// EnergyMicrowattHours is the energy left in the battery: energy_now, or
// on a battery that counts charge instead, charge_now at voltage_now. It
// is false for a battery that reports neither.
func (b Battery) EnergyMicrowattHours() (uint64, bool, error) {
	energy, ok, err := b.optionalCounter("energy_now")
	if err != nil || ok {
		return energy, ok, err
	}
	charge, ok, err := b.optionalCounter("charge_now")
	if err != nil || !ok {
		return 0, false, err
	}
	voltage, err := b.counter("voltage_now")
	if err != nil {
		return 0, false, err
	}
	return uint64(float64(charge) * float64(voltage) / 1e6), true, nil
}

func (b Battery) optionalCounter(attr string) (uint64, bool, error) {
	value, err := b.counter(attr)
	if errors.Is(err, fs.ErrNotExist) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return value, true, nil
}

func (b Battery) counter(attr string) (uint64, error) {
	path := filepath.Join(b.Path, attr)
	raw, err := readTrim(path)
	if err != nil {
		return 0, fmt.Errorf("reading %s: %w", path, err)
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing %s %q: %w", path, raw, err)
	}
	if value < 0 {
		value = -value
	}
	return intconv.Exact[uint64](value)
}

func (b Battery) threshold(attr string) (int, error) {
	raw, err := readTrim(filepath.Join(b.Path, attr))
	if err != nil {
		return 0, fmt.Errorf("reading %s: %w", attr, err)
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("parsing %s %q: %w", attr, raw, err)
	}
	return value, nil
}

func readTrim(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(raw)), nil
}
