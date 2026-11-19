package bar

import (
	"context"
	"errors"
	"log"
	"math"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/barview"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/internal/jinja"
	"github.com/stubbedev/lys/shell/service/brightness"
)

// brightnessNoDevice is NO_DEVICE_LABEL.
const brightnessNoDevice = "--%"

// brightnessAutoIcon stands in for the level icon while automatic
// brightness follows the room's light.
const brightnessAutoIcon = "ld-sun-moon-symbolic"

// brightnessLabel is helpers.rs's format_label: the one variable is
// the rounded percent.
func brightnessLabel(format string, percent float64) string {
	return jinja.RenderOr(format, percentVars(percent))
}

func percentVars(percent float64) map[string]any {
	return map[string]any{"percent": int64(math.Round(percent))}
}

// brightnessIcon is select_icon: level-icons split 0-100% into equal
// bands, the last one closing at 100%; no icons is "".
func brightnessIcon(levels []string, percent float64) string {
	if len(levels) == 0 {
		return ""
	}
	i := int(math.Floor(percent / 100 * float64(len(levels))))
	return levels[max(0, min(i, len(levels)-1))]
}

// autoBrightnessSource is lys-power's automatic brightness
// (*brightness.Auto).
type autoBrightnessSource interface {
	Control() brightness.Control
	Subscribe() (<-chan struct{}, func())
	Resume(ctx context.Context) error
}

// startAutoBrightness follows automatic brightness on the shell's
// session bus for every bar, leaving ctx.AutoBrightness nil without a
// bus; the returned func stops following.
func startAutoBrightness(ctx *ModuleContext, conn *dbus.Conn) func() {
	if conn == nil {
		return func() {}
	}
	auto, err := brightness.FollowAuto(conn, func(err error) { log.Printf("brightness: %v", err) })
	if err != nil {
		log.Printf("brightness: %v", err)
		return func() {}
	}
	ctx.AutoBrightness = auto
	return func() {
		if err := auto.Close(); err != nil {
			log.Printf("brightness: %v", err)
		}
	}
}

// followControl applies who sets the backlight now, then each change
// of it until life ends; without automatic brightness it is
// ControlManual for good.
func followControl(mc ModuleContext, life context.Context, auto autoBrightnessSource, apply func(brightness.Control)) {
	if auto == nil {
		apply(brightness.ControlManual)
		return
	}
	ticks, stop := auto.Subscribe()
	apply(auto.Control())
	mc.Go(func() {
		defer stop()
		for {
			select {
			case <-life.Done():
				return
			case _, ok := <-ticks:
				if !ok {
					return
				}
				control := auto.Control()
				mc.Invoke(func() {
					if life.Err() == nil {
						apply(control)
					}
				})
			}
		}
	})
}

// brightnessModule is the bar button: the mean percentage across
// backlights with its level icon, refreshed on sysfs changes, and
// automatic brightness's icon while it follows the room's light.
type brightnessModule struct {
	ctx     ModuleContext
	source  brightness.Source
	btn     *barButton
	icon    *widget.Icon
	devices []brightness.Device
	control brightness.Control
	firstRead
}

func newBrightness(ctx ModuleContext) (Module, error) {
	if ctx.Brightness == nil {
		return nil, errors.New("brightness: no backlight source available")
	}
	cfg := ctx.Config.Brightness
	m := &brightnessModule{ctx: ctx, source: ctx.Brightness}
	m.icon = barview.Icon(config.IconWith(cfg.IconShow, brightnessIcon(cfg.LevelIcons, 0), cfg.IconColor))
	m.btn = newBarButton(ctx, m.icon, "")
	followControl(ctx, ctx.Life(), ctx.AutoBrightness, m.applyControl)
	followReads(ctx, "brightness", &m.firstRead, infallibleStop(ctx.Brightness.Subscribe), m.source.Devices, m.apply)
	return m, nil
}

func (m *brightnessModule) apply(devices []brightness.Device) {
	m.devices = devices
	m.show()
}

func (m *brightnessModule) applyControl(control brightness.Control) {
	m.control = control
	m.show()
}

// show is refresh_display: the label and icon for the mean level and
// its threshold colors, or "--%" and the first icon without a device;
// the automatic icon while automatic brightness follows, and who sets
// the backlight in the description.
func (m *brightnessModule) show() {
	cfg := m.ctx.Config.Brightness
	percent, ok := brightness.AveragePercentage(m.devices)
	label := brightnessNoDevice
	if ok {
		label = brightnessLabel(cfg.Format, percent)
	}
	m.btn.SetLabel(label)
	m.btn.SetDescription(brightnessDescription(m.control, label))
	icon := brightnessIcon(cfg.LevelIcons, percent)
	if m.control == brightness.ControlFollowing {
		icon = brightnessAutoIcon
	}
	m.setIcon(icon)
	if ok {
		m.btn.ApplyThresholds(percent, cfg.Thresholds)
	}
}

// brightnessDescription is the module's state in words: the label
// alone while the user sets the backlight, and whether automatic
// brightness follows or is paused otherwise.
func brightnessDescription(control brightness.Control, label string) string {
	switch control {
	case brightness.ControlFollowing:
		return i18n.T("bar-brightness-following", i18n.Str("level", label))
	case brightness.ControlPaused:
		return i18n.T("bar-brightness-paused", i18n.Str("level", label))
	}
	return ""
}

func (m *brightnessModule) setIcon(name string) {
	if m.icon != nil && name != "" {
		m.icon.SetThemeName(name)
	}
}

func (m *brightnessModule) Root() widget.Widget { return m.btn }
