package config

// State views for the modules whose icon (and, for power profiles,
// color) follows a service state: the schema declares one key per
// state, the bar looks them up by state name.

import "github.com/stubbedev/lys/power/profile"

// Recorder states.
const (
	RecorderIdle      = "idle"
	RecorderRecording = "recording"
	RecorderPaused    = "paused"
	RecorderReplay    = "replay"
)

// Icons maps each recorder state to its icon.
func (c RecorderConfig) Icons() map[string]IconConfig {
	return map[string]IconConfig{
		RecorderIdle:      IconWith(c.IconShow, c.IconIdle, c.IconColor),
		RecorderRecording: IconWith(c.IconShow, c.IconRecording, c.IconColor),
		RecorderPaused:    IconWith(c.IconShow, c.IconPaused, c.IconColor),
		RecorderReplay:    IconWith(c.IconShow, c.IconReplay, c.IconColor),
	}
}

// Bitwarden vault states: a logged-out vault shows as locked.
const (
	BitwardenLocked   = "locked"
	BitwardenUnlocked = "unlocked"
)

// Icons maps each vault state to its icon.
func (c BitwardenConfig) Icons() map[string]IconConfig {
	return map[string]IconConfig{
		BitwardenLocked:   IconWith(c.IconShow, c.IconLocked, c.IconColor),
		BitwardenUnlocked: IconWith(c.IconShow, c.IconUnlocked, c.IconColor),
	}
}

// Idle-inhibit states.
const (
	IdleInhibitActive   = "active"
	IdleInhibitInactive = "inactive"
)

// Icons maps each idle-inhibit state to its icon.
func (c IdleInhibitConfig) Icons() map[string]IconConfig {
	return map[string]IconConfig{
		IdleInhibitActive:   IconWith(c.IconShow, c.IconActive, c.IconColor),
		IdleInhibitInactive: IconWith(c.IconShow, c.IconInactive, c.IconColor),
	}
}

// Icons maps each power profile to its icon.
func (c PowerProfilesConfig) Icons() map[profile.Profile]IconConfig {
	return map[profile.Profile]IconConfig{
		profile.PowerSaver:  IconWith(c.IconShow, c.IconPowerSaver, c.IconColor),
		profile.Balanced:    IconWith(c.IconShow, c.IconBalanced, c.IconColor),
		profile.Performance: IconWith(c.IconShow, c.IconPerformance, c.IconColor),
	}
}

// Colors maps each power profile to its icon color.
func (c PowerProfilesConfig) Colors() map[profile.Profile]ColorValue {
	return map[profile.Profile]ColorValue{
		profile.PowerSaver:  c.ColorPowerSaver,
		profile.Balanced:    c.ColorBalanced,
		profile.Performance: c.ColorPerformance,
	}
}

// IconOnView is the nightlight icon while the filter runs.
func (c NightlightConfig) IconOnView() IconConfig { return IconWith(c.IconShow, c.IconOn, c.IconColor) }

// IconOffView is the nightlight icon while the filter is off.
func (c NightlightConfig) IconOffView() IconConfig {
	return IconWith(c.IconShow, c.IconOff, c.IconColor)
}
