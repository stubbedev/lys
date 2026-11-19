package settings

import "github.com/stubbedev/lys/desktop/config"

// barGeneralPage is pages/bar/general.
func barPage(*config.Config) pageSpec {
	return pageSpec{id: "bar", navKey: "settings-nav-bar", icon: "ld-layout-dashboard-symbolic", header: "settings-page-bar", sections: []sectionSpec{
		{title: "settings-section-layout", rows: []rowSpec{
			field("bar.location", screenPosition),
			field("bar.exclusive"),
			field("bar.auto-hide"),
			field("bar.auto-hide-delay-ms"),
			field("bar.layout", layoutRow),
		}},
		{title: "settings-section-appearance", rows: []rowSpec{
			field("bar.bg"),
			field("bar.background-opacity", percentage),
			field("bar.rounding", roundingChoice),
			field("bar.shadow", pictureChoice(shadowPictures)),
		}},
		{title: "settings-section-bar-size", advanced: true, rows: fields("bar.scale", "bar.layer", "bar.inset-edge", "bar.inset-ends", "bar.padding", "bar.padding-ends", "bar.module-gap")},
		{title: "settings-section-border", advanced: true, rows: bordered("bar.border-location", field("bar.border-width", borderWidth), field("bar.border-color"))},

		{title: "settings-section-buttons", rows: []rowSpec{
			field("bar.button-variant", pictureChoice(buttonVariantPictures)),
			field("bar.button-opacity", percentage),
			field("bar.button-bg-opacity", percentage),
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
			field("bar.button-group-opacity", percentage),
			field("bar.button-group-rounding", roundingChoice),
			field("bar.button-group-padding"),
			field("bar.button-group-module-gap"),
			field("bar.button-group-background"),
		}, bordered("bar.button-group-border-location", field("bar.button-group-border-width", borderWidth), field("bar.button-group-border-color"))...)},
		{title: "settings-section-dropdowns", rows: []rowSpec{
			field("bar.dropdown-shadow"), field("bar.dropdown-autohide"), field("bar.dropdown-freeze-label"),
			field("bar.dropdown-opacity", percentage),
		}},
		{title: "settings-section-dropdown-animation", advanced: true, rows: surfaceAnimationRows("animations.dropdown")},
		{title: "settings-animations-menu", advanced: true, rows: surfaceAnimationRows("animations.menu")},
	}}
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

// roundingChoice is a rounding level chosen on a picture of its corners.
var roundingChoice = pictureChoice(roundingPictures)

// notificationsPage is pages/notifications.
func notificationsPage(*config.Config) pageSpec {
	return pageSpec{id: "notifications", navKey: "settings-nav-notifications", icon: "ld-bell-symbolic", header: "settings-page-notifications", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field("modules.notifications.enabled"),
			field("modules.notifications.history-memory"),
			field("modules.notifications.history-stored"),
		}},
		{title: "settings-section-popup-display", rows: []rowSpec{
			field("modules.notifications.popup-position", screenPosition),
			field("modules.notifications.popup-max-visible"),
			field("modules.notifications.popup-stacking-order"),
			field("modules.notifications.popup-stack"),
			field("modules.notifications.popup-duration"),
			field("modules.notifications.popup-hover-pause"),
			field("modules.notifications.popup-shadow"),
			field("modules.notifications.popup-opacity", percentage),
			field("modules.notifications.popup-close-behavior"),
			field("modules.notifications.popup-urgency-bar"),
		}},
		{title: "settings-section-positioning", rows: []rowSpec{
			field("modules.notifications.popup-monitor"),
			field("modules.notifications.popup-layer"),
			field("modules.notifications.popup-margin-x", sizeBase(config.PopupMarginBaseRem)),
			field("modules.notifications.popup-margin-y", sizeBase(config.PopupMarginBaseRem)),
			field("modules.notifications.popup-gap", sizeBase(config.PopupGapBaseRem)),
		}},
		{title: "settings-section-filtering", rows: []rowSpec{
			field("modules.notifications.icon-source"),
			field("modules.notifications.blocklist", appList),
		}},
		{title: "settings-section-animation", rows: surfaceAnimationRows("animations.notifications")},
		{title: "settings-section-focus-automatic", rows: append(fields("modules.notifications.dnd.while-sharing", "modules.notifications.dnd.while-fullscreen"), field("modules.notifications.dnd.schedule", dndScheduleList))},
		{title: "settings-section-focus-allowed", rows: []rowSpec{field("modules.notifications.dnd.allow-critical"), field("modules.notifications.dnd.allow-apps", appList)}},
		{title: "settings-section-bell", rows: append([]rowSpec{
			field("modules.notifications.icon-name", iconRow),
			field("modules.notifications.icon-unread", iconRow),
			field("modules.notifications.icon-dnd", iconRow),
			field("modules.notifications.thresholds", thresholdList),
		}, barItemSection("modules.notifications", actionChoices("notification")).rows...)},
		dropdownSizeSection("notification"),
	}}
}

