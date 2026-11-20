package config

import "slices"

// NapLevel is part of the battery module's configuration.
//
// How hard an app naps while the user cannot see it.
type NapLevel string

// NapLevel values.
const (
	// Lower CPU and I/O weight and coalesced timers while it is unseen and silent.
	NapLevelDefault NapLevel = "default"
	// Never naps.
	NapLevelNever NapLevel = "never"
	// Naps by default, and runs on the efficiency cores alone while it does.
	NapLevelEfficiencyCores NapLevel = "efficiency-cores"
	// Naps by default, and is frozen while it does on battery power.
	NapLevelFrozen NapLevel = "frozen"
)

var napLevels = []NapLevel{NapLevelDefault, NapLevelNever, NapLevelEfficiencyCores, NapLevelFrozen}

var _ = registerEnum(napLevels...)

// NapLevels are the nap levels in schema order.
func NapLevels() []NapLevel { return slices.Clone(napLevels) }

// BatteryConfig configures the battery module.
//
// Battery level, charging state, and a dropdown with the batteries of
// peripherals UPower reports (wireless mice, keyboards, controllers,
// headsets) and power-profile controls. Low and critical battery levels
// send critical notifications whether or not the module is placed.
//
// ::: warning
//
// This module uses `upower` (D-Bus) for battery information. Ensure `upower` daemon is running and exposes a battery device (verify with `upower --battery` or `upower --dump`).
//
// :::
type BatteryConfig struct {
	// Icons for battery levels from empty to full.
	//
	// The percentage is divided evenly among icons. With 5 icons:
	// 0-20% uses icons\[0\], 21-40% uses icons\[1\], etc.
	LevelIcons []string `cfg:"level-icons"`
	// Icon shown when battery is charging.
	ChargingIcon string `cfg:"charging-icon"`
	// Icon shown when battery is not present or in an error state.
	AlertIcon string `cfg:"alert-icon"`
	// Display border around button.
	BorderShow bool `cfg:"border-show"`
	// Border color token.
	BorderColor ColorValue `cfg:"border-color"`
	// Display module icon.
	IconShow bool `cfg:"icon-show"`
	// Show the module's state in a tooltip on hover.
	Tooltip bool `cfg:"tooltip"`
	// Icon foreground color. Auto selects based on variant for contrast.
	IconColor ColorValue `cfg:"icon-color"`
	// Icon container background color token.
	IconBgColor ColorValue `cfg:"icon-bg-color"`
	// Display percentage label.
	LabelShow bool `cfg:"label-show"`
	// Label text color token.
	LabelColor ColorValue `cfg:"label-color"`
	// Format string for the label.
	//
	// ## Examples
	//
	// - `"{{ percent }}%"` - "45%"
	Format string `cfg:"format"`
	// Max label characters before truncation with ellipsis. Set to 0 to disable.
	LabelMaxLength uint32 `cfg:"label-max-length"`
	// Button background color token.
	ButtonBgColor ColorValue `cfg:"button-bg-color"`
	// Action on left click.
	LeftClick ClickAction `cfg:"left-click"`
	// Action on right click.
	RightClick ClickAction `cfg:"right-click"`
	// Action on middle click.
	MiddleClick ClickAction `cfg:"middle-click"`
	// Action on scroll up.
	ScrollUp ClickAction `cfg:"scroll-up"`
	// Action on scroll down.
	ScrollDown ClickAction `cfg:"scroll-down"`
	// Send a critical notification when the battery, or a peripheral UPower
	// reports such as a wireless mouse, enters UPower's low or critical
	// warning level.
	//
	// The levels follow UPower's `PercentageLow` and `PercentageCritical`.
	// Each level notifies once; plugging in and unplugging again re-arms them.
	// At 20% on battery it also offers Low Power (the power-saver profile)
	// with a button that turns it on, once until the next unplug, and never
	// while power-saver is already on.
	NotifyLow bool `cfg:"notify-low"`
	// Notify once when the charging battery reaches this percentage. Set to
	// 0 to disable.
	NotifyChargedAt Percentage `cfg:"notify-charged-at"`
	// How hard each app naps while the user cannot see it, by app ID.
	//
	// An app not listed naps at `default`. Freezing and the efficiency
	// cores are never the default: a frozen app's downloads and sockets
	// stop, and a hidden build is real work.
	//
	// Example: `{"org.mozilla.firefox": "never", "com.slack.Slack": "frozen"}`
	AppNap map[string]NapLevel `cfg:"app-nap"`
}

// DefaultsBattery returns the schema defaults.
func DefaultsBattery() BatteryConfig {
	return BatteryConfig{
		LevelIcons: []string{
			"ld-battery-symbolic",
			"ld-battery-low-symbolic",
			"ld-battery-medium-symbolic",
			"ld-battery-full-symbolic",
		},
		ChargingIcon:   "ld-battery-charging-symbolic",
		AlertIcon:      "ld-battery-warning-symbolic",
		BorderShow:     false,
		BorderColor:    mustColor("yellow"),
		IconShow:       true,
		Tooltip:        true,
		IconColor:      mustColor("auto"),
		IconBgColor:    mustColor("yellow"),
		LabelShow:      true,
		LabelColor:     mustColor("fg-default"),
		Format:         "{{ percent }}%",
		LabelMaxLength: 0,
		ButtonBgColor:  mustColor("bg-surface-elevated"),
		LeftClick:      ParseClickAction("dropdown:battery"),
		RightClick:     ClickAction{},
		MiddleClick:    ClickAction{},
		ScrollUp:       ClickAction{},
		ScrollDown:     ClickAction{},
		NotifyLow:      true,
		AppNap:         map[string]NapLevel{},
	}
}

// Clicks returns the five input bindings.
func (c BatteryConfig) Clicks() ClickConfig {
	return ClickConfig{c.LeftClick, c.RightClick, c.MiddleClick, c.ScrollUp, c.ScrollDown}
}
