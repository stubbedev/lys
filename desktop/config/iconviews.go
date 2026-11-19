package config

// Icon views: each module's icon-show/icon-color with the icon a
// module shows by default. Modules that swap icons by state build the
// view for the state's icon with IconWith.

// IconWith builds an icon view.
func IconWith(show bool, name string, color ColorValue) IconConfig {
	return IconConfig{Show: show, Name: name, Color: color}
}

// Icon is the battery's fallback (alert) icon.
func (c BatteryConfig) Icon() IconConfig { return IconWith(c.IconShow, c.AlertIcon, c.IconColor) }

// Icon is the bluetooth disconnected icon.
func (c BluetoothConfig) Icon() IconConfig {
	return IconWith(c.IconShow, c.DisconnectedIcon, c.IconColor)
}

// Icon is the keybind-mode icon.
func (c KeybindModeConfig) Icon() IconConfig { return IconWith(c.IconShow, c.IconName, c.IconColor) }

// Icon is the network offline icon.
func (c NetworkConfig) Icon() IconConfig {
	return IconWith(c.IconShow, c.WifiOfflineIcon, c.IconColor)
}

// Icon is the notifications bell.
func (c NotificationConfig) Icon() IconConfig { return IconWith(c.IconShow, c.IconName, c.IconColor) }

// Icon is the power icon; the power module always shows it.
func (c PowerConfig) Icon() IconConfig { return IconWith(true, c.IconName, c.IconColor) }

// Icon is the recorder's idle icon.
func (c RecorderConfig) Icon() IconConfig { return IconWith(c.IconShow, c.IconIdle, c.IconColor) }

// Icon is the CPU icon.
func (c CpuConfig) Icon() IconConfig { return IconWith(c.IconShow, c.IconName, c.IconColor) }

// Icon is the RAM icon.
func (c RamConfig) Icon() IconConfig { return IconWith(c.IconShow, c.IconName, c.IconColor) }

// Icon is the storage icon.
func (c StorageConfig) Icon() IconConfig { return IconWith(c.IconShow, c.IconName, c.IconColor) }

// Icon is the treeman stable icon.
func (c TreemanConfig) Icon() IconConfig { return IconWith(c.IconShow, c.IconName, c.IconColor) }

// Icon is the weather fallback icon.
func (c WeatherConfig) Icon() IconConfig { return IconWith(c.IconShow, c.IconName, c.IconColor) }

// Icon is the world-clock icon.
func (c WorldClockConfig) Icon() IconConfig { return IconWith(c.IconShow, c.IconName, c.IconColor) }

// Icon is the clock icon.
func (c ClockConfig) Icon() IconConfig { return IconWith(c.IconShow, c.IconName, c.IconColor) }

// Icon is the netstat icon.
func (c NetstatConfig) Icon() IconConfig { return IconWith(c.IconShow, c.IconName, c.IconColor) }

// Icon is the mail icon.
func (c MailConfig) Icon() IconConfig { return IconWith(c.IconShow, c.IconName, c.IconColor) }

// Icon is the media fallback icon.
func (c MediaConfig) Icon() IconConfig { return IconWith(c.IconShow, c.IconName, c.IconColor) }

// Icon is the window-title fallback icon.
func (c WindowTitleConfig) Icon() IconConfig { return IconWith(c.IconShow, c.IconName, c.IconColor) }

// Icon is the keyboard-input icon.
func (c KeyboardInputConfig) Icon() IconConfig {
	return IconWith(c.IconShow, c.IconName, c.IconColor)
}

// Icon is the audio module's speaker icon at full level; the module
// swaps the name by level and mute state.
func (c AudioConfig) Icon() IconConfig {
	name := c.SpeakerIconMuted
	if n := len(c.SpeakerIcons); n > 0 {
		name = c.SpeakerIcons[n-1]
	}
	return IconWith(c.IconShow, name, c.IconColor)
}

