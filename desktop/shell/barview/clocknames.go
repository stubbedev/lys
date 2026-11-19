package barview

import (
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/strftime"
)

var (
	dayIDs   = [7]string{"cal-day-sunday", "cal-day-monday", "cal-day-tuesday", "cal-day-wednesday", "cal-day-thursday", "cal-day-friday", "cal-day-saturday"}
	monthIDs = [12]string{
		"cal-month-january", "cal-month-february", "cal-month-march", "cal-month-april", "cal-month-may", "cal-month-june",
		"cal-month-july", "cal-month-august", "cal-month-september", "cal-month-october", "cal-month-november", "cal-month-december",
	}
	shortDayIDs = [7]string{
		"cal-day-short-sunday", "cal-day-short-monday", "cal-day-short-tuesday", "cal-day-short-wednesday", "cal-day-short-thursday", "cal-day-short-friday", "cal-day-short-saturday",
	}
	shortMonthIDs = [12]string{
		"cal-month-short-january", "cal-month-short-february", "cal-month-short-march", "cal-month-short-april", "cal-month-short-may", "cal-month-short-june",
		"cal-month-short-july", "cal-month-short-august", "cal-month-short-september", "cal-month-short-october", "cal-month-short-november", "cal-month-short-december",
	}
)

// ClockNames is the weekday, month and period names a strftime format
// renders in the bar's clocks, in the loader's language.
func ClockNames(l *i18n.Loader) strftime.Names {
	n := strftime.Names{AM: l.Get("cal-period-am"), PM: l.Get("cal-period-pm")}
	for d := range n.Weekdays {
		n.Weekdays[d], n.ShortWeekdays[d] = l.Get(dayIDs[d]), l.Get(shortDayIDs[d])
	}
	for m := range n.Months {
		n.Months[m], n.ShortMonths[m] = l.Get(monthIDs[m]), l.Get(shortMonthIDs[m])
	}
	return n
}
