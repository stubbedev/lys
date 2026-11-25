package settings

import (
	"log"
	"strings"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// The module pages, in the order the sidebar lists them.

func modulePages(cfg *config.Config) []pageSpec {
	pages := []pageSpec{
		audioPage(cfg),
		backgroundAppsPage(cfg),
		batteryPage(cfg),
		bitwardenPage(cfg),
		bluetoothPage(cfg),
		brightnessPage(cfg),
		cavaPage(cfg),
		clockPage(cfg),
		cpuPage(cfg),
		customPage(cfg),
		dashboardPage(cfg),
		nightlightPage(cfg),
		idleInhibitPage(cfg),
		keybindModePage(cfg),
		keyboardInputPage(cfg),
		mailPage(cfg),
		mediaPage(cfg),
		minimizedWindowsPage(cfg),
		netstatPage(cfg),
		networkPage(cfg),
		notificationBellPage(cfg),
		powerPage(cfg),
		powerProfilesPage(cfg),
		ramPage(cfg),
		recorderPage(cfg),
		screenshotPage(cfg),
		separatorPage(cfg),
		storagePage(cfg),
		systrayPage(cfg),
		treemanPage(cfg),
		weatherPage(cfg),
		windowTitlePage(cfg),
		workspacesPage(cfg),
		worldClockPage(cfg),
	}
	return pages
}

// moduleLinks are the pages a bar module's page points to for what the
// bar item only shows, by module.
var moduleLinks = map[config.BarModule][]pageLink{
	config.ModuleAudio:         {{soundID, "settings-module-link-sound-detail"}},
	config.ModuleBattery:       {{powerPageID, "settings-module-link-power-detail"}},
	config.ModuleBluetooth:     {{bluetoothPageID, "settings-module-link-bluetooth-detail"}},
	config.ModuleBrightness:    {{displaysID, "settings-module-link-brightness-detail"}},
	config.ModuleClock:         {{dateTimeID, "settings-module-link-date-time-detail"}},
	config.ModuleDashboard:     {thresholdsLink},
	config.ModuleIdleInhibit:   {{"idle", "settings-module-link-idle-detail"}},
	config.ModuleKeybindMode:   {{wmBindsID, "settings-module-link-binds-detail"}},
	config.ModuleKeyboardInput: {{"wm-keyboard", "settings-module-link-input-detail"}},
	config.ModuleNightlight:    {{displaysID, "settings-module-link-nightlight-detail"}},
}

const thresholdsPageID = "thresholds"

var thresholdsLink = pageLink{thresholdsPageID, "settings-module-link-thresholds-detail"}

func thresholdsPage(*config.Config) pageSpec {
	return pageSpec{id: thresholdsPageID, navKey: "settings-nav-thresholds", icon: "ld-activity-symbolic", header: "settings-page-thresholds", sections: []sectionSpec{
		{title: "settings-section-thresholds", rows: []rowSpec{
			field("thresholds.cpu", thresholdList),
			field("thresholds.ram", thresholdList),
			field("thresholds.storage", thresholdList),
			field("thresholds.battery", levelThresholdList(levelScale{icons: "modules.battery.level-icons"})),
		}},
	}}
}

var dropdownModules = map[string]config.BarModule{
	"audio": "audio", "battery": "battery", "bitwarden": "bitwarden", "bluetooth": "bluetooth", "brightness": "brightness",
	"calendar": "clock", "dashboard": "dashboard", "mail": "mail", "media": "media", "network": "network",
	"notification": "notifications", "treeman": "treeman", "weather": "weather",
}

var dropdownSizesLink = pageLink{barPageID, "settings-module-link-dropdown-size-detail"}

func dropdownNames() []string {
	var names []string
	for _, p := range config.LeafPaths() {
		if name, ok := strings.CutPrefix(p, "dropdowns."); ok {
			names = append(names, name)
		}
	}
	return names
}

func dropdownSizeTable() rowSpec {
	names := dropdownNames()
	rows := make([]gridRow, len(names))
	for i, name := range names {
		m, ok := dropdownModules[name]
		if !ok {
			log.Panicf("settings: the dropdown %s belongs to no bar module", name)
		}
		rows[i] = gridRow{key: "settings-module-" + string(m), cells: dropdownSizeRows("dropdowns." + name)}
	}
	return cellGridRow("settings-section-sizes", []string{"settings-dropdown-width", "settings-dropdown-height"}, rows...)
}

func hasDropdown(module config.BarModule) bool {
	for _, m := range dropdownModules {
		if m == module {
			return true
		}
	}
	return false
}

func backgroundAppsPage(*config.Config) pageSpec {
	return barItemPage(config.ModuleBackgroundApps, pageSpec{id: "background-apps", navKey: "settings-nav-background-apps", icon: "ld-app-window-symbolic", header: "settings-page-background-apps"}, barItemParts{
		icons: iconPerState("modules.background-apps.icon"),
	})
}

var barItemGroups = []struct {
	key, icon string
	ids       []string
}{
	{"settings-nav-bar-items-status", "", []string{
		"audio", "background-apps", "battery", "bluetooth", "brightness", "cpu", "dashboard", "idle-inhibit", "keybind-mode",
		"keyboard-input", "netstat", "network", "nightlight", notificationBellID, "power", "power-profiles", "ram", "storage", "systray",
	}},
	{"settings-nav-bar-items-media", "ld-music-symbolic", []string{"cava", "media", "recorder", "screenshot"}},
	{"settings-nav-bar-items-productivity", "", []string{"bitwarden", "clock", "custom", "mail", "separator", "treeman", "weather", "world-clock"}},
	{"settings-nav-bar-items-windows", "ld-layers-symbolic", []string{"minimized-windows", "window-title", "workspaces"}},
}

func barItemSections(cfg *config.Config) []navSection {
	byID := map[string]pageSpec{}
	for _, p := range modulePages(cfg) {
		byID[p.id] = p
	}
	out := make([]navSection, 0, len(barItemGroups))
	for _, g := range barItemGroups {
		pages := make([]pageSpec, 0, len(g.ids))
		for _, id := range g.ids {
			p, ok := byID[id]
			if !ok {
				log.Panicf("settings: bar item group %s names no module page %q", g.key, id)
			}
			p.unlisted = unlistedOptional(cfg, id)
			pages = append(pages, p)
		}
		out = append(out, navSection{key: g.key, pages: pages, icon: g.icon})
	}
	return out
}

// audioPage is pages/modules/audio.
func audioPage(*config.Config) pageSpec {
	const m = "modules.audio"
	return barItemPage(config.ModuleAudio, pageSpec{id: "audio", navKey: "settings-nav-audio", icon: "ld-volume-2-symbolic", header: "settings-page-audio"}, barItemParts{
		sections: []sectionSpec{
			{title: "settings-section-general", rows: []rowSpec{field(m + ".format")}},
			{title: "settings-section-capture", rows: []rowSpec{captureTilesRow(m)}},
			{title: "settings-section-dropdown", rows: []rowSpec{field(m + ".dropdown-app-icons")}},
		},
		icons:      iconPerState(m + ".speaker-icon-muted"),
		iconRows:   []rowSpec{field(m+".speaker-icons", iconList)},
		thresholds: []rowSpec{field(m+".thresholds", levelThresholdList(levelScale{icons: m + ".speaker-icons", muted: m + ".speaker-icon-muted"}))},
	})
}

// batteryPage is pages/modules/battery.
func batteryPage(*config.Config) pageSpec {
	const m = "modules.battery"
	return barItemPage(config.ModuleBattery, pageSpec{id: "battery", navKey: "settings-nav-battery", icon: "ld-battery-full-symbolic", header: "settings-page-battery"}, barItemParts{
		sections: []sectionSpec{{title: "settings-section-general", rows: []rowSpec{field(m + ".format")}}},
		icons:    iconPerState(m+".charging-icon", m+".alert-icon"),
		iconRows: []rowSpec{field(m+".level-icons", iconList)},
	})
}

// bitwardenPage is pages/modules/bitwarden.
func bitwardenPage(*config.Config) pageSpec {
	const m = "modules.bitwarden"
	return barItemPage(config.ModuleBitwarden, pageSpec{id: "bitwarden", navKey: "settings-nav-bitwarden", icon: "ld-shield-check-symbolic", header: "settings-page-bitwarden"}, barItemParts{
		sections: []sectionSpec{
			{title: "settings-section-general", rows: []rowSpec{field(m + ".format")}},
			{title: "settings-section-bitwarden-account", preview: newVaultAccount, rows: []rowSpec{field(m + ".server"), field(m+".email", hinted(func() string { return i18n.T("dropdown-bitwarden-email-placeholder") }))}},
			{title: "settings-section-bitwarden-security", rows: []rowSpec{
				field(m + ".lock-with-session"),
				field(m + ".unlock-with-session"),
				field(m+".clipboard-clear", durationChips(inSeconds, after(10*time.Second), after(30*time.Second), after(time.Minute), after(5*time.Minute), zeroAs(never))),
			}},
			{title: "settings-section-bitwarden-integrations", rows: []rowSpec{field(m+".network-folder", vaultFolderRow), field(m+".portal-folder", vaultFolderRow)}},
			{title: "settings-section-generator", rows: []rowSpec{
				field(m+".generator-passwords", passwordPresetList),
				field(m+".generator-passphrases", passphrasePresetList),
			}},
		},
		icons: iconPerState(m+".icon-locked", m+".icon-unlocked"),
	})
}

// bluetoothPage is pages/modules/bluetooth.
func bluetoothPage(*config.Config) pageSpec {
	const m = "modules.bluetooth"
	return barItemPage(config.ModuleBluetooth, pageSpec{id: "bluetooth", navKey: "settings-nav-bluetooth", icon: "ld-bluetooth-symbolic", header: "settings-page-bluetooth"}, barItemParts{
		icons: iconPerState(m+".disabled-icon", m+".disconnected-icon", m+".connected-icon", m+".searching-icon"),
	})
}

// brightnessPage is pages/modules/brightness.
func brightnessPage(*config.Config) pageSpec {
	const m = brightnessPath
	return barItemPage(config.ModuleBrightness, pageSpec{id: "brightness", navKey: "settings-nav-brightness", icon: "ld-sun-symbolic", header: "settings-page-brightness"}, barItemParts{
		sections:   []sectionSpec{{title: "settings-section-general", rows: []rowSpec{field(m + ".format")}}},
		iconRows:   []rowSpec{field(m+".level-icons", iconList)},
		thresholds: []rowSpec{field(m+".thresholds", levelThresholdList(levelScale{icons: m + ".level-icons", floor: m + ".min-brightness"}))},
	})
}

// clockPage is pages/modules/clock.
func clockPage(*config.Config) pageSpec {
	const m = "modules.clock"
	return barItemPage(config.ModuleClock, pageSpec{id: "clock", navKey: "settings-nav-clock", icon: "ld-clock-symbolic", header: "settings-page-clock"}, barItemParts{
		sections: []sectionSpec{
			{title: "settings-section-general", rows: []rowSpec{field(m + ".format")}},
			{title: "settings-section-dropdown", rows: []rowSpec{field(m + ".dropdown-show-seconds")}},
			{title: "settings-section-calendars", rows: []rowSpec{field(m+".calendars", calendarList)}},
		},
		icons: iconPerState(m + ".icon-name"),
	})
}

var pollInterval = durationChips(inMilliseconds, after(time.Second), after(2*time.Second), after(5*time.Second), after(10*time.Second))

// cpuPage is pages/modules/cpu.
func cpuPage(*config.Config) pageSpec {
	const m = "modules.cpu"
	return barItemPage(config.ModuleCPU, pageSpec{id: "cpu", navKey: "settings-nav-cpu", icon: "ld-cpu-symbolic", header: "settings-page-cpu"}, barItemParts{
		sections: []sectionSpec{{title: "settings-section-general", rows: []rowSpec{
			field(m+".poll-interval-ms", pollInterval),
			field(m + ".temp-sensor"),
			field(m + ".format"),
		}}},
		icons: iconPerState(m + ".icon-name"),
	})
}

// dashboardPage is pages/modules/dashboard.
func dashboardPage(*config.Config) pageSpec {
	const m = "modules.dashboard"
	return barItemPage(config.ModuleDashboard, pageSpec{id: "dashboard", navKey: "settings-nav-dashboard", icon: "ld-layout-dashboard-symbolic", header: "settings-page-dashboard"}, barItemParts{
		sections: []sectionSpec{{title: "settings-section-commands", rows: []rowSpec{field(m+".user-session.actions", enumList)}}},
		icons:    iconPerState(m + ".icon-override"),
		thresholds: []rowSpec{rangeRow(m+".temp-warning", m+".temp-error", "settings-dashboard-temp-levels",
			levelRange(celsiusLevels, "settings-level-unit-celsius", [2]string{"settings-level-warning", "settings-level-error"}, risingLevels))},
	})
}

// nightlightPage is pages/modules/nightlight.
func nightlightPage(*config.Config) pageSpec {
	const m = nightlightPath
	return barItemPage(config.ModuleNightlight, pageSpec{id: "nightlight", navKey: "settings-nav-nightlight", icon: "ld-sun-symbolic", header: "settings-page-nightlight"}, barItemParts{
		sections: []sectionSpec{{title: "settings-section-general", rows: []rowSpec{field(m + ".format")}}},
		icons:    iconPerState(m+".icon-off", m+".icon-on"),
	})
}

const (
	nightlightPath = "modules.nightlight"
	brightnessPath = "modules.brightness"
)

// displayLightSections are the Displays page's brightness and night
// light: what the brightness and night light modules do to the screen.
func displayLightSections() []sectionSpec {
	const m = nightlightPath
	scheduled := shownWhen(m+".auto-schedule", switchedOn)
	following := func(s config.NightlightSchedule) rowOpt { return shownWhen(m+".schedule", valueIn(string(s))) }
	return []sectionSpec{
		{title: "settings-section-displays-brightness", rows: []rowSpec{
			field(brightnessPath + ".min-brightness"),
			field(brightnessPath+".enable-external", ddcRow),
		}},
		{title: "settings-section-nightlight-filter", lead: nightlightNow, rows: []rowSpec{field(m+".temperature", kelvin)}},
		{title: "settings-section-nightlight-schedule", preview: nightlightDayPreview, rows: []rowSpec{
			field(m + ".auto-schedule"),
			field(m+".schedule", scheduled),
			field(m+".custom-hours", clockRangeRow, following(config.NightlightScheduleCustom)),
			field(m+".transition-duration", durationChips(inMinutes, zeroAs(instant), after(15*time.Minute), after(30*time.Minute), after(time.Hour)), scheduled),
			field(m+".location", locationRow(automaticOrPinned), following(config.NightlightScheduleSun)),
		}},
		{title: "settings-section-nightlight-display", advanced: true, rows: []rowSpec{field(m+".gamma", gammaPercent)}},
	}
}

// The night light's strength, over the range its dropdown card offers.
const (
	warmestKelvin = 1000
	coolestKelvin = 6600
	gammaMost     = 200
	kelvinStops   = 8
)

var gammaPercent = withEditor(func(k *kit, s slot, _ config.FieldMeta) control {
	return newSlider(k, s, 0, gammaMost, true, unitPercent.format(0))
})

// idleInhibitPage is pages/modules/idle_inhibit.
func idleInhibitPage(*config.Config) pageSpec {
	const m = "modules.idle-inhibit"
	return barItemPage(config.ModuleIdleInhibit, pageSpec{id: "idle-inhibit", navKey: "settings-nav-idle-inhibit", icon: "ld-coffee-symbolic", header: "settings-page-idle-inhibit"}, barItemParts{
		sections: []sectionSpec{{title: "settings-section-general", rows: []rowSpec{
			field(m+".startup-duration", durationChips(inMinutes, after(15*time.Minute), after(30*time.Minute), after(time.Hour), after(2*time.Hour), zeroAs(untilOff))),
			field(m + ".format"),
		}}},
		icons: iconPerState(m+".icon-inactive", m+".icon-active"),
	})
}

// keybindModePage is pages/modules/keybind_mode.
func keybindModePage(*config.Config) pageSpec {
	const m = "modules.keybind-mode"
	return barItemPage(config.ModuleKeybindMode, pageSpec{id: "keybind-mode", navKey: "settings-nav-keybind-mode", icon: "ld-layers-symbolic", header: "settings-page-keybind-mode"}, barItemParts{
		preview:  steppingThrough(config.ModuleKeybindMode, submapStates),
		sections: []sectionSpec{{title: "settings-section-general", rows: []rowSpec{field(m + ".format"), field(m + ".auto-hide")}}},
		icons:    iconPerState(m + ".icon-name"),
	})
}

// keyboardInputPage is pages/modules/keyboard_input.
func keyboardInputPage(*config.Config) pageSpec {
	const m = "modules.keyboard-input"
	return barItemPage(config.ModuleKeyboardInput, pageSpec{id: "keyboard-input", navKey: "settings-nav-keyboard-input", icon: "ld-keyboard-symbolic", header: "settings-page-keyboard-input"}, barItemParts{
		preview:  steppingThrough(config.ModuleKeyboardInput, layoutStates),
		sections: []sectionSpec{{title: "settings-section-general", rows: []rowSpec{field(m + ".format"), field(m+".layout-alias-map", layoutAliasRow)}}},
		icons:    iconPerState(m + ".icon-name"),
	})
}

// mediaPage is pages/modules/media.
func mediaPage(*config.Config) pageSpec {
	const m = "modules.media"
	iconType := func(t config.MediaIconType) rowOpt { return shownWhen(m+".icon-type", valueIn(string(t))) }
	return barItemPage(config.ModuleMedia, pageSpec{id: "media", navKey: "settings-nav-media", icon: "ld-music-symbolic", header: "settings-page-media"}, barItemParts{
		sections: []sectionSpec{
			{title: "settings-section-general", rows: []rowSpec{field(m + ".format")}},
			{title: "settings-section-media-players", rows: []rowSpec{
				field(m+".players-ignored", mediaPlayersRow),
				field(m+".player-priority", mediaPlayersRow),
			}},
			{title: "settings-section-online-lyrics", rows: []rowSpec{field(m + ".lyrics")}},
		},
		icons: iconPerState(m+".icon-name", m+".spinning-disc-icon"),
		iconRows: []rowSpec{
			field(m+".icon-type", pictureChoice(mediaIconPictures)),
			field(m+".player-icons", iconMap, iconType(config.MediaIconTypeApplicationMapped)),
		},
	})
}

var mediaIconPictures = map[string]picture{
	string(config.MediaIconTypeDefault):           {icon: "ld-music-symbolic"},
	string(config.MediaIconTypeApplication):       {icon: "ld-app-window-symbolic"},
	string(config.MediaIconTypeSpinningDisc):      {icon: "ld-disc-3-symbolic"},
	string(config.MediaIconTypeApplicationMapped): {icon: "ld-layout-grid-symbolic"},
}

// netstatPage is pages/modules/netstat.
func netstatPage(*config.Config) pageSpec {
	const m = "modules.netstat"
	return barItemPage(config.ModuleNetstat, pageSpec{id: "netstat", navKey: "settings-nav-netstat", icon: "ld-activity-symbolic", header: "settings-page-netstat"}, barItemParts{
		sections: []sectionSpec{{title: "settings-section-general", rows: []rowSpec{
			field(m+".poll-interval-ms", pollInterval),
			field(m + ".interface"),
			field(m + ".format"),
		}}},
		icons: iconPerState(m + ".icon-name"),
	})
}

// networkPage is pages/modules/network.
func networkPage(*config.Config) pageSpec {
	const m = "modules.network"
	return barItemPage(config.ModuleNetwork, pageSpec{id: "network", navKey: "settings-nav-network", icon: "ld-wifi-symbolic", header: "settings-page-network"}, barItemParts{
		sections: []sectionSpec{{title: "settings-section-general", rows: []rowSpec{field(m + ".vpn-show")}}},
		icons: stateIcons{states: []string{"settings-wifi-title", "settings-wifi-wired", "settings-vpn-section"}, rows: []gridRow{
			{key: "settings-state-off", cells: stateIconCells(m+".wifi-disabled-icon", "", "")},
			{key: "settings-state-disconnected", cells: stateIconCells(m+".wifi-offline-icon", m+".wired-disconnected-icon", m+".vpn-disconnected-icon")},
			{key: "settings-state-connecting", cells: stateIconCells(m+".wifi-acquiring-icon", m+".wired-acquiring-icon", m+".vpn-connecting-icon")},
			{key: "settings-state-connected", cells: stateIconCells(m+".wifi-connected-icon", m+".wired-connected-icon", m+".vpn-connected-icon")},
		}},
		iconRows: []rowSpec{field(m+".wifi-signal-icons", iconList)},
	})
}

// powerProfilesPage is pages/modules/power_profiles.
func powerProfilesPage(*config.Config) pageSpec {
	const m = "modules.power-profiles"
	return barItemPage(config.ModulePowerProfiles, pageSpec{id: "power-profiles", navKey: "settings-nav-power-profiles", icon: "ld-scale-symbolic", header: "settings-page-power-profiles"}, barItemParts{
		sections: []sectionSpec{{title: "settings-section-general", rows: []rowSpec{field(m + ".format")}}},
		icons: stateIcons{states: []string{"settings-power-mode-power-saver", "settings-power-mode-balanced", "settings-power-mode-performance"}, rows: []gridRow{
			{key: "settings-grid-icon", cells: stateIconCells(m+".icon-power-saver", m+".icon-balanced", m+".icon-performance")},
			{key: "settings-grid-color", cells: gridCells([]string{m + ".color-power-saver", m + ".color-balanced", m + ".color-performance"})},
		}},
	})
}

var powerMenuActions = []string{"lock", "logout", "suspend", "reboot", "shutdown"}

func sessionCommandRow(path string) rowOpt {
	return withEditor(func(k *kit, s slot, _ config.FieldMeta) control {
		return newCommandMenu(k, s, commandChoices(path), sessionMenu)
	})
}

// powerPage is pages/modules/power: the power menu's buttons, with what
// each runs under Advanced.
func powerPage(*config.Config) pageSpec {
	const m = "modules.power"
	buttons := make([]rowSpec, len(powerMenuActions))
	commands := make([]rowSpec, len(powerMenuActions))
	for i, a := range powerMenuActions {
		buttons[i] = field(m + ".show-" + a)
		command := m + "." + a + "-command"
		commands[i] = field(command, enabledBy(m+".show-"+a), sessionCommandRow(command))
	}
	return barItemPage(config.ModulePower, pageSpec{id: "power", navKey: "settings-nav-power-menu", icon: "ld-power-symbolic", header: "settings-page-power-menu"}, barItemParts{
		sections: []sectionSpec{
			{title: "settings-section-power-menu", rows: buttons},
			{title: "settings-section-power-menu-commands", advanced: true, rows: commands},
		},
		icons: iconPerState(m + ".icon-name"),
	})
}

// ramPage is pages/modules/ram.
func ramPage(*config.Config) pageSpec {
	return barItemPage(config.ModuleRAM, pageSpec{id: "ram", navKey: "settings-nav-ram", icon: "ld-memory-stick-symbolic", header: "settings-page-ram"}, barItemParts{
		sections: []sectionSpec{{title: "settings-section-general", rows: []rowSpec{
			field(ramPath+".poll-interval-ms", pollInterval),
			field(ramPath + ".format"),
		}}},
		icons: iconPerState(ramPath + ".icon-name"),
	})
}

const ramPath = "modules.ram"

// screenshotPage is pages/modules/screenshot.
func screenshotPage(*config.Config) pageSpec {
	const m = "modules.screenshot"
	return barItemPage(config.ModuleScreenshot, pageSpec{id: "screenshot", navKey: "settings-nav-screenshot", icon: "ld-camera-symbolic", header: "settings-page-screenshot"}, barItemParts{
		sections: []sectionSpec{
			{title: "settings-section-general", rows: []rowSpec{field(m+".label", hinted(func() string { return locale.T().Get("settings-screenshot-label-placeholder") }))}},
			{title: "settings-section-screenshot-save", rows: []rowSpec{
				field(screenshotDirPath),
				field(m+".filename-format", screenshotNameRow),
			}},
			{title: "settings-section-screenshot-after", rows: []rowSpec{
				field(m + ".copy-to-clipboard"),
				field(m + ".notify"),
				field(m+".thumbnail-ms", durationChips(inMilliseconds, after(3*time.Second), after(5*time.Second), after(10*time.Second), zeroAs(off))),
				field(m+".thumbnail-position", screenPosition),
			}},
			{title: "settings-section-screenshot-text", rows: []rowSpec{field(m+".ocr-languages", ocrLanguagesRow)}},
			{title: "settings-section-screenshot-shortcuts", rows: []rowSpec{screenshotShortcutsRow()}},
			{title: "settings-section-colorpicker", rows: fields("colorpicker.format")},
		},
		icons: iconPerState(m + ".icon"),
	})
}

// separatorPage is pages/modules/separator.
func separatorPage(*config.Config) pageSpec {
	return barItemPage(config.ModuleSeparator, pageSpec{id: "separator", navKey: "settings-nav-separator", icon: "ld-minus-symbolic", header: "settings-page-separator"}, barItemParts{
		sections: []sectionSpec{{title: "settings-section-general", rows: []rowSpec{
			field("modules.separator.size", softPixels(1, 4)),
			field("modules.separator.length"),
		}}},
	})
}

// storagePage is pages/modules/storage.
func storagePage(*config.Config) pageSpec {
	const m = "modules.storage"
	return barItemPage(config.ModuleStorage, pageSpec{id: "storage", navKey: "settings-nav-storage", icon: "ld-hard-drive-symbolic", header: "settings-page-storage"}, barItemParts{
		sections: []sectionSpec{{title: "settings-section-general", rows: []rowSpec{
			field(m+".poll-interval-ms", durationChips(inMilliseconds, after(10*time.Second), after(30*time.Second), after(time.Minute), after(5*time.Minute))),
			field(storageMountPath, mountPoints),
			field(m + ".format"),
		}}},
		icons: iconPerState(m + ".icon-name"),
	})
}

// treemanPage is pages/modules/treeman.
func treemanPage(*config.Config) pageSpec {
	const m = "modules.treeman"
	return barItemPage(config.ModuleTreeman, pageSpec{id: "treeman", navKey: "settings-nav-treeman", icon: "ld-graph-star-symbolic", header: "settings-page-treeman"}, barItemParts{
		note:     treemanNote,
		sections: []sectionSpec{{title: "settings-section-general", rows: []rowSpec{field(m + ".format"), field(m + ".hide-if-empty")}}},
		icons: stateIcons{states: []string{"settings-state-ready", "settings-state-preparing", "settings-state-tearing-down", "settings-state-failed"}, rows: []gridRow{
			{cells: stateIconCells(m+".icon-name", m+".icon-preparing", m+".icon-tearing-down", m+".icon-failed")},
		}},
	})
}

// weatherPage is pages/modules/weather.
func weatherPage(*config.Config) pageSpec {
	const m = "modules.weather"
	return barItemPage(config.ModuleWeather, pageSpec{id: "weather", navKey: "settings-nav-weather", icon: "ld-cloud-sun-symbolic", header: "settings-page-weather"}, barItemParts{
		sections: []sectionSpec{
			{title: "settings-section-general", rows: []rowSpec{
				field(m + ".provider"),
				field(m+".location", locationRow(pinnedOnly)),
				field(m+".units", temperatureFollowsRegion),
				field(m + ".format"),
				field(m+".time-format", clockFollowsRegion),
				field(m+".refresh-interval-seconds", durationChips(inSeconds, after(15*time.Minute), after(30*time.Minute), after(time.Hour), after(3*time.Hour))),
			}},
			{title: "settings-section-api-key", when: valueIs(m+".provider", string(config.WeatherProviderVisualCrossing)), rows: []rowSpec{field(m + ".visual-crossing-key")}},
			{title: "settings-section-api-key", when: valueIs(m+".provider", string(config.WeatherProviderWeatherApi)), rows: []rowSpec{field(m + ".weatherapi-key")}},
		},
		icons: iconPerState(m + ".icon-name"),
	})
}

// windowTitlePage is pages/modules/window_title.
func windowTitlePage(*config.Config) pageSpec {
	const m = "modules.window-title"
	return barItemPage(config.ModuleWindowTitle, pageSpec{id: "window-title", navKey: "settings-nav-window-title", icon: "ld-app-window-symbolic", header: "settings-page-window-title"}, barItemParts{
		sections: []sectionSpec{
			{title: "settings-section-general", rows: []rowSpec{field(windowTitleFormatPath, windowTitleShow)}},
			{title: "settings-section-custom-format", advanced: true, rows: []rowSpec{field(windowTitleFormatPath)}},
		},
		icons:    iconPerState(m + ".icon-name"),
		iconRows: []rowSpec{field(m+".icon-mappings", iconMap)},
	})
}

// worldClockPage is pages/modules/world_clock.
func worldClockPage(*config.Config) pageSpec {
	return barItemPage(config.ModuleWorldClock, pageSpec{id: "world-clock", navKey: "settings-nav-world-clock", icon: "ld-globe-symbolic", header: "settings-page-world-clock"}, barItemParts{
		sections: []sectionSpec{{title: "settings-section-general", rows: []rowSpec{field(worldClockPath, worldClockRow)}}},
		icons:    iconPerState("modules.world-clock.icon-name"),
	})
}

// recorderPage is pages/modules/recorder.
func recorderPage(*config.Config) pageSpec {
	const m = "modules.recorder"
	return barItemPage(config.ModuleRecorder, pageSpec{id: "recorder", navKey: "settings-nav-recorder", icon: "ld-video-symbolic", header: "settings-page-recorder"}, barItemParts{
		sections: []sectionSpec{
			{title: "settings-section-general", rows: []rowSpec{
				field(m + ".format"),
				field(m + ".output-format"),
				field(m + ".output-directory"),
				field(m + ".show-cursor"),
				field(m+".start-delay-ms", durationChips(inMilliseconds, after(time.Second), after(3*time.Second), after(5*time.Second), zeroAs(off))),
			}},
			{title: "settings-section-video", rows: []rowSpec{field(m+".framerate", countChips(inFrames, 30, 60, 120))}},
			{title: "settings-section-audio", rows: []rowSpec{
				field(m + ".system-audio"),
				field(m + ".microphone"),
				field(m+".microphone-device", microphoneDevice),
			}},
			{title: "settings-section-webcam", rows: []rowSpec{
				field(m + ".webcam-enabled"),
				field(m+".webcam-device", webcamDevice),
				field(webcamXPath, withKey("settings-recorder-webcam-place"), webcamPlacerRow),
			}},
			{title: "settings-section-instant-replay", rows: []rowSpec{
				field(m + ".replay.enable"),
				field(m+".replay.seconds", durationChips(inSeconds, after(15*time.Second), after(30*time.Second), after(time.Minute), after(2*time.Minute), after(5*time.Minute))),
				field(m + ".replay.output"),
			}},
		},
		icons: stateIcons{states: []string{"settings-state-idle", "settings-state-recording", "settings-state-paused", "settings-state-replay"}, rows: []gridRow{
			{cells: stateIconCells(m+".icon-idle", m+".icon-recording", m+".icon-paused", m+".icon-replay")},
		}},
	})
}

// mailPage is pages/modules/mail.
func mailPage(*config.Config) pageSpec {
	const m = "modules.mail"
	return barItemPage(config.ModuleMail, pageSpec{id: "mail", navKey: "settings-nav-mail", icon: "ld-mail-symbolic", header: "settings-page-mail"}, barItemParts{
		sections: []sectionSpec{
			{title: "settings-section-general", rows: []rowSpec{
				field(m + ".format"),
				field(m+".query", mailQueryRow),
				field(m + ".hide-when-zero"),
			}},
			{title: "settings-section-mail-notifications", preview: mailNotificationPreview, rows: []rowSpec{
				field(m + ".notify"),
				field(m+".notify-summary", enabledBy(m+".notify")),
				field(m+".notify-body", enabledBy(m+".notify")),
				field(m+".notify-click", enabledBy(m+".notify")),
			}},
			{title: "settings-section-mail-accounts", rows: []rowSpec{field(m+".accounts", mailAccountList)}},
		},
		icons: iconPerState(m + ".icon-name"),
	})
}

// systrayPage is pages/modules/systray.
func systrayPage(*config.Config) pageSpec {
	const m = "modules.systray"
	return barItemPage(config.ModuleSystray, pageSpec{id: "systray", navKey: "settings-nav-systray", icon: "ld-panel-top-symbolic", header: "settings-page-systray"}, barItemParts{sections: []sectionSpec{
		{title: "settings-section-tray-items", rows: []rowSpec{field(trayBlacklistPath, trayItemsRow)}},
		{title: "settings-section-general", rows: []rowSpec{
			field(m+".icon-scale", sizeBase(config.SystrayIconBaseRem)),
			field(m + ".item-gap"),
			field(m + ".internal-padding"),
		}},
		{title: "settings-section-tray-patterns", advanced: true, rows: []rowSpec{
			field(trayBlacklistPath, stringList),
			field(trayOverridesPath, trayOverrideList),
		}},
	}})
}

// customPage is pages/modules/custom: a card per custom module, each
// opening with its own placement and preview and closing with its own
// Bar Item parts.
func customPage(*config.Config) pageSpec {
	return asBarItemPage(pageSpec{id: customModulesID, navKey: "settings-nav-custom", icon: "ld-code-symbolic", header: "settings-page-custom", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{field(customModulesPath, withKey("settings-custom-modules-editor"), customModulesRow)}},
	}}, func(m config.BarModule) bool {
		_, custom := m.CustomID()
		return custom
	})
}

// minimizedWindowsPage is the minimized-windows module.
func minimizedWindowsPage(*config.Config) pageSpec {
	const m = "modules.minimized-windows"
	return barItemPage(config.ModuleMinimizedWindows, pageSpec{id: "minimized-windows", navKey: "settings-nav-minimized-windows", icon: "ld-app-window-symbolic", header: "settings-page-minimized-windows"}, barItemParts{
		sections: []sectionSpec{{title: "settings-section-general", rows: []rowSpec{
			field(m+".scope", pictureChoice(windowScopePictures)),
			field(m + ".show-when-empty"),
		}}},
		icons:    iconPerState(m + ".icon"),
		iconRows: []rowSpec{field(m+".icon-size", sizeBase(config.TaskIconBaseRem))},
	})
}
