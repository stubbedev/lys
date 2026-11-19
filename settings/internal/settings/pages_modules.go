package settings

import (
	"log"
	"slices"
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
		if d, ok := moduleDropdowns[p.id]; ok {
			pages[i].sections = append(p.sections, dropdownSizeSection(d))
		}
		if m := config.BarModule(p.id); slices.Contains(config.BuiltinModules, m) {
			pages[i].lead = withPlacement(m, p.lead)
		}
	}
	return pages
}

// moduleDropdowns are the dropdowns.<name> sizes the bar module pages
// carry, by page.
var moduleDropdowns = map[string]string{
	"audio": "audio", "battery": "battery", "bitwarden": "bitwarden", "bluetooth": "bluetooth", "brightness": "brightness",
	"clock": "calendar", "dashboard": "dashboard", "media": "media", "mail": "mail", "network": "network",
	"treeman": "treeman", "weather": "weather",
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

func dropdownSizeSection(dropdown string) sectionSpec {
	return sectionSpec{title: "settings-section-dropdown-size", rows: dropdownSizeRows("dropdowns." + dropdown)}
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
			field("modules.audio.thresholds", thresholdList),
		}},
		{title: "settings-section-speaker", rows: []rowSpec{
			field("modules.audio.speaker-icons", iconList),
			field("modules.audio.speaker-icon-muted", iconRow),
		}},
		{title: "settings-section-microphone", rows: []rowSpec{
			field("modules.audio.mic-icon", iconRow),
			field("modules.audio.mic-icon-muted", iconRow),
			field("modules.audio.mic-hide-when-idle"),
		}},
		{title: "settings-section-capture", rows: []rowSpec{
			field("modules.audio.capture-camera-icon", iconRow),
			field("modules.audio.capture-screen-icon", iconRow),
			field("modules.audio.capture-color"),
		}},
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
			field("modules.battery.thresholds", thresholdList),
		}},
		barItemSection("modules.battery", actionChoices("battery")),
	}}
}

// bitwardenPage is pages/modules/bitwarden.
func bitwardenPage(*config.Config) pageSpec {
	const m = "modules.bitwarden"
	return pageSpec{id: "bitwarden", navKey: "settings-nav-bitwarden", icon: "ld-shield-check-symbolic", header: "settings-page-bitwarden", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field(m + ".server"),
			field(m + ".email"),
			field(m + ".lock-with-session"),
			field(m + ".unlock-with-session"),
			field(m + ".clipboard-clear"),
			field(m + ".format"),
			field(m+".icon-locked", iconRow),
			field(m+".icon-unlocked", iconRow),
			field(m + ".network-folder"),
			field(m + ".portal-folder"),
		}},
		{title: "settings-section-generator", rows: []rowSpec{
			field(m+".generator-passwords", passwordPresetList),
			field(m+".generator-passphrases", passphrasePresetList),
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
			field("modules.brightness.min-brightness", spin(0, 100, 1, 0)),
			field("modules.brightness.enable-external"),
			field("modules.brightness.thresholds", thresholdList),
		}},
		barItemSection("modules.brightness", actionChoices("brightness")),
	}}
}

// cavaPage is pages/modules/cava.
func cavaPage(*config.Config) pageSpec {
	return pageSpec{id: "cava", navKey: "settings-nav-cava", icon: "ld-audio-lines-symbolic", header: "settings-page-cava", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field("modules.cava.bars", spin(1.0, 256.0, 1.0, 0)),
			field("modules.cava.framerate", spin(1.0, 360.0, 1.0, 0)),
			field("modules.cava.stereo"),
			field("modules.cava.noise-reduction", normalized),
			field("modules.cava.monstercat", spin(0.0, 10.0, 0.1, 1)),
			field("modules.cava.waves"),
			field("modules.cava.low-cutoff", spin(1.0, 50000.0, 1.0, 0)),
			field("modules.cava.high-cutoff", spin(1.0, 50000.0, 1.0, 0)),
			field("modules.cava.input"),
			field("modules.cava.source"),
			field("modules.cava.style"),
			field("modules.cava.direction"),
			field("modules.cava.bar-width"),
			field("modules.cava.bar-gap"),
			field("modules.cava.internal-padding"),
		}},
		barItemSection("modules.cava", actionChoices("cava")),
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

