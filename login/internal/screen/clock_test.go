package screen

import (
	"testing"
	"time"
)

// clockRig is a ticker over a fake wall clock that runs what it armed when the test
// moves the time past it.
type clockRig struct {
	now     time.Time
	due     time.Time
	fn      func()
	formats int
	shown   int
	wakes   int
	ticker  *ClockTicker
}

func (r *clockRig) Now() time.Time              { return r.now }
func (r *clockRig) Arm(at time.Time, fn func()) { r.due, r.fn = at, fn }
func (r *clockRig) Disarm()                     { r.fn = nil }

func newClockRig(start time.Time, layouts ...string) *clockRig {
	r := &clockRig{now: start}
	formats := make([]func(time.Time) string, len(layouts))
	for i, l := range layouts {
		formats[i] = func(t time.Time) string {
			r.formats++
			return t.Format(l)
		}
	}
	r.ticker = NewClockTicker(r, func() {
		r.shown++
		for _, f := range formats {
			f(r.now)
		}
	}, formats...)
	r.formats = 0
	return r
}

func (r *clockRig) run(d time.Duration) {
	end := r.now.Add(d)
	for r.fn != nil && !r.due.After(end) {
		r.now = r.due
		fn := r.fn
		r.fn = nil
		r.wakes++
		fn()
	}
	r.now = end
}

func TestAMinuteClockFormatsAtMostTwicePerMinute(t *testing.T) {
	start := time.Date(2026, 3, 5, 9, 7, 30, 0, time.UTC)
	for name, tc := range map[string]struct {
		layouts []string
		wakes   int
		formats int
	}{
		"hours and minutes with a date": {[]string{"15:04", "Monday, January 2"}, 1, 2},
		"hours and minutes":             {[]string{"15:04"}, 1, 1},
		"seconds":                       {[]string{"15:04:05"}, 60, 60},
		"a date alone":                  {[]string{"Monday, January 2"}, 0, 0},
		"seconds beside a date":         {[]string{"Monday, January 2", "15:04:05"}, 60, 120},
	} {
		t.Run(name, func(t *testing.T) {
			r := newClockRig(start, tc.layouts...)
			r.ticker.Arm()
			r.run(60 * time.Second)
			if r.wakes != tc.wakes || r.formats != tc.formats {
				t.Errorf("over 60s: %d wakes and %d Format calls, want %d and %d", r.wakes, r.formats, tc.wakes, tc.formats)
			}
		})
	}
}

func TestAMinuteClockChangesOnTheMinute(t *testing.T) {
	r := newClockRig(time.Date(2026, 3, 5, 9, 7, 30, 500, time.UTC), "15:04")
	r.ticker.Arm()
	want := time.Date(2026, 3, 5, 9, 8, 0, 0, time.UTC)
	if !r.due.Equal(want) {
		t.Errorf("due %v, want %v", r.due, want)
	}
}

func TestAClockSurvivesAZoneOfHalfHoursAndTheClocksGoingBack(t *testing.T) {
	india := time.FixedZone("IST", 5*3600+1800)
	if got, want := everyHour.next(time.Date(2026, 3, 5, 9, 45, 0, 0, india)), time.Date(2026, 3, 5, 10, 0, 0, 0, india); !got.Equal(want) {
		t.Errorf("the next hour in a half-hour zone = %v, want %v", got, want)
	}
	berlin, err := time.LoadLocation("Europe/Berlin")
	if err != nil {
		t.Skipf("no time zone database: %v", err)
	}
	second := time.Date(2026, 10, 25, 0, 30, 0, 0, time.UTC).In(berlin).Add(time.Hour)
	if h, _, _ := second.Clock(); h != 2 {
		t.Fatalf("the second 2 o'clock is %v", second)
	}
	if next := everyMinute.next(second); !next.After(second) || next.Sub(second) > time.Minute {
		t.Errorf("the next minute of the repeated hour is %v after %v", next, second)
	}
}

func TestAClockShowsAJumpOfTheWallClockWithoutAKey(t *testing.T) {
	r := newClockRig(time.Date(2026, 3, 5, 9, 7, 30, 0, time.UTC), "15:04")
	r.ticker.Arm()
	r.now = r.now.Add(5 * time.Hour)
	r.fn()
	if r.shown != 1 {
		t.Errorf("a jump showed the clock %d times, want once", r.shown)
	}
	if !r.due.Equal(time.Date(2026, 3, 5, 14, 8, 0, 0, time.UTC)) {
		t.Errorf("after the jump the clock is armed for %v", r.due)
	}
}

func TestAStoppedClockStaysQuiet(t *testing.T) {
	r := newClockRig(time.Date(2026, 3, 5, 9, 7, 30, 0, time.UTC), "15:04")
	r.ticker.Arm()
	fn := r.fn
	r.ticker.Stop()
	if r.fn != nil {
		t.Error("Stop left a timer armed")
	}
	fn()
	r.run(time.Hour)
	if r.shown != 0 || r.wakes != 0 {
		t.Errorf("a stopped clock: shown %d times, %d wakes", r.shown, r.wakes)
	}
}