// Icon is the brightness module's lowest level icon, the one it shows
// before the backlight reports.
func (c BrightnessConfig) Icon() IconConfig {
	name := ""
	if len(c.LevelIcons) > 0 {
		name = c.LevelIcons[0]
	}
	return IconWith(c.IconShow, name, c.IconColor)
}

// Icon is the idle-inhibit icon while nothing inhibits idle, the state
// the module starts in.
func (c IdleInhibitConfig) Icon() IconConfig { return c.Icons()[IdleInhibitInactive] }

// Icon is the balanced profile's icon, the module's icon before the
// daemon reports a profile.
func (c PowerProfilesConfig) Icon() IconConfig { return c.Icons()[ProfileBalanced] }

// Icon is the locked vault's icon, the module's icon before the vault
// reports.
func (c BitwardenConfig) Icon() IconConfig { return c.Icons()[BitwardenLocked] }

// IconView is the screenshot button's icon.
func (c ScreenshotConfig) IconView() IconConfig { return IconWith(c.IconShow, c.Icon, c.IconColor) }

// IconView is the background-apps icon.
func (c BackgroundAppsConfig) IconView() IconConfig {
	return IconWith(c.IconShow, c.Icon, c.IconColor)
}

// DashboardFallbackIcon is the dashboard glyph shown when no distro
// icon is found.
const DashboardFallbackIcon = "ld-layout-dashboard-symbolic"

// Icon is the dashboard's icon before the distro's is looked up: the
// override when set, the dashboard glyph otherwise. The dashboard
// always shows it.
func (c DashboardConfig) Icon() IconConfig {
	name := c.IconOverride
	if name == "" {
		name = DashboardFallbackIcon
	}
	return IconWith(true, name, c.IconColor)
}

// ModuleIcon is the icon the module a layout names shows before its
// service reports anything; ok is false for a module that draws no icon
// of its own (the workspaces, the tray, the visualiser, the separator,
// the minimized windows) and for a custom module without a definition.
func (c *Config) ModuleIcon(m BarModule) (IconConfig, bool) {
	if id, ok := m.CustomID(); ok {
		def, ok := c.CustomByID(id)
		if !ok {
			return IconConfig{}, false
		}
		return def.Icon(), true
	}
	view, ok := map[BarModule]func() IconConfig{
		ModuleAudio:          c.Audio.Icon,
		ModuleBackgroundApps: c.BackgroundApps.IconView,
		ModuleBattery:        c.Battery.Icon,
		ModuleBluetooth:      c.Bluetooth.Icon,
		ModuleBrightness:     c.Brightness.Icon,
		ModuleBitwarden:      c.Bitwarden.Icon,
		ModuleClock:          c.Clock.Icon,
		ModuleCPU:            c.CPU.Icon,
		ModuleDashboard:      c.Dashboard.Icon,
		ModuleIdleInhibit:    c.IdleInhibit.Icon,
		ModuleKeybindMode:    c.KeybindMode.Icon,
		ModuleKeyboardInput:  c.KeyboardInput.Icon,
		ModuleMail:           c.Mail.Icon,
		ModuleMedia:          c.Media.Icon,
		ModuleNetstat:        c.Netstat.Icon,
		ModuleNetwork:        c.Network.Icon,
		ModuleNightlight:     c.Nightlight.IconOffView,
		ModuleNotifications:  c.Notification.Icon,
		ModulePower:          c.Power.Icon,
		ModulePowerProfiles:  c.PowerProfiles.Icon,
		ModuleRAM:            c.RAM.Icon,
		ModuleRecorder:       c.Recorder.Icon,
		ModuleScreenshot:     c.Screenshot.IconView,
		ModuleStorage:        c.Storage.Icon,
		ModuleTreeman:        c.Treeman.Icon,
		ModuleWeather:        c.Weather.Icon,
		ModuleWindowTitle:    c.WindowTitle.Icon,
		ModuleWorldClock:     c.WorldClock.Icon,
	}[m]
	if !ok {
		return IconConfig{}, false
	}
	return view(), true
}
