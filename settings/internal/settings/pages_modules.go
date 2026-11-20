package settings

import (
	"log"
	"slices"
	"strings"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/widget"
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
	for i, p := range pages {
		if m := config.BarModule(p.id); slices.Contains(config.BuiltinModules, m) {
			pages[i].lead = withPlacement(m, p.lead)
		}
		links := moduleLinks[p.id]
		if hasDropdown(p.id) {
			links = append(slices.Clone(links), dropdownSizesLink)
		}
		if len(links) > 0 {
			pages[i].links = links
		}
	}
	return pages
}

// moduleLinks are the pages a bar module page points to for what the
// bar item only shows, by page.
var moduleLinks = map[string][]pageLink{
	"audio":          {{soundID, "settings-module-link-sound-detail"}},
	"battery":        {{powerPageID, "settings-module-link-power-detail"}, thresholdsLink},
	"bluetooth":      {{bluetoothPageID, "settings-module-link-bluetooth-detail"}},
	"brightness":     {{displaysID, "settings-module-link-brightness-detail"}},
	"clock":          {{dateTimeID, "settings-module-link-date-time-detail"}},
	"cpu":            {thresholdsLink},
	"dashboard":      {thresholdsLink},
	"idle-inhibit":   {{"idle", "settings-module-link-idle-detail"}},
	"keybind-mode":   {{wmBindsID, "settings-module-link-binds-detail"}},
	"keyboard-input": {{"wm-keyboard", "settings-module-link-input-detail"}},
	"nightlight":     {{displaysID, "settings-module-link-nightlight-detail"}},
	"ram":            {thresholdsLink},
	"storage":        {thresholdsLink},
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

func hasDropdown(pageID string) bool {
	for _, m := range dropdownModules {
		if string(m) == pageID {
			return true
		}
	}
	return false
}

func withPlacement(m config.BarModule, lead func(k *kit) widget.Widget) func(k *kit) widget.Widget {
	if lead == nil {
		return barPlacement(m)
	}
	return func(k *kit) widget.Widget {
		box := sectionStack()
		box.Append(barPlacement(m)(k), false)
		box.Append(lead(k), false)
		return box
	}
}

func backgroundAppsPage(*config.Config) pageSpec {
	const m = "modules.background-apps"
	return pageSpec{id: "background-apps", navKey: "settings-nav-background-apps", icon: "ld-app-window-symbolic", header: "settings-page-background-apps", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{field(m+".icon", iconRow)}},
		barItemSection(m, actionChoices("background-apps")),
	}}
}

var barItemGroups = []struct {
	key string
	ids []string
}{
	{"settings-nav-bar-items-status", []string{
		"audio", "background-apps", "battery", "bluetooth", "brightness", "cpu", "dashboard", "idle-inhibit", "keybind-mode",
		"keyboard-input", "netstat", "network", "nightlight", "power", "power-profiles", "ram", "storage", "systray",
	}},
	{"settings-nav-bar-items-media", []string{"cava", "media", "recorder", "screenshot"}},
	{"settings-nav-bar-items-productivity", []string{"bitwarden", "clock", "custom", "mail", "separator", "treeman", "weather", "world-clock"}},
	{"settings-nav-bar-items-windows", []string{"minimized-windows", "window-title", "workspaces"}},
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
		out = append(out, navSection{key: g.key, pages: pages})
	}
	return out
}

// audioPage is pages/modules/audio.
func audioPage(*config.Config) pageSpec {
	return pageSpec{id: "audio", navKey: "settings-nav-audio", icon: "ld-volume-2-symbolic", header: "settings-page-audio", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field("modules.audio.format"),
		}},
		{title: "settings-section-speaker", rows: []rowSpec{
			field("modules.audio.speaker-icons", iconList),
			field("modules.audio.speaker-icon-muted", iconRow),
			field("modules.audio.thresholds", levelThresholdList(levelScale{icons: "modules.audio.speaker-icons", muted: "modules.audio.speaker-icon-muted"})),
		}},
		{title: "settings-section-capture", rows: []rowSpec{captureTilesRow("modules.audio")}},
		{title: "settings-section-dropdown", rows: []rowSpec{
			field("modules.audio.dropdown-app-icons"),
		}},
		barItemSection("modules.audio", actionChoices("audio")),
	}}
}

