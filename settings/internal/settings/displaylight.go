package settings

import (
	"context"
	"fmt"
	"math"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/render"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/shell/service/brightness"
	"github.com/stubbedev/lys/shell/service/nightlight"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// brightnessControl is lys-shell's brightness daemon: the level of each
// output a backlight or DDC/CI monitor drives, and setting one.
type brightnessControl interface {
	levels(ctx context.Context, outputs []string) (map[string]float64, error)
	set(ctx context.Context, output string, percent int32) (float64, error)
	close() error
}

type shellBrightness struct{ svc sysService }

func openBrightness(dial func() (*dbus.Conn, error)) (brightnessControl, error) {
	conn, err := dial()
	if err != nil {
		return nil, fmt.Errorf("connect to the session bus: %w", err)
	}
	return &shellBrightness{svc: sysService{conn: conn, name: brightness.ServiceName, iface: brightness.ServiceName, path: brightness.ServicePath}}, nil
}

func (c *shellBrightness) levels(ctx context.Context, outputs []string) (map[string]float64, error) {
	var out map[string]float64
	if err := c.svc.store(ctx, "Levels", &out, outputs); err != nil {
		return nil, fmt.Errorf("read the displays' brightness from lys-shell: %w", err)
	}
	return out, nil
}

func (c *shellBrightness) set(ctx context.Context, output string, percent int32) (float64, error) {
	var name string
	var level float64
	if err := c.svc.obj().CallWithContext(ctx, brightness.ServiceName+".Apply", 0, output, false, percent).Store(&name, &level); err != nil {
		return 0, fmt.Errorf("set %s's brightness to %d%% in lys-shell: %w", output, percent, err)
	}
	return level, nil
}

func (c *shellBrightness) close() error { return c.svc.conn.Close() }

// displayLight is the brightness of the displays the Displays page
// shows: read from lys-shell for every output, set for one.
type displayLight struct {
	ctl    brightnessControl
	levels map[string]float64
	err    error
	known  bool
}

type lightRead struct {
	ctl    brightnessControl
	levels map[string]float64
	err    error
}

func (p *displaysPanel) readBrightness() {
	names := make([]string, len(p.heads))
	for i, h := range p.heads {
		names[i] = h.Name
	}
	ctl := p.light.ctl
	async(p.k, func() lightRead {
		if ctl == nil {
			opened, err := p.k.brightness()
			if err != nil {
				return lightRead{err: err}
			}
			ctl = opened
		}
		levels, err := callWithin(shellCallWait, func(ctx context.Context) (map[string]float64, error) { return ctl.levels(ctx, names) })
		return lightRead{ctl: ctl, levels: levels, err: err}
	}, func(r lightRead) {
		if p.closed {
			if r.ctl != nil {
				_ = r.ctl.close()
			}
			return
		}
		p.light = displayLight{ctl: r.ctl, levels: r.levels, err: r.err, known: true}
		p.showBrightness()
	})
}

func (p *displaysPanel) setBrightness(v float64) {
	if p.light.ctl == nil || p.selected >= len(p.draft) {
		return
	}
	ctl, name, percent := p.light.ctl, p.draft[p.selected].Name, int32(math.Round(v))
	type result struct {
		level float64
		err   error
	}
	async(p.k, func() result {
		level, err := callWithin(shellCallWait, func(ctx context.Context) (float64, error) { return ctl.set(ctx, name, percent) })
		return result{level, err}
	}, func(r result) {
		if p.closed {
			return
		}
		if r.err != nil {
			p.say(r.err.Error())
			return
		}
		p.light.levels[name] = r.level
		p.showBrightness()
	})
}

// showBrightness shows the selected display's level, or why there is
// none.
func (p *displaysPanel) showBrightness() {
	t := locale.T()
	p.lightRow.SetVisible(p.light.known && len(p.draft) > 0)
	if !p.light.known || len(p.draft) == 0 {
		return
	}
	level, ok := p.light.levels[p.draft[p.selected].Name]
	p.lightSlider.SetVisible(ok)
	switch {
	case p.light.err != nil:
		p.lightNote.SetText(t.Get("settings-displays-brightness-unavailable", i18n.Str("reason", p.light.err.Error())))
	case !ok:
		p.lightNote.SetText(t.Get("settings-displays-brightness-none"))
	default:
		p.lightNote.SetText("")
		p.lightSlider.Set(level)
	}
	p.lightNote.SetVisible(p.lightNote.Text() != "")
}

func (p *displaysPanel) closeBrightness() {
	if p.light.ctl != nil {
		_ = p.light.ctl.close()
		p.light.ctl = nil
	}
}

// nightlightControl is lys-shell's night light, on or off.
type nightlightControl = switchControl[bool]

type shellNightlight struct{ svc sysService }

func openNightlight(dial func() (*dbus.Conn, error)) (nightlightControl, error) {
	conn, err := dial()
	if err != nil {
		return nil, fmt.Errorf("connect to the session bus: %w", err)
	}
	return &shellNightlight{svc: sysService{conn: conn, name: nightlight.ServiceName, iface: nightlight.ServiceName, path: nightlight.ServicePath}}, nil
}

func (c *shellNightlight) read(ctx context.Context) (bool, error) {
	p, err := c.svc.props(ctx)
	if err != nil {
		return false, fmt.Errorf("read the night light from lys-shell: %w", err)
	}
	on, ok := p[nightlight.EnabledProperty].Value().(bool)
	if !ok {
		return false, fmt.Errorf("read the night light from lys-shell: %s is %v", nightlight.EnabledProperty, p[nightlight.EnabledProperty])
	}
	return on, nil
}

func (c *shellNightlight) changes(ctx context.Context) (<-chan struct{}, error) {
	return c.svc.ticks(ctx, "the night light")
}

func (c *shellNightlight) set(ctx context.Context, on bool) error {
	return c.svc.call(ctx, "SetEnabled", on)
}

func (c *shellNightlight) close() error { return c.svc.conn.Close() }

var nightlightSwitch = switchSpec[bool]{
	open: func(k *kit) (switchControl[bool], error) { return k.nightlight() },
	on:   func(on bool) bool { return on },
	words: func(_ *kit, on bool) string {
		if on {
			return locale.T().Get("settings-nightlight-is-on")
		}
		return locale.T().Get("settings-nightlight-is-off")
	},
	label:       "settings-nightlight-now",
	loading:     "settings-shell-asking",
	unavailable: "settings-nightlight-unavailable",
	refused:     "settings-nightlight-refused",
}

func nightlightNow(k *kit) widget.Widget { return newShellSwitch(k, nightlightSwitch) }

// kelvinTrack is white as the night light tints it, at even steps from
// the warmest temperature the slider offers to the coolest.
func kelvinTrack() []render.Color {
	out := make([]render.Color, kelvinStops)
	for i := range out {
		temp := warmestKelvin + (coolestKelvin-warmestKelvin)*i/(kelvinStops-1)
		tint, err := nightlight.NewSetting(temp, 100)
		if err != nil {
			panic(fmt.Sprintf("settings: the night light slider's %dK: %v", temp, err))
		}
		m := tint.Matrix()
		out[i] = render.RGB(channel(m[0]), channel(m[4]), channel(m[8]))
	}
	return out
}

func channel(scale float64) uint8 { return uint8(math.Round(scale * math.MaxUint8)) }
