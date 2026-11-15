package ambient

import (
	"testing"

	"github.com/stubbedev/lys/power/autobrightness"
)

func panel(raw uint32) []Level { return []Level{{Name: "intel_backlight", Raw: raw}} }

type step struct {
	wrote   []Level
	observe []Level
	resume  []Level
	idle    bool
	want    autobrightness.State
	changed bool
}

func run(t *testing.T, steps []step) {
	t.Helper()
	o := NewOwner(4)
	for i, s := range steps {
		var changed bool
		switch {
		case s.wrote != nil:
			o.Wrote(s.wrote)
		case s.resume != nil:
			changed = o.Resume(s.resume)
		case s.idle:
			changed = o.Observe(s.observe, Idle)
		default:
			changed = o.Observe(s.observe, Attended)
		}
		if o.State() != s.want || changed != s.changed {
			t.Fatalf("step %d: %v (changed %v), want %v (changed %v)", i, o.State(), changed, s.want, s.changed)
		}
	}
}

const (
	following = autobrightness.Following
	paused    = autobrightness.Paused
)

func TestOwnerAdoptsTheFirstLevelAndFollowsItsOwnWrites(t *testing.T) {
	run(t, []step{
		{observe: panel(400), want: following},
		{wrote: panel(450), want: following},
		{observe: panel(450), want: following},
		{wrote: panel(500), want: following},
		{observe: panel(500), want: following},
	})
}

func TestOwnerPausesOnAManualRaise(t *testing.T) {
	run(t, []step{
		{observe: panel(400), want: following},
		{observe: panel(600), want: paused, changed: true},
		{observe: panel(400), want: paused},
	})
}

func TestOwnerPausesOnAManualLowerThatStays(t *testing.T) {
	run(t, []step{
		{wrote: panel(400), want: following},
		{observe: panel(300), want: paused, changed: true},
		{observe: panel(300), want: paused},
	})
}

func TestOwnerTakesAnIdleDimAndItsRestoreAsAnExcursion(t *testing.T) {
	run(t, []step{
		{wrote: panel(400), want: following},
		{observe: panel(120), idle: true, want: paused, changed: true},
		{observe: panel(400), want: following, changed: true},
		{observe: panel(400), want: following},
		{observe: panel(410), want: paused, changed: true},
	})
}

func TestOwnerTakesARestoreToAnEarlierStepOfItsTransition(t *testing.T) {
	run(t, []step{
		{wrote: panel(400), want: following},
		{wrote: panel(420), want: following},
		{wrote: panel(440), want: following},
		{observe: panel(126), idle: true, want: paused, changed: true},
		{observe: panel(420), want: following, changed: true},
		{observe: panel(420), want: following},
	})
}

func TestOwnerHoldsAManualLowerThatComesBackInSteps(t *testing.T) {
	run(t, []step{
		{wrote: panel(400), want: following},
		{observe: panel(350), want: paused, changed: true},
		{observe: panel(300), want: paused},
		{observe: panel(400), want: paused},
	})
}

func TestOwnerHoldsAManualLowerAndRaiseBackToItsLevel(t *testing.T) {
	run(t, []step{
		{wrote: panel(400), want: following},
		{observe: panel(360), want: paused, changed: true},
		{observe: panel(400), want: paused},
		{observe: panel(400), idle: true, want: paused},
	})
}

func TestOwnerHoldsAnIdleDimThatMovesBeforeItsRestore(t *testing.T) {
	run(t, []step{
		{wrote: panel(400), want: following},
		{observe: panel(120), idle: true, want: paused, changed: true},
		{observe: panel(90), idle: true, want: paused},
		{observe: panel(400), idle: true, want: paused},
	})
}

func TestOwnerHoldsAManualRaiseThroughAnIdleDim(t *testing.T) {
	run(t, []step{
		{wrote: panel(400), want: following},
		{observe: panel(600), want: paused, changed: true},
		{observe: panel(180), idle: true, want: paused},
		{observe: panel(600), want: paused},
		{observe: panel(400), want: paused},
	})
}

func TestOwnerResumesAtTheLevelsRead(t *testing.T) {
	run(t, []step{
		{wrote: panel(400), want: following},
		{observe: panel(700), want: paused, changed: true},
		{resume: panel(700), want: following, changed: true},
		{observe: panel(700), want: following},
		{resume: panel(700), want: following},
		{observe: panel(650), want: paused, changed: true},
	})
}

func TestOwnerPausesWhenAnyBacklightMoves(t *testing.T) {
	both := func(a, b uint32) []Level {
		return []Level{{Name: "acpi_video0", Raw: a}, {Name: "intel_backlight", Raw: b}}
	}
	run(t, []step{
		{wrote: both(40, 400), want: following},
		{observe: both(40, 400), want: following},
		{observe: both(40, 380), idle: true, want: paused, changed: true},
		{observe: both(40, 400), want: following, changed: true},
		{observe: both(12, 400), idle: true, want: paused, changed: true},
		{observe: both(12, 120), want: paused},
		{observe: both(40, 400), want: paused},
	})
}

func TestOwnerForgetsLevelsOlderThanItKeeps(t *testing.T) {
	run(t, []step{
		{wrote: panel(100), want: following},
		{wrote: panel(110), want: following},
		{wrote: panel(120), want: following},
		{wrote: panel(130), want: following},
		{wrote: panel(140), want: following},
		{observe: panel(30), idle: true, want: paused, changed: true},
		{observe: panel(100), want: paused},
	})
}
