// The surfx coordinator: the dismissal state machine's contract.
// Dismiss flips logical state exactly once and seals input before the
// first exit frame; exit frames repaint until the tween lands and only
// then destroys; double dismissal and teardown interruptions cannot
// double-fire or double-destroy; the enter's first frame sits at
// progress 0; per-kind defaults differ and every instant path
// (reduced motion) runs the same machine with zero durations.
package surfx

import (
	"fmt"
	"testing"
	"time"

	"github.com/stubbedev/lys/ui/anim"
	"github.com/stubbedev/lys/ui/internal/animclock"
)

// recDriver is a fake Driver recording the call order, so tests can
// assert that sealing precedes the first exit frame and that the
// destroy follows the last.
type recDriver struct {
	events    []string
	reveal    float64
	frames    int
	sealed    bool
	destroyed int
	morph     bool
}

func (d *recDriver) ApplyVisual(reveal float64, morph bool) {
	d.morph = morph
	d.reveal = reveal
	d.frames++
	d.events = append(d.events, fmt.Sprintf("frame %.3f", reveal))
}

func (d *recDriver) MarkFrame() {}

func (d *recDriver) SealInput() {
	d.sealed = true
	d.events = append(d.events, "seal")
}

func (d *recDriver) Destroy() {
	d.destroyed++
	d.events = append(d.events, "destroy")
}

// moves reports whether the spec animates at all.
func (s Spec) moves() bool { return s.Duration > 0 || s.Spring != nil }

// sameStyle compares two styles' shapes; easings are functions, so
// only whether each has one counts.
func sameStyle(a, b Style) bool {
	same := func(x, y Spec) bool {
		return x.Duration == y.Duration && x.Spring == y.Spring && (x.Easing == nil) == (y.Easing == nil)
	}
	return same(a.Enter, b.Enter) && same(a.Exit, b.Exit) && a.Morph == b.Morph && a.From == b.From
}

// last, without the trailing element.
func last[T any](s []T) T { return s[len(s)-1] }

func TestDismissFiresOnceAndDestroysAfterLastFrame(t *testing.T) {
	restore := animclock.SetClock(func() time.Time { return time.Unix(1750000000, 0) })
	defer restore()
	var clock anim.Clock

	d := &recDriver{}
	fx := NewCoordinator(&clock, KindMenu, d, func() bool { return true })
	closed := 0
	fx.SetOnDismissed(func() { closed++ })

	fx.Enter()
	if d.frames != 1 || d.reveal != 0 {
		t.Fatalf("Enter's first frame: reveal = %v frames = %d, want one frame at 0", d.reveal, d.frames)
	}

	if !fx.Dismiss() {
		t.Fatal("first Dismiss reported already dismissed")
	}
	if !d.sealed {
		t.Fatal("Dismiss did not seal input")
	}
	// Seal strictly before the first exit frame.
	if d.events[0] != "frame 0.000" || d.events[1] != "seal" {
		t.Fatalf("event order before exit frames: %v", d.events)
	}

	// A second dismissal mid-exit — another outside click — is a no-op.
	if fx.Dismiss() {
		t.Error("second Dismiss re-entered the machine")
	}

	// Drive the exit: every tick repaints, the destroy comes only
	// after the last frame, and nothing fires twice.
	framesBefore := d.frames
	now := time.Unix(1750000000, 0)
	for steps := 0; ; steps++ {
		if _, ok := clock.Next(anim.PaceTimer); !ok {
			break
		}
		steps++
		if steps > 1000 {
			t.Fatal("exit tween never landed")
		}
		now = now.Add(anim.FrameInterval)
		clock.Tick(now)
	}
	if d.frames == framesBefore {
		t.Error("exit tween painted no frames")
	}
	if last(d.events) != "destroy" {
		t.Errorf("final event = %q, want the destroy after the last frame", last(d.events))
	}
	if d.destroyed != 1 {
		t.Errorf("destroyed %d times, want exactly 1", d.destroyed)
	}
	if closed != 1 {
		t.Errorf("OnClosed fired %d times, want exactly 1", closed)
	}
	if !fx.Dismissed() || !fx.Destroyed() {
		t.Error("Dismissed/Destroyed flags not set after the exit landed")
	}
}

