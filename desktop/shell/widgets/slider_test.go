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
	"github.com/stubbedev/lys/ui/widget"

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
	if got := d.ValueLabel().Text(); got != "0.9s" {
		t.Errorf("label = %q, want the new format", got)
	}
	d.Set(9000)
	if d.Value() != 5000 {
		t.Errorf("value = %v, want clamped to the max", d.Value())
	}
}

// A percentage slider shows its value in its label, whole percent by
// default, the label tracking the knob.
func TestDebouncedSliderShowsItsValueInItsLabel(t *testing.T) {
	d := NewDebouncedSlider(42, testFont(t), 12, 0, func(fn func()) { fn() }, fakeclock.New())
	if lo, hi := d.Knob.Range(); lo != 0 || hi != 100 || d.ValueLabel() == nil || d.ValueLabel().Text() != "42%" {
		t.Fatalf("range [%v, %v], label %v, want [0, 100] showing 42%%", lo, hi, d.ValueLabel())
	}
	d.Knob.SetValue(7)
	if got := d.ValueLabel().Text(); got != "7%" {
		t.Errorf("label = %q after a move to 7, want 7%%", got)
	}
}

// A labelled slider without a font is refused at construction, naming
// the constructor for a slider without a label.
func TestALabelledSliderRefusesANilFont(t *testing.T) {
	for name, build := range map[string]func(){
		"NewDebouncedSlider": func() { NewDebouncedSlider(0, nil, 12, 0, func(fn func()) { fn() }, fakeclock.New()) },
		"NewRangedSlider":    func() { NewRangedSlider(0, 10, 0, nil, 12, 0, func(fn func()) { fn() }, fakeclock.New()) },
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				const want = "widgets: a slider's value label needs a font; NewUnlabeledSlider builds one without a label"
				if got := recover(); got != want {
					t.Errorf("panic = %v, want %q", got, want)
				}
			}()
			build()
		})
	}
}

// An unlabelled slider is a 0-100 knob alone in its row, with no
// reading beside it, that still commits, takes outside values and
// takes a format or a range without a label to render them in.
func TestAnUnlabeledSliderIsTheKnobAlone(t *testing.T) {
	var commits []float64
	d := NewUnlabeledSlider(30, func(fn func()) { fn() }, fakeclock.New())
	d.OnCommit = func(v float64) { commits = append(commits, v) }
	sz := widget.SizeWithin(d, widget.Size{W: 1000, H: 100})
	d.Arrange(render.Rect{W: sz.W, H: sz.H})
	if lo, hi := d.Knob.Range(); lo != 0 || hi != 100 || d.Value() != 30 {
		t.Fatalf("range [%v, %v] at %v, want [0, 100] at 30", lo, hi, d.Value())
	}
	if kids := d.Children(); d.ValueLabel() != nil || len(kids) != 1 || d.Knob.Bounds().W != sz.W {
		t.Errorf("label %v, %d children, knob %dpx of %dpx; want the knob alone across the row", d.ValueLabel(), len(kids), d.Knob.Bounds().W, sz.W)
	}
	d.Knob.SetValue(60)
	d.SetFormat(func(v float64) string { return strconv.FormatFloat(v, 'f', 0, 64) + " s" })
	d.SetRange(0, 50)
	d.Set(20)
	if !slices.Equal(commits, []float64{60}) || d.Value() != 20 {
		t.Errorf("commits %v at %v, want the one move committed and the outside 20 shown", commits, d.Value())
	}
}

