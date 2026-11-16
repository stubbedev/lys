package looptest

import (
	"testing"
	"time"

	"github.com/stubbedev/lys/wm/internal/loop"
)

func newLoop(t *testing.T) *loop.Loop {
	t.Helper()
	l, err := loop.New()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := l.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return l
}

func fireAfter(t *testing.T, l *loop.Loop, d time.Duration) *bool {
	t.Helper()
	fired := new(bool)
	timer, err := l.AddTimer(func() error {
		*fired = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := timer.Arm(d); err != nil {
		t.Fatal(err)
	}
	return fired
}

func TestRunUntilRunsTheLoopUntilItsTimerFires(t *testing.T) {
	l := newLoop(t)
	fired := fireAfter(t, l, 3*slice)
	RunUntil(t, l, "the timer", func() bool { return *fired })
}

func TestRunAwhileSaysWhenTheConditionHeld(t *testing.T) {
	l := newLoop(t)
	fired := fireAfter(t, l, 0)
	if !RunAwhile(t, l, time.Hour, func() bool { return *fired }) {
		t.Fatal("RunAwhile returned without the timer that fired at once")
	}
}

func TestRunAwhileGivesUpAfterItsWindow(t *testing.T) {
	l := newLoop(t)
	fired := fireAfter(t, l, time.Hour)
	if RunAwhile(t, l, slice, func() bool { return *fired }) {
		t.Fatal("a timer an hour away fired within the window")
	}
}
