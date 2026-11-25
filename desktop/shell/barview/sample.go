package barview

import (
	"time"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/desktop/compositor"
	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/jinja"
)

// Preview is the bar connector's layout gives on compositor on, every
// module a Sample; false when the config shows no bar there.
func Preview(cfg *config.Config, font render.Font, connector string, on compositor.Kind) (*widget.Bar, bool) {
	layout, ok := LayoutFor(cfg, connector)
	if !ok {
		return nil, false
	}
	return SampleBar(cfg, font, layout, on), true
}

// SampleBar is a bar of layout's items on compositor on, every module a
// Sample in the state states give it.
func SampleBar(cfg *config.Config, font render.Font, layout config.BarLayout, on compositor.Kind, states ...State) *widget.Bar {
	return NewBar(cfg, layout, "", func(row *widget.Box, item config.BarItem) {
		row.Append(Sample(cfg, font, item, on, states...), false)
	})
}

// State is a real state of one module for its Sample to show in place of
// its sample placeholders, and what a press on its button does.
type State struct {
	module  config.BarModule
	vars    map[string]any
	pressed func()
}

// KeybindModeState is keybind-mode in submap, the default one for "".
func KeybindModeState(submap string) State {
	return State{module: config.ModuleKeybindMode, vars: KeybindModeVars(submap)}
}

// KeyboardInputState is keyboard-input on the layout the compositor
// names, under the short names aliases gives layouts.
func KeyboardInputState(layout string, aliases map[string]string) State {
	return State{module: config.ModuleKeyboardInput, vars: KeyboardLayoutVars(layout, aliases)}
}

// OnPress is s with pressed run on a press of the module's button.
func (s State) OnPress(pressed func()) State {
	s.pressed = pressed
	return s
}

// KeybindModeVars are keybind-mode's format variables in submap: the
// default submap, "", is the "default" vocabulary word.
func KeybindModeVars(submap string) map[string]any {
	mode := submap
	if mode == "" {
		mode = i18n.T("bar-keybind-mode-default")
	}
	return map[string]any{"mode": mode}
}

// KeyboardLayoutVars are keyboard-input's format variables on layout:
// the layout and its alias, the layout itself while aliases gives it
// none.
func KeyboardLayoutVars(layout string, aliases map[string]string) map[string]any {
	alias, ok := aliases[layout]
	if !ok {
		alias = layout
	}
	return map[string]any{"layout": layout, "alias": alias}
}

func stateOf(m config.BarModule, states []State) (State, bool) {
	for _, s := range states {
		if s.module == m {
			return s, true
		}
	}
	return State{}, false
}

// Sample is item's module as the bar draws it, holding sample state
// instead of what a service would report: the separator's rule, the
// workspace strip of SampleWorkspaces on compositor on, or the module's
// button in its module box with the icon the module starts with and a
// sample label, under the module's button config. A module with no
// icon of its own shows its title. A module one of states is for shows
// that state instead.
func Sample(cfg *config.Config, font render.Font, item config.BarItem, on compositor.Kind, states ...State) widget.Widget {
	switch item.Module {
	case config.ModuleSeparator:
		rule := NewSeparator(cfg)
		OwnChrome(rule, item, cfg)
		return rule
	case config.ModuleWorkspaces:
		strip := SampleWorkspaces(cfg, font, on)
		OwnChrome(strip.Segments(), item, cfg)
		return strip.Segments()
	}
	name := string(item.Module)
	icon, hasIcon := cfg.ModuleIcon(item.Module)
	label, failure := sampleLabel(cfg, item.Module, hasIcon)
	state, live := stateOf(item.Module, states)
	if live {
		label = jinja.RenderOr(moduleFormats[item.Module](cfg), state.vars)
	}
	b := NewButton(cfg, font, Icon(icon), label)
	b.Configure(ModuleTitle(name, cfg), ModuleButton(name, cfg))
	if failure != "" {
		b.SetFailure(failure)
	}
	if state.pressed != nil {
		b.OnClick = state.pressed
	}
	wrap := ModuleBox(item)
	wrap.Append(b, false)
	return wrap
}

// SampleWorkspaces is the workspace strip under cfg as compositor on
// reads it, holding sample workspaces: the active one named and holding
// two windows, one more holding a window, and an empty last one; mango
// shows them as its tags, and Hyprland numbers them as the workspaces
// bound to a second monitor.
func SampleWorkspaces(cfg *config.Config, font render.Font, on compositor.Kind) *WorkspaceStrip {
	c := cfg.Workspaces.View(on)
	strip := NewWorkspaceStrip(cfg, c, "", nil, WorkspaceFace(font), WorkspaceHandlers{})
	switch on {
	case compositor.Mango:
		strip.Set(WorkspaceButtons(FlavorTags, sampleWorkspaces, sampleWindows(c, 0), c, "", strip.Vertical(), false))
	case compositor.Hyprland:
		strip.Set(sampleHyprland(cfg.Workspaces, c, strip.Vertical()))
	default:
		strip.Set(WorkspaceButtons(FlavorWorkspaces, sampleWorkspaces, sampleWindows(c, 0), c, "", strip.Vertical(), false))
	}
	return strip
}

var sampleWorkspaces = []Workspace{
	{ID: 1, Num: 1, Name: "web", HasName: true, HasOutput: true, Active: true, Focused: true, HasWindows: true},
	{ID: 2, Num: 2, HasOutput: true, HasWindows: true},
	{ID: 3, Num: 3, HasOutput: true},
}