// batteryPage is pages/modules/battery.
func batteryPage(*config.Config) pageSpec {
	return pageSpec{id: "battery", navKey: "settings-nav-battery", icon: "ld-battery-full-symbolic", header: "settings-page-battery", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field("modules.battery.charging-icon", iconRow),
			field("modules.battery.alert-icon", iconRow),
			field("modules.battery.format"),
			field("modules.battery.level-icons", iconList),
		}},
		barItemSection("modules.battery", actionChoices("battery")),
	}}
}

// bitwardenPage is pages/modules/bitwarden.
func bitwardenPage(*config.Config) pageSpec {
	const m = "modules.bitwarden"
	return pageSpec{id: "bitwarden", navKey: "settings-nav-bitwarden", icon: "ld-shield-check-symbolic", header: "settings-page-bitwarden", sections: []sectionSpec{
		{title: "settings-section-bitwarden-account", preview: newVaultAccount, rows: fields(m+".server", m+".email")},
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
		{title: "settings-section-bitwarden-look", rows: []rowSpec{
			field(m + ".format"),
			field(m+".icon-locked", iconRow),
			field(m+".icon-unlocked", iconRow),
		}},
		barItemSection(m, actionChoices("bitwarden")),
	}}
}

// bluetoothPage is pages/modules/bluetooth.
func bluetoothPage(*config.Config) pageSpec {
	return pageSpec{id: "bluetooth", navKey: "settings-nav-bluetooth", icon: "ld-bluetooth-symbolic", header: "settings-page-bluetooth", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field("modules.bluetooth.disabled-icon", iconRow),
			field("modules.bluetooth.disconnected-icon", iconRow),
			field("modules.bluetooth.connected-icon", iconRow),
			field("modules.bluetooth.searching-icon", iconRow),
		}},
		barItemSection("modules.bluetooth", actionChoices("bluetooth")),
	}}
}

// brightnessPage is pages/modules/brightness.
func brightnessPage(*config.Config) pageSpec {
	return pageSpec{id: "brightness", navKey: "settings-nav-brightness", icon: "ld-sun-symbolic", header: "settings-page-brightness", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field("modules.brightness.format"),
			field("modules.brightness.level-icons", iconList),
			field("modules.brightness.thresholds", levelThresholdList(levelScale{icons: "modules.brightness.level-icons", floor: brightnessPath + ".min-brightness"})),
		}},
		barItemSection("modules.brightness", actionChoices("brightness")),
	}}
}

// clockPage is pages/modules/clock.
func clockPage(*config.Config) pageSpec {
	return pageSpec{id: "clock", navKey: "settings-nav-clock", icon: "ld-clock-symbolic", header: "settings-page-clock", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field("modules.clock.format"),
			field("modules.clock.icon-name", iconRow),
		}},
		{title: "settings-section-dropdown", rows: []rowSpec{
			field("modules.clock.dropdown-show-seconds"),
		}},
		{title: "settings-section-calendars", rows: []rowSpec{field("modules.clock.calendars", calendarList)}},
		barItemSection("modules.clock", actionChoices("clock")),
	}}
}

var pollInterval = durationChips(inMilliseconds, after(time.Second), after(2*time.Second), after(5*time.Second), after(10*time.Second))

// cpuPage is pages/modules/cpu.
func cpuPage(*config.Config) pageSpec {
	return pageSpec{id: "cpu", navKey: "settings-nav-cpu", icon: "ld-cpu-symbolic", header: "settings-page-cpu", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field("modules.cpu.poll-interval-ms", pollInterval),
			field("modules.cpu.temp-sensor"),
			field("modules.cpu.format"),
			field("modules.cpu.icon-name", iconRow),
		}},
		barItemSection("modules.cpu", actionChoices("cpu")),
	}}
}

