package app

import (
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/third_party/wayland/wl"

	"github.com/stubbedev/lys/ui/internal/wlsession"
)

func TestARepeatInfoThatArrivesAfterTheApplicationStartedGovernsItsRepeat(t *testing.T) {
	sess := &wlsession.Session{}
	a := NewTestApplication(t, sess)
	sess.HandleKeyboardRepeatInfo(wl.KeyboardRepeatInfoEvent{Rate: 50, Delay: 300})
	a.rep.press(30, 0)
	want := [2]time.Duration{20 * time.Millisecond, 300 * time.Millisecond}
	if got := [2]time.Duration{a.rep.period, a.rep.delay}; got != want {
		t.Errorf("a key held after repeat_info 50/300 repeats on %v, want %v", got, want)
	}
}

func TestARepeatInfoWithRateZeroStopsTheApplicationRepeating(t *testing.T) {
	sess := &wlsession.Session{}
	a := NewTestApplication(t, sess)
	sess.HandleKeyboardRepeatInfo(wl.KeyboardRepeatInfoEvent{Rate: 0, Delay: 300})
	a.rep.press(30, 0)
	if at, ok := a.rep.nextDeadline(); ok {
		t.Errorf("a key held after repeat_info rate 0 repeats at %v", at)
	}
}
