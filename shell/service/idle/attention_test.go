package idle

import (
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/service/idle/idlepolicy"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/power/attention"
)

type fakeTimer struct {
	after   time.Duration
	f       func()
	stopped bool
}

type fakeClock struct{ timers []*fakeTimer }

func (c *fakeClock) afterFunc(d time.Duration, f func()) func() {
	t := &fakeTimer{after: d, f: f}
	c.timers = append(c.timers, t)
	return func() { t.stopped = true }
}

func (c *fakeClock) pending() []*fakeTimer {
	var out []*fakeTimer
	for _, t := range c.timers {
		if !t.stopped {
			out = append(out, t)
		}
	}
	return out
}

func (c *fakeClock) fire(t *fakeTimer) {
	t.stopped = true
	t.f()
}

type attentionRig struct {
	w       *fakeWatcher
	a       *fakeActions
	clock   *fakeClock
	s       *Service
	changes <-chan struct{}
}

func newAttentionRig(t *testing.T, mode idlepolicy.Attention, seen ...idlepolicy.Presence) *attentionRig {
	t.Helper()
	r := &attentionRig{w: &fakeWatcher{}, a: &fakeActions{seen: seen}, clock: &fakeClock{}}
	r.s = New(r.w, r.a)
	r.s.afterFunc = r.clock.afterFunc
	changes, stop := r.s.Changes()
	t.Cleanup(stop)
	r.changes = changes
	p := testPolicy(t)
	p.Attention = mode
	if err := r.s.Configure(p, true, idlepolicy.AC); err != nil {
		t.Fatal(err)
	}
	waittest.For(t, r.changes, "the configuration")
	return r
}

func (r *attentionRig) dimWatch(t *testing.T) *fakeWatch {
	t.Helper()
	for _, w := range r.w.live() {
		if w.after == 4*time.Minute {
			return w
		}
	}
	t.Fatal("no dim watch")
	return nil
}

func (r *attentionRig) idleToTheDim(t *testing.T) {
	t.Helper()
	r.dimWatch(t).idle()
	waittest.For(t, r.changes, "the check's outcome")
}

func (r *attentionRig) fireRecheck(t *testing.T) {
	t.Helper()
	pending := r.clock.pending()
	if len(pending) != 1 || pending[0].after != 4*time.Minute {
		t.Fatalf("pending timers %v, want the one recheck a dim interval away", afterOf(pending))
	}
	r.clock.fire(pending[0])
	waittest.For(t, r.changes, "the recheck's outcome")
}

func afterOf(ts []*fakeTimer) []time.Duration {
	out := make([]time.Duration, len(ts))
	for i, t := range ts {
		out[i] = t.after
	}
	return out
}

func TestWhileAttendingTheStagesAfterTheDimHaveNoWatchOfTheirOwn(t *testing.T) {
	r := newAttentionRig(t, idlepolicy.AttentionPostpone)
	if got, want := afters(r.w.live()), []time.Duration{4 * time.Minute}; !slices.Equal(got, want) {
		t.Errorf("watches %v, want the dim's alone: a lock watch would lock a reader whose dim waits", got)
	}
	if err := r.s.Configure(testPolicy(t), true, idlepolicy.AC); err != nil {
		t.Fatal(err)
	}
	if got, want := afters(r.w.live()), []time.Duration{4 * time.Minute, 5 * time.Minute, 10 * time.Minute}; !slices.Equal(got, want) {
		t.Errorf("attention off: watches %v, want every stage's %v", got, want)
	}
}

func TestAFacePostponesTheDimUpToTheCapThenItDims(t *testing.T) {
	seen := slices.Repeat([]idlepolicy.Presence{idlepolicy.Present}, idlepolicy.MaxPostponements+1)
	r := newAttentionRig(t, idlepolicy.AttentionPostpone, seen...)
	r.idleToTheDim(t)
	for i := range idlepolicy.MaxPostponements {
		if len(r.a.calls) != 0 {
			t.Fatalf("after %d checks seeing a face the stages ran %v", i+1, r.a.calls)
		}
		r.fireRecheck(t)
	}
	if r.a.looks != idlepolicy.MaxPostponements+1 {
		t.Errorf("%d checks, want %d", r.a.looks, idlepolicy.MaxPostponements+1)
	}
	if want := []string{"dim"}; !slices.Equal(r.a.calls, want) {
		t.Fatalf("at the cap ran %v, want %v: a face cannot hold the screen on forever", r.a.calls, want)
	}
	if got, want := afterOf(r.clock.pending()), []time.Duration{time.Minute, 6 * time.Minute}; !slices.Equal(got, want) {
		t.Errorf("after the dim, timers %v, want the lock and screen-off at their distances %v", got, want)
	}
	for _, timer := range r.clock.pending() {
		r.clock.fire(timer)
	}
	if want := []string{"dim", "lock", "screen-off"}; !slices.Equal(r.a.calls, want) {
		t.Errorf("ran %v, want %v", r.a.calls, want)
	}
}

