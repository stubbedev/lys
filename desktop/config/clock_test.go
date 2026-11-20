package config

import (
	"strings"
	"testing"
	"time"
)

func TestTheWeekStartsWhereItIsSetOrWhereTheLocaleStartsIt(t *testing.T) {
	asked := 0
	locale := func() time.Weekday {
		asked++
		return time.Saturday
	}
	if got := DefaultsClock().CalendarWeekdayStart.Weekday(locale); got != time.Saturday || asked != 1 {
		t.Fatalf("the default week starts on %s after asking the locale %d times", got, asked)
	}
	for w, want := range weekStartDays {
		if got := w.Weekday(locale); got != want {
			t.Errorf("%s starts the week on %s", w, got)
		}
	}
	if asked != 1 {
		t.Errorf("a set week start asked the locale %d times", asked-1)
	}
	if len(weekStartDays) != 7 {
		t.Errorf("%d days have a week start", len(weekStartDays))
	}
}

func TestAnUnknownWeekStartIsRefused(t *testing.T) {
	for value, ok := range map[string]bool{"auto": true, "monday": true, "someday": false} {
		path := writeFile(t, t.TempDir(), "config.yaml", "modules:\n  clock:\n    calendar-weekday-start: "+value+"\n")
		cfg, err := LoadFile(path)
		if ok && (err != nil || string(cfg.Clock.CalendarWeekdayStart) != value) {
			t.Errorf("%s: %v", value, err)
		}
		if !ok && (err == nil || !strings.Contains(err.Error(), "calendar-weekday-start")) {
			t.Errorf("%s: err = %v, want the field's diagnostic", value, err)
		}
	}
	defer func() {
		if recover() == nil {
			t.Error("Weekday of an unparsed week start returned a day")
		}
	}()
	WeekStart("someday").Weekday(func() time.Weekday { return time.Sunday })
}
