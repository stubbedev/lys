package bar

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/barview"
	"github.com/stubbedev/lys/desktop/strftime"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/shell/service/calendar"
)

// calendarWeekStart maps calendar-weekday-start.
func calendarWeekStart(w config.WeekStart) time.Weekday {
	switch w {
	case config.WeekStartMonday:
		return time.Monday
	case config.WeekStartTuesday:
		return time.Tuesday
	case config.WeekStartWednesday:
		return time.Wednesday
	case config.WeekStartThursday:
		return time.Thursday
	case config.WeekStartFriday:
		return time.Friday
	case config.WeekStartSaturday:
		return time.Saturday
	}
	return time.Sunday
}

var calendarWeekdayIDs = [7]string{"cal-weekday-sun", "cal-weekday-mon", "cal-weekday-tue", "cal-weekday-wed", "cal-weekday-thu", "cal-weekday-fri", "cal-weekday-sat"}

// shellClockNames is barview.ClockNames in the desktop locale, built once.
var shellClockNames = sync.OnceValue(func() strftime.Names { return barview.ClockNames(i18n.Shell()) })

// calendarNames gives widget.Calendar the shell's localized month and
// weekday names and its configured week start.
type calendarNames struct{ weekStart time.Weekday }

func (calendarNames) MonthName(m time.Month) string { return shellClockNames().Months[m-1] }

func (calendarNames) WeekdayName(w time.Weekday) string { return i18n.T(calendarWeekdayIDs[w]) }

func (n calendarNames) FirstWeekday() time.Weekday { return n.weekStart }

func (calendarNames) FormatDate(d widget.Date) string {
	return i18n.T("cal-clock-date-rest",
		i18n.Str("month", shellClockNames().Months[d.Month-1]),
		i18n.Str("day", strconv.Itoa(d.Day)),
		i18n.Str("year", strconv.Itoa(d.Year)))
}

// clockTimes is how every calendar clock shows a time: the hero's
// hour, unpadded on the 12-hour clock and two digits on the 24-hour
// one, and the agenda's time, which shows the hour the same way.
type clockTimes struct{ hour, time *strftime.Layout }

func newClockTimes(hour, period string) clockTimes {
	return clockTimes{hour: strftime.MustCompile(hour), time: strftime.MustCompile(hour + ":%M" + period)}
}

const (
	clockHour12h = "%-I"
	clockPeriod  = " %p"
)

var (
	clock12h     = newClockTimes(clockHour12h, clockPeriod)
	clock24h     = newClockTimes("%H", "")
	minuteLayout = strftime.MustCompile("%M")
	secondLayout = strftime.MustCompile("%S")
	periodLayout = strftime.MustCompile("%p")
)

func calendarClock(use12h bool) clockTimes {
	if use12h {
		return clock12h
	}
	return clock24h
}

// calendarIs12h reports whether the clock format uses %I or %p.
func calendarIs12h(format string) bool {
	return strings.Contains(format, "%I") || strings.Contains(format, "%p")
}

// calendarView is the calendar dropdown: the clock hero over the month
// grid, ticking every second while open.
type calendarView struct {
	ctx  ModuleContext
	font render.Font
	px   float64

	*widget.Box
	hours, minutes, seconds, ampm *widget.Label
	secSep                        *widget.Label
	dayName, dateRest             *widget.Label
	cal                           *widget.Calendar
	names                         calendarNames
	timeRow                       *widget.Box
	agenda                        *calendarAgenda
	agendaRule                    *widget.Separator

	today widget.Date

	*openFollow
	now func() time.Time
}

