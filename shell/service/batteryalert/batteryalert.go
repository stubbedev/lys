// Package batteryalert decides when a battery deserves a notification
// and when the power profile follows the battery: pure functions over
// consecutive battery readings, so every transition is a table test.
package batteryalert

import (
	"github.com/stubbedev/lys/desktop/service/powerprofiles"
	"github.com/stubbedev/lys/desktop/service/powersupply"
)

// Reading is the part of a battery the alerts follow.
type Reading struct {
	Warning    powersupply.WarningLevel
	Percentage float64
	State      powersupply.DeviceState
}

// FromDevice is the DisplayDevice's reading.
func FromDevice(d powersupply.Device) Reading {
	return Reading{Warning: d.WarningLevel, Percentage: d.Percentage, State: d.State}
}

// FromPeripheral is a peripheral's reading.
func FromPeripheral(p powersupply.Peripheral) Reading {
	return Reading{Warning: p.WarningLevel, Percentage: p.Percentage, State: p.State}
}

// OnAC reports a battery being charged or held full: what plugging in
// looks like from the device.
func (r Reading) OnAC() bool {
	switch r.State {
	case powersupply.StateCharging, powersupply.StateFullyCharged, powersupply.StatePendingCharge:
		return true
	}
	return false
}

// Level is what an alert announces.
type Level uint8

// Alert levels.
const (
	LevelLow Level = iota + 1
	LevelCritical
	LevelCharged
)

// Next is the alert the step from prev to cur raises, if any. Low and
// Critical follow the battery's WarningLevel, so the thresholds are
// powersupply's PercentageLow and PercentageCritical: one alert on entering each, none
// while staying, and since plugging in clears the warning, unplugging
// again re-arms both. Charged fires once when a battery charging
// crosses chargedAt; zero turns it off.
func Next(prev, cur Reading, chargedAt uint8) (Level, bool) {
	if r := cur.Warning.Rank(); r > prev.Warning.Rank() {
		if r == powersupply.WarningCritical.Rank() {
			return LevelCritical, true
		}
		return LevelLow, true
	}
	limit := float64(chargedAt)
	if chargedAt > 0 && prev.OnAC() && cur.OnAC() && prev.Percentage < limit && cur.Percentage >= limit {
		return LevelCharged, true
	}
	return 0, false
}

// SaverAt is the warning level that switches to power-saver.
type SaverAt uint8

// Saver thresholds.
const (
	SaverNever SaverAt = iota
	SaverLow
	SaverCritical
)

// Saver is the automatic power-saver's memory: whether it switched,
// and the profile it switched away from ("" when power-saver was
// already active, so there is nothing to give back).
type Saver struct {
	Engaged bool
	Restore string
}

// Step is the saver after cur with active the current profile, and the
// profile to switch to, if any: power-saver on reaching at while on
// battery, the remembered profile once the battery is on AC again.
func (s Saver) Step(at SaverAt, cur Reading, active string) (Saver, string, bool) {
	switch {
	case s.Engaged && cur.OnAC():
		return Saver{}, s.Restore, s.Restore != ""
	case s.Engaged, at == SaverNever, cur.OnAC(), cur.Warning.Rank() < at.warning().Rank():
		return s, "", false
	case active == powerprofiles.ProfilePowerSaver:
		return Saver{Engaged: true}, "", false
	}
	return Saver{Engaged: true, Restore: active}, powerprofiles.ProfilePowerSaver, true
}

func (a SaverAt) warning() powersupply.WarningLevel {
	if a == SaverLow {
		return powersupply.WarningLow
	}
	return powersupply.WarningCritical
}

// OfferAt is the charge, in percent, at which a battery discharging is
// offered Low Power, the power-saver profile, as iOS offers it at 20%.
const OfferAt = 20

// Offer remembers whether Low Power was offered since the last unplug.
type Offer struct{ Made bool }

// Step is the offer after cur, and whether to make it now: once per
// discharge, at or below OfferAt, while active, which reads the current
// profile only when it is asked, says the profile is not power-saver
// already. ok false from active means there is no profile to switch.
// Plugging in re-arms it for the next unplug.
func (o Offer) Step(cur Reading, active func() (profile string, ok bool)) (Offer, bool) {
	switch {
	case cur.OnAC():
		return Offer{}, false
	case o.Made, cur.State != powersupply.StateDischarging, cur.Percentage > OfferAt:
		return o, false
	}
	profile, ok := active()
	if !ok || profile == powerprofiles.ProfilePowerSaver {
		return o, false
	}
	return Offer{Made: true}, true
}