// osdPage is pages/osd.
func osdPage(*config.Config) pageSpec {
	return pageSpec{id: "osd", navKey: "settings-nav-osd", icon: "ld-monitor-symbolic", header: "settings-page-osd", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field("osd.enabled"),
		}},
		{title: "settings-section-display", rows: []rowSpec{
			field("osd.position", screenPosition),
			field("osd.layer"),
			field("osd.text-align"),
			field("osd.duration"),
			field("osd.monitor"),
			field("osd.margin", sizeBase(config.OsdMarginBaseRem)),
			field("osd.border"),
			field("osd.media"),
		}},
		{title: "settings-section-presets", rows: []rowSpec{
			field("osd.presets", toastPresetList),
		}},
		{title: "settings-section-animation", rows: surfaceAnimationRows("animations.osd")},
	}}
}

// launcherPage is pages/launcher entry.
func launcherPage(*config.Config) pageSpec {
	return pageSpec{id: "launcher", navKey: "settings-nav-launcher-page", icon: "ld-rocket-symbolic", header: "settings-page-launcher", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field("launcher.location", screenPosition),
			field("launcher.width", sizeBase(config.LauncherWidthBaseRem)),
			field("launcher.lines"),
			field("launcher.monitor"),
			field("launcher.modes", launcherModesRow),
			field("launcher.cycle"),
			field("launcher.fixed-num-lines"),
			field("launcher.sidebar-mode"),
		}},
		{title: "settings-section-launcher-matching", advanced: true, rows: []rowSpec{
			field("launcher.matching"),
			field("launcher.tokenize"),
			field("launcher.negate-char"),
			field("launcher.normalize-match"),
			field("launcher.sort"),
			field("launcher.sorting-method"),
			field("launcher.case"),
			field("launcher.auto-select"),
			field("launcher.hover-select"),
		}},
		{title: "settings-section-launcher-appearance", rows: []rowSpec{
			field("launcher.show-icons"),
			field("launcher.icon-theme"),
			field("launcher.font", fontRow),
			field("launcher.styles", stringMap),
			field("launcher.preview-cmd"),
			field("launcher.emoji.skin-tone"),
			field("launcher.terminal", terminalRow),
			field("launcher.display-names", displayNamesRow),
		}},
		{title: "settings-section-launcher-history", rows: []rowSpec{
			field("launcher.history.enable"),
			field("launcher.history.max-size"),
		}},
		{title: "settings-section-launcher-keybindings", rows: []rowSpec{
			field("launcher.keybindings", stringMap),
			field("launcher.mouse-bindings", stringMap),
		}},
		{title: "settings-section-animation", rows: surfaceAnimationRows("animations.launcher")},
	}}
}

