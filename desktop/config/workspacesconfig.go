package config

import (
	"slices"

	"github.com/stubbedev/lys/desktop/compositor"
)

// DisplayMode is part of the workspaces module's configuration.
//
// What identifies a workspace in the UI.
type DisplayMode string

// DisplayMode values.
const (
	// Show workspace number or name.
	DisplayModeLabel DisplayMode = "label"
	// Show icon from `workspace-map` (falls back to label if unmapped).
	DisplayModeIcon DisplayMode = "icon"
	// Show nothing - only app icons visible.
	DisplayModeNone DisplayMode = "none"
)

var _ = registerEnum(DisplayModeLabel, DisplayModeIcon, DisplayModeNone)

// Numbering is part of the workspaces module's configuration.
//
// How workspace numbers are displayed.
type Numbering string

// Numbering values.
const (
	// Show actual Hyprland workspace IDs (1, 2, 3, 4, 5, 6...).
	NumberingAbsolute Numbering = "absolute"
	// Show numbers relative to monitor's starting workspace.
	//
	// If monitor has workspaces 4, 5, 6 assigned, they display as 1, 2, 3.
	// Useful when keybinds use per-monitor numbering (Shift+1 for ws 4, etc.).
	NumberingRelative Numbering = "relative"
)

var _ = registerEnum(NumberingAbsolute, NumberingRelative)

// UrgentMode is part of the workspaces module's configuration.
//
// Where the urgent pulse animation is applied.
type UrgentMode string

// UrgentMode values.
const (
	// Pulse the entire workspace.
	UrgentWorkspace UrgentMode = "workspace"
	// Pulse only the app icon(s) belonging to the urgent window.
	//
	// Falls back to `workspace` when app icons are disabled.
	UrgentApplication UrgentMode = "application"
)

var _ = registerEnum(UrgentWorkspace, UrgentApplication)

// ActiveIndicator is part of the workspaces module's configuration.
//
// Visual indicator style for the active workspace.
type ActiveIndicator string

// ActiveIndicator values.
const (
	// Entire button gets a colored background.
	ActiveBackground ActiveIndicator = "background"
	// Small colored bar under the workspace button.
	ActiveUnderline ActiveIndicator = "underline"
)

var _ = registerEnum(ActiveBackground, ActiveUnderline)

// WorkspaceStyle is part of the workspaces module's configuration.
//
// Per-workspace styling override.
type WorkspaceStyle struct {
	// Custom icon for this workspace. When set, the icon is shown regardless
	// of the module's `display-mode`, so a row can mix labeled workspaces
	// with icon-only ones (e.g. `[1][2][icon]`).
	Icon *string `cfg:"icon"`
	// Custom background color for this workspace when active.
	Color *ColorValue `cfg:"color"`
	// Text shown instead of the workspace's name or index.
	Label *string `cfg:"label"`
}

// LabelStrategy is part of the workspaces module's configuration.
//
// What identifies each workspace's label.
type LabelStrategy string

// LabelStrategy values.
const (
	// Always show the index (e.g. `1`, `2`, `3`).
	LabelIndex LabelStrategy = "index"
	// Show the name when set, fall back to the index.
	LabelNameOrIndex LabelStrategy = "name-or-index"
	// Show only the name; unnamed workspaces show nothing.
	LabelNameOnly LabelStrategy = "name-only"
	// Show both, joined as `"1: web"`. Unnamed workspaces show the index alone.
	LabelIndexAndName LabelStrategy = "index-and-name"
)

var _ = registerEnum(LabelIndex, LabelNameOrIndex, LabelNameOnly, LabelIndexAndName)

