package settings

import (
	"fmt"
	"math"
	"slices"
	"strconv"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	day        = 24 * time.Hour
	chipRadius = 999
)

func durationWords(d time.Duration) string {
	t := locale.T()
	switch {
	case d >= day && d%day == 0:
		return t.Get("settings-duration-days", i18n.Int("n", int64(d/day)))
	case d >= time.Hour && d%time.Hour == 0:
		return t.Get("settings-duration-hours", i18n.Int("n", int64(d/time.Hour)))
	case d >= time.Minute && d%time.Minute == 0:
		return t.Get("settings-duration-minutes", i18n.Int("n", int64(d/time.Minute)))
	case d%time.Second == 0:
		return t.Get("settings-duration-seconds", i18n.Int("n", int64(d/time.Second)))
	}
	return t.Get("settings-duration-seconds", i18n.Float("n", d.Seconds()))
}

type customUnit struct {
	per    float64
	digits int
	name   string
}

func (u customUnit) scaled(s slot) slot {
	return slot{
		get: func() any {
			v, ok := s.get().(int64)
			if !ok {
				return nil
			}
			return float64(v) / u.per
		},
		set: func(v any) error {
			f, ok := v.(float64)
			if !ok {
				return fmt.Errorf("a custom value of %T, not a number", v)
			}
			return s.set(int64(math.Round(f * u.per)))
		},
		unset: s.unset,
	}
}

type timeUnit int

const (
	inMilliseconds timeUnit = iota
	inMinutes
	inDays
)

func (u timeUnit) span() time.Duration {
	switch u {
	case inMilliseconds:
		return time.Millisecond
	case inMinutes:
		return time.Minute
	case inDays:
		return day
	}
	panic(fmt.Sprintf("settings: time unit %d", u))
}

func (u timeUnit) custom() customUnit {
	switch u {
	case inMilliseconds:
		return customUnit{per: float64(time.Second / time.Millisecond), digits: 1, name: "settings-duration-unit-seconds"}
	case inMinutes:
		return customUnit{per: 1, name: "settings-duration-unit-minutes"}
	case inDays:
		return customUnit{per: 1, name: "settings-duration-unit-days"}
	}
	panic(fmt.Sprintf("settings: time unit %d", u))
}

func (u timeUnit) count(d time.Duration) int64 {
	if d%u.span() != 0 {
		panic(fmt.Sprintf("settings: a %v chip is not a whole number of %v", d, u.span()))
	}
	return int64(d / u.span())
}

type countUnit int

const (
	inPixels countUnit = iota
	inEntries
)

func (u countUnit) custom() customUnit {
	switch u {
	case inPixels:
		return customUnit{per: 1, name: "settings-unit-pixels"}
	case inEntries:
		return customUnit{per: 1, name: "settings-unit-entries"}
	}
	panic(fmt.Sprintf("settings: count unit %d", u))
}

type zeroWord string

const (
	never       zeroWord = "settings-duration-never"
	immediately zeroWord = "settings-duration-immediately"
)

type durationChoice struct {
	span time.Duration
	zero zeroWord
}

func after(d time.Duration) durationChoice {
	if d <= 0 {
		panic(fmt.Sprintf("settings: a %v chip, which zeroAs names", d))
	}
	return durationChoice{span: d}
}

func zeroAs(word zeroWord) durationChoice { return durationChoice{zero: word} }

func (c durationChoice) label() string {
	if c.zero != "" {
		return locale.T().Get(string(c.zero))
	}
	return durationWords(c.span)
}

type preset struct {
	value int64
	label func() string
}

func durationChips(u timeUnit, choices ...durationChoice) rowOpt {
	presets := make([]preset, len(choices))
	for i, c := range choices {
		presets[i] = preset{value: u.count(c.span), label: c.label}
	}
	return presetChips(u.custom(), presets)
}

func countChips(u countUnit, values ...int64) rowOpt {
	presets := make([]preset, len(values))
	for i, v := range values {
		presets[i] = preset{value: v, label: func() string { return strconv.FormatInt(v, 10) }}
	}
	return presetChips(u.custom(), presets)
}

func presetChips(unit customUnit, presets []preset) rowOpt {
	return func(r *rowSpec) {
		r.fullWidth = true
		r.editor = func(k *kit, s slot, meta config.FieldMeta) control { return newPresetPicker(k, s, meta, unit, presets) }
	}
}

type presetPicker struct {
	*widget.Box
	slot     slot
	presets  []preset
	chips    []*widget.ToggleButton
	custom   *number
	typed    *widget.Box
	choosing bool
	syncing  bool
}

func newPresetPicker(k *kit, s slot, meta config.FieldMeta, unit customUnit, presets []preset) *presetPicker {
	t := locale.T()
	p := &presetPicker{Box: gapBox(widget.Column), slot: s, presets: presets}
	row := widget.NewWrapBox(choiceCardGap, choiceCardGap, widget.JustifyStart)
	for i := range len(presets) + 1 {
		label := t.Get("settings-duration-custom")
		if i < len(presets) {
			label = presets[i].label()
		}
		chip := widget.NewToggleButton(k.label(label), widget.Current().Padding, chipRadius)
		chip.AddClass(css.Small)
		if i > 0 {
			chip.SetGroup(p.chips[0])
		}
		chip.OnToggled = func(on bool) {
			if on {
				p.pick(i)
			}
		}
		p.chips = append(p.chips, chip)
		row.Append(chip)
	}
	p.custom = newNumber(k, unit.scaled(s), meta.Min/unit.per, meta.Max/unit.per, 1, unit.digits)
	p.custom.SetAccessibleName(t.Get("settings-duration-custom"))
	p.typed = gapBox(widget.Row)
	p.typed.AppendAligned(p.custom, false, widget.AlignCenter)
	p.typed.AppendAligned(k.label(t.Get(unit.name), css.DimLabel), false, widget.AlignCenter)
	p.Append(row, false)
	p.AppendAligned(p.typed, false, widget.AlignStart)
	p.refresh()
	return p
}

func (p *presetPicker) pick(i int) {
	if p.syncing {
		return
	}
	p.choosing = i == len(p.presets)
	if !p.choosing {
		_ = p.slot.set(p.presets[i].value)
	}
	p.refresh()
}

func (p *presetPicker) refresh() {
	at := -1
	if v, ok := p.slot.get().(int64); ok {
		at = slices.IndexFunc(p.presets, func(pr preset) bool { return pr.value == v })
	}
	custom := p.choosing || at < 0
	if custom {
		at = len(p.presets)
	}
	p.syncing = true
	p.chips[at].SetActive(true)
	p.syncing = false
	p.typed.SetVisible(custom)
	p.custom.refresh()
}

func (p *presetPicker) active() int {
	return slices.IndexFunc(p.chips, (*widget.ToggleButton).Active)
}
