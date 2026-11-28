package backend

import (
	"errors"
	"testing"
	"time"

	"github.com/stubbedev/lys/wm/internal/loop"
)

func TestAVblankTimerFiresOnTheRefreshGridAndCountsEveryTick(t *testing.T) {
	const interval = 10 * time.Millisecond
	l, clock, err := loop.NewManual(time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = l.Close() })
	var fired []time.Duration
	var v *VblankTimer
	v, err = NewVblankTimer(l, func() error {
		fired = append(fired, v.Last())
		l.Stop()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	run := func(at time.Duration) {
		t.Helper()
		if err := v.Arm(interval); err != nil {
			t.Fatal(err)
		}
		if err := v.Arm(interval); err != nil {
			t.Fatalf("arming an armed timer: %v", err)
		}
		next, ok := clock.NextFrame()
		if !ok || next != at {
			t.Fatalf("the vblank is due at %v (%v), want %v", next, ok, at)
		}
		clock.Set(next)
		if err := l.Run(); err != nil {
			t.Fatal(err)
		}
	}
	run(time.Second + interval)
	if v.Seq() != 1 || v.Last() != time.Second+interval {
		t.Fatalf("after one vblank: seq %d at %v", v.Seq(), v.Last())
	}
	clock.Set(time.Second + 3*interval + interval/2)
	run(time.Second + 4*interval)
	if v.Seq() != 4 || v.Last() != time.Second+4*interval || len(fired) != 2 {
		t.Fatalf("after a late arm: seq %d at %v, fired %v", v.Seq(), v.Last(), fired)
	}
	if err := v.Remove(); err != nil {
		t.Fatal(err)
	}
	if err := v.Arm(interval); !errors.Is(err, loop.ErrClosed) {
		t.Fatalf("arming a removed vblank timer gave %v", err)
	}
}
