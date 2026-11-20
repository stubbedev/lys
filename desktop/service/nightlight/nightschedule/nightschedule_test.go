package nightschedule

import (
	"errors"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/daytime"
)

func TestNewTransitionHoldsItsRange(t *testing.T) {
	for _, minutes := range []int64{0, 15, MaxTransitionMinutes} {
		tr, err := NewTransition(minutes)
		if err != nil || int64(tr.Minutes()) != minutes || tr.Duration() != time.Duration(minutes)*time.Minute {
			t.Errorf("NewTransition(%d) = %+v, %v", minutes, tr, err)
		}
	}
	for _, minutes := range []int64{-1, MaxTransitionMinutes + 1} {
		if _, err := NewTransition(minutes); err == nil {
			t.Errorf("NewTransition(%d) was accepted", minutes)
		}
	}
}

func TestCustomHoursEndAtAnotherTimeThanTheyStart(t *testing.T) {
	at := func(raw string) daytime.Time {
		v, err := daytime.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	for _, pair := range [][2]string{{"21:00", "07:00"}, {"01:00", "06:00"}, {"23:59", "00:00"}} {
		h, err := NewHours(at(pair[0]), at(pair[1]))
		if err != nil || h.From() != at(pair[0]) || h.To() != at(pair[1]) {
			t.Errorf("NewHours(%s, %s) = %+v, %v", pair[0], pair[1], h, err)
		}
	}
	if _, err := NewHours(at("07:00"), at("07:00")); !errors.Is(err, ErrEmptyHours) {
		t.Errorf("hours starting when they end: %v, want ErrEmptyHours", err)
	}
}
