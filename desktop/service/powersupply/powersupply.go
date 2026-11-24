// Package powersupply reads battery and power source state from the
// kernel's power_supply class in sysfs: the machine's batteries as one
// aggregate, the peripherals that report a battery of their own (HID
// mice, keyboards, controllers, pens), whether the machine runs on its
// battery, and live updates through udevd's power_supply events.
package powersupply

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/stubbedev/lys/lib/intconv"
)

// Dir is where the kernel lists its power supplies.
const Dir = "/sys/class/power_supply"

var (
	// ErrMissing is an attribute a supply must have and does not.
	ErrMissing = errors.New("powersupply: attribute missing")
	// ErrMalformed is an attribute whose value the kernel never writes.
	ErrMalformed = errors.New("powersupply: attribute malformed")
)

// DeviceState is what a battery is doing.
type DeviceState uint8

// Device states, from the kernel's status attribute.
const (
	StateUnknown DeviceState = iota
	StateCharging
	StateDischarging
	StateFullyCharged
	// StatePendingCharge is a battery on AC that does not charge: held
	// under a charge limit.
	StatePendingCharge
)

var statuses = map[string]DeviceState{
	"Unknown":      StateUnknown,
	"Charging":     StateCharging,
	"Discharging":  StateDischarging,
	"Full":         StateFullyCharged,
	"Not charging": StatePendingCharge,
}

// WarningLevel is how close a battery is to empty.
type WarningLevel uint8

// Warning levels.
const (
	WarningNone WarningLevel = iota
	WarningLow
	WarningCritical
	// WarningAction is the machine's batteries about to run out.
	WarningAction
)

// Percentages at or under which a discharging battery warns.
const (
	PercentageLow      = 20
	PercentageCritical = 5
	PercentageAction   = 2
)

// Low reports Low, Critical, or Action: the levels the dropdown flags.
func (w WarningLevel) Low() bool { return w != WarningNone }

// Rank orders the levels a battery alert follows: 0 for none, 1 for
// Low, 2 for Critical and Action.
func (w WarningLevel) Rank() uint8 {
	switch w {
	case WarningLow:
		return 1
	case WarningCritical, WarningAction:
		return 2
	}
	return 0
}

// Device is the machine's batteries as one.
type Device struct {
	Percentage  float64
	State       DeviceState
	TimeToEmpty time.Duration
	TimeToFull  time.Duration
	// IsPresent is a battery in its bay.
	IsPresent bool
	// EnergyRate is the charge or discharge rate in watts.
	EnergyRate float64
	// Energy and EnergyFull are watt-hours now and at full charge.
	Energy     float64
	EnergyFull float64
	// Capacity is the health percentage (full against design); 0 when
	// unknown.
	Capacity float64
	// ChargeCycles is the charge cycle count; 0 when unknown.
	ChargeCycles uint32
	WarningLevel WarningLevel
}

// Peripheral is a device with a battery that does not power the
// machine.
type Peripheral struct {
	// Name is the supply's name under Dir.
	Name         string
	Kind         DeviceKind
	Model        string
	Percentage   float64
	State        DeviceState
	WarningLevel WarningLevel
}

// Source is what the bar module, its dropdown and the battery alerts
// read; tests swap in a fake.
type Source interface {
	// Read returns the machine's batteries.
	Read(ctx context.Context) (Device, error)
	// Subscribe ticks per change to what Read returns. The returned
	// stop function unsubscribes and returns once it has; the channel
	// closes then, or on ctx completion.
	Subscribe(ctx context.Context) (<-chan struct{}, func() error, error)
	// Peripherals returns the peripherals, ordered by name.
	Peripherals(ctx context.Context) ([]Peripheral, error)
	// SubscribePeripherals ticks per change to what Peripherals
	// returns.
	SubscribePeripherals(ctx context.Context) (<-chan struct{}, func() error, error)
}

type supplyType uint8

const (
	typeOther supplyType = iota
	typeBattery
	typeMains
)

var supplyTypes = map[string]supplyType{
	"Battery": typeBattery,
	"Mains":   typeMains,
	"USB":     typeMains,
}

type supply struct {
	name, dir string
}

func (s supply) path(attr string) string { return filepath.Join(s.dir, attr) }

func (s supply) text(attr string) (string, error) {
	raw, err := os.ReadFile(s.path(attr))
	if errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("%w: %s", ErrMissing, s.path(attr))
	}
	if err != nil {
		return "", fmt.Errorf("powersupply: read %s: %w", s.path(attr), err)
	}
	return strings.TrimSpace(string(raw)), nil
}

func (s supply) optionalText(attr string) (string, bool, error) {
	v, err := s.text(attr)
	if errors.Is(err, ErrMissing) {
		return "", false, nil
	}
	return v, err == nil, err
}