func TestNobodyThereDimsAtOnceAndSoonerBringsTheLockAndScreenOffCloser(t *testing.T) {
	for _, c := range []struct {
		mode  idlepolicy.Attention
		after []time.Duration
	}{
		{idlepolicy.AttentionPostpone, []time.Duration{time.Minute, 6 * time.Minute}},
		{idlepolicy.AttentionSooner, []time.Duration{idlepolicy.AbsentGrace, idlepolicy.AbsentGrace}},
	} {
		r := newAttentionRig(t, c.mode, idlepolicy.Absent)
		r.idleToTheDim(t)
		if want := []string{"dim"}; !slices.Equal(r.a.calls, want) {
			t.Errorf("mode %d: nobody there ran %v, want %v at once", c.mode, r.a.calls, want)
		}
		if got := afterOf(r.clock.pending()); !slices.Equal(got, c.after) {
			t.Errorf("mode %d: the lock and screen-off follow after %v, want %v", c.mode, got, c.after)
		}
	}
}

func TestNoAnswerOrAFailedCheckDimsAsWithoutIt(t *testing.T) {
	r := newAttentionRig(t, idlepolicy.AttentionSooner, idlepolicy.Unseen)
	r.idleToTheDim(t)
	if want := []string{"dim"}; !slices.Equal(r.a.calls, want) {
		t.Errorf("unseen ran %v, want %v", r.a.calls, want)
	}
	if got, want := afterOf(r.clock.pending()), []time.Duration{time.Minute, 6 * time.Minute}; !slices.Equal(got, want) {
		t.Errorf("unseen: followers after %v, want their own distances %v, unhurried", got, want)
	}
	r = newAttentionRig(t, idlepolicy.AttentionSooner)
	r.a.fail = errors.New("lys-power is not installed")
	r.idleToTheDim(t)
	if want := []string{"dim"}; !slices.Equal(r.a.calls, want) {
		t.Errorf("a failed check ran %v, want %v", r.a.calls, want)
	}
	if got, want := afterOf(r.clock.pending()), []time.Duration{time.Minute, 6 * time.Minute}; !slices.Equal(got, want) {
		t.Errorf("a failed check: followers after %v, want %v", got, want)
	}
}

func TestInputWhileTheDimWaitsStartsTheCountAgain(t *testing.T) {
	seen := slices.Repeat([]idlepolicy.Presence{idlepolicy.Present}, 2*idlepolicy.MaxPostponements)
	r := newAttentionRig(t, idlepolicy.AttentionPostpone, seen...)
	r.idleToTheDim(t)
	r.fireRecheck(t)
	stale := r.clock.pending()
	r.dimWatch(t).resume()
	waittest.For(t, r.changes, "the input")
	if len(r.clock.pending()) != 0 {
		t.Fatalf("after input, timers %v still pending", afterOf(r.clock.pending()))
	}
	stale[0].f()
	if r.a.looks != 2 {
		t.Errorf("a recheck stopped by input still looked: %d checks", r.a.looks)
	}
	r.idleToTheDim(t)
	for range idlepolicy.MaxPostponements - 1 {
		r.fireRecheck(t)
	}
	if len(r.a.calls) != 0 {
		t.Errorf("the count carried over the input: ran %v after %d checks since it", r.a.calls, idlepolicy.MaxPostponements)
	}
}