// cpuPage is pages/modules/cpu.
func cpuPage(*config.Config) pageSpec {
	return pageSpec{id: "cpu", navKey: "settings-nav-cpu", icon: "ld-cpu-symbolic", header: "settings-page-cpu", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field("modules.cpu.poll-interval-ms", durationChips(inMilliseconds, after(time.Second), after(2*time.Second), after(5*time.Second), after(10*time.Second))),
			field("modules.cpu.temp-sensor"),
			field("modules.cpu.format"),
			field("modules.cpu.icon-name", iconRow),
			field("modules.cpu.thresholds", thresholdList),
		}},
		barItemSection("modules.cpu", actionChoices("cpu")),
	}}
}

// dashboardPage is pages/modules/dashboard.
func dashboardPage(*config.Config) pageSpec {
	return pageSpec{id: "dashboard", navKey: "settings-nav-dashboard", icon: "ld-layout-dashboard-symbolic", header: "settings-page-dashboard", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field("modules.dashboard.icon-override", iconRow),
		}},
		{title: "settings-section-commands", rows: []rowSpec{
			field("modules.dashboard.user-session.actions", enumList),
		}},
		{title: "settings-section-thresholds", rows: []rowSpec{
			field("modules.dashboard.usage-warning", spin(0, 100, 1, 0)),
			field("modules.dashboard.usage-error", spin(0, 100, 1, 0)),
			field("modules.dashboard.temp-warning", spin(0, 150, 1, 0)),
			field("modules.dashboard.temp-error", spin(0, 150, 1, 0)),
			field("modules.dashboard.battery-warning", spin(0, 100, 1, 0)),
			field("modules.dashboard.battery-critical", spin(0, 100, 1, 0)),
		}},
		barItemSection("modules.dashboard", actionChoices("dashboard")),
	}}
}

// nightlightPage is pages/modules/nightlight.
func nightlightPage(*config.Config) pageSpec {
	return pageSpec{id: "nightlight", navKey: "settings-nav-nightlight", icon: "ld-sun-symbolic", header: "settings-page-nightlight", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field("modules.nightlight.format"),
			field("modules.nightlight.temperature"),
			field("modules.nightlight.gamma"),
			field("modules.nightlight.icon-off", iconRow),
			field("modules.nightlight.icon-on", iconRow),
		}},
		{title: "settings-section-nightlight-schedule", rows: []rowSpec{
			field("modules.nightlight.auto-schedule"),
			field("modules.nightlight.transition-duration"),
			field("modules.nightlight.latitude", optionalFloat(-90.0, 90.0, 0.1, 4)),
			field("modules.nightlight.longitude", optionalFloat(-180.0, 180.0, 0.1, 4)),
		}},
		barItemSection("modules.nightlight", actionChoices("nightlight")),
	}}
}

// idleInhibitPage is pages/modules/idle_inhibit.
func idleInhibitPage(*config.Config) pageSpec {
	return pageSpec{id: "idle-inhibit", navKey: "settings-nav-idle-inhibit", icon: "ld-coffee-symbolic", header: "settings-page-idle-inhibit", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field("modules.idle-inhibit.startup-duration"),
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
		barItemSection("modules.keybind-mode", actionChoices("keybind-mode")),
	}}
}

// keyboardInputPage is pages/modules/keyboard_input.
func keyboardInputPage(*config.Config) pageSpec {
	return pageSpec{id: "keyboard-input", navKey: "settings-nav-keyboard-input", icon: "ld-keyboard-symbolic", header: "settings-page-keyboard-input", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field("modules.keyboard-input.format"),
			field("modules.keyboard-input.icon-name", iconRow),
			field("modules.keyboard-input.layout-alias-map", stringMap),
		}},
		barItemSection("modules.keyboard-input", actionChoices("keyboard-input")),
	}}
}

