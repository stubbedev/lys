package barview

import (
	"time"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/desktop/compositor"
	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
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
// Sample.
func SampleBar(cfg *config.Config, font render.Font, layout config.BarLayout, on compositor.Kind) *widget.Bar {
	return NewBar(cfg, layout, "", func(row *widget.Box, item config.BarItem) {
		row.Append(Sample(cfg, font, item, on), false)
	})
}

// Sample is item's module as the bar draws it, holding sample state
// instead of what a service would report: the separator's rule, the
// workspace strip of SampleWorkspaces on compositor on, or the module's
// button in its module box with the icon the module starts with and a
// sample label, under the module's button config. A module with no
// icon of its own shows its title.
func Sample(cfg *config.Config, font render.Font, item config.BarItem, on compositor.Kind) widget.Widget {
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
	b := NewButton(cfg, font, Icon(icon), sampleLabel(cfg, item.Module, hasIcon))
	b.Configure(ModuleTitle(name, cfg), ModuleButton(name, cfg))
	wrap := ModuleBox(item)
	wrap.Append(b, false)
	return wrap
}

// SampleWorkspaces is the workspace strip under cfg as compositor on
// reads it, holding sample workspaces: the active one named and holding
// two windows, one more holding a window, and an empty last one; mango
// shows them as its tags.
func SampleWorkspaces(cfg *config.Config, font render.Font, on compositor.Kind) *WorkspaceStrip {
	c := cfg.Workspaces.View(on)
	flavor := FlavorWorkspaces
	if on == compositor.Mango {
		flavor = FlavorTags
	}
	strip := NewWorkspaceStrip(cfg, c, "", nil, WorkspaceFace(font), WorkspaceHandlers{})
	windows := []WorkspaceWindow{
		{ID: 1, Workspace: 1, HasWorkspace: true, Icon: c.AppIconsFallback},
		{ID: 2, Workspace: 1, HasWorkspace: true, Icon: c.AppIconsFallback},
		{ID: 3, Workspace: 2, HasWorkspace: true, Icon: c.AppIconsFallback},
	}
	strip.Set(WorkspaceButtons(flavor, sampleWorkspaces, windows, c, "", strip.Vertical(), false))
	return strip
}

var sampleWorkspaces = []Workspace{
	{ID: 1, Num: 1, Name: "web", HasName: true, HasOutput: true, Active: true, Focused: true, HasWindows: true},
	{ID: 2, Num: 2, HasOutput: true, HasWindows: true},
	{ID: 3, Num: 3, HasOutput: true},
}

var sampleTime = time.Date(2026, time.March, 14, 9, 41, 0, 0, time.UTC)

var sampleLabels = map[config.BarModule]string{
	config.ModuleAudio:          "64%",
	config.ModuleBackgroundApps: "2",
	config.ModuleBattery:        "82%",
	config.ModuleBrightness:     "70%",
	config.ModuleCPU:            "12%",
	config.ModuleKeyboardInput:  "us",
	config.ModuleMail:           "3",
	config.ModuleRAM:            "38%",
	config.ModuleStorage:        "54%",
	config.ModuleWeather:        "18°",
}

func sampleLabel(cfg *config.Config, m config.BarModule, hasIcon bool) string {
	switch m {
	case config.ModuleClock:
		if cfg.Clock.Format.Layout() == nil {
			return ""
		}
		return ClockLabel(cfg.Clock, sampleTime, ClockNames(i18n.Shell()), func() string { return "" })
	case config.ModuleScreenshot:
		return cfg.Screenshot.Label
	}
	if label, ok := sampleLabels[m]; ok {
		return label
	}
	if hasIcon {
		return ""
	}
	return ModuleTitle(string(m), cfg)
}
