package widgets

import (
	"fmt"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/waittest"

	"github.com/stubbedev/lys/desktop/fakeclock"
	"github.com/stubbedev/lys/desktop/shell/treetest"
)

const quietWindow = 200 * time.Millisecond

// testLoop stands in for the application loop: Invoke queues, and the
// loop goroutine runs each function under the lock the test reads with.
type testLoop struct {
	mu sync.Mutex
}

func (l *testLoop) invoke(fn func()) {
	go func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		fn()
	}()
}

// wait polls cond under the loop lock until it holds.
func (l *testLoop) wait(t *testing.T, what string, cond func() bool) {
	t.Helper()
	waittest.Until(t, func() bool {
		l.mu.Lock()
		defer l.mu.Unlock()
		return cond()
	}, what)
}

func testFont(t *testing.T) render.Font { return treetest.Font(t) }

func newTestSlider(t *testing.T) (*DebouncedSlider, *[]float64, *fakeclock.Clock) {
	t.Helper()
	return newTestSliderOn(t, func(fn func()) { fn() })
}

func newTestSliderOn(t *testing.T, invoke func(func())) (*DebouncedSlider, *[]float64, *fakeclock.Clock) {
	t.Helper()
	clock := fakeclock.New()
	commits := &[]float64{}
	d := NewDebouncedSlider(50, testFont(t), 12, 0, invoke, clock)
	d.OnCommit = func(v float64) { *commits = append(*commits, v) }
	return d, commits, clock
}

// The first move commits at once; moves inside 100ms leave one trailing
// commit of the last value; the release commits the final value.
func TestDebouncedSliderThrottlesAndCommitsOnRelease(t *testing.T) {
	d, commits, clock := newTestSlider(t)
	d.Knob.SetPressed(true)
	d.Knob.SetValue(60)
	d.Knob.SetValue(61)
	d.Knob.SetValue(62)
	if !slices.Equal(*commits, []float64{60}) {
		t.Fatalf("commits = %v, want only the first move", *commits)
	}
	if d.trailing == nil {
		t.Fatal("no trailing commit scheduled")
	}
	clock.Advance(150 * time.Millisecond)
	if !slices.Equal(*commits, []float64{60, 62}) || d.trailing != nil {
		t.Fatalf("after the throttle window commits = %v (trailing %v), want the last move's trailing commit", *commits, d.trailing != nil)
	}
	d.Knob.SetValue(70)
	if !slices.Equal(*commits, []float64{60, 62}) || d.trailing == nil {
		t.Fatalf("a move 50ms after the trailing commit committed at once: %v (trailing %v)", *commits, d.trailing != nil)
	}
	d.Knob.SetPressed(false)
	if !slices.Equal(*commits, []float64{60, 62, 70}) || d.trailing != nil {
		t.Fatalf("release commits = %v (trailing %v), want the final value once and no trailing commit left", *commits, d.trailing != nil)
	}
	if got := d.label.Text(); got != "70%" {
		t.Errorf("label = %q", got)
	}
}

func TestDebouncedSliderTrailingCommitLands(t *testing.T) {
	loop := &testLoop{}
	d, commits, clock := newTestSliderOn(t, loop.invoke)
	loop.mu.Lock()
	d.Knob.SetValue(10)
	d.Knob.SetValue(20)
	early := len(*commits)
	loop.mu.Unlock()
	if early != 1 {
		t.Fatalf("%d commits before the throttle passed, want only the first move", early)
	}
	clock.Advance(sliderThrottle)
	loop.wait(t, "the trailing commit", func() bool { return len(*commits) == 2 })
	if (*commits)[1] != 20 {
		t.Errorf("trailing = %v, want the last value", *commits)
	}
}

// External values are ignored while held and for 150ms after release,
// and never commit.
func TestDebouncedSliderIgnoresExternalValuesWhileInteracting(t *testing.T) {
	d, commits, clock := newTestSlider(t)
	d.Set(30)
	if d.Value() != 30 || len(*commits) != 0 {
		t.Fatalf("external set: value %v commits %v", d.Value(), *commits)
	}
	d.Knob.SetPressed(true)
	d.Set(80)
	if d.Value() != 30 {
		t.Error("an external value moved a held slider")
	}
	d.Knob.SetPressed(false)
	clock.Advance(100 * time.Millisecond)
	d.Set(80)
	if d.Value() != 30 {
		t.Error("an external value landed inside the release grace")
	}
	clock.Advance(100 * time.Millisecond)
	d.Set(80)
	if d.Value() != 80 {
		t.Error("an external value after the grace did not land")
	}
}