func TestInputAfterTheDimUndoesItAndWhatFollowed(t *testing.T) {
	r := newAttentionRig(t, idlepolicy.AttentionSooner, idlepolicy.Absent)
	r.idleToTheDim(t)
	for _, timer := range r.clock.pending() {
		r.clock.fire(timer)
	}
	r.dimWatch(t).resume()
	waittest.For(t, r.changes, "the input")
	if want := []string{"dim", "lock", "screen-off", "undim", "screen-on"}; !slices.Equal(r.a.calls, want) {
		t.Errorf("ran %v, want %v", r.a.calls, want)
	}
	if st := r.s.Status(); len(st.Reached) != 0 {
		t.Errorf("after input, reached %v", st.Reached)
	}
	r.a.seen = []idlepolicy.Presence{idlepolicy.Absent}
	r.idleToTheDim(t)
	if r.a.calls[len(r.a.calls)-1] != "dim" {
		t.Errorf("the next idle stretch did not dim again: %v", r.a.calls)
	}
}

func TestAFollowerStoppedByInputNeverRuns(t *testing.T) {
	r := newAttentionRig(t, idlepolicy.AttentionPostpone, idlepolicy.Absent)
	r.idleToTheDim(t)
	followers := r.clock.pending()
	r.dimWatch(t).resume()
	waittest.For(t, r.changes, "the input")
	for _, f := range followers {
		f.f()
	}
	if want := []string{"dim", "undim"}; !slices.Equal(r.a.calls, want) {
		t.Errorf("ran %v, want %v: a lock after input would lock a returning user", r.a.calls, want)
	}
}

func TestWithoutAttentionNothingLooks(t *testing.T) {
	r := newAttentionRig(t, idlepolicy.AttentionOff)
	r.dimWatch(t).idle()
	if r.a.looks != 0 || !slices.Equal(r.a.calls, []string{"dim"}) {
		t.Errorf("attention off: %d checks, ran %v, want the dim alone", r.a.looks, r.a.calls)
	}
}

func TestNoCameraDimsAndStopsAskingOnceTheUserIsBack(t *testing.T) {
	r := newAttentionRig(t, idlepolicy.AttentionPostpone)
	r.a.fail = fmt.Errorf("check for a face: %w", attention.ErrNoCamera)
	r.idleToTheDim(t)
	if want := []string{"dim"}; !slices.Equal(r.a.calls, want) {
		t.Fatalf("no camera ran %v, want %v", r.a.calls, want)
	}
	if got, want := afterOf(r.clock.pending()), []time.Duration{time.Minute, 6 * time.Minute}; !slices.Equal(got, want) {
		t.Errorf("no camera: followers after %v, want %v", got, want)
	}
	r.dimWatch(t).resume()
	waittest.For(t, r.changes, "the input")
	all := []time.Duration{4 * time.Minute, 5 * time.Minute, 10 * time.Minute}
	if got := afters(r.w.live()); !slices.Equal(got, all) {
		t.Fatalf("after no camera, watches %v, want every stage's %v", got, all)
	}
	r.dimWatch(t).idle()
	p := testPolicy(t)
	p.Attention = idlepolicy.AttentionSooner
	if err := r.s.Configure(p, true, idlepolicy.AC); err != nil {
		t.Fatal(err)
	}
	if got := afters(r.w.live()); !slices.Equal(got, all) {
		t.Errorf("attention configured again, watches %v, want every stage's %v", got, all)
	}
	r.dimWatch(t).idle()
	if r.a.looks != 1 {
		t.Errorf("%d checks, want only the one that found no camera", r.a.looks)
	}
	if want := []string{"dim", "undim", "dim", "undim", "dim"}; !slices.Equal(r.a.calls, want) {
		t.Errorf("ran %v, want %v", r.a.calls, want)
	}
}

func TestAFailedCheckIsAskedAgain(t *testing.T) {
	r := newAttentionRig(t, idlepolicy.AttentionPostpone)
	r.a.fail = errors.New("the camera vanished")
	r.idleToTheDim(t)
	r.dimWatch(t).resume()
	waittest.For(t, r.changes, "the input")
	if got, want := afters(r.w.live()), []time.Duration{4 * time.Minute}; !slices.Equal(got, want) {
		t.Fatalf("after a failed check, watches %v, want the dim's alone", got)
	}
	r.idleToTheDim(t)
	if r.a.looks != 2 {
		t.Errorf("%d checks, want a failed check asked again", r.a.looks)
	}
}

func TestLookWithoutTheSessionBusIsItsError(t *testing.T) {
	if _, err := (System{}).Look(t.Context()); !errors.Is(err, ErrNoAttention) {
		t.Errorf("Look without attention: %v, want ErrNoAttention", err)
	}
}
