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

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	day            = 24 * time.Hour
	shortestPeriod = 250 * time.Millisecond
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
	unit   unit
	least  float64
}

type presetValue interface{ int64 | float64 }

func scaledSlot[T presetValue](u customUnit, s slot) slot {
	return slot{
		get: func() any {
			v, ok := s.get().(T)
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
			return s.set(storedAs[T](f * u.per))
		},
		unset: s.unset,
	}
}

func storedAs[T presetValue](f float64) T {
	var zero T
	if _, whole := any(zero).(int64); whole {
		return T(math.Round(f))
	}
	return T(f)
}

type timeUnit int

const (
	inMilliseconds timeUnit = iota
	inSeconds
	inMinutes
	inDays
)

func (u timeUnit) span() time.Duration {
	switch u {
	case inMilliseconds:
		return time.Millisecond
	case inSeconds:
		return time.Second
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
		return customUnit{per: float64(time.Second / time.Millisecond), digits: 2, unit: unitSeconds, least: shortestPeriod.Seconds()}
	case inSeconds:
		return customUnit{per: 1, unit: unitSeconds, least: 1}
	case inMinutes:
		return customUnit{per: 1, unit: unitMinutes, least: 1}
	case inDays:
		return customUnit{per: 1, unit: unitDays, least: 1}
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
	inFrames
	inVideoLines
)

func (u countUnit) words(n int64) string {
	switch u {
	case inFrames:
		return locale.T().Get("settings-framerate", i18n.Int("n", n))
	case inVideoLines:
		return locale.T().Get("settings-video-height", i18n.Int("n", n))
	}
	return compactCount(locale.T(), n)
}

// compactCount is n as tr's language writes a round count short: a whole
// number of millions or thousands abbreviated, any other count in digits.
func compactCount(tr *i18n.Loader, n int64) string {
	switch {
	case n >= 1_000_000 && n%1_000_000 == 0:
		return tr.Get("settings-count-millions", i18n.Int("n", n/1_000_000))
	case n >= 1_000 && n%1_000 == 0:
		return tr.Get("settings-count-thousands", i18n.Int("n", n/1_000))
	default:
		return strconv.FormatInt(n, 10)
	}
}

func (u countUnit) custom() customUnit {
	switch u {
	case inPixels, inVideoLines:
		return customUnit{per: 1, unit: unitPixels, least: 1}
	case inEntries:
		return customUnit{per: 1, unit: unitEntries, least: 1}
	case inFrames:
		return customUnit{per: 1, unit: unitFrames, least: 1}
	}
	panic(fmt.Sprintf("settings: count unit %d", u))
}

type zeroWord string

const (
	never       zeroWord = "settings-duration-never"
	immediately zeroWord = "settings-duration-immediately"
	off         zeroWord = "settings-duration-off"
	instant     zeroWord = "settings-duration-instant"
	untilOff    zeroWord = "settings-duration-until-off"
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

type preset[T presetValue] struct {
	value T
	label func() string
}

func durationChips(u timeUnit, choices ...durationChoice) rowOpt {
	presets := make([]preset[int64], len(choices))
	for i, c := range choices {
		presets[i] = preset[int64]{value: u.count(c.span), label: c.label}
	}
	return presetChips(u.custom(), presets)
}

func countChips(u countUnit, values ...int64) rowOpt {
	presets := make([]preset[int64], len(values))
	for i, v := range values {
		presets[i] = preset[int64]{value: v, label: func() string { return u.words(v) }}
	}
	return presetChips(u.custom(), presets)
}

func presetChips[T presetValue](unit customUnit, presets []preset[T]) rowOpt {
	return func(r *rowSpec) {
		r.fullWidth = true
		r.editor = func(k *kit, s slot, meta config.FieldMeta) control { return newPresetPicker(k, s, meta, unit, presets) }
	}
}

// presetPicker is a field's preset chips and a Custom… chip, whose
// number input, with the unit as its trailing addon, follows the chips
// while the value is no preset, or Custom… was chosen and the value is
// still the one it was chosen over.
type presetPicker[T presetValue] struct {
	*widget.ChoiceChips
	slot     slot
	presets  []preset[T]
	custom   *number
	typed    *widget.InputGroup
	choosing bool
	chosenAt any
}

func newPresetPicker[T presetValue](k *kit, s slot, meta config.FieldMeta, unit customUnit, presets []preset[T]) *presetPicker[T] {
	t := locale.T()
	labels := make([]string, len(presets)+1)
	for i, pr := range presets {
		labels[i] = pr.label()
	}
	labels[len(presets)] = t.Get("settings-duration-custom")
	p := &presetPicker[T]{ChoiceChips: newChipGroup(k, widget.ChooseOne, labels), slot: s, presets: presets}
	p.OnChanged = func(i int, _ bool) { p.pick(i) }
	lo := meta.Min / unit.per
	if !slices.ContainsFunc(presets, func(pr preset[T]) bool { return pr.value == 0 }) {
		lo = max(lo, unit.least)
	}
	p.custom = newNumber(k, scaledSlot[T](unit, s), lo, meta.Max/unit.per, 1, unit.digits)
	p.custom.SetAccessibleName(t.Get("settings-duration-custom"))
	p.typed = k.inUnit(p.custom, unit.unit)
	p.SetTrailing(p.typed)
	p.refresh()
	return p
}

func (p *presetPicker[T]) pick(i int) {
	p.choosing = i == len(p.presets)
	if p.choosing {
		p.chosenAt = p.slot.get()
	} else {
		_ = p.slot.set(p.presets[i].value)
	}
	p.refresh()
}

func (p *presetPicker[T]) refresh() {
	stored := p.slot.get()
	p.choosing = p.choosing && stored == p.chosenAt
	at := -1
	if v, ok := stored.(T); ok {
		at = slices.IndexFunc(p.presets, func(pr preset[T]) bool { return pr.value == v })
	}
	custom := p.choosing || at < 0
	if custom {
		at = len(p.presets)
	}
	p.SetChecked(at, true)
	p.typed.SetVisible(custom)
	p.custom.refresh()
}

func (p *presetPicker[T]) active() int {
	checked := p.Checked()
	if len(checked) != 1 {
		return -1
	}
	return checked[0]
}