func TestDebouncedSliderResetBypassesTheReleaseGrace(t *testing.T) {
	d, commits, clock := newTestSlider(t)
	d.Knob.SetPressed(true)
	d.Knob.SetValue(80)
	d.Knob.SetPressed(false)
	clock.Advance(10 * time.Millisecond)
	d.Set(50)
	if d.Value() != 80 {
		t.Fatalf("an outside value inside the release grace moved the knob to %v", d.Value())
	}
	d.Reset(50)
	if d.Value() != 50 {
		t.Errorf("a reset right after the release left the knob at %v, want 50", d.Value())
	}
	if got := d.label.Text(); got != "50%" {
		t.Errorf("label = %q after the reset", got)
	}
	if !slices.Equal(*commits, []float64{80, 80}) {
		t.Errorf("commits = %v, want the drag and release only, none from the reset", *commits)
	}
}

func TestDebouncedSliderResetDropsTheTrailingCommit(t *testing.T) {
	loop := &testLoop{}
	d, commits, _ := newTestSliderOn(t, loop.invoke)
	loop.mu.Lock()
	d.Knob.SetValue(10)
	d.Knob.SetValue(20)
	d.Reset(50)
	loop.mu.Unlock()
	time.Sleep(2 * sliderThrottle)
	loop.mu.Lock()
	defer loop.mu.Unlock()
	if !slices.Equal(*commits, []float64{10}) || d.Value() != 50 {
		t.Errorf("commits %v value %v, want the reset to cancel the trailing 20", *commits, d.Value())
	}
}

// A ranged slider spans its own bounds and labels with its format.
func TestRangedSliderBoundsAndFormat(t *testing.T) {
	d := NewRangedSlider(500, 5000, 900, testFont(t), 12, 0, (&testLoop{}).invoke, fakeclock.New())
	d.SetFormat(func(v float64) string { return strconv.FormatFloat(v/1000, 'f', 1, 64) + "s" })
	if got := d.Label().Text(); got != "0.9s" {
		t.Errorf("label = %q, want the new format", got)
	}
	d.Set(9000)
	if d.Value() != 5000 {
		t.Errorf("value = %v, want clamped to the max", d.Value())
	}
	if NewDebouncedSlider(0, nil, 0, 0, nil, fakeclock.New()).Label() != nil {
		t.Error("a fontless slider has a label")
	}
}

// Write runs one write at a time off the loop, the latest commit
// replacing the ones queued behind a slow write; outside values stay
// out until the last write has landed and the release grace after it
// has passed.
func TestDebouncedSliderWritesInOrderLatestWins(t *testing.T) {
	loop := &testLoop{}
	d, _, clock := newTestSliderOn(t, loop.invoke)
	var mu sync.Mutex
	var writes []float64
	inflight, most := 0, 0
	gate := make(chan struct{})
	var write func(float64)
	d.Write = func(v float64) func() { return func() { write(v) } }
	write = func(v float64) {
		mu.Lock()
		writes = append(writes, v)
		inflight++
		most = max(most, inflight)
		mu.Unlock()
		<-gate
		mu.Lock()
		inflight--
		mu.Unlock()
	}
	written := func() []float64 {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(writes)
	}
	waitWrites := func(want ...float64) {
		t.Helper()
		waittest.Until(t, func() bool { return slices.Equal(written(), want) }, fmt.Sprintf("the writes %v", want))
	}

	loop.mu.Lock()
	d.Knob.SetPressed(true)
	d.Knob.SetValue(60)
	loop.mu.Unlock()
	waitWrites(60)

	loop.mu.Lock()
	clock.Advance(150 * time.Millisecond)
	d.Knob.SetValue(70)
	clock.Advance(150 * time.Millisecond)
	d.Knob.SetValue(80)
	d.Knob.SetPressed(false)
	loop.mu.Unlock()

	time.Sleep(quietWindow)
	if got := written(); !slices.Equal(got, []float64{60}) {
		t.Fatalf("writes = %v while the first was still running, want it alone", got)
	}

	loop.mu.Lock()
	clock.Advance(time.Second)
	d.Set(10)
	if d.Value() != 80 {
		t.Error("an outside value landed while a write was pending")
	}
	loop.mu.Unlock()

	gate <- struct{}{}
	waitWrites(60, 80)
	gate <- struct{}{}
	loop.wait(t, "the writes to land", func() bool { return !d.writePending() && !d.landed.IsZero() })
	if most != 1 {
		t.Errorf("%d writes ran at once, want one at a time", most)
	}

	loop.mu.Lock()
	d.Set(10)
	if d.Value() != 80 {
		t.Error("an outside value landed inside the grace after the last write")
	}
	clock.Advance(200 * time.Millisecond)
	d.Set(10)
	if d.Value() != 10 {
		t.Error("an outside value after the grace did not land")
	}
	loop.mu.Unlock()
}