// An unlabelled slider needs a clock like every other.
func TestAnUnlabeledSliderRefusesANilClock(t *testing.T) {
	defer func() {
		if got := recover(); got != "widgets: a slider needs a clock" {
			t.Errorf("panic = %v, want the missing clock named", got)
		}
	}()
	NewUnlabeledSlider(0, func(fn func()) { fn() }, nil)
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

// reading lays d out and returns its reading's bounds and where its
// knob ends.
func reading(d *DebouncedSlider) (render.Rect, int) {
	big := widget.Size{W: 1000, H: 100}
	sz := widget.SizeWithin(d, big)
	d.Arrange(render.Rect{W: sz.W, H: sz.H})
	kids := d.Children()
	knob := d.Knob.Bounds()
	return kids[len(kids)-1].(widget.Boundser).Bounds(), knob.X + knob.W
}

// measured is text's width in font at 12px.
func measured(font render.Font, text string) int {
	return widget.SizeWithin(widget.NewLabel(font, 12, text, 0), widget.Size{W: 1000, H: 100}).W
}

// The value label reserves the width of the widest of its range's ends
// as they render, measured in its face, not counted in bytes or
// approximate characters: a degree sign or a kelvin unit reserves what
// it draws.
func TestTheValueLabelReservesTheWidestEndsMeasuredWidth(t *testing.T) {
	font := testFont(t)
	for _, c := range []struct {
		min, max float64
		unit     string
		widest   string
	}{{0, 360, "°", "360°"}, {1000, 6500, " K", "6500 K"}, {0, 100, " K", "100 K"}} {
		d := NewRangedSlider(c.min, c.max, c.min, font, 12, 0, func(fn func()) { fn() }, fakeclock.New())
		d.SetFormat(func(v float64) string { return strconv.FormatFloat(v, 'f', 0, 64) + c.unit })
		if got, _ := reading(d); got.W != measured(font, c.widest) {
			t.Errorf("%q to %q reserves %dpx, want %q's measured %dpx", d.format(c.min), d.format(c.max), got.W, c.widest, measured(font, c.widest))
		}
	}
}

// A format whose ends differ in width still holds one column as the
// value moves: the reading keeps the widest end's width and the knob
// its end, and a class reaches the ends with the label.
func TestTheValueColumnHoldsStillAsTheValueChanges(t *testing.T) {
	font := testFont(t)
	d := NewRangedSlider(0, 1000, 0, font, 12, 0, func(fn func()) { fn() }, fakeclock.New())
	d.SetFormat(func(v float64) string { return strconv.FormatFloat(v, 'f', 0, 64) + " ms" })
	want, knobEnd := reading(d)
	if want.W != measured(font, "1000 ms") || want.W == measured(font, "0 ms") {
		t.Fatalf("at 0 the reading is %dpx, want 1000 ms's %dpx", want.W, measured(font, "1000 ms"))
	}
	for _, v := range []float64{1000, 7, 450} {
		d.Set(v)
		if got, end := reading(d); got != want || end != knobEnd {
			t.Errorf("at %v the reading is %v and the knob ends at %d, want %v and %d", v, got, end, want, knobEnd)
		}
	}
	d.ValueLabel().AddClass("brightness-slider-value")
	for _, l := range d.label.ends {
		if !l.HasClass("brightness-slider-value") {
			t.Errorf("the end %q missed the class the label took", l.Text())
		}
	}
	d.SetRange(0, 50)
	if got, _ := reading(d); d.Value() != 50 || got.W != measured(font, "50 ms") {
		t.Errorf("a range narrowed to 50: value %v, reading %dpx; want 50 in 50 ms's %dpx", d.Value(), got.W, measured(font, "50 ms"))
	}
}

// A field slider reads by the caller's field, where a value label would
// sit: the theme's spacing clear of the knob at its maximum, which
// reaches past the slider's end, not of the slider's bounds (#262).
func TestAFieldSliderReadsByItsField(t *testing.T) {
	field := widget.NewSpinButton(testFont(t), 12, 0, 0, 5000, 1, 0)
	d := NewFieldSlider(0, 1000, 300, field, func(fn func()) { fn() }, fakeclock.New())
	got, knobEnd := reading(d)
	over := d.Knob.Overhang()
	if over <= 0 {
		t.Fatalf("the knob reaches %dpx past the slider, want the theme's knob to reach past it", over)
	}
	if d.ValueLabel() != nil || got.W != field.Bounds().W || got.X-(knobEnd+over) != widget.Current().Spacing {
		t.Errorf("the field spans %v after a knob reaching to %d, want the spin %dpx wide %dpx after it", got, knobEnd+over, field.Bounds().W, widget.Current().Spacing)
	}
}