// dashboardPage is pages/modules/dashboard.
func dashboardPage(*config.Config) pageSpec {
	const m = "modules.dashboard"
	return pageSpec{id: "dashboard", navKey: "settings-nav-dashboard", icon: "ld-layout-dashboard-symbolic", header: "settings-page-dashboard", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field("modules.dashboard.icon-override", iconRow),
		}},
		{title: "settings-section-commands", rows: []rowSpec{
			field("modules.dashboard.user-session.actions", enumList),
		}},
		{title: "settings-section-thresholds", rows: []rowSpec{
			rangeRow(m+".temp-warning", m+".temp-error", "settings-dashboard-temp-levels",
				levelRange(celsiusLevels, "settings-level-unit-celsius", [2]string{"settings-level-warning", "settings-level-error"}, risingLevels)),
		}},
		barItemSection("modules.dashboard", actionChoices("dashboard")),
	}}
}

// nightlightPage is pages/modules/nightlight.
func nightlightPage(*config.Config) pageSpec {
	const m = nightlightPath
	return pageSpec{id: "nightlight", navKey: "settings-nav-nightlight", icon: "ld-sun-symbolic", header: "settings-page-nightlight", sections: []sectionSpec{
		{title: "settings-section-nightlight-look", rows: []rowSpec{
			field(m + ".format"),
			field(m+".icon-off", iconRow),
			field(m+".icon-on", iconRow),
		}},
		barItemSection(m, actionChoices("nightlight")),
	}}
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
			field(brightnessPath+".min-brightness", percentage),
			field(brightnessPath+".enable-external", ddcRow),
		}},
		{title: "settings-section-nightlight-filter", preview: nightlightNow, rows: []rowSpec{field(m+".temperature", kelvin)}},
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
	return newSlider(k, s, 0, gammaMost, true, percentLabel)
})

// idleInhibitPage is pages/modules/idle_inhibit.
func idleInhibitPage(*config.Config) pageSpec {
	return pageSpec{id: "idle-inhibit", navKey: "settings-nav-idle-inhibit", icon: "ld-coffee-symbolic", header: "settings-page-idle-inhibit", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field("modules.idle-inhibit.startup-duration", durationChips(inMinutes, after(15*time.Minute), after(30*time.Minute), after(time.Hour), after(2*time.Hour), zeroAs(untilOff))),
			field("modules.idle-inhibit.icon-inactive", iconRow),
			field("modules.idle-inhibit.icon-active", iconRow),
			field("modules.idle-inhibit.format"),
		}},
		barItemSection("modules.idle-inhibit", actionChoices("idle-inhibit")),
	}}
}

// keybindModePage is pages/modules/keybind_mode.
func keybindModePage(*config.Config) pageSpec {
	return pageSpec{id: "keybind-mode", navKey: "settings-nav-keybind-mode", icon: "ld-layers-symbolic", header: "settings-page-keybind-mode", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field("modules.keybind-mode.format"),
			field("modules.keybind-mode.icon-name", iconRow),
			field("modules.keybind-mode.auto-hide"),
		}},
		steppedBarItemSection("modules.keybind-mode", actionChoices("keybind-mode"), submapStates),
	}}
}

// keyboardInputPage is pages/modules/keyboard_input.
func keyboardInputPage(*config.Config) pageSpec {
	return pageSpec{id: "keyboard-input", navKey: "settings-nav-keyboard-input", icon: "ld-keyboard-symbolic", header: "settings-page-keyboard-input", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field("modules.keyboard-input.format"),
			field("modules.keyboard-input.icon-name", iconRow),
			field("modules.keyboard-input.layout-alias-map", layoutAliasRow),
		}},
		steppedBarItemSection("modules.keyboard-input", actionChoices("keyboard-input"), layoutStates),
	}}
}

// mediaPage is pages/modules/media.
func mediaPage(*config.Config) pageSpec {
	const m = "modules.media"
	iconType := func(t config.MediaIconType) rowOpt { return shownWhen(m+".icon-type", valueIn(string(t))) }
	return pageSpec{id: "media", navKey: "settings-nav-media", icon: "ld-music-symbolic", header: "settings-page-media", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{field(m + ".format")}},
		{title: "settings-section-icons", rows: []rowSpec{
			field(m+".icon-type", pictureChoice(mediaIconPictures)),
			field(m+".icon-name", iconRow),
			field(m+".spinning-disc-icon", iconRow, iconType(config.MediaIconTypeSpinningDisc)),
			field(m+".player-icons", iconMap, iconType(config.MediaIconTypeApplicationMapped)),
		}},
		{title: "settings-section-media-players", rows: []rowSpec{
			field(m+".players-ignored", mediaPlayersRow),
			field(m+".player-priority", mediaPlayersRow),
		}},
		{title: "settings-section-online-lyrics", rows: []rowSpec{field(m + ".lyrics")}},
		barItemSection(m, actionChoices("media")),
	}}
}

