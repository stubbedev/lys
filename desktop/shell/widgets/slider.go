// Package widgets holds the controls the shell surfaces and the
// settings app share.
package widgets

import (
	"strconv"
	"sync"
	"time"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

// DebouncedSlider's cadences.
const (
	sliderThrottle = 100 * time.Millisecond
	sliderGrace    = 150 * time.Millisecond
)

// Clock is the time a slider throttles by: now, and fn run once d has
// passed, which the stop it returns cancels. The slider hands fn's work
// to its invoke, so the clock may call fn on any goroutine.
type Clock interface {
	Now() time.Time
	AfterFunc(d time.Duration, fn func()) (stop func() bool)
}

// DebouncedSlider is a slider read by a value label, by a field of the
// caller's, or by nothing, that commits (backend writes) at most every
// 100ms while dragged, with a trailing commit of the last value and one
// on release, and ignores external values while held and for 150ms
// after. The reading sits the theme's spacing after the knob, in the
// width the readings of its widget.ItemGroup share (widget.ItemValue),
// so the knobs of one group end at one x.
type DebouncedSlider struct {
	*widget.Box
	// Knob is the slider itself (class it, read it).
	Knob *pressSlider
	// label is the value label, nil for a slider without one.
	label *ValueText
	// format renders the value label (default: whole percent, "42%").
	format func(float64) string
	invoke func(func())

	// OnCommit hears the throttled user commits on the loop.
	OnCommit func(float64)
	// Write turns a committed value, on the loop, into the backend write
	// that applies it (nil writes nothing), run off the loop one at a
	// time, the latest commit's replacing any queued while one runs;
	// outside values are ignored until the last write has landed and
	// the release grace has passed after it. A slow backend (a display
	// over DDC/CI) otherwise lands concurrent writes out of order and
	// echoes stale values onto the slider.
	Write func(float64) func()
	// OnValue sees every value change, the user's and set's alike
	// (the value-changed notify a #[watch] reads).
	OnValue      func(float64)
	programmatic bool
	dragging     bool
	dragEnded    time.Time
	lastCommit   time.Time
	trailing     func() bool
	clock        Clock

	// wmu guards the write queue: want is the latest commit's write,
	// queued whether it awaits running, writing whether a drain runs.
	// landed is when the last drain finished (loop only).
	wmu     sync.Mutex
	want    func()
	queued  bool
	writing bool
	landed  time.Time
}

// pressSlider routes the press protocol to its owner.
type pressSlider struct {
	*widget.Slider
	owner *DebouncedSlider
}

// HitTest makes the wrapper the input leaf, so SetPressed reaches it.
func (s *pressSlider) HitTest(p widget.Point) widget.Widget { return s.HitLeaf(s, p) }

// CursorName is the hover pointer: the hand, like the wrapped slider
// asks (the wrapper is the input leaf, so its own answer wins).
func (s *pressSlider) CursorName() string { return "pointer" }

// SetPressed tracks the drag; the release commits the final value.
func (s *pressSlider) SetPressed(on bool) {
	was := s.Pressed
	s.Slider.SetPressed(on)
	d := s.owner
	switch {
	case on && !was:
		d.dragging, d.dragEnded = true, time.Time{}
	case !on && was:
		d.dragging, d.dragEnded = false, d.clock.Now()
		d.cancelTrailing()
		d.commit(s.Value())
	}
}

// NewDebouncedSlider builds a 0-100 slider (every lys-shell slider is a
// percentage) read by a value label in font (SetFormat, default whole
// percent). clock times the throttle and invoke runs the trailing
// commit on the loop.
func NewDebouncedSlider(value float64, font render.Font, px float64, color render.Color, invoke func(func()), clock Clock) *DebouncedSlider {
	return NewRangedSlider(0, 100, value, font, px, color, invoke, clock)
}

// NewRangedSlider is NewDebouncedSlider over [min, max].
func NewRangedSlider(min, max, value float64, font render.Font, px float64, color render.Color, invoke func(func()), clock Clock) *DebouncedSlider {
	if font == nil {
		panic("widgets: a slider's value label needs a font; NewUnlabeledSlider builds one without a label")
	}
	d := newDebouncedSlider(min, max, value, invoke, clock)
	d.format = func(v float64) string { return strconv.FormatFloat(v, 'f', 0, 64) + "%" }
	d.label = newValueText(font, px, color)
	d.label.show(d.format(value), d.ends())
	d.read(d.label.stack)
	return d
}

// NewUnlabeledSlider is a 0-100 slider read by nothing beside it: the
// knob spans the whole row (a seek bar, a per-app volume).
func NewUnlabeledSlider(value float64, invoke func(func()), clock Clock) *DebouncedSlider {
	return newDebouncedSlider(0, 100, value, invoke, clock)
}

// NewFieldSlider is a slider over [min, max] read by field, a control
// the caller keeps showing the value (a spin button that also types
// values past the range), in the place and the shared width a value
// label takes.
func NewFieldSlider(min, max, value float64, field widget.Widget, invoke func(func()), clock Clock) *DebouncedSlider {
	d := newDebouncedSlider(min, max, value, invoke, clock)
	d.read(field)
	return d
}

func newDebouncedSlider(min, max, value float64, invoke func(func()), clock Clock) *DebouncedSlider {
	if clock == nil {
		panic("widgets: a slider needs a clock")
	}
	d := &DebouncedSlider{Box: widget.NewBox(widget.Row, widget.Current().Spacing, 0), invoke: invoke, clock: clock}
	d.Knob = &pressSlider{Slider: widget.NewSlider(min, max, 0, value), owner: d}
	d.Knob.OnChanged = d.changed
	d.Append(d.Knob, true)
	return d
}

// read places w, the slider's reading, after the knob.
func (d *DebouncedSlider) read(w widget.Widget) {
	d.AppendAligned(widget.NewItemValue(w), false, widget.AlignCenter)
}

// ends are the range's ends as the value label renders them.
func (d *DebouncedSlider) ends() [2]string {
	lo, hi := d.Knob.Range()
	return [2]string{d.format(lo), d.format(hi)}
}

// ValueText is a slider's value label over the range's two ends,
// rendered alike and never shown: it is as wide as the widest of them
// in the face and size the label shows in, so its column holds still
// as the value changes. A class goes on all three.
type ValueText struct {
	stack *widget.Stack
	shown *widget.Label
	ends  [2]*widget.Label
}

// The pages of a value label's stack.
const (
	valueShown = "value"
	valueMin   = "min"
	valueMax   = "max"
)

func newValueText(font render.Font, px float64, color render.Color) *ValueText {
	v := &ValueText{stack: widget.NewStack(), shown: widget.NewLabel(font, px, "", color)}
	v.shown.SetAlignment(render.AlignEnd)
	v.stack.Add(valueShown, v.shown)
	for i, name := range [...]string{valueMin, valueMax} {
		v.ends[i] = widget.NewLabel(font, px, "", color)
		v.stack.Add(name, v.ends[i])
	}
	return v
}

// show renders text with the range's ends rendered as ends.
func (v *ValueText) show(text string, ends [2]string) {
	v.shown.SetText(text)
	for i, l := range v.ends {
		l.SetText(ends[i])
	}
}

// AddClass classes the label, for the caller's stylesheet to name it
// (audio-slider-value, brightness-slider-value, ...).
func (v *ValueText) AddClass(names ...string) {
	v.shown.AddClass(names...)
	for _, l := range v.ends {
		l.AddClass(names...)
	}
}

// Text is the value shown.
func (v *ValueText) Text() string { return v.shown.Text() }

// ValueLabel hands back the value label, nil for a slider without one.
func (d *DebouncedSlider) ValueLabel() *ValueText { return d.label }

// changed is the value_changed handler: user moves update the label
// and commit through the throttle; programmatic ones do not commit.
func (d *DebouncedSlider) changed(v float64) {
	if d.label != nil {
		d.label.shown.SetText(d.format(v))
	}
	if d.OnValue != nil {
		d.OnValue(v)
	}
	if d.programmatic {
		return
	}
	if d.lastCommit.IsZero() || d.clock.Now().Sub(d.lastCommit) >= sliderThrottle {
		d.cancelTrailing()
		d.commit(v)
		return
	}
	d.cancelTrailing()
	d.trailing = d.clock.AfterFunc(sliderThrottle, func() {
		d.invoke(func() {
			d.trailing = nil
			d.commit(v)
		})
	})
}

func (d *DebouncedSlider) commit(v float64) {
	d.lastCommit = d.clock.Now()
	if d.OnCommit != nil {
		d.OnCommit(v)
	}
	if d.Write != nil {
		if write := d.Write(v); write != nil {
			d.enqueue(write)
		}
	}
}

// enqueue queues write, starting a drain when none runs.
func (d *DebouncedSlider) enqueue(write func()) {
	d.wmu.Lock()
	d.want, d.queued = write, true
	start := !d.writing
	d.writing = true
	d.wmu.Unlock()
	if start {
		go d.drain()
	}
}

// drain runs the latest queued write until none is left, then marks
// the writes landed on the loop.
func (d *DebouncedSlider) drain() {
	for {
		d.wmu.Lock()
		if !d.queued {
			d.writing = false
			d.wmu.Unlock()
			d.invoke(func() { d.landed = d.clock.Now() })
			return
		}
		write := d.want
		d.want, d.queued = nil, false
		d.wmu.Unlock()
		write()
	}
}

// writePending reports whether a write runs or awaits.
func (d *DebouncedSlider) writePending() bool {
	d.wmu.Lock()
	defer d.wmu.Unlock()
	return d.writing || d.queued
}

func (d *DebouncedSlider) cancelTrailing() {
	if d.trailing != nil {
		d.trailing()
		d.trailing = nil
	}
}

// interacting is user_is_interacting: held, writing, or released (or
// its last write landed) under 150ms ago.
func (d *DebouncedSlider) interacting() bool {
	if d.dragging || d.writePending() {
		return true
	}
	since := d.dragEnded
	if d.landed.After(since) {
		since = d.landed
	}
	return !since.IsZero() && d.clock.Now().Sub(since) < sliderGrace
}

// Set is set_value_external: an outside value, ignored while the user
// interacts.
func (d *DebouncedSlider) Set(v float64) {
	if d.interacting() {
		return
	}
	d.setQuietly(v)
}

// Reset moves the knob to v at once, past the hold Set keeps while the
// user interacts, and drops a pending trailing commit: for a value the
// backend refused, which must not stay where the user dropped it.
func (d *DebouncedSlider) Reset(v float64) {
	d.cancelTrailing()
	d.setQuietly(v)
}

func (d *DebouncedSlider) setQuietly(v float64) {
	d.programmatic = true
	d.Knob.SetValue(v)
	d.programmatic = false
}

// Value is the slider's current value.
func (d *DebouncedSlider) Value() float64 { return d.Knob.Value() }

// SetFormat replaces the value label's format and re-renders it.
func (d *DebouncedSlider) SetFormat(format func(float64) string) {
	d.format = format
	if d.label != nil {
		d.label.show(format(d.Value()), d.ends())
	}
}

// SetRange moves the slider's ends to [min, max], clamping the value
// into them without committing it.
func (d *DebouncedSlider) SetRange(min, max float64) {
	d.programmatic = true
	d.Knob.SetRange(min, max)
	d.programmatic = false
	if d.label != nil {
		d.label.show(d.format(d.Value()), d.ends())
	}
}

// SetClock replaces the clock the throttle, the trailing commit and the
// release grace run on.
func (d *DebouncedSlider) SetClock(c Clock) {
	if c == nil {
		panic("widgets: a slider needs a clock")
	}
	d.clock = c
}