func TestTeardownInterruptsTweenWithoutLeaks(t *testing.T) {
	restore := animclock.SetClock(func() time.Time { return time.Unix(1750000000, 0) })
	defer restore()
	var clock anim.Clock

	d := &recDriver{}
	fx := NewCoordinator(&clock, KindMenu, d, func() bool { return true })
	closed := 0
	fx.SetOnDismissed(func() { closed++ })

	fx.Enter()
	fx.Dismiss()
	// Teardown mid-exit: cancel the tween, fire once, destroy once.
	fx.Teardown()
	if closed != 1 {
		t.Errorf("OnClosed fired %d times after teardown, want exactly 1", closed)
	}
	if d.destroyed != 1 {
		t.Errorf("destroyed %d times after teardown, want exactly 1", d.destroyed)
	}
	// A tick after teardown runs nothing: the surface is gone.
	if _, ok := clock.Next(anim.PaceTimer); ok {
		t.Error("canceled tween still holds a wake deadline")
	}
	// Teardown is idempotent.
	fx.Teardown()
	if d.destroyed != 1 {
		t.Errorf("second teardown destroyed again (%d)", d.destroyed)
	}
}

func TestTeardownWithoutDismissFiresOnce(t *testing.T) {
	var clock anim.Clock
	d := &recDriver{}
	fx := NewCoordinator(&clock, KindMenu, d, nil)
	closed := 0
	fx.SetOnDismissed(func() { closed++ })
	// The error path: never opened, never dismissed, surface dies.
	fx.Teardown()
	if closed != 1 || d.destroyed != 1 {
		t.Errorf("teardown: closed = %d destroyed = %d, want 1/1", closed, d.destroyed)
	}
}

func TestEnterFirstFrameAtZeroAndRestAtCompletion(t *testing.T) {
	restore := animclock.SetClock(func() time.Time { return time.Unix(1750000000, 0) })
	defer restore()
	var clock anim.Clock

	d := &recDriver{}
	fx := NewCoordinator(&clock, KindMenu, d, func() bool { return true })
	fx.Enter()
	// The first frame is at progress 0 — hidden or offset — applied
	// synchronously, so the surface's first paint can never flash the
	// finished widget.
	if d.reveal != 0 {
		t.Fatalf("first frame reveal = %v, want 0", d.reveal)
	}
	// Enter is idempotent.
	fx.Enter()
	if d.frames != 1 {
		t.Fatalf("second Enter replayed a frame (frames = %d)", d.frames)
	}
	// Drive to rest.
	now := time.Unix(1750000000, 0)
	for steps := 0; ; steps++ {
		if _, ok := clock.Next(anim.PaceTimer); !ok {
			break
		}
		now = now.Add(anim.FrameInterval)
		clock.Tick(now)
		if steps > 1000 {
			t.Fatal("enter tween never landed")
		}
	}
	if d.reveal != 1 {
		t.Errorf("completion reveal = %v, want 1 (rest state)", d.reveal)
	}
	if d.destroyed != 0 {
		t.Error("a completed enter destroyed the surface")
	}
}

func TestInstantPathsRunTheSameMachine(t *testing.T) {
	type instantCase struct {
		name   string
		motion func() bool
		setup  func(t *testing.T)
	}
	for _, tc := range []instantCase{
		{
			name:   "LYS_UI_NO_ANIM equivalent (anim.SetInstant)",
			motion: func() bool { return true },
			setup:  func(t *testing.T) { t.Cleanup(anim.SetInstant(true)) },
		},
		{
			name:   "package toggle (SetEnabled(false))",
			motion: func() bool { return true },
			setup:  func(t *testing.T) { t.Cleanup(SetEnabled(false)) },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var clock anim.Clock
			tc.setup(t)

			d := &recDriver{}
			fx := NewCoordinator(&clock, KindMenu, d, tc.motion)
			closed := 0
			fx.SetOnDismissed(func() { closed++ })

			fx.Enter()
			if d.reveal != 1 {
				t.Fatalf("instant enter reveal = %v, want 1 at once", d.reveal)
			}
			fx.Dismiss()
			if closed != 1 {
				t.Errorf("instant path fired OnClosed %d times, want exactly 1", closed)
			}
			if d.destroyed != 1 {
				t.Errorf("instant path destroyed %d times, want exactly 1", d.destroyed)
			}
			if !d.sealed {
				t.Error("instant path skipped the input seal")
			}
			// The instant surface holds no wake deadlines: the loop
			// parks immediately after.
			if _, ok := clock.Next(anim.PaceTimer); ok {
				t.Error("instant path left a tween scheduled")
			}
		})
	}
}

