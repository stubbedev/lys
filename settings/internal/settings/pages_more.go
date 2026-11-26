package settings

import (
	"time"

	"github.com/stubbedev/lys/desktop/config"
)

const barPageID = "bar"

// barGeneralPage is pages/bar/general.
func barPage(*config.Config) pageSpec {
	return pageSpec{id: barPageID, navKey: "settings-nav-bar", icon: "ld-layout-dashboard-symbolic", header: "settings-page-bar", extrasOnRows: true, sections: []sectionSpec{
		{title: "settings-section-layout", rows: []rowSpec{
			field("bar.layout", layoutRow),
			field("bar.location", screenPosition),
			field("bar.exclusive"),
			field("bar.auto-hide"),
			field("bar.auto-hide-delay-ms", hideDelay),
		}},
		{title: "settings-section-appearance", rows: []rowSpec{
			field("bar.bg"),
			field("bar.background-opacity"),
			field("bar.rounding", roundingChoice),
			field("bar.shadow", pictureChoice(shadowPictures)),
		}},
		{title: "settings-section-bar-size", advanced: true, rows: fields("bar.scale", "bar.layer", "bar.inset-edge", "bar.inset-ends", "bar.padding", "bar.padding-ends", "bar.module-gap")},
		{title: "settings-section-border", advanced: true, rows: bordered("bar.border-location", field("bar.border-width", borderWidth), field("bar.border-color"))},

		{title: "settings-section-buttons", rows: []rowSpec{
			field("bar.button-variant", pictureChoice(buttonVariantPictures)),
			field("bar.button-opacity"),
			field("bar.button-bg-opacity"),
			field("bar.button-rounding", roundingChoice),
			field("bar.button-gap"),
			field("bar.button-icon-position", pictureChoice(iconPositionPictures)),
			field("bar.button-icon-size"),
			field("bar.button-icon-padding", shownWhen("bar.button-variant", valueIn(string(config.ButtonBlockPrefix), string(config.ButtonIconSquare)))),
			field("bar.button-label-size"),
			field("bar.button-label-weight"),
			field("bar.button-label-padding"),
		}},
		{title: "settings-section-button-border", rows: bordered("bar.button-border-location", field("bar.button-border-width", borderWidth))},
		{title: "settings-section-button-groups", advanced: true, rows: append([]rowSpec{
			field("bar.button-group-opacity"),
			field("bar.button-group-rounding", roundingChoice),
			field("bar.button-group-padding"),
			field("bar.button-group-module-gap"),
			field("bar.button-group-background"),
		}, bordered("bar.button-group-border-location", field("bar.button-group-border-width", borderWidth), field("bar.button-group-border-color"))...)},
		{title: "settings-section-dropdowns", rows: []rowSpec{
			field("bar.dropdown-shadow"), field("bar.dropdown-autohide"), field("bar.dropdown-freeze-label"),
			field("bar.dropdown-opacity"),
			dropdownSizeTable(),
		}},
	}, animates: "animations.dropdown"}
}

// bordered is a border's location row and the rows styling it, shown
// while the border is drawn.
func bordered(location string, styling ...rowSpec) []rowSpec {
	rows := []rowSpec{field(location)}
	for _, r := range styling {
		shownWhen(location, valueNot(string(config.BorderNone)))(&r)
		rows = append(rows, r)
	}
	return rows
}

// borderWidth is a border's width on a slider over the widths a bar
// border takes, beside a number for a wider one.
var borderWidth = softPixels(0, 8)

// hideDelay is how long a surface stays before it hides.
var hideDelay = softMilliseconds(0, 2000)

// roundingChoice is a rounding level chosen on a picture of its corners.
var roundingChoice = pictureChoice(roundingPictures)