// launcherModesPage is pages/launcher modes_entry.
func launcherModesPage(*config.Config) pageSpec {
	return pageSpec{id: "launcher-modes", navKey: "settings-nav-launcher-modes-page", icon: "ld-grid-2x2-symbolic", header: "settings-page-launcher-modes", sections: []sectionSpec{
		{title: "settings-section-launcher-drun", rows: []rowSpec{
			field("launcher.drun.categories", stringList),
			field("launcher.drun.exclude-categories", stringList),
			field("launcher.drun.match-fields", enumList),
			field("launcher.drun.display-format"),
			field("launcher.drun.show-actions"),
			field("launcher.drun.url-launcher"),
		}},
		{title: "settings-section-launcher-run", rows: []rowSpec{
			field("launcher.run.run-command"),
			field("launcher.run.shell-command"),
			field("launcher.run.list-command"),
		}},
		{title: "settings-section-launcher-window", rows: []rowSpec{
			field("launcher.window.format"),
			field("launcher.window.match-fields", enumList),
			field("launcher.window.hide-active"),
			field("launcher.window.close-on-delete"),
		}},
		{title: "settings-section-launcher-ssh", rows: []rowSpec{
			field("launcher.ssh.client"),
			field("launcher.ssh.command"),
			field("launcher.ssh.parse-hosts"),
			field("launcher.ssh.parse-known-hosts"),
		}},
		{title: "settings-section-launcher-filebrowser", rows: []rowSpec{
			field("launcher.filebrowser.directory"),
			field("launcher.filebrowser.sorting-method"),
			field("launcher.filebrowser.directories-first"),
			field("launcher.filebrowser.show-hidden"),
			field("launcher.filebrowser.command"),
		}},
		{title: "settings-section-launcher-combi", rows: []rowSpec{
			field("launcher.combi.modes", launcherModesRow),
			field("launcher.combi.display-format"),
		}},
		{title: "settings-section-launcher-search", rows: []rowSpec{
			field("launcher.search.providers", enumList),
			field(webEnginePath, webEngineRow),
			field("launcher.search.file-roots", folderList),
			field("launcher.search.file-exclude", stringList),
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
	return pageSpec{id: "theme", navKey: "settings-nav-theme", icon: "ld-palette-symbolic", header: "settings-page-theme", sections: []sectionSpec{
		{title: "settings-section-provider", rows: []rowSpec{
			field("styling.appearance", pictureChoice(appearancePictures)),
			field("styling.high-contrast"),
			field("styling.reduce-transparency"),
			field("styling.theme-provider", pictureChoice(themeProviderPictures)),
			field("styling.theming-monitor"),
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
			field("styling.matugen-source-color"),
			field("styling.matugen-light"),
		}},
		{title: "settings-section-wallust", when: themeProviderIs(config.ThemeProviderWallust), rows: []rowSpec{
			field("styling.wallust-palette"),
			field("styling.wallust-saturation", percentage),
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
	return func(s store) bool {
		v, _ := s.value("styling.theme-provider").(string)
		return v == string(provider)
	}
}

func wallpaperPage(*config.Config) pageSpec {
	return pageSpec{id: "wallpaper", navKey: "settings-nav-wallpaper", icon: "ld-image-symbolic", header: "settings-page-wallpaper", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{field("wallpaper.wallpaper", filePath), field("wallpaper.fit-mode", pictureChoice(fitPictures)), field("wallpaper.picker-directories", folderList)}},
		{title: "settings-section-cycling", rows: []rowSpec{
			field("wallpaper.cycling-directory"),
			cyclingReveal("wallpaper.cycling-directory",
				field("wallpaper.cycling-mode"),
				field("wallpaper.cycling-interval-mins", spin(1, 1440, 1, 0)),
				field("wallpaper.cycling-same-image"),
			),
		}},
		{title: "settings-section-display", rows: []rowSpec{field("wallpaper.monitors", monitorWallpaperRow, func(r *rowSpec) { r.fullWidth = true })}},
		{title: "settings-section-animation", rows: surfaceAnimationRows("animations.wallpaper")},
	}}
}

// dockRow is a dock row, in use only while the dock is shown.
var dockRow = enabledBy("dock.enabled")

// dockPage is the standalone dock.
func dockPage(*config.Config) pageSpec {
	return pageSpec{id: "dock", navKey: "settings-nav-dock", icon: "ld-panel-bottom-symbolic", header: "settings-page-dock", sections: []sectionSpec{
		{title: "settings-section-general", rows: []rowSpec{
			field("dock.enabled"),
			field("dock.monitors", monitorsRow, dockRow),
			field("dock.edge", screenPosition, dockRow),
			field("dock.auto-hide", dockRow),
			field("dock.auto-hide-delay-ms", dockRow, shownWhen("dock.auto-hide", switchedOn)),
			field("dock.pinned", appList, dockRow),
		}},
		{title: "settings-section-appearance", rows: []rowSpec{
			field("dock.icon-size", sizeBase(config.TaskIconBaseRem), dockRow),
			field("dock.hover-zoom", dockRow),
			field("dock.hover-zoom-size", sizeBase(config.TaskIconBaseRem), dockRow, shownWhen("dock.hover-zoom", switchedOn)),
			field("dock.icon", iconRow, dockRow),
		}},
	}}
}