// mediaPage is pages/modules/media.
func mediaPage(*config.Config) pageSpec {
	return pageSpec{id: "media", navKey: "settings-nav-media", icon: "ld-music-symbolic", header: "settings-page-media", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field("modules.media.icon-type"),
			field("modules.media.format"),
			field("modules.media.icon-name", iconRow),
			field("modules.media.spinning-disc-icon", iconRow),
			field("modules.media.player-icons", iconMap),
			field("modules.media.players-ignored", stringList),
			field("modules.media.player-priority", stringList),
			field("modules.media.lyrics"),
		}},
		barItemSection("modules.media", actionChoices("media")),
	}}
}

// netstatPage is pages/modules/netstat.
func netstatPage(*config.Config) pageSpec {
	return pageSpec{id: "netstat", navKey: "settings-nav-netstat", icon: "ld-activity-symbolic", header: "settings-page-netstat", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field("modules.netstat.poll-interval-ms", durationChips(inMilliseconds, after(time.Second), after(2*time.Second), after(5*time.Second), after(10*time.Second))),
			field("modules.netstat.interface"),
			field("modules.netstat.format"),
			field("modules.netstat.icon-name", iconRow),
		}},
		barItemSection("modules.netstat", actionChoices("netstat")),
	}}
}

// networkPage is pages/modules/network.
func networkPage(*config.Config) pageSpec {
	return pageSpec{id: "network", navKey: "settings-nav-network", icon: "ld-wifi-symbolic", header: "settings-page-network", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field("modules.network.wifi-disabled-icon", iconRow),
			field("modules.network.wifi-acquiring-icon", iconRow),
			field("modules.network.wifi-offline-icon", iconRow),
			field("modules.network.wifi-connected-icon", iconRow),
			field("modules.network.wired-connected-icon", iconRow),
			field("modules.network.wired-acquiring-icon", iconRow),
			field("modules.network.wired-disconnected-icon", iconRow),
			field("modules.network.wifi-signal-icons", iconList),
			field("modules.network.vpn-show"),
			field("modules.network.vpn-connected-icon", iconRow),
			field("modules.network.vpn-connecting-icon", iconRow),
			field("modules.network.vpn-disconnected-icon", iconRow),
		}},
		barItemSection("modules.network", actionChoices("network")),
	}}
}

// powerProfilesPage is pages/modules/power_profiles.
func powerProfilesPage(*config.Config) pageSpec {
	return pageSpec{id: "power-profiles", navKey: "settings-nav-power-profiles", icon: "ld-scale-symbolic", header: "settings-page-power-profiles", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field("modules.power-profiles.format"),
		}},
		{title: "settings-section-power-profiles-profiles", rows: []rowSpec{
			field("modules.power-profiles.icon-power-saver", iconRow),
			field("modules.power-profiles.color-power-saver"),
			field("modules.power-profiles.icon-balanced", iconRow),
			field("modules.power-profiles.color-balanced"),
			field("modules.power-profiles.icon-performance", iconRow),
			field("modules.power-profiles.color-performance"),
		}},
		barItemSection("modules.power-profiles", actionChoices("power-profiles")),
	}}
}

var powerMenuActions = []string{"lock", "logout", "suspend", "reboot", "shutdown"}

// powerPage is pages/modules/power: the power menu's buttons, with what
// each runs under Advanced.
func powerPage(*config.Config) pageSpec {
	const m = "modules.power"
	buttons := make([]rowSpec, len(powerMenuActions))
	commands := make([]rowSpec, len(powerMenuActions))
	for i, a := range powerMenuActions {
		buttons[i] = field(m + ".show-" + a)
		commands[i] = field(m+"."+a+"-command", enabledBy(m+".show-"+a))
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
			field("modules.ram.poll-interval-ms", durationChips(inMilliseconds, after(time.Second), after(2*time.Second), after(5*time.Second), after(10*time.Second))),
			field("modules.ram.format"),
			field("modules.ram.icon-name", iconRow),
			field("modules.ram.thresholds", thresholdList),
		}},
		barItemSection("modules.ram", actionChoices("ram")),
	}}
}

