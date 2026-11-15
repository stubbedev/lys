package bar

import (
	"errors"
	"math"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/upower"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/internal/jinja"
)

// batteryLabel renders the format template: the only variable its
// context carries is "percent" (rounded), and the braces are
// whitespace-insensitive. No battery maps to the unavailable label.
func batteryLabel(format string, percentage float64, present bool) string {
	if !present {
		return i18n.T("bar-battery-unavailable")
	}
	return jinja.RenderOr(format, batteryVars(percentage))
}

func batteryVars(percentage float64) map[string]any {
	return map[string]any{"percent": int64(math.Round(percentage))}
}

// battery is the module: a label fed from the UPower device snapshot,
// refreshed on PropertiesChanged.
type battery struct {
	ctx    ModuleContext
	source upower.Source
	btn    *barButton
	icon   *widget.Icon
	firstRead
}

func newBattery(ctx ModuleContext) (Module, error) {
	if ctx.App == nil {
		// Headless construction has no loop to subscribe on.
		return nil, errors.New("battery: requires the application loop")
	}
	if ctx.Battery == nil {
		return nil, errors.New("battery: no battery source available")
	}
	m := &battery{ctx: ctx, source: ctx.Battery}
	m.icon = moduleIcon(ctx, ctx.Config.Battery.Icon())
	m.btn = newBarButton(ctx, m.icon, "")
	followReads(ctx, "battery", &m.firstRead, m.source.Subscribe, m.source.Read, m.apply)
	return m, nil
}

// apply updates the label and state icon for the device.
func (m *battery) apply(dev upower.Device) {
	cfg := m.ctx.Config.Battery
	label := batteryLabel(cfg.Format, dev.Percentage, dev.IsPresent)
	m.btn.applyThresholds(dev.Percentage, cfg.Thresholds)
	m.btn.SetLabel(label)
	m.setIcon(cfg, dev)
}

// setIcon picks the icon: alert when absent
// or unknown, charging when charging, else the level list bucketed by
// percentage, a battery held at its charge limit among them.
func (m *battery) setIcon(cfg config.BatteryConfig, dev upower.Device) {
	setter := m.icon
	if setter == nil {
		return
	}
	var name string
	switch {
	case !dev.IsPresent || dev.State == upower.StateUnknown:
		name = cfg.AlertIcon
	case batteryCharging(dev.State):
		name = cfg.ChargingIcon
	case len(cfg.LevelIcons) == 0:
		name = cfg.AlertIcon
	default:
		name = cfg.LevelIcons[levelIndexFloor(dev.Percentage, len(cfg.LevelIcons))]
	}
	setter.SetThemeName(name)
}

func (m *battery) Root() widget.Widget { return m.btn }