func calendarDropdown(ctx ModuleContext) widget.Widget {
	font, px := dropdownFont(ctx)
	v := &calendarView{ctx: ctx, font: font, px: px, now: time.Now}
	v.openFollow = newOpenFollow(ctx, v.follow)
	v.Box = widget.NewBox(widget.Column, 0, 0)
	v.AddClass("dropdown", "calendar-dropdown")
	v.Append(dropdownHeader(font, "ld-calendar-clock-symbolic", i18n.T("dropdown-calendar-title")), false)
	content := dropdownContent()
	content.Append(v.hero(), false)
	content.Append(widget.NewSeparator(widget.Horizontal), false)
	content.Append(v.calendar(), false)
	v.agendaRule = widget.NewSeparator(widget.Horizontal)
	v.agenda = newCalendarAgenda(font, px)
	content.Append(v.agendaRule, false)
	content.Append(v.agenda, false)
	v.Append(content, false)
	v.startFollowing()
	return v
}

// label paints with the constructor ink unset (0): the cascade's
// color — a classed rule, or the content box's default — supplies it.
func (v *calendarView) label(text string, role styling.TextRole) *widget.Label {
	return widget.NewLabel(v.font, v.px*role.Ratio(), text, 0)
}

// hero is the clock-hero: HH:MM[:SS][ AM] in the display size over
// "Weekday, Month D, YYYY", the day in the heading weight and the rest
// dimmed.
func (v *calendarView) hero() widget.Widget {
	th := widget.Current()
	col := widget.NewBox(widget.Column, th.Spacing/2, 0)
	col.AddClass("clock-hero")
	row := widget.NewBox(widget.Row, 0, 0)
	row.AddClass("clock-time-row")
	v.timeRow = row
	timeLabel := func(text string, classes ...string) *widget.Label {
		l := v.label(text, styling.Hero)
		l.AddClass(append([]string{"clock-time", css.Numeric}, classes...)...)
		return l
	}
	v.hours, v.minutes, v.seconds = timeLabel(""), timeLabel(""), timeLabel("")
	v.secSep = timeLabel(":", "clock-separator")
	v.ampm = v.label("", styling.Body)
	v.ampm.AddClass("clock-ampm", css.Heading, css.DimLabel)
	// The time and the ampm share the row's text baseline.
	for _, w := range []widget.Widget{v.hours, timeLabel(":", "clock-separator"), v.minutes, v.secSep, v.seconds, v.ampm} {
		row.AppendAligned(w, false, widget.AlignBaseline)
	}
	col.AppendAligned(row, false, widget.AlignCenter)
	date := widget.NewBox(widget.Row, 0, 0)
	date.AddClass("clock-date")
	v.dayName = v.label("", styling.Body)
	v.dayName.AddClass("clock-date-day", css.Heading)
	v.dateRest = v.label("", styling.Body)
	v.dateRest.AddClass(css.DimLabel)
	date.Append(v.dayName, false)
	date.Append(v.dateRest, false)
	col.AppendAligned(date, false, widget.AlignCenter)
	return col
}

// calendar is the Calendar component: the Today button over lys-ui's
// month grid, which carries the month title and its navigation. Today
// opens picked, so it wears the accent until another day is picked.
func (v *calendarView) calendar() widget.Widget {
	col := widget.NewBox(widget.Column, widget.Current().Spacing, 0)
	col.AddClass("cal-section")
	header := widget.NewBox(widget.Row, 0, 0)
	header.AddClass("cal-header")
	today := dropdownButton(v.label(i18n.T("cal-today"), styling.Body), v.goToToday, css.Ghost, css.Small, "cal-today-btn")
	header.Append(widget.NewSpacer(0, 0), true)
	header.Append(today, false)
	col.Append(header, false)
	v.today = widget.DateOf(v.now())
	v.cal = widget.NewCalendar(v.font, v.px, v.today)
	v.names = calendarNames{weekStart: calendarWeekStart(v.ctx.Config.Clock.CalendarWeekdayStart)}
	v.cal.SetNames(v.names)
	v.cal.SetToday(v.today)
	v.cal.OnMonth = func(widget.Date) { v.markMonth() }
	v.cal.OnSelect = func(widget.Date) { v.showAgenda() }
	v.cal.Detail = v.dayDetail
	col.Append(v.cal, false)
	return col
}

// goToToday goes back to this month, today picked.
func (v *calendarView) goToToday() { v.cal.Select(v.today) }

