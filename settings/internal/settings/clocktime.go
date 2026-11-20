package settings

import (
	"fmt"
	"log"
	"slices"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/notifications/notifypolicy"
	"github.com/stubbedev/lys/desktop/strftime"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	hoursPerDay  = 24
	minuteStep   = 5
	minutesPerHr = 60
)

func clockHourFormat(cycle strftime.HourCycle) string {
	if cycle == strftime.Hour12 {
		return "%I %p"
	}
	return "%H"
}

func hourLabels(k *kit) []string {
	labels := make([]string, hoursPerDay)
	format := clockHourFormat(strftime.Hour24)
	if l, err := clockLayout(k.store); err == nil {
		format = clockHourFormat(l.HourCycle())
	} else {
		log.Printf("settings: the hours as the clock shows them, showing 24-hour: %v", err)
	}
	names := sessionTimeLocale()
	for h := range hoursPerDay {
		at := time.Date(2000, time.January, 1, h, 0, 0, 0, time.UTC)
		text, err := names.Format(format, at)
		if err != nil {
			log.Printf("settings: hour %d as the clock shows it, showing it as digits: %v", h, err)
			text = fmt.Sprintf("%02d", h)
		}
		labels[h] = text
	}
	return labels
}

func minuteChoices(current int) []int {
	out := make([]int, 0, minutesPerHr/minuteStep+1)
	for m := 0; m < minutesPerHr; m += minuteStep {
		out = append(out, m)
	}
	if !slices.Contains(out, current) {
		out = append(out, current)
		slices.Sort(out)
	}
	return out
}

type clockTimeEditor struct {
	*widget.Box
	slot       slot
	avoid      func() (notifypolicy.ClockTime, bool)
	hourLabels []string
	hour       *widget.Dropdown
	minute     *widget.Dropdown
	hours      []int
	minutes    []int
	problem    *widget.Label
}

func newClockTimeEditor(k *kit, s slot, name string, avoid func() (notifypolicy.ClockTime, bool)) *clockTimeEditor {
	t := locale.T()
	e := &clockTimeEditor{Box: gapBox(widget.Column), slot: s, avoid: avoid, hourLabels: hourLabels(k)}
	e.hour = widget.NewDropdown(k.face, k.textPx(), nil, -1)
	e.hour.SetAccessibleName(t.Get("settings-clock-time-hour", i18n.Str("name", name)))
	e.minute = widget.NewDropdown(k.face, k.textPx(), nil, -1)
	e.minute.SetAccessibleName(t.Get("settings-clock-time-minute", i18n.Str("name", name)))
	e.hour.OnSelect = func(int) { e.write() }
	e.minute.OnSelect = func(int) { e.write() }
	e.problem = wrapped(k.label("", "setting-problem", css.Caption, css.Error))
	row := gapBox(widget.Row)
	row.AppendAligned(e.hour, false, widget.AlignCenter)
	row.AppendAligned(k.label(":", css.DimLabel), false, widget.AlignCenter)
	row.AppendAligned(e.minute, false, widget.AlignCenter)
	e.Append(row, false)
	e.Append(e.problem, false)
	e.refresh()
	return e
}

func anyClockTime() (notifypolicy.ClockTime, bool) { return notifypolicy.ClockTime{}, false }

func clockTimeOf(s slot) func() (notifypolicy.ClockTime, bool) {
	return func() (notifypolicy.ClockTime, bool) {
		raw, _ := s.get().(string)
		at, err := notifypolicy.ParseClockTime(raw)
		return at, err == nil
	}
}

func (e *clockTimeEditor) value() (notifypolicy.ClockTime, error) {
	raw, ok := e.slot.get().(string)
	if !ok {
		return notifypolicy.ClockTime{}, fmt.Errorf("read the time of day: %v is not HH:MM", e.slot.get())
	}
	return notifypolicy.ParseClockTime(raw)
}

func (e *clockTimeEditor) write() {
	h, mi := e.hour.Selected(), e.minute.Selected()
	if _, err := e.value(); err != nil || h < 0 || h >= len(e.hours) || mi < 0 || mi >= len(e.minutes) {
		return
	}
	at, err := notifypolicy.ClockTimeAt(e.hours[h], e.minutes[mi])
	if err == nil {
		err = e.slot.set(at.String())
	}
	e.refresh()
	e.show(err)
}

func (e *clockTimeEditor) show(err error) {
	e.problem.SetVisible(err != nil)
	if err != nil {
		e.problem.SetText(err.Error())
	}
}

func (e *clockTimeEditor) refresh() {
	at, err := e.value()
	if err != nil {
		e.show(err)
		e.hour.SetEnabled(false)
		e.minute.SetEnabled(false)
		return
	}
	e.show(nil)
	e.hour.SetEnabled(true)
	e.minute.SetEnabled(true)
	avoided, avoiding := e.avoid()
	clashes := func(h, m int) bool { return avoiding && h == avoided.Hour() && m == avoided.Minute() }
	e.hours = e.hours[:0]
	labels := make([]string, 0, hoursPerDay)
	for h := range hoursPerDay {
		if h == at.Hour() || !clashes(h, at.Minute()) {
			e.hours = append(e.hours, h)
			labels = append(labels, e.hourLabels[h])
		}
	}
	e.minutes = slices.DeleteFunc(minuteChoices(at.Minute()), func(m int) bool { return m != at.Minute() && clashes(at.Hour(), m) })
	setChoices(e.hour, labels, slices.Index(e.hours, at.Hour()))
	setChoices(e.minute, minuteLabels(e.minutes), slices.Index(e.minutes, at.Minute()))
}

func minuteLabels(minutes []int) []string {
	labels := make([]string, len(minutes))
	for i, m := range minutes {
		labels[i] = fmt.Sprintf("%02d", m)
	}
	return labels
}
