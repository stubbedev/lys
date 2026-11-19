package barview

import (
	"time"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
)

// Preview is the bar connector's layout gives, every module a Sample;
// false when the config shows no bar there.
func Preview(cfg *config.Config, font render.Font, connector string) (*widget.Bar, bool) {
	layout, ok := LayoutFor(cfg, connector)
	if !ok {
		return nil, false
	}
	return SampleBar(cfg, font, layout), true
}

// SampleBar is a bar of layout's items, every module a Sample.
func SampleBar(cfg *config.Config, font render.Font, layout config.BarLayout) *widget.Bar {
	return NewBar(cfg, layout, "", func(row *widget.Box, item config.BarItem) {
		row.Append(Sample(cfg, font, item), false)
	})
}

// Sample is item's module as the bar draws it, holding sample state
// instead of what a service would report: the separator's rule, or the
// module's button in its module box with the icon the module starts
// with and a sample label, under the module's button config. A module
// with no icon of its own shows its title.
func Sample(cfg *config.Config, font render.Font, item config.BarItem) widget.Widget {
	if item.Module == config.ModuleSeparator {
		rule := NewSeparator(cfg)
		OwnChrome(rule, item, cfg)
		return rule
	}
	name := string(item.Module)
	icon, hasIcon := cfg.ModuleIcon(item.Module)
	b := NewButton(cfg, font, Icon(icon), sampleLabel(cfg, item.Module, hasIcon))
	b.Configure(ModuleTitle(name, cfg), ModuleButton(name, cfg))
	wrap := ModuleBox(item)
	wrap.Append(b, false)
	return wrap
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