// notificationsPage is pages/notifications.
func notificationsPage(*config.Config) pageSpec {
	return pageSpec{id: "notifications", navKey: "settings-nav-notifications", icon: "ld-bell-symbolic", header: "settings-page-notifications", lead: notificationsLead, extrasOnRows: true, panel: []panelSetting{{"notifications.dnd", "settings-dnd-now", "settings-section-dnd"}}, sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field("modules.notifications.enabled"),
		}},
		{title: "settings-section-popup-display", preview: notificationPreview, rows: []rowSpec{
			field("modules.notifications.popup-position", screenPosition),
			field("modules.notifications.popup-monitor", monitorChoiceRow(primaryMonitor)),
			field("modules.notifications.popup-duration", durationChips(inMilliseconds, after(3*time.Second), after(5*time.Second), after(10*time.Second), after(30*time.Second))),
			field("modules.notifications.popup-hover-pause"),
			field("modules.notifications.popup-max-visible", countSlider(1, popupMaxVisibleSoftMax)),
			field("modules.notifications.popup-stack", pictureChoice(popupStackPictures)),
			field("modules.notifications.popup-stacking-order"),
			field("modules.notifications.popup-shadow"),
			field("modules.notifications.popup-opacity"),
			field("modules.notifications.popup-urgency-bar"),
		}},
		{title: "settings-section-filtering", rows: []rowSpec{
			notifyAppsRow(),
		}},
		{title: "settings-section-focus-automatic", rows: append(fields("modules.notifications.dnd.while-sharing", "modules.notifications.dnd.while-fullscreen"), field("modules.notifications.dnd.schedule", dndScheduleList))},
		{title: "settings-section-focus-allowed", rows: []rowSpec{field("modules.notifications.dnd.allow-critical")}},
		{title: "settings-section-notifications-advanced", advanced: true, rows: []rowSpec{
			field("modules.notifications.popup-close-behavior"),
			field("modules.notifications.icon-source"),
			field("modules.notifications.history-memory"),
			field("modules.notifications.history-stored", countChips(inEntries, 1_000, 10_000, 100_000)),
			field("modules.notifications.popup-layer"),
			field("modules.notifications.popup-margin-x", sizeBase(config.PopupMarginBaseRem)),
			field("modules.notifications.popup-margin-y", sizeBase(config.PopupMarginBaseRem)),
			field("modules.notifications.popup-gap", sizeBase(config.PopupGapBaseRem)),
		}},
	}, related: linksTo(pageLink{notificationBellID, "settings-module-link-bell-detail"}), animates: "animations.notifications"}
}

const notificationBellID = "notification-bell"

// notificationBellPage is the notifications' bar item: the bell.
func notificationBellPage(*config.Config) pageSpec {
	const m = "modules.notifications"
	return barItemPage(config.ModuleNotifications, pageSpec{id: notificationBellID, navKey: "settings-nav-notification-bell", icon: "ld-bell-symbolic", header: "settings-page-notification-bell"}, barItemParts{
		icons:      iconPerState(m+".icon-name", m+".icon-unread", m+".icon-dnd"),
		thresholds: []rowSpec{field(m+".thresholds", thresholdList)},
	})
}

const popupMaxVisibleSoftMax = 10

// osdPage is pages/osd.
func osdPage(*config.Config) pageSpec {
	return pageSpec{id: "osd", navKey: "settings-nav-osd", icon: "ld-monitor-symbolic", header: "settings-page-osd", lead: tryLead("settings-show-osd", showOsd), extrasOnRows: true, sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field("osd.enabled"),
		}},
		{title: "settings-section-display", preview: osdPreview, rows: []rowSpec{
			field("osd.position", screenPosition),
			field("osd.monitor", monitorChoiceRow(primaryMonitor)),
			field("osd.text-align", pictureChoice(textAlignPictures)),
			field("osd.duration", durationChips(inMilliseconds, after(time.Second), after(1500*time.Millisecond), after(2500*time.Millisecond), after(5*time.Second))),
			field("osd.border"),
			field("osd.media"),
		}},
		{title: "settings-section-presets", rows: []rowSpec{
			field("osd.presets", toastPresetList),
		}},
		{title: "settings-section-osd-advanced", advanced: true, rows: []rowSpec{
			field("osd.layer"),
			field("osd.margin", sizeBase(config.OsdMarginBaseRem)),
		}},
	}, animates: "animations.osd"}
}

