package settings

import (
	"errors"
	"fmt"
	"time"

	"github.com/stubbedev/lys/desktop/daytime"
	"github.com/stubbedev/lys/lib/glibclocale"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const clockTimeLayout = "15:04"

type localeCalendar struct {
	names *glibclocale.Time
	first time.Weekday
}

func (c localeCalendar) text(format string, at time.Time) string {
	s, err := c.names.Format(format, at)
	if err != nil {
		panic(fmt.Sprintf("settings: the calendar's %q: %v", format, err))
	}
	return s
}

func (c localeCalendar) MonthName(m time.Month) string {
	return c.text("%B", time.Date(2000, m, 1, 0, 0, 0, 0, time.UTC))
}

func (c localeCalendar) WeekdayName(w time.Weekday) string {
	return c.text("%a", time.Date(2000, time.January, 2+int(w), 0, 0, 0, 0, time.UTC))
}

func (c localeCalendar) FirstWeekday() time.Weekday { return c.first }

func (c localeCalendar) FormatDate(d widget.Date) string { return c.text("%x", d.Time()) }

type manualTime struct {
	calendar *widget.Calendar
	clock    *clockTimeEditor
	set      *widget.Button
	time     string
}

func newManualTime(k *kit, now time.Time) *manualTime {
	t := locale.T()
	m := &manualTime{time: now.Format(clockTimeLayout)}
	m.calendar = widget.NewCalendar(k.face, k.textPx(), widget.DateOf(now))
	names := sessionTimeLocale()
	m.calendar.SetNames(localeCalendar{names: names, first: k.store.svc.Config().Clock.CalendarWeekdayStart.Weekday(names.FirstWeekday())})
	m.calendar.SetAccessibleName(t.Get("settings-date-manual-day"))
	held := slot{
		get: func() any { return m.time },
		set: func(v any) error {
			s, ok := v.(string)
			if !ok {
				return fmt.Errorf("a time of day of %T, not HH:MM", v)
			}
			m.time = s
			return nil
		},
		unset: func() {},
	}
	m.clock = newClockTimeEditor(k, held, t.Get("settings-date-manual"), everyMinute, anyClockTime)
	m.set = k.button(k.label(t.Get("settings-date-set")), nil, css.Secondary)
	return m
}

func (m *manualTime) at(zone string) (time.Time, error) {
	if zone == "" {
		return time.Time{}, errors.New("set the date and time: timedated names no time zone")
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		return time.Time{}, fmt.Errorf("set the date and time in the time zone %q: %w", zone, err)
	}
	day := m.calendar.Selection()
	if day.IsZero() {
		return time.Time{}, fmt.Errorf("set the date and time: %s", locale.T().Get("settings-date-manual-no-day"))
	}
	clock, err := daytime.Parse(m.time)
	if err != nil {
		return time.Time{}, fmt.Errorf("set the date and time: %w", err)
	}
	return time.Date(day.Year, day.Month, day.Day, clock.Hour(), clock.Minute(), 0, 0, loc), nil
}

func (m *manualTime) setEnabled(on bool) {
	m.calendar.SetEnabled(on)
	m.clock.hour.SetEnabled(on)
	m.clock.minute.SetEnabled(on)
	m.set.SetEnabled(on)
}