var mediaIconPictures = map[string]picture{
	string(config.MediaIconTypeDefault):           {icon: "ld-music-symbolic"},
	string(config.MediaIconTypeApplication):       {icon: "ld-app-window-symbolic"},
	string(config.MediaIconTypeSpinningDisc):      {icon: "ld-disc-3-symbolic"},
	string(config.MediaIconTypeApplicationMapped): {icon: "ld-layout-grid-symbolic"},
}

// netstatPage is pages/modules/netstat.
func netstatPage(*config.Config) pageSpec {
	return pageSpec{id: "netstat", navKey: "settings-nav-netstat", icon: "ld-activity-symbolic", header: "settings-page-netstat", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field("modules.netstat.poll-interval-ms", pollInterval),
			field("modules.netstat.interface"),
			field("modules.netstat.format"),
			field("modules.netstat.icon-name", iconRow),
		}},
		barItemSection("modules.netstat", actionChoices("netstat")),
	}}
}

// networkPage is pages/modules/network.
func networkPage(*config.Config) pageSpec {
	const m = "modules.network"
	return pageSpec{id: "network", navKey: "settings-nav-network", icon: "ld-wifi-symbolic", header: "settings-page-network", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field(m + ".vpn-show"),
		}},
		{title: "settings-section-icons", advanced: true, rows: []rowSpec{
			cellGridRow("settings-network-icons", []string{"settings-wifi-title", "settings-wifi-wired", "settings-vpn-section"},
				gridRow{key: "settings-state-off", cells: gridCells([]string{m + ".wifi-disabled-icon", "", ""}, iconRow)},
				gridRow{key: "settings-state-disconnected", cells: gridCells([]string{m + ".wifi-offline-icon", m + ".wired-disconnected-icon", m + ".vpn-disconnected-icon"}, iconRow)},
				gridRow{key: "settings-state-connecting", cells: gridCells([]string{m + ".wifi-acquiring-icon", m + ".wired-acquiring-icon", m + ".vpn-connecting-icon"}, iconRow)},
				gridRow{key: "settings-state-connected", cells: gridCells([]string{m + ".wifi-connected-icon", m + ".wired-connected-icon", m + ".vpn-connected-icon"}, iconRow)},
			),
			field(m+".wifi-signal-icons", iconList),
		}},
		barItemSection(m, actionChoices("network")),
	}}
}

// powerProfilesPage is pages/modules/power_profiles.
func powerProfilesPage(*config.Config) pageSpec {
	const m = "modules.power-profiles"
	return pageSpec{id: "power-profiles", navKey: "settings-nav-power-profiles", icon: "ld-scale-symbolic", header: "settings-page-power-profiles", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field(m + ".format"),
		}},
		{title: "settings-section-power-profiles-profiles", rows: []rowSpec{
			cellGridRow("settings-power-profiles-look", []string{"settings-power-mode-power-saver", "settings-power-mode-balanced", "settings-power-mode-performance"},
				gridRow{key: "settings-grid-icon", cells: gridCells([]string{m + ".icon-power-saver", m + ".icon-balanced", m + ".icon-performance"}, iconRow)},
				gridRow{key: "settings-grid-color", cells: gridCells([]string{m + ".color-power-saver", m + ".color-balanced", m + ".color-performance"})},
			),
		}},
		barItemSection(m, actionChoices("power-profiles")),
	}}
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
	return pageSpec{id: "power", navKey: "settings-nav-power-menu", icon: "ld-power-symbolic", header: "settings-page-power-menu", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field(m+".icon-name", iconRow),
		}},
		{title: "settings-section-power-menu", rows: buttons},
		{title: "settings-section-power-menu-commands", advanced: true, rows: commands},
		barItemSection(m, actionChoices("power")),
	}}
}