// WorkspacesConfig configures the workspaces module, the same on every
// compositor; a key only some compositors honour says which.
//
// Workspace indicators with click-to-switch, on whichever compositor runs.
type WorkspacesConfig struct {
	// What identifies each workspace button.
	//
	// - `label` (default): show the workspace label per `label-strategy`
	// - `icon`: show an icon from `workspace-map` (falls back to label if unmapped)
	// - `none`: show nothing — only app icons visible (if enabled)
	DisplayMode DisplayMode `cfg:"display-mode"`
	// Pulse animation on workspaces with urgent windows.
	UrgentShow bool `cfg:"urgent-show"`
	// Where the urgent pulse is applied.
	//
	// - `workspace` (default): whole button pulses
	// - `application`: only the urgent app icon pulses, falling back to
	//   `workspace` when app icons are disabled
	UrgentMode UrgentMode `cfg:"urgent-mode"`
	// Preview a workspace's windows when the pointer rests on its button.
	//
	// When `false` (default), hovering a workspace does nothing and no
	// window is captured.
	WindowPreviews bool `cfg:"window-previews"`
	// Visual indicator for the active workspace.
	ActiveIndicator ActiveIndicator `cfg:"active-indicator"`
	// Text separator between workspace identity and app icons.
	Divider string `cfg:"divider"`
	// Show application icons for windows on each workspace.
	AppIconsShow bool `cfg:"app-icons-show"`
	// Deduplicate application icons within a workspace.
	//
	// When `true`, one icon per unique `app_id`. When `false`, one icon
	// per window.
	AppIconsDedupe bool `cfg:"app-icons-dedupe"`
	// Fallback icon for applications not matched by `app-icon-map`.
	AppIconsFallback string `cfg:"app-icons-fallback"`
	// Icon shown when a workspace has no application windows.
	AppIconsEmpty string `cfg:"app-icons-empty"`
	// Gap between app icons within a workspace button. Accepts a scale multiplier or pixels (e.g. `"4px"`).
	IconGap Size `cfg:"icon-gap"`
	// Padding along the bar direction for content wider than the circle: app icons, a divider or a long name. Accepts a scale multiplier or pixels (e.g. `"8px"`).
	//
	// A short label takes no padding. It sets the circle's size instead: at least a digit's width plus this padding on each side.
	WorkspacePadding Size `cfg:"workspace-padding"`
	// Workspace icon size. Accepts a scale multiplier or pixels (e.g. `"16px"`).
	//
	// Applies to identity icons and custom icons from `workspace-map`.
	IconSize Size `cfg:"icon-size"`
	// Workspace label and divider size. Accepts a scale multiplier or pixels (e.g. `"16px"`).
	LabelSize Size `cfg:"label-size"`
	// Color for the active (visible on its output) workspace.
	//
	// In `background` indicator mode, also used as the button background.
	ActiveColor ColorValue `cfg:"active-color"`
	// Color for occupied workspaces (have windows but not active).
	OccupiedColor ColorValue `cfg:"occupied-color"`
	// Color for empty workspaces and placeholder slots.
	EmptyColor ColorValue `cfg:"empty-color"`
	// Background color for the workspaces container.
	ContainerBgColor ColorValue `cfg:"container-bg-color"`
	// Display border around the workspaces container.
	BorderShow bool `cfg:"border-show"`
	// Border color for the workspaces container.
	BorderColor ColorValue `cfg:"border-color"`
	// Per-workspace icon and color overrides, keyed by name or id-as-string.
	//
	// ## Example
	//
	// ```yaml
	// modules:
	//   workspaces:
	//     workspace-map:
	//       web: {icon: ld-globe-symbolic, color: "#4a90d9"}
	//       terminal: {icon: ld-terminal-symbolic}
	// ```
	WorkspaceMap WorkspaceMap `cfg:"workspace-map"`
	// Application icon mapping with glob pattern support.
	//
	// Maps window `app_id` or title to symbolic icon names. Supports:
	// - No prefix: matches `app_id` (e.g. `"*firefox*"`)
	// - `app:` prefix: explicit `app_id` match (e.g. `"app:org.mozilla.*"`)
	// - `title:` prefix: matches window title (e.g. `"title:*YouTube*"`)
	//
	// ## Example
	//
	// ```yaml
	// modules:
	//   workspaces:
	//     app-icon-map:
	//       "*firefox*": ld-globe-symbolic
	//       "title:*YouTube*": ld-youtube-symbolic
	// ```
	AppIconMap map[string]string `cfg:"app-icon-map"`
	// Action on left click.
	LeftClick WorkspaceClickAction `cfg:"left-click"`
	// Action on middle click.
	MiddleClick WorkspaceClickAction `cfg:"middle-click"`
	// Action on right click.
	RightClick WorkspaceClickAction `cfg:"right-click"`
	// Action on scroll up.
	ScrollUp WorkspaceClickAction `cfg:"scroll-up"`
	// Action on scroll down.
	ScrollDown WorkspaceClickAction `cfg:"scroll-down"`
	// Show only workspaces on this bar's monitor (all but mango).
	//
	// When `true` (default), each bar shows only its own output's
	// workspaces. When `false`, all workspaces from every output are shown.
	MonitorSpecific bool `cfg:"monitor-specific"`
	// Workspaces to hide from the display (all but mango).
	//
	// Glob patterns matched against the workspace's name (if set), then
	// its index, then its stable id. Examples:
	// - `"scratch"` — hide the workspace named `scratch`
	// - `"1?"` — hide indices 10-19
	WorkspaceIgnore []string `cfg:"workspace-ignore"`
	// Hide the trailing empty workspace niri keeps (niri, sway).
	//
	// Niri keeps one empty workspace at the tail of every output for
	// dynamic allocation.
	HideTrailingEmpty bool `cfg:"hide-trailing-empty"`
	// How to compose the workspace label when `display-mode: label` (niri, sway).
	//
	// - `index`: index only (`"1"`, `"2"`)
	// - `name-or-index` (default): name when set, index otherwise
	// - `name-only`: name only; unnamed workspaces show nothing
	// - `index-and-name`: `"1: web"` form; unnamed workspaces show the index alone
	LabelStrategy LabelStrategy `cfg:"label-strategy"`
	// Minimum number of workspace buttons to display (Hyprland).
	//
	// When set to 0 (default), only active and occupied workspaces are shown.
	// When set to N, at least N buttons are always visible, with empty ones
	// using `empty-color` styling.
	MinWorkspaceCount uint8 `cfg:"min-workspace-count"`
	// Include special workspaces (scratchpads) in the display (Hyprland).
	//
	// Special workspaces have negative IDs in Hyprland.
	ShowSpecial bool `cfg:"show-special"`
	// Use workspace name instead of number when displaying labels (Hyprland).
	//
	// Only applies when `display-mode: label` or as fallback for unmapped
	// workspaces in `display-mode: icon`.
	LabelUseName bool `cfg:"label-use-name"`
	// How workspace numbers are displayed (Hyprland).
	//
	// - `absolute`: Show actual Hyprland workspace IDs (1, 2, 3, 4, 5, 6...)
	// - `relative`: Show numbers relative to monitor's starting workspace.
	//   If a monitor has workspaces 4, 5, 6 assigned, they display as 1, 2, 3.
	//   Useful when keybinds use per-monitor numbering.
	Numbering Numbering `cfg:"numbering"`
	// Highlight workspaces active on other monitors with a different color (Hyprland).
	//
	// When true, workspaces active on a different monitor are highlighted differently.
	// When false, workspaces active on another monitor are not specially highlighted.
	//
	// This setting only makes sense when `monitor-specific` is false.
	HighlightActiveOnOtherMonitor bool `cfg:"highlight-active-on-other-monitor"`
	// Active on other monitor indicator color (Hyprland).
	//
	// Only applies when `highlight-active-on-other-monitor` is `true`.
	ActiveOnOtherMonitorColor ColorValue `cfg:"active-on-other-monitor-color"`
	// Hide tags that hold no clients and are not active (mango).
	HideEmpty bool `cfg:"hide-empty"`
	// Always show tags up to this one-based index, even when empty (mango).
	//
	// `0` shows only occupied or active tags (subject to `hide-empty`). A
	// value above the compositor's tag count just shows every tag.
	MinTagCount uint8 `cfg:"min-tag-count"`
}

