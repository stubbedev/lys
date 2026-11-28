package wlsession

import (
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/wlr"
	"golang.org/x/sys/unix"
)

func TestPresentationTimesOnCLOCK_MONOTONICLandOnTheLocalClock(t *testing.T) {
	var s Session
	if _, ok := s.PresentationTime(0, 1, 0); ok {
		t.Fatal("a presentation time converted before the compositor named its clock")
	}
	s.presentClock.HandleWpPresentationClockId(wlr.WpPresentationClockIdEvent{ClkId: unix.CLOCK_MONOTONIC})
	var ts unix.Timespec
	before := time.Now()
	if err := unix.ClockGettime(unix.CLOCK_MONOTONIC, &ts); err != nil {
		t.Fatal(err)
	}
	after := time.Now()
	sec := uint64(ts.Sec)
	got, ok := s.PresentationTime(uint32(sec>>32), uint32(sec), uint32(ts.Nsec))
	if !ok {
		t.Fatal("a CLOCK_MONOTONIC presentation time did not convert")
	}
	if slack := time.Millisecond; got.Before(before.Add(-slack)) || got.After(after.Add(slack)) {
		t.Errorf("CLOCK_MONOTONIC now converted to %v, want between %v and %v", got, before, after)
	}
	if later, _ := s.PresentationTime(uint32(sec>>32), uint32(sec)+1, uint32(ts.Nsec)); later.Sub(got) != time.Second {
		t.Errorf("a second later converted %v later", later.Sub(got))
	}
}

func TestPresentationTimesOnAnotherClockDoNotConvert(t *testing.T) {
	var s Session
	s.presentClock.HandleWpPresentationClockId(wlr.WpPresentationClockIdEvent{ClkId: unix.CLOCK_MONOTONIC})
	s.presentClock.HandleWpPresentationClockId(wlr.WpPresentationClockIdEvent{ClkId: unix.CLOCK_REALTIME})
	if at, ok := s.PresentationTime(0, 1, 0); ok {
		t.Errorf("a CLOCK_REALTIME presentation time converted to %v", at)
	}
}