func (s supply) optionalInt(attr string) (int64, bool, error) {
	raw, ok, err := s.optionalText(attr)
	if !ok {
		return 0, false, err
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, false, fmt.Errorf("%w: %s is %q", ErrMalformed, s.path(attr), raw)
	}
	return v, true, nil
}

func (s supply) kind() (supplyType, error) {
	raw, err := s.text("type")
	if err != nil {
		return typeOther, err
	}
	return supplyTypes[raw], nil
}

func (s supply) present() (bool, error) {
	v, ok, err := s.optionalInt("present")
	return !ok || v != 0, err
}

func (s supply) status() (DeviceState, error) {
	raw, err := s.text("status")
	if err != nil {
		return StateUnknown, err
	}
	state, ok := statuses[raw]
	if !ok {
		return StateUnknown, fmt.Errorf("%w: %s is %q", ErrMalformed, s.path("status"), raw)
	}
	return state, nil
}

func (s supply) percent(attr string) (float64, bool, error) {
	v, ok, err := s.optionalInt(attr)
	if !ok {
		return 0, false, err
	}
	if v < 0 || v > 100 {
		return 0, false, fmt.Errorf("%w: %s is %d, not a percentage", ErrMalformed, s.path(attr), v)
	}
	return float64(v), true, nil
}

// Supplies lists what dir holds.
func supplies(dir string) ([]supply, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("powersupply: list %s: %w", dir, err)
	}
	out := make([]supply, 0, len(entries))
	for _, e := range entries {
		out = append(out, supply{name: e.Name(), dir: filepath.Join(dir, e.Name())})
	}
	return out, nil
}

// powersSystem says whether a battery powers the machine: the kernel's
// scope is Device for a peripheral's and absent or System otherwise.
func (s supply) powersSystem() (bool, error) {
	scope, _, err := s.optionalText("scope")
	return scope != "Device", err
}

type cell struct {
	percent              float64
	state                DeviceState
	energy, full, design float64
	rate                 float64
	hasEnergy            bool
	cycles               uint32
}

type energyAttrs struct {
	now, full, design, rate string
	scaled                  bool
}

var energyKinds = [...]energyAttrs{
	{"energy_now", "energy_full", "energy_full_design", "power_now", false},
	{"charge_now", "charge_full", "charge_full_design", "current_now", true},
}

const micro = 1e6

func (s supply) cell() (cell, error) {
	var c cell
	var err error
	if c.state, err = s.status(); err != nil {
		return cell{}, err
	}
	if err := s.readEnergy(&c); err != nil {
		return cell{}, err
	}
	percent, ok, err := s.percent("capacity")
	switch {
	case err != nil:
		return cell{}, err
	case ok:
		c.percent = percent
	case c.hasEnergy && c.full > 0:
		c.percent = min(100, c.energy*100/c.full)
	default:
		return cell{}, fmt.Errorf("%w: %s has neither capacity nor energy_now and energy_full", ErrMissing, s.dir)
	}
	cycles, _, err := s.optionalInt("cycle_count")
	if err != nil {
		return cell{}, err
	}
	c.cycles = intconv.Clamp[uint32](cycles)
	return c, nil
}

func (s supply) readEnergy(c *cell) error {
	for _, attrs := range energyKinds {
		now, ok, err := s.optionalInt(attrs.now)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		scale := 1.0
		if attrs.scaled {
			volts, ok, err := s.optionalInt("voltage_now")
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("%w: %s counts charge without voltage_now", ErrMissing, s.dir)
			}
			scale = float64(abs(volts)) / micro
		}
		values := [3]float64{}
		for i, attr := range [...]string{attrs.full, attrs.design, attrs.rate} {
			v, _, err := s.optionalInt(attr)
			if err != nil {
				return err
			}
			values[i] = float64(abs(v)) * scale / micro
		}
		c.energy = float64(abs(now)) * scale / micro
		c.full, c.design, c.rate = values[0], values[1], values[2]
		c.hasEnergy = true
		return nil
	}
	return nil
}

func abs(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}

// aggregate is the cells as one battery, as a desktop shows a laptop
// with two.
func aggregate(cells []cell) Device {
	if len(cells) == 0 {
		return Device{}
	}
	d := Device{IsPresent: true}
	var design, percentSum float64
	allEnergy := true
	for _, c := range cells {
		d.Energy += c.energy
		d.EnergyFull += c.full
		d.EnergyRate += c.rate
		design += c.design
		percentSum += c.percent
		allEnergy = allEnergy && c.hasEnergy
		d.ChargeCycles = max(d.ChargeCycles, c.cycles)
	}
	d.Percentage = percentSum / float64(len(cells))
	if allEnergy && d.EnergyFull > 0 && len(cells) > 1 {
		d.Percentage = min(100, d.Energy*100/d.EnergyFull)
	}
	if design > 0 && d.EnergyFull > 0 {
		d.Capacity = min(100, d.EnergyFull*100/design)
	}
	d.State = aggregateState(cells)
	if d.EnergyRate > 0 {
		switch d.State {
		case StateDischarging:
			d.TimeToEmpty = hours(d.Energy / d.EnergyRate)
		case StateCharging:
			d.TimeToFull = hours(max(0, d.EnergyFull-d.Energy) / d.EnergyRate)
		}
	}
	d.WarningLevel = systemWarning(d.State, d.Percentage)
	return d
}

