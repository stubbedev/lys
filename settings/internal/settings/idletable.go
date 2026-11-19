package settings

import (
	"fmt"
	"log"
	"slices"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const idleNever = "never"

var idlePresets = []string{"1m", "2m", "3m", "5m", "10m", "15m", "30m", "1h", idleNever}

type idleStage string

const (
	idleDim       idleStage = "dim"
	idleLock      idleStage = "lock"
	idleScreenOff idleStage = "screen-off"
	idleSuspend   idleStage = "suspend"
)

var idleStages = []idleStage{idleDim, idleLock, idleScreenOff, idleSuspend}

type idleSource string

const (
	idleOnAC      idleSource = "ac"
	idleOnBattery idleSource = "battery"
)

var idleSources = []idleSource{idleOnAC, idleOnBattery}

func (s idleStage) path(source idleSource) string { return "idle." + string(s) + "." + string(source) }

var idlePrecedes = []struct{ first, then idleStage }{
	{idleDim, idleScreenOff},
	{idleDim, idleSuspend},
	{idleLock, idleSuspend},
	{idleScreenOff, idleSuspend},
}

type idleTime struct {
	after time.Duration
	never bool
}

func parseIdleTime(value string) (idleTime, error) {
	if value == idleNever {
		return idleTime{never: true}, nil
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return idleTime{}, fmt.Errorf("read the idle time %q: %w", value, err)
	}
	return idleTime{after: d}, nil
}

func idleOvertaken(times map[idleStage]idleTime, stage idleStage) (idleStage, bool) {
	own := times[stage]
	if own.never {
		return "", false
	}
	for _, p := range idlePrecedes {
		then := times[p.then]
		if p.first == stage && !then.never && own.after >= then.after {
			return p.then, true
		}
	}
	return "", false
}

func idleTimesOn(s store, source idleSource) (map[idleStage]idleTime, error) {
	times := make(map[idleStage]idleTime, len(idleStages))
	for _, stage := range idleStages {
		path := stage.path(source)
		raw, ok := s.value(path).(string)
		if !ok {
			return nil, fmt.Errorf("read %s: not a duration", path)
		}
		t, err := parseIdleTime(raw)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		times[stage] = t
	}
	return times, nil
}

func durationLabel(value string) string {
	if value == idleNever {
		return locale.T().Get("settings-duration-never")
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return value
	}
	return durationWords(d)
}

func durationWords(d time.Duration) string {
	t := locale.T()
	switch {
	case d >= time.Hour && d%time.Hour == 0:
		return t.Get("settings-duration-hours", i18n.Int("n", int64(d/time.Hour)))
	case d >= time.Minute && d%time.Minute == 0:
		return t.Get("settings-duration-minutes", i18n.Int("n", int64(d/time.Minute)))
	}
	return t.Get("settings-duration-seconds", i18n.Int("n", int64(d/time.Second)))
}

func durationChoices(current string) []string {
	if current == "" || slices.Contains(idlePresets, current) {
		return idlePresets
	}
	return append(slices.Clone(idlePresets), current)
}

type durationMenu struct {
	*widget.Dropdown
	slot   slot
	values []string
}

func newDurationMenu(k *kit, s slot, name string) *durationMenu {
	m := &durationMenu{Dropdown: widget.NewDropdown(k.face, k.textPx(), nil, 0), slot: s}
	m.SetAccessibleName(name)
	m.OnSelect = func(i int) {
		if i < 0 || i >= len(m.values) {
			return
		}
		if err := m.slot.set(m.values[i]); err != nil {
			log.Printf("settings: %s: %v", name, err)
		}
		m.refresh()
	}
	m.refresh()
	return m
}

func (m *durationMenu) refresh() {
	current, _ := m.slot.get().(string)
	m.values = durationChoices(current)
	labels := make([]string, len(m.values))
	for i, v := range m.values {
		labels[i] = durationLabel(v)
	}
	setChoices(m.Dropdown, labels, slices.Index(m.values, current))
}

type idlePair struct {
	*widget.Box
	k            *kit
	stage        idleStage
	ac, battery  *durationMenu
	batteryIcon  *widget.Icon
	warnings     map[idleSource]*widget.Label
	batteryShown bool
}

func (p *idlePair) refresh() {
	p.ac.refresh()
	p.battery.refresh()
	for _, source := range idleSources {
		p.warn(source)
	}
}

func (p *idlePair) warn(source idleSource) {
	w := p.warnings[source]
	times, err := idleTimesOn(p.k.store, source)
	if err != nil {
		w.SetText(err.Error())
		w.SetVisible(true)
		return
	}
	then, overtaken := idleOvertaken(times, p.stage)
	w.SetVisible(overtaken && (source == idleOnAC || p.batteryShown))
	if overtaken {
		w.SetText(locale.T().Get("settings-idle-overtaken", i18n.Str("source", string(source)), i18n.Str("then", string(then))))
	}
}

func (p *idlePair) showBattery(present bool) {
	p.batteryShown = present
	p.batteryIcon.SetVisible(present)
	p.battery.SetVisible(present)
	p.warn(idleOnBattery)
}

func idleTimeoutsRow(stage idleStage) rowSpec {
	ac, battery := stage.path(idleOnAC), stage.path(idleOnBattery)
	return field(ac, withKey("settings-section-idle-"+string(stage)), idleEnabled, func(r *rowSpec) {
		r.also = []string{battery}
		r.editor = func(k *kit, s slot, _ config.FieldMeta) control {
			t := locale.T()
			p := &idlePair{Box: gapBox(widget.Column), k: k, stage: stage, batteryShown: true, warnings: make(map[idleSource]*widget.Label, len(idleSources))}
			p.ac = newDurationMenu(k, s, t.Get("settings-idle-on-ac"))
			p.battery = newDurationMenu(k, k.store.slot(battery), t.Get("settings-idle-on-battery"))
			p.batteryIcon = k.icon("ld-battery-symbolic")
			menus := gapBox(widget.Row)
			menus.AppendAligned(k.icon("ld-plug-zap-symbolic"), false, widget.AlignCenter)
			menus.AppendAligned(p.ac, false, widget.AlignCenter)
			menus.AppendAligned(p.batteryIcon, false, widget.AlignCenter)
			menus.AppendAligned(p.battery, false, widget.AlignCenter)
			p.AppendAligned(menus, false, widget.AlignEnd)
			for _, source := range idleSources {
				w := wrapped(k.label("", "setting-problem", css.Caption, css.Warning))
				w.SetVisible(false)
				p.warnings[source] = w
				p.AppendAligned(w, false, widget.AlignEnd)
			}
			p.refresh()
			k.whenBatteryKnown(p.showBattery)
			return p
		}
	})
}
