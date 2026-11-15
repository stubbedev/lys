package harness

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/stubbedev/lys/wm/internal/anim"
	"github.com/stubbedev/lys/wm/internal/backend/headless"
	"github.com/stubbedev/lys/wm/internal/compositor"
	"github.com/stubbedev/lys/wm/internal/testclient"
)

var (
	lockDesktop = [3]byte{0xc8, 0xc8, 0xc8}
	lockCover   = [3]byte{0x28, 0x28, 0x28}
)

type lockFadeRig struct {
	*inputRig
}

func lockFades(t *testing.T, d time.Duration) anim.Table {
	t.Helper()
	off, on := false, true
	table, errs := anim.Resolve(true, map[anim.Event]anim.Setting{
		anim.Global:  {Enabled: &off},
		anim.FadeIn:  {Enabled: &on, Shape: anim.Linear, Duration: &d},
		anim.FadeOut: {Enabled: &on, Shape: anim.Linear, Duration: &d},
	})
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	return table
}

func startLockFade(t *testing.T, table anim.Table) *lockFadeRig {
	t.Helper()
	h := StartWith(t, func(cfg *compositor.Config) {
		cfg.Decor = compositor.PlainDecor()
		cfg.Animations = table
	}, small)
	c := testclient.Connect(t, h.Socket())
	seat, in := c.Seat()
	w := c.Window("win", testclient.Fill(lockDesktop[2], lockDesktop[1], lockDesktop[0]))
	in.Name(w.Surface, "win")
	c.DispatchUntil("the window to map", w.Mapped)
	r := &lockFadeRig{inputRig: &inputRig{t: t, h: h, c: c, seat: seat, in: in, w: w}}
	r.h.Eventually("the window faded in", showing(10, 10, lockDesktop))
	r.h.holdQuiet()
	return r
}

func halfLocked(b *headless.Backend, _ *compositor.Compositor) bool {
	return nearProgress(pixel(snapshot(b.Outputs()[0]), 10, 10), lockCover, lockDesktop, 0.5, frameSlack(small, time.Second))
}

func halfUnlocked(b *headless.Backend, _ *compositor.Compositor) bool {
	return nearProgress(pixel(snapshot(b.Outputs()[0]), 10, 10), lockDesktop, lockCover, 0.5, frameSlack(small, time.Second))
}

func (r *lockFadeRig) lockAndCover() *locker {
	r.t.Helper()
	l := lockSession(r.t, r.h)
	l.cover(small.Size, lockCover[2], lockCover[1], lockCover[0])
	return l
}

func (r *lockFadeRig) unlock(l *locker) {
	r.t.Helper()
	if err := errors.Join(l.lock.UnlockAndDestroy(), l.c.Display.Flush()); err != nil {
		r.t.Fatal(err)
	}
}

func TestTheLockFadesInOverTheDesktopBeforeItIsConfirmed(t *testing.T) {
	t.Parallel()
	r := startLockFade(t, lockFades(t, time.Second))
	l := r.lockAndCover()
	l.c.Sync()
	r.h.frameNow()
	r.h.Eventually("the lock still transparent at its start", showing(10, 10, lockDesktop))
	r.h.Advance(500 * time.Millisecond)
	r.h.Eventually("the lock halfway over the desktop", halfLocked)
	if err := l.c.Display.Roundtrip(); err != nil {
		t.Fatal(err)
	}
	if slices.Contains(l.log, "locked") {
		t.Fatalf("locked was sent with the lock half transparent: %v", l.log)
	}
	r.h.Advance(500*time.Millisecond + small.Interval())
	l.seen("locked")
	r.h.Eventually("the lock opaque", showing(10, 10, lockCover))
}

func TestNoInputReachesTheSessionWhileTheLockFadesIn(t *testing.T) {
	t.Parallel()
	r := startLockFade(t, lockFades(t, time.Second))
	r.expect("kb_enter win", "enter win 79,49", "frame")
	l := r.lockAndCover()
	r.expect("kb_leave win", "leave win", "frame")
	l.c.DispatchUntil("the keyboard on the lock surface", func() bool { return slices.Contains(l.in.Log, "kb_enter lock") })
	r.h.Advance(250 * time.Millisecond)
	r.inject(func(b *headless.Backend, _ *headless.Output) error { return b.Keyboard().Key(30, true) })
	l.c.DispatchUntil("the key at the lock", func() bool { return slices.Contains(l.in.Log, "key 30 1") })
	if err := r.c.Display.Roundtrip(); err != nil {
		t.Fatal(err)
	}
	if slices.ContainsFunc(r.in.Log[r.seen:], func(e string) bool { return e == "key 30 1" }) {
		t.Fatalf("the session saw a key typed during the fade-in: %v", r.in.Log)
	}
	if slices.Contains(l.log, "locked") {
		t.Fatalf("locked was sent mid fade: %v", l.log)
	}
}

func TestTheUnlockFadesASnapshotOfTheLockOutOverTheDesktop(t *testing.T) {
	t.Parallel()
	r := startLockFade(t, lockFades(t, time.Second))
	l := r.lockAndCover()
	r.h.Advance(time.Second + small.Interval())
	l.seen("locked")
	r.h.Eventually("the lock opaque", showing(10, 10, lockCover))
	r.h.holdQuiet()
	r.unlock(l)
	r.expect("kb_enter win")
	r.h.frameNow()
	r.h.Eventually("the lock still covering at the unlock", showing(10, 10, lockCover))
	r.h.Advance(500 * time.Millisecond)
	r.h.Eventually("the desktop halfway through the lock", halfUnlocked)
	if err := l.c.Display.Close(); err != nil {
		t.Fatal(err)
	}
	r.h.Eventually("the snapshot fading after the locker left", halfUnlocked)
	r.h.Advance(500*time.Millisecond + small.Interval())
	r.h.Eventually("the desktop back", showing(10, 10, lockDesktop))
}

func TestARelockDuringTheFadeOutStartsWhereItWas(t *testing.T) {
	t.Parallel()
	r := startLockFade(t, lockFades(t, time.Second))
	l := r.lockAndCover()
	r.h.Advance(time.Second + small.Interval())
	l.seen("locked")
	r.unlock(l)
	r.h.Advance(500 * time.Millisecond)
	r.h.Eventually("the desktop halfway through the lock", halfUnlocked)
	again := r.lockAndCover()
	r.h.Advance(2 * small.Interval())
	r.h.Eventually("the new lock from the opacity reached", func(b *headless.Backend, _ *compositor.Compositor) bool {
		return nearProgress(pixel(snapshot(b.Outputs()[0]), 10, 10), lockCover, lockDesktop, 0.5, 3*frameSlack(small, time.Second))
	})
	r.h.Advance(time.Second + small.Interval())
	again.seen("locked")
	r.h.Eventually("the new lock opaque", showing(10, 10, lockCover))
}

func TestWithAnimationsOffTheLockAndUnlockAreInstant(t *testing.T) {
	t.Parallel()
	off := false
	table, errs := anim.Resolve(true, map[anim.Event]anim.Setting{anim.Global: {Enabled: &off}})
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	r := startLockFade(t, table)
	l := r.lockAndCover()
	r.h.Advance(2 * small.Interval())
	l.seen("locked")
	r.h.Eventually("the lock on the next refresh", showing(10, 10, lockCover))
	r.unlock(l)
	r.h.Advance(2 * small.Interval())
	r.h.Eventually("the desktop on the next refresh", showing(10, 10, lockDesktop))
}
