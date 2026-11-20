package settings

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/render"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/shell/service/brightness"
	"github.com/stubbedev/lys/shell/service/nightlight"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

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

// nightlightControl is lys-shell's night light, on or off, and a
// temperature shown for nightlight.PreviewLength.
type nightlightControl interface {
	switchControl[bool]
	preview(ctx context.Context, kelvin uint32) error
}

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

func (c *shellNightlight) preview(ctx context.Context, kelvin uint32) error {
	if err := c.svc.call(ctx, "Preview", kelvin); err != nil {
		return fmt.Errorf("preview %dK in lys-shell: %w", kelvin, err)
	}
	return nil
}

func (c *shellNightlight) close() error { return c.svc.conn.Close() }

var nightlightSwitch = switchSpec[bool]{
	open: func(k *kit) (switchControl[bool], error) {
		ctl, err := k.nightlight()
		if err != nil {
			return nil, err
		}
		return ctl, nil
	},
	on:          func(on bool) bool { return on },
	words:       func(k *kit, on bool) string { return nightlightWords(k, on, k.clock.Now()) },
	label:       "settings-nightlight-now",
	loading:     "settings-shell-asking",
	unavailable: "settings-nightlight-unavailable",
	refused:     "settings-nightlight-refused",
}

func nightlightNow(k *kit) widget.Widget { return newShellSwitch(k, nightlightSwitch) }

// nightlightWords say whether the night light is on and, under a
// schedule, until when a switch holds: a switch made now lasts until
// the schedule's next change and the schedule then keeps it to the end
// of the night.
func nightlightWords(k *kit, on bool, now time.Time) string {
	t := locale.T()
	cfg := k.store.svc.Config().Nightlight
	switch {
	case !cfg.AutoSchedule && on:
		return t.Get("settings-nightlight-is-on")
	case !cfg.AutoSchedule:
		return t.Get("settings-nightlight-is-off")
	case cfg.Schedule == config.NightlightScheduleSun && on:
		return t.Get("settings-nightlight-on-until-sunrise")
	case cfg.Schedule == config.NightlightScheduleSun:
		return t.Get("settings-nightlight-turn-on-until-sunrise")
	}
	morning := cfg.CustomHours.Hours().To()
	end := morning.On(now)
	if !end.After(now) {
		end = morning.On(now.AddDate(0, 0, 1))
	}
	at, err := timeOfDayText(sessionTimeLocale(), k.store, end, now)
	if err != nil {
		at = morning.String()
		log.Printf("settings: the night light's end as the clock shows it, showing %s: %v", at, err)
	}
	if on {
		return t.Get("settings-nightlight-on-until", i18n.Str("time", at))
	}
	return t.Get("settings-nightlight-turn-on-until", i18n.Str("time", at))
}

var kelvin = withEditor(func(k *kit, s slot, _ config.FieldMeta) control { return newKelvinEditor(k, s) })

// kelvinEditor is the night light's temperature: a slider tinted as the
// filter tints white, and a button showing the slider's temperature on
// the screen for nightlight.PreviewLength.
type kelvinEditor struct {
	*widget.Box
	k       *kit
	slider  *slider
	button  *widget.Button
	label   *widget.Label
	problem *widget.Label
}

func newKelvinEditor(k *kit, s slot) *kelvinEditor {
	t := locale.T()
	e := &kelvinEditor{Box: gapBox(widget.Column), k: k}
	e.slider = newSlider(k, s, warmestKelvin, coolestKelvin, true, func(v float64) string {
		return t.Get("settings-kelvin", i18n.Int("n", int64(math.Round(v))))
	})
	e.slider.Knob.SetTrack(kelvinTrack()...)
	e.label = k.label(t.Get("settings-nightlight-preview"))
	e.button = k.button(e.label, e.preview, css.Small)
	e.button.SetTooltip(t.Get("settings-nightlight-preview-tooltip", i18n.Int("seconds", int64(nightlight.PreviewLength/time.Second))))
	e.problem = wrapped(k.label("", "setting-problem", css.Caption, css.Error))
	e.problem.SetVisible(false)
	line := gapBox(widget.Row)
	line.Append(e.slider, true)
	line.AppendAligned(e.button, false, widget.AlignCenter)
	e.Append(line, false)
	e.Append(e.problem, false)
	return e
}

func (e *kelvinEditor) refresh() { e.slider.refresh() }

// preview asks lys-shell to show the slider's temperature, the button
// saying so until the preview ends.
func (e *kelvinEditor) preview() {
	t := locale.T()
	temp := uint32(math.Round(e.slider.Value()))
	e.previewing(true)
	e.problem.SetVisible(false)
	async(e.k, func() error {
		ctl, err := e.k.nightlight()
		if err != nil {
			return err
		}
		err = doWithin(shellCallWait, func(ctx context.Context) error { return ctl.preview(ctx, temp) })
		return errors.Join(err, ctl.close())
	}, func(err error) {
		if err != nil {
			e.previewing(false)
			e.problem.SetText(t.Get("settings-nightlight-preview-failed", i18n.Str("reason", err.Error())))
			e.problem.SetVisible(true)
			return
		}
		e.k.clock.AfterFunc(nightlight.PreviewLength, func() { e.k.invoke(func() { e.previewing(false) }) })
	})
}

func (e *kelvinEditor) previewing(on bool) {
	key := "settings-nightlight-preview"
	if on {
		key = "settings-nightlight-previewing"
	}
	e.label.SetText(locale.T().Get(key))
	e.button.SetEnabled(!on)
}

// kelvinTrack is white as the night light tints it, at even steps from
// the warmest temperature the slider offers to the coolest.
func kelvinTrack() []render.Color {
	out := make([]render.Color, kelvinStops)
	for i := range out {
		temp := warmestKelvin + (coolestKelvin-warmestKelvin)*i/(kelvinStops-1)
		setting, err := nightlight.NewSetting(temp, 100)
		if err != nil {
			panic(fmt.Sprintf("settings: the night light slider's %dK: %v", temp, err))
		}
		out[i] = tint(setting)
	}
	return out
}

func channel(scale float64) uint8 { return uint8(math.Round(scale * math.MaxUint8)) }