// tick refreshes the hero's fields and, across midnight, today.
func (v *calendarView) tick() {
	now := v.now()
	clock := v.ctx.Config.Clock
	use12h := calendarIs12h(clock.Format.String())
	names := shellClockNames()
	v.hours.SetText(calendarClock(use12h).hour.FormatIn(now, names))
	v.minutes.SetText(minuteLayout.FormatIn(now, names))
	v.seconds.SetText(secondLayout.FormatIn(now, names))
	v.ampm.SetText(periodLayout.FormatIn(now, names))
	v.seconds.SetVisible(clock.DropdownShowSeconds)
	v.secSep.SetVisible(clock.DropdownShowSeconds)
	v.ampm.SetVisible(use12h)
	// show-seconds stops the separator blink.
	setClass(v.timeRow, "show-seconds", clock.DropdownShowSeconds)
	v.dayName.SetText(shellClockNames().Weekdays[now.Weekday()])
	v.dateRest.SetText(i18n.T("cal-clock-date-rest",
		i18n.Str("month", shellClockNames().Months[now.Month()-1]),
		i18n.Str("day", strconv.Itoa(now.Day())),
		i18n.Str("year", strconv.Itoa(now.Year()))))
	if today := widget.DateOf(now); today != v.today {
		v.today = today
		v.cal.SetToday(today)
	}
}

func (v *calendarView) follow(life context.Context) {
	v.tick()
	v.refreshEvents()
	if v.ctx.App == nil {
		return
	}
	if v.ctx.Calendar != nil {
		changes, stop := v.ctx.Calendar.Changes()
		v.ctx.Go(func() {
			defer stop()
			for {
				select {
				case <-life.Done():
					return
				case <-changes:
					v.ctx.Invoke(v.refreshEvents)
				}
			}
		})
	}
	seconds := v.ctx.Config.Clock.DropdownShowSeconds
	v.ctx.Go(func() {
		timer := time.NewTimer(calendarTickDelay(v.now(), seconds))
		defer timer.Stop()
		for {
			select {
			case <-life.Done():
				return
			case <-timer.C:
				v.ctx.Invoke(v.tick)
				timer.Reset(calendarTickDelay(v.now(), seconds))
			}
		}
	})
}

// calendarTickDelay is how long from now until the hero next changes:
// the next second while it shows seconds, else the next minute.
func calendarTickDelay(now time.Time, seconds bool) time.Duration {
	step := time.Minute
	if seconds {
		step = time.Second
	}
	return now.Truncate(step).Add(step).Sub(now)
}

func (v *calendarView) hasEvents() bool { return v.ctx.Calendar != nil && v.ctx.Calendar.Configured() }

func (v *calendarView) refreshEvents() {
	shown := v.hasEvents()
	v.agenda.SetVisible(shown)
	v.agendaRule.SetVisible(shown)
	v.markMonth()
	v.showAgenda()
}

func (v *calendarView) day(d widget.Date) []calendar.Occurrence {
	if !v.hasEvents() || d.IsZero() {
		return nil
	}
	return v.ctx.Calendar.Month(d.Year, d.Month).Day(d.Day)
}

func (v *calendarView) markMonth() {
	v.cal.ClearMarks()
	if !v.hasEvents() {
		return
	}
	first := v.cal.Shown()
	month := v.ctx.Calendar.Month(first.Year, first.Month)
	for d := 1; d <= 31; d++ {
		if len(month.Day(d)) > 0 {
			v.cal.MarkDay(widget.Date{Year: first.Year, Month: first.Month, Day: d})
		}
	}
}

func (v *calendarView) showAgenda() {
	if !v.hasEvents() {
		return
	}
	day := v.cal.Selection()
	if day.IsZero() {
		day = v.today
	}
	v.agenda.show(v.day(day), calendarIs12h(v.ctx.Config.Clock.Format.String()), shellClockNames())
}

func (v *calendarView) dayDetail(d widget.Date) string { return dayDetail(v.day(d)) }