// screenshotPage is pages/modules/screenshot.
func screenshotPage(*config.Config) pageSpec {
	return pageSpec{id: "screenshot", navKey: "settings-nav-screenshot", icon: "ld-camera-symbolic", header: "settings-page-screenshot", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field("modules.screenshot.output-directory"),
			field("modules.screenshot.filename-format"),
			field("modules.screenshot.label"),
			field("modules.screenshot.copy-to-clipboard"),
			field("modules.screenshot.notify"),
			field("modules.screenshot.thumbnail-ms", durationChips(inMilliseconds, after(3*time.Second), after(5*time.Second), after(10*time.Second), zeroAs(off))),
			field("modules.screenshot.thumbnail-position", screenPosition),
			field("modules.screenshot.ocr-languages", stringList),
		}},
		{title: "settings-section-icons", rows: []rowSpec{
			field("modules.screenshot.icon", iconRow),
		}},
		{title: "settings-section-colorpicker", rows: fields("colorpicker.format")},
		barItemSection("modules.screenshot", actionChoices("screenshot")),
	}}
}

// separatorPage is pages/modules/separator.
func separatorPage(*config.Config) pageSpec {
	return pageSpec{id: "separator", navKey: "settings-nav-separator", icon: "ld-minus-symbolic", header: "settings-page-separator", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field("modules.separator.size"),
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
			field("modules.storage.mount-point", mountPoints),
			field("modules.storage.format"),
			field("modules.storage.icon-name", iconRow),
			field("modules.storage.thresholds", thresholdList),
		}},
		barItemSection("modules.storage", actionChoices("storage")),
	}}
}

// treemanPage is pages/modules/treeman.
func treemanPage(*config.Config) pageSpec {
	return pageSpec{id: "treeman", navKey: "settings-nav-treeman", icon: "ld-graph-star-symbolic", header: "settings-page-treeman", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field("modules.treeman.format"),
			field("modules.treeman.icon-name", iconRow),
			field("modules.treeman.icon-preparing", iconRow),
			field("modules.treeman.icon-tearing-down", iconRow),
			field("modules.treeman.icon-failed", iconRow),
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
			field("modules.weather.location"),
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
			field("modules.window-title.format"),
			field("modules.window-title.icon-name", iconRow),
			field("modules.window-title.icon-mappings", iconMap),
		}},
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
		// Quality and the encoder are chosen automatically, so the
		// framerate is the only video knob.
		{title: "settings-section-video", rows: []rowSpec{field(m+".framerate", spin(1, 240, 1, 0))}},
		{title: "settings-section-audio", rows: []rowSpec{
			field(m + ".system-audio"),
			field(m + ".microphone"),
			field(m+".microphone-device", microphoneDevice),
		}},
		{title: "settings-section-webcam", rows: []rowSpec{
			field(m + ".webcam-enabled"),
			field(m+".webcam-device", webcamDevice),
			field(m+".webcam-x", percentage),
			field(m+".webcam-y", percentage),
			field(m+".webcam-size", percentage),
		}},
		{title: "settings-section-instant-replay", rows: []rowSpec{
			field(m + ".replay.enable"),
			field(m+".replay.seconds", durationChips(inSeconds, after(15*time.Second), after(30*time.Second), after(time.Minute), after(2*time.Minute), after(5*time.Minute))),
			field(m + ".replay.output"),
		}},
		{title: "settings-section-icons", rows: []rowSpec{
			field(m+".icon-idle", iconRow),
			field(m+".icon-recording", iconRow),
			field(m+".icon-paused", iconRow),
			field(m+".icon-replay", iconRow),
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
			field(m + ".notify"),
			field(m + ".notify-summary"),
			field(m + ".notify-body"),
			field(m + ".notify-click"),
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
	return pageSpec{id: "custom", navKey: "settings-nav-custom", icon: "ld-code-symbolic", header: "settings-page-custom", sections: []sectionSpec{
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
