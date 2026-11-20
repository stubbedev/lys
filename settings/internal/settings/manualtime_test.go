package settings

import (
	"slices"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/glibclocale"
	"github.com/stubbedev/lys/lib/glibclocale/glibclocaletest"
	"github.com/stubbedev/lys/ui/widget"
)

func manualTimeAt(t *testing.T, config, timeLocale string, now time.Time) *manualTime {
	t.Helper()
	useLocales(t, glibclocale.NewSource("", glibclocaletest.Dirs(t)))
	t.Setenv("LC_ALL", timeLocale)
	return newManualTime(testKit(t, config), now)
}

func TestTheManualTimeStartsAtNowAndSetsThePickedMinuteInTheSystemZone(t *testing.T) {
	m := manualTimeAt(t, "", "en_US.utf8", time.Date(2026, time.November, 19, 14, 5, 0, 0, time.UTC))
	if got := m.calendar.Selection(); got != (widget.Date{Year: 2026, Month: time.November, Day: 19}) {
		t.Fatalf("the calendar starts at %v", got)
	}
	if m.clock.hours[m.clock.hour.Selected()] != 14 || m.clock.minutes[m.clock.minute.Selected()] != 5 || len(m.clock.minutes) != minutesPerHr {
		t.Fatalf("the clock starts at %d:%d with %d minutes on offer", m.clock.hour.Selected(), m.clock.minute.Selected(), len(m.clock.minutes))
	}
	m.calendar.Select(widget.Date{Year: 2026, Month: time.December, Day: 24})
	m.clock.minute.SetSelected(slices.Index(m.clock.minutes, 59))
	at, err := m.at("Europe/Copenhagen")
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, time.December, 24, 13, 59, 0, 0, time.UTC); !at.Equal(want) {
		t.Errorf("24 December 14:59 in Copenhagen is %v, want %v", at.UTC(), want)
	}
	for _, zone := range []string{"", "Mars/Olympus"} {
		if _, err := m.at(zone); err == nil {
			t.Errorf("a time was set in the time zone %q", zone)
		}
	}
}

func TestTheManualDateNamesTheMonthsAndWeekdaysInTheTimeLocale(t *testing.T) {
	m := manualTimeAt(t, "modules:\n  clock:\n    calendar-weekday-start: monday\n", "de_DE.utf8", time.Date(2026, time.March, 4, 9, 0, 0, 0, time.UTC))
	names := localeCalendar{names: sessionTimeLocale(), first: time.Monday}
	if got := names.MonthName(time.March); got != "März" {
		t.Errorf("March in de_DE is %q", got)
	}
	if got := names.WeekdayName(time.Sunday); got != "So" {
		t.Errorf("Sunday in de_DE is %q", got)
	}
	if got := names.FormatDate(widget.Date{Year: 2026, Month: time.March, Day: 4}); got != "04.03.2026" {
		t.Errorf("4 March 2026 in de_DE is %q", got)
	}
	m.setEnabled(false)
	if widget.IsEnabled(m.calendar) || widget.IsEnabled(m.clock.hour) || widget.IsEnabled(m.set) {
		t.Error("with the time set automatically the manual date stays editable")
	}
}