// ramPage is pages/modules/ram.
func ramPage(*config.Config) pageSpec {
	return pageSpec{id: "ram", navKey: "settings-nav-ram", icon: "ld-memory-stick-symbolic", header: "settings-page-ram", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field(ramPath+".poll-interval-ms", pollInterval),
			field(ramPath + ".format"),
			field(ramPath+".icon-name", iconRow),
		}},
		barItemSection(ramPath, actionChoices("ram")),
	}}
}

const ramPath = "modules.ram"

// screenshotPage is pages/modules/screenshot.
func screenshotPage(*config.Config) pageSpec {
	const m = "modules.screenshot"
	return pageSpec{id: "screenshot", navKey: "settings-nav-screenshot", icon: "ld-camera-symbolic", header: "settings-page-screenshot", sections: []sectionSpec{
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
		{title: "settings-section-icons", rows: []rowSpec{
			field(m+".icon", iconRow),
			field(m + ".label"),
		}},
		{title: "settings-section-colorpicker", rows: fields("colorpicker.format")},
		barItemSection(m, actionChoices("screenshot")),
	}}
}

// separatorPage is pages/modules/separator.
func separatorPage(*config.Config) pageSpec {
	return pageSpec{id: "separator", navKey: "settings-nav-separator", icon: "ld-minus-symbolic", header: "settings-page-separator", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field("modules.separator.size", softPixels(1, 4)),
			field("modules.separator.length"),
		}},
		barItemSection("modules.separator", nil),
	}}
}

// storagePage is pages/modules/storage.
func storagePage(*config.Config) pageSpec {
	return pageSpec{id: "storage", navKey: "settings-nav-storage", icon: "ld-hard-drive-symbolic", header: "settings-page-storage", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field("modules.storage.poll-interval-ms", durationChips(inMilliseconds, after(10*time.Second), after(30*time.Second), after(time.Minute), after(5*time.Minute))),
			field(storageMountPath, mountPoints),
			field("modules.storage.format"),
			field("modules.storage.icon-name", iconRow),
		}},
		barItemSection("modules.storage", actionChoices("storage")),
	}}
}

// treemanPage is pages/modules/treeman.
func treemanPage(*config.Config) pageSpec {
	return pageSpec{id: "treeman", navKey: "settings-nav-treeman", icon: "ld-graph-star-symbolic", header: "settings-page-treeman", lead: treemanNote, sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field("modules.treeman.format"),
			cellGridRow("settings-treeman-icons", []string{"settings-state-ready", "settings-state-preparing", "settings-state-tearing-down", "settings-state-failed"},
				gridRow{cells: gridCells([]string{"modules.treeman.icon-name", "modules.treeman.icon-preparing", "modules.treeman.icon-tearing-down", "modules.treeman.icon-failed"}, iconRow)},
			),
			field("modules.treeman.hide-if-empty"),
		}},
		barItemSection("modules.treeman", actionChoices("treeman")),
	}}
}

// weatherPage is pages/modules/weather.
func weatherPage(*config.Config) pageSpec {
	return pageSpec{id: "weather", navKey: "settings-nav-weather", icon: "ld-cloud-sun-symbolic", header: "settings-page-weather", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field("modules.weather.provider"),
			field("modules.weather.location", locationRow(pinnedOnly)),
			field("modules.weather.units"),
			field("modules.weather.format"),
			field("modules.weather.time-format"),
			field("modules.weather.refresh-interval-seconds", durationChips(inSeconds, after(15*time.Minute), after(30*time.Minute), after(time.Hour), after(3*time.Hour))),
			field("modules.weather.icon-name", iconRow),
		}},
		{title: "settings-section-api-key", when: valueIs("modules.weather.provider", string(config.WeatherProviderVisualCrossing)), rows: []rowSpec{
			field("modules.weather.visual-crossing-key"),
		}},
		{title: "settings-section-api-key", when: valueIs("modules.weather.provider", string(config.WeatherProviderWeatherApi)), rows: []rowSpec{
			field("modules.weather.weatherapi-key"),
		}},
		barItemSection("modules.weather", actionChoices("weather")),
	}}
}