// launcherPage is pages/launcher entry.
func launcherPage(*config.Config) pageSpec {
	return pageSpec{id: "launcher", navKey: "settings-nav-launcher-page", icon: "ld-rocket-symbolic", header: "settings-page-launcher", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			launcherShortcutRow(),
			field("launcher.monitor", monitorChoiceRow(focusedMonitor)),
		}},
		{title: "settings-section-launcher-search-behaviour", rows: []rowSpec{
			field("launcher.matching"),
			field("launcher.case"),
		}},
		{title: "settings-section-launcher-appearance", preview: launcherPreview, rows: []rowSpec{
			field("launcher.location", screenPosition),
			field("launcher.width", sizeSlider(config.LauncherWidthBaseRem, launcherWidthSoftMinPx, launcherWidthSoftMaxPx)),
			field("launcher.lines", softCount(1, launcherLinesSoftMax)),
			field("launcher.sidebar-mode"),
			field("launcher.show-icons"),
			field("launcher.icon-theme"),
			field("launcher.font", fontRow),
			field("launcher.emoji.skin-tone"),
		}},
		{title: "settings-section-launcher-history", rows: []rowSpec{
			field("launcher.history.enable"),
			field("launcher.history.max-size", countChips(inEntries, 25, 50, 100, 500), shownWhen("launcher.history.enable", switchedOn)),
			clearHistoryRow(),
		}},
		{title: "settings-section-launcher-matching", advanced: true, rows: []rowSpec{
			field("launcher.tokenize"),
			field("launcher.negate-char", negateCharRow),
			field("launcher.normalize-match"),
			field("launcher.sort"),
			field("launcher.sorting-method", enabledBy("launcher.sort")),
			field("launcher.auto-select"),
			field("launcher.hover-select"),
		}},
		{title: "settings-section-launcher-expert", advanced: true, rows: []rowSpec{
			field("launcher.cycle"),
			field("launcher.fixed-num-lines"),
			field("launcher.styles", launcherStylesRow),
		}},
		{title: "settings-section-launcher-keybindings", rows: []rowSpec{
			field("launcher.keybindings", launcherKeysRow),
			field("launcher.mouse-bindings", launcherMouseRow),
		}},
	}, related: linksTo(pageLink{"launcher-modes", "settings-launcher-link-modes"}), animates: "animations.launcher"}
}

// The launcher's sliders: the widths and line counts nearly every setup
// sits in, a number beside each taking one past them.
const (
	launcherWidthSoftMinPx = 360
	launcherWidthSoftMaxPx = 1440
	launcherLinesSoftMax   = 20
)

// launcherModesPage is pages/launcher modes_entry.
func launcherModesPage(*config.Config) pageSpec {
	return pageSpec{id: "launcher-modes", navKey: "settings-nav-launcher-modes-page", icon: "ld-grid-2x2-symbolic", header: "settings-page-launcher-modes", sections: []sectionSpec{
		{title: "settings-section-launcher-modes", rows: []rowSpec{launcherModeListRow()}},
		{title: "settings-section-launcher-search", rows: []rowSpec{
			field("launcher.search.providers", enumList),
			field(webEnginePath, webEngineRow),
			field("launcher.search.file-roots", folderList),
			field("launcher.search.file-exclude", fileExcludeRow),
			field("launcher.search.commands-dir"),
			field("launcher.search.currency-rates"),
			field("launcher.search.previews"),
			field("launcher.search.quicklinks", cardRow(quicklinkCards)),
			field("launcher.search.snippets", cardRow(snippetCards)),
		}},
		{title: "settings-section-launcher-scripts", rows: []rowSpec{
			field(launcherScriptsPath, scriptsRow),
		}},
	}}
}