func TestPerKindDefaults(t *testing.T) {
	menu, tooltip, dialog := Plan(KindMenu, true), Plan(KindTooltip, true), Plan(KindDialog, true)
	// Menus and dialogs morph out of their anchor on springs; tooltips
	// stay a quick fade.
	if menu.Enter.Spring == nil || !menu.Morph || dialog.Enter.Spring == nil || !dialog.Morph {
		t.Errorf("menu %+v / dialog %+v: want spring enters that morph", menu, dialog)
	}
	if tooltip.Enter.Spring != nil || tooltip.Enter.Duration <= 0 || tooltip.Morph {
		t.Errorf("tooltip %+v: want a short tween fade", tooltip)
	}
	if menu.Exit.Duration <= 0 || menu.Exit.Spring != nil {
		t.Errorf("menu exit %+v: want a short tween", menu.Exit)
	}
	// Reduced motion (the theme's switch) is a short fade, no movement;
	// the package toggle and LYS_UI_NO_ANIM are instant.
	if got := Plan(KindMenu, false); got.Morph || got.Enter.Spring != nil || got.Enter.Duration <= 0 || got.Enter.Duration > 100*time.Millisecond {
		t.Errorf("reduced-motion plan = %+v, want a short plain fade", got)
	}
	t.Cleanup(anim.SetInstant(true))
	if got := Plan(KindMenu, true); got.Enter.moves() || got.Exit.moves() {
		t.Errorf("instant plan = %+v, want no motion", got)
	}
}

// driveClock steps the pinned animation clock until nothing runs.
func driveClock(t *testing.T, clock *anim.Clock, onFrame func()) {
	t.Helper()
	now := time.Unix(1750000000, 0)
	for steps := 0; ; steps++ {
		if _, ok := clock.Next(anim.PaceTimer); !ok {
			return
		}
		now = now.Add(anim.FrameInterval)
		clock.Tick(now)
		if onFrame != nil {
			onFrame()
		}
		if steps > 2000 {
			t.Fatal("the motion never settled")
		}
	}
}

// A menu's spring enter swells past rest, settles exactly at it, and
// morphs all the way; reduced motion only fades.
func TestSpringEnterOvershootsAndSettles(t *testing.T) {
	t.Cleanup(animclock.SetClock(func() time.Time { return time.Unix(1750000000, 0) }))
	var clock anim.Clock
	d := &recDriver{}
	fx := NewCoordinator(&clock, KindMenu, d, func() bool { return true })
	fx.Enter()
	peak := 0.0
	driveClock(t, &clock, func() { peak = max(peak, d.reveal) })
	if peak <= 1 || d.reveal != 1 || !d.morph {
		t.Errorf("peak %v, rest %v, morph %v: want an overshoot settling at exactly 1, morphing", peak, d.reveal, d.morph)
	}

	var reduced anim.Clock
	d = &recDriver{}
	fx = NewCoordinator(&reduced, KindMenu, d, func() bool { return false })
	fx.Enter()
	driveClock(t, &reduced, nil)
	if d.reveal != 1 || d.morph || d.frames < 3 {
		t.Errorf("reduced motion: rest %v, morph %v over %d frames; want a plain fade over a few frames", d.reveal, d.morph, d.frames)
	}
}

// Dismissing mid-enter reverses from where the surface is: the exit's
// first frame starts at the enter's last reveal, not at rest.
func TestDismissMidEnterReverses(t *testing.T) {
	t.Cleanup(animclock.SetClock(func() time.Time { return time.Unix(1750000000, 0) }))
	var clock anim.Clock
	d := &recDriver{}
	fx := NewCoordinator(&clock, KindDialog, d, func() bool { return true })
	fx.Enter()
	now := time.Unix(1750000000, 0)
	for range 4 {
		now = now.Add(anim.FrameInterval)
		clock.Tick(now)
	}
	mid := d.reveal
	if mid <= 0 || mid >= 1 {
		t.Fatalf("mid-enter reveal %v, want a fraction", mid)
	}
	fx.Dismiss()
	now = now.Add(anim.FrameInterval)
	clock.Tick(now)
	if d.reveal >= mid || d.reveal <= 0 {
		t.Errorf("first exit frame at %v from %v: want it heading down from the mid-enter reveal", d.reveal, mid)
	}
	driveClock(t, &clock, nil)
	if d.destroyed != 1 {
		t.Errorf("destroyed %d times, want once", d.destroyed)
	}
}

