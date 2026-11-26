package config

// PowerConfig configures the power module.
//
// Shutdown, reboot, and logout menu.
type PowerConfig struct {
	// Icon name to display.
	IconName string `cfg:"icon-name"`
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
	// Action on left click. Default opens lys-shell's native power menu (`:menu`).
	LeftClick ClickAction `cfg:"left-click"`
	// What the power menu's Lock button runs. The default `:lock` asks logind
	// to lock the session, which the lock screen answers; any other string
	// is a shell command line.
	LockCommand PowerCommand `cfg:"lock-command,type=Command"`
	// What the power menu's Log out button runs. The default `:logout` ends
	// the session through lys-wm, else logind; any other string is a shell
	// command line.
	LogoutCommand PowerCommand `cfg:"logout-command,type=Command"`
	// What the power menu's Suspend button runs. The default `:suspend`
	// asks logind to suspend; any other string is a shell command line.
	SuspendCommand PowerCommand `cfg:"suspend-command,type=Command"`
	// What the power menu's Reboot button runs. The default `:reboot` asks
	// logind to reboot; any other string is a shell command line.
	RebootCommand PowerCommand `cfg:"reboot-command,type=Command"`
	// What the power menu's Shut down button runs. The default `:poweroff`
	// asks logind to power off; any other string is a shell command line.
	ShutdownCommand PowerCommand `cfg:"shutdown-command,type=Command"`
	// Show the Lock button in the power menu.
	ShowLock bool `cfg:"show-lock"`
	// Show the Log out button in the power menu.
	ShowLogout bool `cfg:"show-logout"`
	// Show the Suspend button in the power menu.
	ShowSuspend bool `cfg:"show-suspend"`
	// Show the Reboot button in the power menu.
	ShowReboot bool `cfg:"show-reboot"`
	// Show the Shut down button in the power menu.
	ShowShutdown bool `cfg:"show-shutdown"`
}

// DefaultsPower returns the schema defaults.
func DefaultsPower() PowerConfig {
	return PowerConfig{
		IconName:        "ld-power-symbolic",
		BorderShow:      false,
		BorderColor:     mustColor("red"),
		IconColor:       mustColor("auto"),
		IconBgColor:     mustColor("red"),
		RightClick:      ClickAction{},
		MiddleClick:     ClickAction{},
		ScrollUp:        ClickAction{},
		ScrollDown:      ClickAction{},
		LeftClick:       ParseClickAction(":menu"),
		LockCommand:     BuiltinPowerCommand(PowerBuiltinLock),
		LogoutCommand:   BuiltinPowerCommand(PowerBuiltinLogout),
		SuspendCommand:  BuiltinPowerCommand(PowerBuiltinSuspend),
		RebootCommand:   BuiltinPowerCommand(PowerBuiltinReboot),
		ShutdownCommand: BuiltinPowerCommand(PowerBuiltinPowerOff),
		ShowLock:        true,
		ShowLogout:      true,
		ShowSuspend:     true,
		ShowReboot:      true,
		ShowShutdown:    true,
	}
}

// Clicks returns the five input bindings.
func (c PowerConfig) Clicks() ClickConfig {
	return ClickConfig{c.LeftClick, c.RightClick, c.MiddleClick, c.ScrollUp, c.ScrollDown}
}
