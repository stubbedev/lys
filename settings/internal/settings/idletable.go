package settings

import (
	"fmt"
	"slices"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/idle/idlepolicy"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const idleNever = "never"

type idleStage string

const (
	idleKeyboard  idleStage = "keyboard-backlight"
	idleDim       idleStage = "dim"
	idleLock      idleStage = "lock"
	idleScreenOff idleStage = "screen-off"
	idleSuspend   idleStage = "suspend"
)

var idleStages = []idleStage{idleKeyboard, idleDim, idleLock, idleScreenOff, idleSuspend}

type idleSource string

const (
	idleOnAC      idleSource = "ac"
	idleOnBattery idleSource = "battery"
)

var idleSources = []idleSource{idleOnAC, idleOnBattery}

func (s idleSource) icon() string {
	if s == idleOnBattery {
		return "ld-battery-symbolic"
	}
	return "ld-plug-zap-symbolic"
}

func (s idleSource) name() string { return locale.T().Get("settings-idle-on-" + string(s)) }

func (s idleStage) path(source idleSource) string { return "idle." + string(s) + "." + string(source) }

func (s idleStage) name() string { return locale.T().Get("settings-section-idle-" + string(s)) }

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

var idleChoices = []durationChoice{
	after(time.Minute), after(2 * time.Minute), after(5 * time.Minute), after(10 * time.Minute),
	after(15 * time.Minute), after(30 * time.Minute), after(time.Hour), zeroAs(never),
}

func idleMinutes(s slot) slot {
	return slot{
		get: func() any {
			raw, ok := s.get().(string)
			if !ok {
				return nil
			}
			t, err := parseIdleTime(raw)
			if err != nil {
				return nil
			}
			if t.never {
				return float64(0)
			}
			return t.after.Minutes()
		},
		set: func(v any) error {
			m, ok := v.(float64)
			if !ok {
				return fmt.Errorf("an idle time of %T, not minutes", v)
			}
			if m == 0 {
				return s.set(idleNever)
			}
			return s.set(idlepolicy.FormatDuration(time.Duration(m * float64(time.Minute)).Round(time.Second)))
		},
		unset: s.unset,
	}
}

func newIdleChips(k *kit, s slot) *presetPicker[float64] {
	presets := make([]preset[float64], len(idleChoices))
	for i, c := range idleChoices {
		presets[i] = preset[float64]{value: c.span.Minutes(), label: c.label}
	}
	least := idlepolicy.MinTimeout.Minutes()
	bounds := config.FieldMeta{Min: least, Max: idlepolicy.MaxTimeout.Minutes()}
	unit := customUnit{per: 1, digits: 1, unit: unitMinutes, least: least}
	return newPresetPicker(k, idleMinutes(s), bounds, unit, presets)
}

type idleCell struct {
	*widget.Box
	k       *kit
	stage   idleStage
	source  idleSource
	chips   *presetPicker[float64]
	warning *widget.Label
}

func newIdleCell(k *kit, stage idleStage, source idleSource) *idleCell {
	c := &idleCell{Box: gapBox(widget.Column), k: k, stage: stage, source: source}
	c.chips = newIdleChips(k, k.store.slot(stage.path(source)))
	c.chips.SetAccessibleName(locale.T().Get("settings-idle-cell", i18n.Str("stage", stage.name()), i18n.Str("source", source.name())))
	c.warning = wrapped(k.label("", css.Caption, css.Warning))
	c.Append(c.chips, false)
	c.Append(c.warning, false)
	c.refresh()
	return c
}

func (c *idleCell) refresh() {
	c.chips.refresh()
	times, err := idleTimesOn(c.k.store, c.source)
	if err != nil {
		c.warning.SetText(err.Error())
		c.warning.SetVisible(true)
		return
	}
	then, overtaken := idleOvertaken(times, c.stage)
	c.warning.SetVisible(overtaken)
	if overtaken {
		c.warning.SetText(locale.T().Get("settings-idle-overtaken", i18n.Str("source", string(c.source)), i18n.Str("then", string(then))))
	}
}

type idleTable struct {
	*widget.Grid
	cells   []*idleCell
	battery []widget.Widget
}

func newIdleTable(k *kit) *idleTable {
	g := &idleTable{Grid: widget.NewGrid(choiceCardGap*2, choiceCardGap)}
	g.AddClass("cell-grid")
	for c, source := range idleSources {
		head := gapBox(widget.Row)
		head.AppendAligned(k.icon(source.icon()), false, widget.AlignCenter)
		head.AppendAligned(k.label(source.name(), css.Caption, css.DimLabel), false, widget.AlignCenter)
		g.Attach(head, c+1, 0, 1, 1)
		if source == idleOnBattery {
			g.battery = append(g.battery, head)
		}
	}
	for r, stage := range idleStages {
		g.Attach(k.label(stage.name()), 0, r+1, 1, 1)
		for c, source := range idleSources {
			cell := newIdleCell(k, stage, source)
			g.Attach(cell, c+1, r+1, 1, 1)
			g.cells = append(g.cells, cell)
			if source == idleOnBattery {
				g.battery = append(g.battery, cell)
			}
		}
	}
	k.whenBatteryKnown(g.showBattery)
	return g
}

func (g *idleTable) showBattery(present bool) {
	if present {
		return
	}
	for _, w := range g.battery {
		g.Remove(w)
	}
	g.battery = nil
	g.cells = slices.DeleteFunc(g.cells, func(c *idleCell) bool { return c.source == idleOnBattery })
}

func (g *idleTable) refresh() {
	for _, c := range g.cells {
		c.refresh()
	}
}

func idleTimesRow() rowSpec {
	paths := make([]string, 0, len(idleStages)*len(idleSources))
	for _, stage := range idleStages {
		for _, source := range idleSources {
			paths = append(paths, stage.path(source))
		}
	}
	return field(paths[0], withKey("settings-idle-times"), idleEnabled, func(r *rowSpec) {
		r.fullWidth = true
		r.also = paths[1:]
		r.editor = func(k *kit, _ slot, _ config.FieldMeta) control { return newIdleTable(k) }
	})
}