// A kind the app styled keeps its profile with the package toggle
// off, its zero profile opens at once whatever the toggle says, and
// restoring brings the default back.
func TestSetStyleOverridesAndRestores(t *testing.T) {
	t.Cleanup(SetEnabled(true))
	own := Style{Enter: Spec{Duration: 5 * time.Millisecond, Easing: anim.Linear}, Exit: Spec{Duration: 5 * time.Millisecond}, Morph: true, From: 0.4}
	restore := SetStyle(KindDialog, own)
	if got := Plan(KindDialog, true); !sameStyle(got, own) {
		t.Errorf("styled plan = %+v, want %+v", got, own)
	}
	t.Cleanup(SetEnabled(false))
	if got := Plan(KindDialog, true); !sameStyle(got, own) {
		t.Errorf("styled plan with the toggle off = %+v, want the app's %+v", got, own)
	}
	if got := Plan(KindOverlay, true); got.Enter.moves() || got.Exit.moves() {
		t.Errorf("unstyled plan with the toggle off = %+v, want no motion", got)
	}
	SetEnabled(true)
	instant := SetStyle(KindDialog, Style{})
	if got := Plan(KindDialog, true); got.Enter.moves() || got.Exit.moves() {
		t.Errorf("zero style plan = %+v, want no motion with the toggle on", got)
	}
	instant()
	if got := Plan(KindDialog, true); !sameStyle(got, own) {
		t.Errorf("restored plan = %+v, want the earlier override", got)
	}
	restore()
	if got := Plan(KindDialog, true); !sameStyle(got, Defaults[KindDialog]) {
		t.Errorf("restored plan = %+v, want the default back", got)
	}
}

// Reduced motion keeps an app's durations and drops only the growth;
// KindStill and LYS_UI_NO_ANIM never move, styled or not.
func TestStyledPlanUnderReducedMotionAndStill(t *testing.T) {
	own := Style{Enter: Spec{Duration: 300 * time.Millisecond}, Exit: Spec{Duration: 200 * time.Millisecond}, Morph: true, From: 0.4}
	t.Cleanup(SetStyle(KindMenu, own))
	got := Plan(KindMenu, false)
	if got.Morph || got.Enter.Duration != own.Enter.Duration || got.Exit.Duration != own.Exit.Duration || got.From != own.From {
		t.Errorf("reduced styled plan = %+v, want the app's durations as a plain fade", got)
	}
	t.Cleanup(SetStyle(KindStill, own))
	if got := Plan(KindStill, true); got.Enter.moves() || got.Exit.moves() {
		t.Errorf("still plan = %+v, want no motion even when styled", got)
	}
	t.Cleanup(anim.SetInstant(true))
	if got := Plan(KindMenu, true); got.Enter.moves() || got.Exit.moves() {
		t.Errorf("instant styled plan = %+v, want no motion", got)
	}
}

// A styled enter with a From above zero shows the surface on its first
// frame: synchronously at From, rising to rest over the duration; a
// zero duration lands at rest before Enter returns.
func TestStyledEnterStartsAtFrom(t *testing.T) {
	t.Cleanup(animclock.SetClock(func() time.Time { return time.Unix(1750000000, 0) }))
	t.Cleanup(SetStyle(KindMenu, Style{Enter: Spec{Duration: 400 * time.Millisecond, Easing: anim.Linear}, Morph: true, From: 0.4}))
	var clock anim.Clock
	d := &recDriver{}
	fx := NewCoordinator(&clock, KindMenu, d, func() bool { return true })
	fx.Enter()
	if d.events[0] != "frame 0.400" || fx.Reveal() != 0.4 {
		t.Fatalf("first frame %v reveal %v, want 0.4", d.events, fx.Reveal())
	}
	frames := 0
	driveClock(t, &clock, func() {
		frames++
		if d.reveal < 0.4 {
			t.Errorf("a frame fell below From: %v", d.reveal)
		}
	})
	if d.reveal != 1 || frames < 10 {
		t.Errorf("rest %v after %d frames, want 1 after a 400ms rise", d.reveal, frames)
	}

	SetStyle(KindMenu, Style{From: 0.4})
	var instant anim.Clock
	d = &recDriver{}
	NewCoordinator(&instant, KindMenu, d, func() bool { return true }).Enter()
	if d.reveal != 1 || instant.Active() {
		t.Errorf("zero duration: reveal %v, clock active %v; want rest inside Enter", d.reveal, instant.Active())
	}
}