// DefaultsWorkspaces returns the schema defaults.
func DefaultsWorkspaces() WorkspacesConfig {
	return WorkspacesConfig{
		DisplayMode:                   DisplayModeLabel,
		UrgentShow:                    true,
		UrgentMode:                    UrgentWorkspace,
		WindowPreviews:                false,
		ActiveIndicator:               ActiveBackground,
		Divider:                       " ",
		AppIconsShow:                  false,
		AppIconsDedupe:                true,
		AppIconsFallback:              "ld-app-window-symbolic",
		AppIconsEmpty:                 "ld-minus-symbolic",
		IconGap:                       Size{Value: 0.3, Unit: SizeMultiplier},
		WorkspacePadding:              Size{Value: 0.5, Unit: SizeMultiplier},
		IconSize:                      Size{Value: 1, Unit: SizeMultiplier},
		LabelSize:                     Size{Value: 1, Unit: SizeMultiplier},
		ActiveColor:                   mustColor("accent"),
		OccupiedColor:                 mustColor("fg-muted"),
		EmptyColor:                    mustColor("fg-subtle"),
		ContainerBgColor:              mustColor("bg-surface-elevated"),
		BorderShow:                    false,
		BorderColor:                   mustColor("border-default"),
		WorkspaceMap:                  WorkspaceMap{},
		AppIconMap:                    map[string]string{},
		LeftClick:                     ParseWorkspaceClickAction("focus:this"),
		MiddleClick:                   ParseWorkspaceClickAction(""),
		RightClick:                    ParseWorkspaceClickAction(""),
		ScrollUp:                      ParseWorkspaceClickAction("focus:previous"),
		ScrollDown:                    ParseWorkspaceClickAction("focus:next"),
		MonitorSpecific:               true,
		WorkspaceIgnore:               []string{},
		HideTrailingEmpty:             true,
		LabelStrategy:                 LabelNameOrIndex,
		MinWorkspaceCount:             0,
		ShowSpecial:                   true,
		LabelUseName:                  false,
		Numbering:                     NumberingAbsolute,
		HighlightActiveOnOtherMonitor: true,
		ActiveOnOtherMonitorColor:     mustColor("accent"),
		HideEmpty:                     true,
		MinTagCount:                   0,
	}
}

