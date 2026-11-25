package settings

import (
	"slices"
	"strconv"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/daytime"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

var weekdays = []config.Weekday{
	config.WeekdayMon, config.WeekdayTue, config.WeekdayWed, config.WeekdayThu,
	config.WeekdayFri, config.WeekdaySat, config.WeekdaySun,
}

const (
	scheduleFrom = "22:00"
	scheduleTo   = "07:00"
)

// dndScheduleList is the do-not-disturb schedule, each window titled by
// its times as the clock shows them.
var dndScheduleList = cardRowFor(func(k *kit) cardSpec {
	return cardSpec{
		title: func(item map[string]any, _ int) string { return scheduleTitle(k, item) },
		fields: []cardField{
			{key: "days", label: "settings-schedule-days", editor: func(k *kit, s slot) control { return newWeekdaysEditor(k, s) }},
			scheduleTimeField("from", "to", "settings-schedule-from"),
			scheduleTimeField("to", "from", "settings-schedule-to"),
		},
		blank: func() map[string]any { return map[string]any{"from": scheduleFrom, "to": scheduleTo} },
		keep: func(m map[string]any) bool {
			from, _ := m["from"].(string)
			to, _ := m["to"].(string)
			return from != "" && to != ""
		},
		clean: func(m map[string]any) map[string]any {
			if days, _ := m["days"].([]any); len(days) == 0 {
				delete(m, "days")
			}
			return m
		},
	}
})

func scheduleTimeField(key, other, label string) cardField {
	return cardField{label: label, keys: []string{key}, item: func(k *kit, field func(key string) slot) control {
		return clockTimeApart(k, field, key, other, label)
	}}
}

func clockTimeApart(k *kit, field func(key string) slot, key, other, label string) *clockTimeEditor {
	return newClockTimeEditor(k, field(key), locale.T().Get(label), everyFiveMinutes, clockTimeOf(field(other)))
}

func clockRangeRow(r *rowSpec) {
	r.editor = func(k *kit, s slot, _ config.FieldMeta) control { return newClockRange(k, s) }
}

type clockRange struct {
	*widget.Box
	from, to *clockTimeEditor
}

func newClockRange(k *kit, s slot) *clockRange {
	field := func(key string) slot { return nestedSlot(s, []string{key}) }
	c := &clockRange{
		Box:  gapBox(widget.Row),
		from: clockTimeApart(k, field, "from", "to", "settings-schedule-from"),
		to:   clockTimeApart(k, field, "to", "from", "settings-schedule-to"),
	}
	c.AddClass("clock-range")
	t := locale.T()
	for _, part := range []struct {
		label  string
		editor *clockTimeEditor
	}{{"settings-schedule-from", c.from}, {"settings-schedule-to", c.to}} {
		c.AppendAligned(k.label(t.Get(part.label), css.DimLabel), false, widget.AlignCenter)
		c.AppendAligned(part.editor, false, widget.AlignCenter)
	}
	return c
}

func (c *clockRange) refresh() {
	c.from.refresh()
	c.to.refresh()
}

// scheduleTitle is the window's times, or that one is no time of day,
// which the card's time editor shows with the error behind it.
func scheduleTitle(k *kit, item map[string]any) string {
	rawFrom, _ := item["from"].(string)
	rawTo, _ := item["to"].(string)
	from, fromErr := daytime.Parse(rawFrom)
	to, toErr := daytime.Parse(rawTo)
	if fromErr != nil || toErr != nil {
		return locale.T().Get("settings-clock-time-unreadable")
	}
	return locale.T().Get("settings-schedule-range", i18n.Str("from", clockTimeLabel(k, from)), i18n.Str("to", clockTimeLabel(k, to)), i18n.Str("overnight", strconv.FormatBool(!to.After(from))))
}

type weekdaysEditor struct {
	*widget.ChoiceChips
	slot slot
}

func newWeekdaysEditor(k *kit, s slot) *weekdaysEditor {
	t := locale.T()
	labels := make([]string, len(weekdays))
	for i, d := range weekdays {
		labels[i] = t.Get("settings-weekday-" + string(d))
	}
	e := &weekdaysEditor{ChoiceChips: newChipGroup(k, widget.ChooseMany, labels), slot: s}
	e.AddClass("weekdays-editor")
	e.OnChanged = func(int, bool) { _ = e.slot.set(e.selected()) }
	e.refresh()
	return e
}

func (e *weekdaysEditor) selected() []any {
	out := []any{}
	for _, i := range e.Checked() {
		out = append(out, string(weekdays[i]))
	}
	return out
}

func (e *weekdaysEditor) refresh() {
	stored, _ := e.slot.get().([]any)
	markChips(e.ChoiceChips, func(i int) bool { return slices.Contains(stored, any(string(weekdays[i]))) })
}