// windowTitlePage is pages/modules/window_title.
func windowTitlePage(*config.Config) pageSpec {
	return pageSpec{id: "window-title", navKey: "settings-nav-window-title", icon: "ld-app-window-symbolic", header: "settings-page-window-title", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field(windowTitleFormatPath, windowTitleShow),
			field("modules.window-title.icon-name", iconRow),
			field("modules.window-title.icon-mappings", iconMap),
		}},
		{title: "settings-section-custom-format", advanced: true, rows: []rowSpec{field(windowTitleFormatPath)}},
		barItemSection("modules.window-title", actionChoices("window-title")),
	}}
}

// worldClockPage is pages/modules/world_clock.
func worldClockPage(*config.Config) pageSpec {
	return pageSpec{id: "world-clock", navKey: "settings-nav-world-clock", icon: "ld-globe-symbolic", header: "settings-page-world-clock", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field(worldClockPath, worldClockRow),
			field("modules.world-clock.icon-name", iconRow),
		}},
		barItemSection("modules.world-clock", actionChoices("world-clock")),
	}}
}

// recorderPage is pages/modules/recorder.
func recorderPage(*config.Config) pageSpec {
	const m = "modules.recorder"
	return pageSpec{id: "recorder", navKey: "settings-nav-recorder", icon: "ld-video-symbolic", header: "settings-page-recorder", sections: []sectionSpec{
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
		{title: "settings-section-icons", rows: []rowSpec{
			cellGridRow("settings-recorder-icons", []string{"settings-state-idle", "settings-state-recording", "settings-state-paused", "settings-state-replay"},
				gridRow{cells: gridCells([]string{m + ".icon-idle", m + ".icon-recording", m + ".icon-paused", m + ".icon-replay"}, iconRow)},
			),
		}},
		barItemSection(m, actionChoices("recorder")),
	}}
}

// mailPage is pages/modules/mail.
func mailPage(*config.Config) pageSpec {
	const m = "modules.mail"
	return pageSpec{id: "mail", navKey: "settings-nav-mail", icon: "ld-mail-symbolic", header: "settings-page-mail", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field(m + ".format"),
			field(m+".query", mailQueryRow),
			field(m+".icon-name", iconRow),
			field(m + ".hide-when-zero"),
		}},
		{title: "settings-section-mail-notifications", preview: mailNotificationPreview, rows: []rowSpec{
			field(m + ".notify"),
			field(m+".notify-summary", enabledBy(m+".notify")),
			field(m+".notify-body", enabledBy(m+".notify")),
			field(m+".notify-click", enabledBy(m+".notify")),
		}},
		{title: "settings-section-mail-accounts", rows: []rowSpec{field(m+".accounts", mailAccountList)}},
		barItemSection(m, actionChoices("mail")),
	}}
}

// systrayPage is pages/modules/systray.
func systrayPage(*config.Config) pageSpec {
	const m = "modules.systray"
	return pageSpec{id: "systray", navKey: "settings-nav-systray", icon: "ld-panel-top-symbolic", header: "settings-page-systray", sections: []sectionSpec{
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
		barItemSection(m, nil),
	}}
}

// customPage is pages/modules/custom: a card per custom module.
func customPage(*config.Config) pageSpec {
	return pageSpec{id: customModulesID, navKey: "settings-nav-custom", icon: "ld-code-symbolic", header: "settings-page-custom", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{field(customModulesPath, withKey("settings-custom-modules-editor"), customModulesRow)}},
	}}
}

// minimizedWindowsPage is the minimized-windows module.
func minimizedWindowsPage(*config.Config) pageSpec {
	return pageSpec{id: "minimized-windows", navKey: "settings-nav-minimized-windows", icon: "ld-app-window-symbolic", header: "settings-page-minimized-windows", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field("modules.minimized-windows.scope", pictureChoice(windowScopePictures)),
			field("modules.minimized-windows.show-when-empty"),
		}},
		{title: "settings-section-icons", rows: []rowSpec{
			field("modules.minimized-windows.icon", iconRow),
			field("modules.minimized-windows.icon-size", sizeBase(config.TaskIconBaseRem)),
		}},
	}}
}