// themePage is pages/styling.
func themePage(*config.Config) pageSpec {
	return pageSpec{id: "theme", navKey: "settings-nav-theme", icon: "ld-palette-symbolic", header: "settings-page-theme", lead: themePreview, extrasOnRows: true, sections: []sectionSpec{
		{title: "settings-section-provider", rows: []rowSpec{
			field("styling.appearance", pictureChoice(appearancePictures)),
			field("styling.high-contrast"),
			field("styling.reduce-transparency"),
			field("styling.theme-provider", pictureChoice(themeProviderPictures)),
			field("styling.theming-monitor", monitorChoiceRow(firstMonitor)),
		}},
		{title: "settings-section-palette", when: themeProviderIs(config.ThemeProviderLysShell), rows: []rowSpec{
			themePreset(),
			field("styling.palette.bg"),
			field("styling.palette.surface"),
			field("styling.palette.elevated"),
			field("styling.palette.fg"),
			field("styling.palette.fg-muted"),
			field("styling.palette.primary"),
			field("styling.palette.red"),
			field("styling.palette.yellow"),
			field("styling.palette.green"),
			field("styling.palette.blue"),
		}},
		{title: "settings-section-matugen", when: themeProviderIs(config.ThemeProviderMatugen), rows: []rowSpec{
			field("styling.matugen-scheme"),
			field("styling.matugen-contrast", signedNormalized),
			field("styling.matugen-source-color", sourceColorRow),
			field("styling.matugen-light"),
		}},
		{title: "settings-section-wallust", when: themeProviderIs(config.ThemeProviderWallust), rows: []rowSpec{
			field("styling.wallust-palette"),
			field("styling.wallust-saturation"),
			field("styling.wallust-check-contrast"),
			field("styling.wallust-backend"),
			field("styling.wallust-colorspace"),
			field("styling.wallust-apply-globally"),
		}},
		{title: "settings-section-pywal", when: themeProviderIs(config.ThemeProviderPywal), rows: []rowSpec{
			field("styling.pywal-saturation", normalized),
			field("styling.pywal-contrast", spin(1.0, 21.0, 0.5, 1)),
			field("styling.pywal-light"),
			field("styling.pywal-apply-globally"),
		}},
	}}
}

// wallpaperPage is pages/wallpaper: the image and its scaling, the
// cycling directory with its options, the per-monitor overrides and
// the change animation.
func themeProviderIs(provider config.ThemeProvider) func(store) bool {
	return valueIs("styling.theme-provider", string(provider))
}

func wallpaperPage(*config.Config) pageSpec {
	return pageSpec{id: "wallpaper", navKey: "settings-nav-wallpaper", icon: "ld-image-symbolic", header: "settings-page-wallpaper", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{field("wallpaper.wallpaper", wallpaperGridRow), field("wallpaper.fit-mode", pictureChoice(fitPictures)), field("wallpaper.picker-directories", folderList), field("wallpaper.oled-dim")}},
		{title: "settings-section-cycling", rows: []rowSpec{
			field("wallpaper.cycling-directory"),
			cyclingReveal("wallpaper.cycling-directory",
				field("wallpaper.cycling-mode"),
				field("wallpaper.cycling-interval-mins", durationChips(inMinutes, after(5*time.Minute), after(15*time.Minute), after(30*time.Minute), after(time.Hour), after(3*time.Hour), after(day))),
				field("wallpaper.cycling-same-image"),
			),
		}},
		{title: "settings-section-display", rows: []rowSpec{field("wallpaper.monitors", monitorWallpaperRow, func(r *rowSpec) { r.fullWidth = true })}},
	}, animates: "animations.wallpaper"}
}

// dockRow is a dock row, in use only while the dock is shown.
var dockRow = enabledBy("dock.enabled")

// dockPage is the standalone dock.
func dockPage(*config.Config) pageSpec {
	return pageSpec{id: "dock", navKey: "settings-nav-dock", icon: "ld-panel-bottom-symbolic", header: "settings-page-dock", lead: dockPreviewLead, extrasOnRows: true, sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field("dock.enabled"),
			field("dock.monitors", monitorSetRow, dockRow),
			field("dock.edge", screenPosition, dockRow),
			field("dock.auto-hide", dockRow),
			field("dock.auto-hide-delay-ms", hideDelay, dockRow, shownWhen("dock.auto-hide", switchedOn)),
			field("dock.pinned", pinnedRow, dockRow),
		}},
		{title: "settings-section-appearance", rows: []rowSpec{
			field("dock.icon-size", iconScale(config.TaskIconBaseRem, 1, 4), dockRow),
			field("dock.hover-zoom", dockRow),
			field("dock.hover-zoom-size", iconScale(config.TaskIconBaseRem, 1, 6), dockRow, shownWhen("dock.hover-zoom", switchedOn)),
			field("dock.icon", iconRow, dockRow),
		}},
	}}
}
