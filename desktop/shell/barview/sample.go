package barview

import (
	"log"
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
	placeholders := config.Placeholders(formatPath(m))
	if len(placeholders) == 0 {
		log.Panicf("barview: %s declares no placeholders", formatPath(m))
	}
	vars := make(map[string]string, len(placeholders))
	for _, p := range placeholders {
		vars[p.Name] = p.Sample
	}
	if m == config.ModuleMail {
		return jinja.ReplaceVars(format, vars)
	}
	return jinja.RenderOr(format, vars)
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
	case config.ModuleBackgroundApps:
		return sampleBackgroundApps
	}
	if format, ok := moduleFormats[m]; ok {
		return sampleFormatLabel(m, format(cfg))
	}
	if hasIcon {
		return ""
	}
	return ModuleTitle(string(m), cfg)
}
