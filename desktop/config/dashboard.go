package config

// DashboardConfig configures the dashboard module.
//
// Quick-access button with a distro icon; opens the dashboard dropdown.
//
// `lys-shell panel dashboard` opens or closes it from a compositor bind, on
// the focused monitor or the one named.
//
// While it holds the keyboard, the arrow keys move between its controls,
// Home and End go to the first and the last, Return or Space press the
// focused one, Escape closes it, and on a slider Left and Right change the
// value while Up and Down leave it.
type DashboardConfig struct {
	// Override the auto-detected distro icon.
	IconOverride string `cfg:"icon-override"`
	// Display border around button.
	BorderShow bool `cfg:"border-show"`
	// Border color token.
	BorderColor ColorValue `cfg:"border-color"`
	// Icon foreground color. Auto selects based on variant for contrast.
	IconColor ColorValue `cfg:"icon-color"`
	// Icon container background color token.
	IconBgColor ColorValue `cfg:"icon-bg-color"`
	// Action on right click.
	RightClick ClickAction `cfg:"right-click"`
	// Action on middle click.
	MiddleClick ClickAction `cfg:"middle-click"`
	// Action on scroll up.
	ScrollUp ClickAction `cfg:"scroll-up"`
	// Action on scroll down.
	ScrollDown ClickAction `cfg:"scroll-down"`
	// Action on left click.
	LeftClick ClickAction `cfg:"left-click"`
	// CPU/RAM/disk usage percent at which the dashboard rings turn warning.
	UsageWarning float32 `cfg:"usage-warning"`
	// CPU/RAM/disk usage percent at which the dashboard rings turn error.
	UsageError float32 `cfg:"usage-error"`
	// CPU temperature (°C) at which the dashboard temp ring turns warning.
	TempWarning float32 `cfg:"temp-warning"`
	// CPU temperature (°C) at which the dashboard temp ring turns error.
	TempError float32 `cfg:"temp-error"`
	// Battery percent at or below which the dashboard battery shows warning.
	BatteryWarning float32 `cfg:"battery-warning"`
	// Battery percent at or below which the dashboard battery shows critical.
	BatteryCritical float32 `cfg:"battery-critical"`
	// User session configuration
	UserSession UserSessionConfig `cfg:"user-session"`
}

// DefaultsDashboard returns the schema defaults.
func DefaultsDashboard() DashboardConfig {
	return DashboardConfig{
		IconOverride:    "",
		BorderShow:      false,
		BorderColor:     mustColor("yellow"),
		IconColor:       mustColor("auto"),
		IconBgColor:     mustColor("yellow"),
		RightClick:      ClickAction{},
		MiddleClick:     ClickAction{},
		ScrollUp:        ClickAction{},
		ScrollDown:      ClickAction{},
		LeftClick:       ParseClickAction("dropdown:dashboard"),
		UsageWarning:    60,
		UsageError:      85,
		TempWarning:     65,
		TempError:       85,
		BatteryWarning:  30,
		BatteryCritical: 15,
		UserSession: UserSessionConfig{
			Actions: []SessionAction{
				SessionActionLock,
				SessionActionLogOut,
				SessionActionSuspend,
				SessionActionReboot,
				SessionActionPowerOff,
			},
		},
	}
}

// Clicks returns the five input bindings.
func (c DashboardConfig) Clicks() ClickConfig {
	return ClickConfig{c.LeftClick, c.RightClick, c.MiddleClick, c.ScrollUp, c.ScrollDown}
}

// UserSessionConfig is part of the dashboard module's configuration.
//
// Settings for user session the in dashboard
// ## Examples
//
// ```yaml
// modules: {dashboard: {user-session: {actions: [lock, log-out, reboot, power-off]}}}
// ```
type UserSessionConfig struct {
	// Session actions to show on dashboard
	Actions []SessionAction `cfg:"actions"`
}

// DefaultsUserSession returns the schema defaults.
func DefaultsUserSession() UserSessionConfig {
	return UserSessionConfig{
		Actions: []SessionAction{
			SessionActionLock,
			SessionActionLogOut,
			SessionActionSuspend,
			SessionActionReboot,
			SessionActionPowerOff,
		},
	}
}

// SessionAction is part of the dashboard module's configuration.
//
// One action the dashboard session actions
type SessionAction string

// SessionAction values.
const (
	// Lock the session
	SessionActionLock SessionAction = "lock"
	// Logout of the current session
	SessionActionLogOut SessionAction = "log-out"
	// Suspend the machine
	SessionActionSuspend SessionAction = "suspend"
	// Reboot the machine
	SessionActionReboot SessionAction = "reboot"
	// Power off the machine
	SessionActionPowerOff SessionAction = "power-off"
)

var _ = registerEnum(SessionActionLock, SessionActionLogOut, SessionActionSuspend, SessionActionReboot, SessionActionPowerOff)