func hours(h float64) time.Duration {
	return time.Duration(h * float64(time.Hour)).Round(time.Second)
}

func aggregateState(cells []cell) DeviceState {
	has := func(s DeviceState) bool {
		return slices.ContainsFunc(cells, func(c cell) bool { return c.state == s })
	}
	switch {
	case has(StateCharging):
		return StateCharging
	case has(StateDischarging):
		return StateDischarging
	case !slices.ContainsFunc(cells, func(c cell) bool { return c.state != StateFullyCharged }):
		return StateFullyCharged
	case has(StatePendingCharge):
		return StatePendingCharge
	}
	return StateUnknown
}

func systemWarning(state DeviceState, percent float64) WarningLevel {
	if state != StateDischarging {
		return WarningNone
	}
	if percent <= PercentageAction {
		return WarningAction
	}
	return peripheralWarning(state, percent)
}

func peripheralWarning(state DeviceState, percent float64) WarningLevel {
	switch {
	case state == StateCharging || state == StateFullyCharged:
		return WarningNone
	case percent <= PercentageCritical:
		return WarningCritical
	case percent <= PercentageLow:
		return WarningLow
	}
	return WarningNone
}

var capacityLevels = map[string]float64{
	"Full":     100,
	"High":     70,
	"Normal":   55,
	"Low":      10,
	"Critical": 5,
}

func (s supply) peripheralPercent() (float64, bool, error) {
	percent, ok, err := s.percent("capacity")
	if _, unreadable := errors.AsType[syscall.Errno](err); unreadable {
		return 0, false, nil
	}
	if err != nil || ok {
		return percent, ok, err
	}
	level, ok, err := s.optionalText("capacity_level")
	if !ok {
		return 0, false, err
	}
	if level == "Unknown" {
		return 0, false, nil
	}
	percent, known := capacityLevels[level]
	if !known {
		return 0, false, fmt.Errorf("%w: %s is %q", ErrMalformed, s.path("capacity_level"), level)
	}
	return percent, true, nil
}

func (s supply) peripheral() (Peripheral, bool, error) {
	percent, ok, err := s.peripheralPercent()
	if err != nil || !ok {
		return Peripheral{}, false, err
	}
	state, err := s.status()
	if err != nil {
		return Peripheral{}, false, err
	}
	model, _, err := s.optionalText("model_name")
	if err != nil {
		return Peripheral{}, false, err
	}
	kind, err := kindOf(s.dir)
	if err != nil {
		return Peripheral{}, false, err
	}
	return Peripheral{
		Name: s.name, Kind: kind, Model: model, Percentage: percent, State: state,
		WarningLevel: peripheralWarning(state, percent),
	}, true, nil
}

// snapshot is everything under a Dir, read once.
type snapshot struct {
	cells       []cell
	peripherals []Peripheral
	onAC        bool
}

func read(dir string) (snapshot, error) {
	list, err := supplies(dir)
	if err != nil {
		return snapshot{}, err
	}
	var snap snapshot
	for _, s := range list {
		if err := snap.add(s); err != nil {
			return snapshot{}, err
		}
	}
	return snap, nil
}

func (snap *snapshot) add(s supply) error {
	typ, err := s.kind()
	if err != nil {
		return err
	}
	switch typ {
	case typeMains:
		online, err := s.online()
		snap.onAC = snap.onAC || online
		return err
	case typeBattery:
	default:
		return nil
	}
	present, err := s.present()
	if err != nil || !present {
		return err
	}
	system, err := s.powersSystem()
	if err != nil {
		return err
	}
	if !system {
		p, ok, err := s.peripheral()
		if ok {
			snap.peripherals = append(snap.peripherals, p)
		}
		return err
	}
	c, err := s.cell()
	if err != nil {
		return err
	}
	snap.cells = append(snap.cells, c)
	return nil
}

// OnAC reports whether any mains or USB source under dir is online,
// reading no battery.
func OnAC(dir string) (bool, error) {
	list, err := supplies(dir)
	if err != nil {
		return false, err
	}
	for _, s := range list {
		typ, err := s.kind()
		if err != nil {
			return false, err
		}
		if typ != typeMains {
			continue
		}
		online, err := s.online()
		if err != nil || online {
			return online, err
		}
	}
	return false, nil
}

func (s supply) online() (bool, error) {
	v, _, err := s.optionalInt("online")
	return v == 1, err
}