func sampleWindows(c config.CompositorWorkspacesConfig, shift uint64) []WorkspaceWindow {
	return []WorkspaceWindow{
		{ID: 1, Workspace: 1 + shift, HasWorkspace: true, Icon: c.AppIconsFallback},
		{ID: 2, Workspace: 1 + shift, HasWorkspace: true, Icon: c.AppIconsFallback},
		{ID: 3, Workspace: 2 + shift, HasWorkspace: true, Icon: c.AppIconsFallback},
	}
}

const (
	sampleHyprMonitor = "DP-2"
	sampleHyprShift   = 3
)

func sampleHyprland(workspaces config.WorkspacesConfig, c config.CompositorWorkspacesConfig, vertical bool) []WorkspaceButton {
	shifted := make([]Workspace, len(sampleWorkspaces))
	rules := make(map[int]string, len(sampleWorkspaces))
	for i, ws := range sampleWorkspaces {
		ws.ID += sampleHyprShift
		ws.Num += sampleHyprShift
		ws.Output = sampleHyprMonitor
		shifted[i] = ws
		rules[ws.Num] = sampleHyprMonitor
	}
	buttons := WorkspaceButtons(FlavorHyprland, shifted, sampleWindows(c, sampleHyprShift), c, sampleHyprMonitor, vertical, false)
	numbering := NewHyprNumbering(workspaces, sampleHyprMonitor, rules)
	for i, b := range buttons {
		buttons[i].Label, buttons[i].HasLabel = numbering.Label(b.WS.Num, b.WS.Name), true
	}
	return buttons
}

// SampleTime is the moment every sample the bar's previews show reads,
// in UTC so a preview reads alike in every time zone.
var SampleTime = time.Date(2026, time.March, 14, 9, 41, 0, 0, time.UTC)

const sampleBackgroundApps = "2"

var moduleFormats = map[config.BarModule]func(*config.Config) string{
	config.ModuleAudio:         func(c *config.Config) string { return c.Audio.Format },
	config.ModuleBattery:       func(c *config.Config) string { return c.Battery.Format },
	config.ModuleBitwarden:     func(c *config.Config) string { return c.Bitwarden.Format },
	config.ModuleBrightness:    func(c *config.Config) string { return c.Brightness.Format },
	config.ModuleCPU:           func(c *config.Config) string { return c.CPU.Format },
	config.ModuleIdleInhibit:   func(c *config.Config) string { return c.IdleInhibit.Format },
	config.ModuleKeybindMode:   func(c *config.Config) string { return c.KeybindMode.Format },
	config.ModuleKeyboardInput: func(c *config.Config) string { return c.KeyboardInput.Format },
	config.ModuleMail:          func(c *config.Config) string { return c.Mail.Format },
	config.ModuleMedia:         func(c *config.Config) string { return c.Media.Format },
	config.ModuleNetstat:       func(c *config.Config) string { return c.Netstat.Format },
	config.ModuleNightlight:    func(c *config.Config) string { return c.Nightlight.Format },
	config.ModulePowerProfiles: func(c *config.Config) string { return c.PowerProfiles.Format },
	config.ModuleRAM:           func(c *config.Config) string { return c.RAM.Format },
	config.ModuleRecorder:      func(c *config.Config) string { return c.Recorder.Format },
	config.ModuleStorage:       func(c *config.Config) string { return c.Storage.Format },
	config.ModuleTreeman:       func(c *config.Config) string { return c.Treeman.Format },
	config.ModuleWeather:       func(c *config.Config) string { return c.Weather.Format },
	config.ModuleWindowTitle:   func(c *config.Config) string { return c.WindowTitle.Format },
}

func formatPath(m config.BarModule) string { return "modules." + string(m) + ".format" }

func sampleFormatLabel(m config.BarModule, format string) string {
	vars := config.PlaceholderSamples(formatPath(m))
	if m == config.ModuleMail {
		return jinja.ReplaceVars(format, vars)
	}
	return jinja.RenderOr(format, vars)
}

// sampleLabel is the label m shows on its sample state, or why the
// module would fail to show one.
func sampleLabel(cfg *config.Config, m config.BarModule, hasIcon bool) (label, failure string) {
	switch m {
	case config.ModuleClock:
		if cfg.Clock.Format.Layout() == nil {
			return "", ""
		}
		return ClockLabel(cfg.Clock, SampleTime, ClockNames(i18n.Shell()), func() string { return "" }), ""
	case config.ModuleWorldClock:
		text, err := NewWorldClock(cfg.WorldClock.Format, ClockNames(i18n.Shell()), time.LoadLocation).Render(SampleTime)
		if err != nil {
			return "", WorldClockFailure(err)
		}
		return text, ""
	case config.ModuleScreenshot:
		return cfg.Screenshot.Label, ""
	case config.ModuleBackgroundApps:
		return sampleBackgroundApps, ""
	}
	if format, ok := moduleFormats[m]; ok {
		return sampleFormatLabel(m, format(cfg)), ""
	}
	if def, ok := CustomDefinition(string(m), cfg); ok {
		return jinja.RenderOr(def.Format, config.ItemPlaceholderSamples[config.CustomModuleDefinition]("format")), ""
	}
	if hasIcon {
		return "", ""
	}
	return ModuleTitle(string(m), cfg), ""
}
