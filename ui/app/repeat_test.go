package app

import (
	"testing"
	"time"

	"github.com/stubbedev/lys/ui/internal/wlsession"
)

func advertisedRepeat(rate, delayMs uint32) repeatInfo {
	return func() (uint32, uint32, bool) { return rate, delayMs, true }
}

func unadvertisedRepeat() (rate, delayMs uint32, advertised bool) { return 0, 0, false }

func TestKeyRepeater(t *testing.T) {
	t.Run("no repeat before the initial delay", func(t *testing.T) {
		r := newKeyRepeater(advertisedRepeat(20, 400))
		r.press(30, wlsession.ModShift)
		if _, _, ok := r.tick(); ok {
			t.Error("tick fired before the delay elapsed")
		}
	})

	t.Run("one repeat after the delay, then at the rate", func(t *testing.T) {
		r := newKeyRepeater(advertisedRepeat(20, 400))
		r.press(30, 0)
		r.next = time.Now().Add(-time.Millisecond)
		code, _, ok := r.tick()
		if !ok || code != 30 {
			t.Errorf("tick = %d, %v, want 30, true", code, ok)
		}
		if _, _, ok := r.tick(); ok {
			t.Error("second tick fired immediately after the first repeat")
		}
		r.next = time.Now().Add(-time.Millisecond)
		if _, _, ok = r.tick(); !ok {
			t.Error("tick did not fire at the repeat rate")
		}
	})

	t.Run("release stops repeats", func(t *testing.T) {
		r := newKeyRepeater(advertisedRepeat(20, 400))
		r.press(30, 0)
		r.release(30)
		r.next = time.Now().Add(-time.Millisecond)
		if _, _, ok := r.tick(); ok {
			t.Error("tick fired after release")
		}
	})

	t.Run("a different key release does not stop the held key", func(t *testing.T) {
		r := newKeyRepeater(advertisedRepeat(20, 400))
		r.press(30, 0)
		r.release(31)
		r.next = time.Now().Add(-time.Millisecond)
		if _, _, ok := r.tick(); !ok {
			t.Error("unrelated release cancelled the held key's repeat")
		}
	})

	t.Run("a new press re-arms the delay", func(t *testing.T) {
		r := newKeyRepeater(advertisedRepeat(20, 400))
		r.press(30, 0)
		r.next = time.Now().Add(-time.Millisecond)
		r.press(30, 0)
		if _, _, ok := r.tick(); ok {
			t.Error("re-press fired immediately instead of re-arming the delay")
		}
	})

	t.Run("a seat that advertises no repeat gets the fallback", func(t *testing.T) {
		r := newKeyRepeater(unadvertisedRepeat)
		r.press(30, 0)
		want := [2]time.Duration{time.Second / fallbackRepeatRate, fallbackRepeatDelayMs * time.Millisecond}
		if got := [2]time.Duration{r.period, r.delay}; got != want {
			t.Errorf("period and delay %v, want the fallback %v", got, want)
		}
		if _, ok := r.nextDeadline(); !ok {
			t.Error("a key held on a seat without repeat_info does not repeat")
		}
	})

	t.Run("an advertised rate of 0 turns repeat off", func(t *testing.T) {
		r := newKeyRepeater(advertisedRepeat(0, 400))
		r.press(30, 0)
		if _, ok := r.nextDeadline(); ok {
			t.Error("a key held under a repeat rate of 0 schedules a repeat")
		}
		r.next = time.Now().Add(-time.Hour)
		if _, _, ok := r.tick(); ok {
			t.Error("a key held under a repeat rate of 0 repeated")
		}
	})

	t.Run("each press reads the repeat the compositor gives now", func(t *testing.T) {
		rate, delay, advertised := uint32(0), uint32(0), false
		r := newKeyRepeater(func() (uint32, uint32, bool) { return rate, delay, advertised })
		r.press(30, 0)
		r.release(30)
		rate, delay, advertised = 50, 300, true
		r.press(30, 0)
		want := [2]time.Duration{20 * time.Millisecond, 300 * time.Millisecond}
		if got := [2]time.Duration{r.period, r.delay}; got != want {
			t.Errorf("after repeat_info 50/300 the press repeats on %v, want %v", got, want)
		}
		r.release(30)
		rate = 0
		r.press(30, 0)
		if _, ok := r.nextDeadline(); ok {
			t.Error("a press after the compositor turned repeat off still repeats")
		}
	})

	t.Run("late handling keeps the configured rate", func(t *testing.T) {
		clock := time.Unix(1000, 0)
		r := newKeyRepeater(advertisedRepeat(20, 400))
		r.now = func() time.Time { return clock }
		start := clock
		r.press(30, 0)
		const n = 10
		for i := range n {
			due, _ := r.nextDeadline()
			clock = due.Add(7 * time.Millisecond)
			if _, _, ok := r.tick(); !ok {
				t.Fatalf("repeat %d did not fire", i)
			}
		}
		due, _ := r.nextDeadline()
		if want := start.Add(r.delay + n*r.period); !due.Equal(want) {
			t.Errorf("after %d late repeats the next is due %v after the press, want %v", n, due.Sub(start), want.Sub(start))
		}
	})

	t.Run("repeats that fell behind are skipped, not burst", func(t *testing.T) {
		clock := time.Unix(1000, 0)
		r := newKeyRepeater(advertisedRepeat(20, 400))
		r.now = func() time.Time { return clock }
		start := clock
		r.press(30, 0)
		clock = start.Add(r.delay + 3*r.period + r.period/2)
		if _, _, ok := r.tick(); !ok {
			t.Fatal("an overdue repeat did not fire")
		}
		if _, _, ok := r.tick(); ok {
			t.Error("the repeats that fell behind fired in a burst")
		}
		due, _ := r.nextDeadline()
		if want := start.Add(r.delay + 4*r.period); !due.Equal(want) {
			t.Errorf("next repeat due %v after the press, want %v", due.Sub(start), want.Sub(start))
		}
	})

	t.Run("repeats carry the mods held at press time", func(t *testing.T) {
		r := newKeyRepeater(advertisedRepeat(20, 400))
		r.press(30, wlsession.ModCtrl)
		r.next = time.Now().Add(-time.Millisecond)
		_, mods, ok := r.tick()
		if !ok || mods != wlsession.ModCtrl {
			t.Errorf("repeat mods = %v, %v, want ctrl", mods, ok)
		}
	})
}
