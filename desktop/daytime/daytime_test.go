package daytime

import (
	"errors"
	"testing"
	"time"
)

func TestATimeOfDayIsHHMMWithinADay(t *testing.T) {
	for _, raw := range []string{"24:00", "7:00", "07:60", "0700", "ab:cd", "-1:00", ""} {
		if _, err := Parse(raw); !errors.Is(err, ErrInvalid) {
			t.Errorf("%q: %v", raw, err)
		}
	}
	for _, raw := range []string{"00:00", "07:00", "23:59"} {
		if at, err := Parse(raw); err != nil || at.String() != raw {
			t.Errorf("%q = %q, %v", raw, at, err)
		}
	}
}

func TestATimeOfDayIsItsHourAndMinute(t *testing.T) {
	at, err := At(22, 5)
	if err != nil || at.Hour() != 22 || at.Minute() != 5 || at.String() != "22:05" {
		t.Fatalf("22:05 is %d:%d %q (%v)", at.Hour(), at.Minute(), at, err)
	}
	early, err := Parse("07:00")
	if err != nil || !at.After(early) || early.After(at) || at.After(at) {
		t.Fatalf("22:05 after 07:00 %v, 07:00 after 22:05 %v (%v)", at.After(early), early.After(at), err)
	}
	for _, hm := range [][2]int{{24, 0}, {-1, 0}, {7, 60}, {7, -1}} {
		if _, err := At(hm[0], hm[1]); !errors.Is(err, ErrInvalid) {
			t.Errorf("%d:%d: %v", hm[0], hm[1], err)
		}
	}
}

func TestATimeOfDayFallsOnADateInItsZone(t *testing.T) {
	zone := time.FixedZone("UTC+2", 2*60*60)
	at, err := Parse("21:30")
	if err != nil {
		t.Fatal(err)
	}
	got := at.On(time.Date(2026, 3, 2, 23, 59, 0, 0, zone))
	if want := time.Date(2026, 3, 2, 21, 30, 0, 0, zone); !got.Equal(want) || got.Location() != zone {
		t.Errorf("21:30 on 2 March = %s, want %s", got, want)
	}
}

func TestATimeOfDayIsReadOffAClockToTheMinute(t *testing.T) {
	zone := time.FixedZone("UTC-5", -5*60*60)
	clock := time.Date(2026, 3, 2, 6, 45, 59, 999, zone)
	if got := Of(clock); got.String() != "06:45" {
		t.Errorf("Of(%s) = %s, want 06:45 in the clock's zone", clock, got)
	}
	if got := Of(clock).On(clock); !got.Equal(time.Date(2026, 3, 2, 6, 45, 0, 0, zone)) {
		t.Errorf("Of then On = %s, want the clock's minute", got)
	}
}
