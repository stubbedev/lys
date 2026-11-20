package nightschedule

import (
	"testing"
	"time"
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