// Clicks returns the five input bindings.
func (c WorkspacesConfig) Clicks() WorkspaceClicks {
	return WorkspaceClicks{c.LeftClick, c.RightClick, c.MiddleClick, c.ScrollUp, c.ScrollDown}
}

var workspaceKeyCompositors = map[string][]compositor.Kind{
	"monitor-specific":                  {compositor.LysWM, compositor.Hyprland, compositor.Niri, compositor.Sway},
	"workspace-ignore":                  {compositor.LysWM, compositor.Hyprland, compositor.Niri, compositor.Sway},
	"hide-trailing-empty":               {compositor.Niri, compositor.Sway},
	"label-strategy":                    {compositor.Niri, compositor.Sway},
	"min-workspace-count":               {compositor.Hyprland},
	"show-special":                      {compositor.Hyprland},
	"label-use-name":                    {compositor.Hyprland},
	"numbering":                         {compositor.Hyprland},
	"highlight-active-on-other-monitor": {compositor.Hyprland},
	"active-on-other-monitor-color":     {compositor.Hyprland},
	"hide-empty":                        {compositor.Mango},
	"min-tag-count":                     {compositor.Mango},
}

// WorkspaceKeyHonored reports whether the workspaces module reads key
// (`modules.workspaces.<key>`) on compositor c. Every key not tied to some
// compositors is read on all of them.
func WorkspaceKeyHonored(key string, c compositor.Kind) bool {
	only, tied := workspaceKeyCompositors[key]
	return !tied || slices.Contains(only, c)
}

// View is the module's settings on compositor c in the shape every
// compositor's workspace row shares: a key c does not honour takes the
// value that leaves its behaviour out.
func (c WorkspacesConfig) View(on compositor.Kind) CompositorWorkspacesConfig {
	v := CompositorWorkspacesConfig{
		MonitorSpecific: c.MonitorSpecific, HideTrailingEmpty: c.HideTrailingEmpty, DisplayMode: c.DisplayMode,
		LabelStrategy: c.LabelStrategy, UrgentShow: c.UrgentShow, UrgentMode: c.UrgentMode, WindowPreviews: c.WindowPreviews,
		ActiveIndicator: c.ActiveIndicator, Divider: c.Divider, AppIconsShow: c.AppIconsShow,
		AppIconsDedupe: c.AppIconsDedupe, AppIconsFallback: c.AppIconsFallback, AppIconsEmpty: c.AppIconsEmpty,
		IconGap: c.IconGap, WorkspacePad: c.WorkspacePadding, IconSize: c.IconSize, LabelSize: c.LabelSize,
		WorkspaceIgnore: c.WorkspaceIgnore, ActiveColor: c.ActiveColor, OccupiedColor: c.OccupiedColor,
		EmptyColor: c.EmptyColor, ContainerBgColor: c.ContainerBgColor, BorderShow: c.BorderShow,
		BorderColor: c.BorderColor, WorkspaceMap: c.WorkspaceMap, AppIconMap: c.AppIconMap, Click: c.Clicks(),
	}
	if !WorkspaceKeyHonored("monitor-specific", on) {
		v.MonitorSpecific = false
	}
	if !WorkspaceKeyHonored("workspace-ignore", on) {
		v.WorkspaceIgnore = nil
	}
	if !WorkspaceKeyHonored("hide-trailing-empty", on) {
		v.HideTrailingEmpty = false
	}
	if !WorkspaceKeyHonored("label-strategy", on) {
		v.LabelStrategy = LabelIndex
	}
	return v
}
