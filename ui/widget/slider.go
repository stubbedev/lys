package widget

import (
	"math"
	"slices"

	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/ui/internal/style"
)

// Slider is a slider over [min, max] with an optional step, horizontal
// by default; SetAxis(Column) stands it up (GTK's orientation), the
// minimum at the top unless inverted (GTK's inverted, the volume-style
// minimum at the bottom). Pressing warps the value to the pointer and a
// drag follows it.
//
// It is GtkScale's node tree: `scale` (its box, opacity, outline, and
// :hover while the pointer is over it) holding `trough` (background,
// radius, min-height, and min-width as the natural width) holding
// `highlight` (the filled part) and `slider` (the knob: background,
// radius, min size, opacity, box-shadow). Unstyled it keeps the theme's
// 4px trough and 14px knob. The scale carries .horizontal or .vertical;
// across a vertical scale the trough's min-width is its thickness and
// min-height its length.
type Slider struct {
	node
	min, max, step, value float64
	axis                  Axis
	inverted              bool

	// Pressed reports whether a drag is in progress.
	Pressed bool
	hovered bool
	trough  sliderTrough
	track   []render.Color

	// OnChanged fires after every value change, including programmatic
	// ones.
	OnChanged func(v float64)
}

// NewSlider returns a horizontal slider over [min, max] at value, with
// changes snapped to step (0 disables snapping). The pointer turns to
// the hand over it, the way wayle's slider primitives ask.
func NewSlider(min, max, step, value float64) *Slider {
	s := &Slider{min: min, max: max, step: step}
	s.cursorName = cursorPointer
	s.value = s.clamp(value)
	s.AddClass("horizontal")
	s.trough.SetElement("trough")
	s.trough.highlight.SetElement("highlight")
	s.trough.knob.SetElement("slider")
	setParents(s, &s.trough)
	setParents(&s.trough, &s.trough.highlight, &s.trough.knob)
	return s
}

// SetAxis orients the slider: Row horizontal, Column vertical.
func (s *Slider) SetAxis(axis Axis) {
	if s.axis == axis {
		return
	}
	s.axis = axis
	if axis == Column {
		s.RemoveClass("horizontal")
		s.AddClass("vertical")
	} else {
		s.RemoveClass("vertical")
		s.AddClass("horizontal")
	}
	s.InvalidateLayout()
}

// Axis reports the orientation.
func (s *Slider) Axis() Axis { return s.axis }

// SetInverted puts the minimum at the far end: the right of a
// horizontal slider, the bottom of a vertical one.
func (s *Slider) SetInverted(on bool) {
	s.inverted = on
	s.Invalidate()
}

// Inverted reports whether the minimum is at the far end.
func (s *Slider) Inverted() bool { return s.inverted }

// SetTrack paints the trough as an even gradient through colors, from
// the minimum's end to the maximum's, with no highlight over it: a
// scale whose values are colours (a temperature, a hue) shows them.
// None clears it back to the styled trough and highlight; disabled, the
// slider paints its plain disabled trough.
func (s *Slider) SetTrack(colors ...render.Color) {
	s.track = slices.Clone(colors)
	s.Invalidate()
}

// trackGradient is the track along the slider's axis and direction.
func (s *Slider) trackGradient() render.Gradient {
	stops := make([]render.GradientStop, len(s.track))
	last := float64(max(len(s.track)-1, 1))
	for i, c := range s.track {
		stops[i] = render.GradientStop{Pos: float64(i) / last, Color: c}
	}
	angle := 90.0
	if s.axis == Column {
		angle = 180
	}
	if s.inverted {
		angle += 180
	}
	return render.Linear(angle, stops...)
}

// sliderTrough is the scale's trough node and its highlight and knob.
type sliderTrough struct {
	stylePart
	highlight, knob stylePart
}

func (t *sliderTrough) appendStyleKids(buf []Widget) []Widget {
	return append(buf, &t.highlight, &t.knob)
}

// appendStyleKids is the trough (styleKids).
func (s *Slider) appendStyleKids(buf []Widget) []Widget { return append(buf, &s.trough) }

// SetHovered implements HoverSetter: `scale:hover` restyles the
// trough and knob (a knob shown only on hover).
func (s *Slider) SetHovered(on bool) {
	if s.hovered != on {
		s.hovered = on
		s.invalidateState(style.Hover)
	}
}

// sliderPad is an unstyled trough's inset each side, room for the
// knob at the extremes.
const sliderPad = 4

// troughProps are the trough's length and thickness properties along
// the slider's axis.
func (s *Slider) troughProps() (length, thickness style.Prop) {
	if s.axis == Column {
		return style.PropMinHeight, style.PropMinWidth
	}
	return style.PropMinWidth, style.PropMinHeight
}

// troughRect is the trough inside the content box: its thickness (6px
// unstyled) centered across the axis, the content's length along it
// (less the unstyled inset).
func (s *Slider) troughRect() render.Rect {
	_, c := boxRects(boxOf(s.style(s), render.Insets{}), s.bounds)
	th, pad := s.troughThickness(), s.troughPad()
	if s.axis == Column {
		return render.Rect{X: c.X + (c.W-th)/2, Y: c.Y + pad, W: th, H: max(c.H-2*pad, 0)}
	}
	return render.Rect{X: c.X + pad, Y: c.Y + (c.H-th)/2, W: max(c.W-2*pad, 0), H: th}
}

// troughThickness is the trough's thickness across the axis, 6px
// unstyled.
func (s *Slider) troughThickness() int {
	_, thickProp := s.troughProps()
	return picki(s.trough.style(&s.trough), thickProp, 6)
}

// troughPad is the trough's inset each end inside the content box:
// sliderPad unstyled, none once the stylesheet sizes or fills it.
func (s *Slider) troughPad() int {
	tv := s.trough.style(&s.trough)
	if _, thickProp := s.troughProps(); tv.Has(thickProp) || tv.Has(style.PropBackgroundColor) {
		return 0
	}
	return sliderPad
}

// Overhang is how far the knob reaches past the slider's end edge at
// the maximum (and past its start at the minimum): half the knob, less
// the trough's inset and the slider's own box. A neighbour keeps its
// spacing clear of the knob by adding it.
func (s *Slider) Overhang() int {
	const probe = 1 << 16
	_, c := boxRects(boxOf(s.style(s), render.Insets{}), render.Rect{W: probe, H: probe})
	kw, kh := s.knobSize()
	half, inset := kw/2, probe-c.X-c.W
	if s.axis == Column {
		half, inset = kh/2, probe-c.Y-c.H
	}
	return max(half-s.troughPad()-inset, 0)
}

// along maps the trough onto the value axis: the pixel the minimum
// sits at and the signed length to the maximum.
func (s *Slider) along(tr render.Rect) (origin, span int) {
	origin, span = tr.X, tr.W
	if s.axis == Column {
		origin, span = tr.Y, tr.H
	}
	// Vertical runs top-down by default; inverted flips either axis.
	if s.inverted {
		return origin + span, -span
	}
	return origin, span
}

// knobSize is the knob's min size (14px unstyled).
func (s *Slider) knobSize() (w, h int) {
	kv := s.trough.knob.style(&s.trough.knob)
	return picki(kv, style.PropMinWidth, 16), picki(kv, style.PropMinHeight, 16)
}

// Value returns the current value.
func (s *Slider) Value() float64 {
	return s.value
}

// SetValue clamps v to [min, max], snaps it to step, and fires OnChanged
// when it moved.
func (s *Slider) SetValue(v float64) {
	v = s.clamp(v)
	if v == s.value {
		return
	}
	s.value = v
	s.Invalidate()
	if s.OnChanged != nil {
		s.OnChanged(v)
	}
}

// Range returns the bounds.
func (s *Slider) Range() (min, max float64) { return s.min, s.max }

// SetRange changes the bounds to [min, max] and clamps the value into
// them, firing OnChanged when it moved.
func (s *Slider) SetRange(min, max float64) {
	s.min, s.max = min, max
	s.Invalidate()
	s.SetValue(s.value)
}

func (s *Slider) clamp(v float64) float64 {
	if s.step > 0 {
		v = math.Round(v/s.step) * s.step
	}
	return math.Min(s.max, math.Max(s.min, v))
}

// ValueAt maps a point across the trough to a value (an unstyled
// trough is inset 4px each end so the extremes are reachable).
func (s *Slider) ValueAt(p Point) float64 {
	origin, span := s.along(s.troughRect())
	if span == 0 {
		return s.min
	}
	pos := p.X
	if s.axis == Column {
		pos = p.Y
	}
	t := float64(pos-origin) / float64(span)
	t = math.Min(1, math.Max(0, t))
	return s.min + float64(t*(s.max-s.min))
}

// Measure wants the trough's length (200px unstyled) by the thicker
// of the trough and the knob, at least 18px, inside the scale's box.
func (s *Slider) Measure(axis Axis, forSize int) Sizes {
	if sz, ok := s.measureHit(axis, forSize); ok {
		return sz
	}
	v := s.style(s)
	tv := s.trough.style(&s.trough)
	kw, kh := s.knobSize()
	lenProp, thickProp := s.troughProps()
	length := picki(tv, lenProp, 200)
	sz := Size{W: length, H: max(18, picki(tv, thickProp, 4), kh)}
	if s.axis == Column {
		sz = Size{W: max(18, picki(tv, thickProp, 4), kw), H: length}
	}
	return s.measureStore(axis, forSize, measureBox(v, boxOf(v, render.Insets{}), axis, forSize, func(axis Axis, forSize int) Sizes {
		return FixedSizes(sz.Along(axis))
	}))
}

// troughAlpha is the unfilled trough's share of the text color, which
// shows the track on whatever background the slider sits on, as
// libadwaita's alpha(currentColor) trough does.
const troughAlpha = 0.2

// Paint draws the trough, its highlight, and the knob, each from its
// node's style over the theme's colors. Disabled, the theme colors fade
// through the derived disabled colors (a stylesheet fades with its own
// :disabled rules).
func (s *Slider) Paint(cv *render.Canvas) {
	v := s.style(s)
	enabled := IsEnabled(s)
	fx := pushEffects(cv, v, s.bounds, render.Corners{})
	cv = fx.cv
	defer fx.pop()
	tr := s.troughRect()
	origin, span := s.along(tr)
	at := origin + int(float64(span)*s.fraction())
	if len(s.track) > 0 && enabled {
		cv.PaintGradient(tr, s.troughRadii(tr), s.trackGradient())
	} else {
		s.paintTrough(cv, tr, enabled)
		s.paintFill(cv, s.highlightRect(tr, origin, at), enabled)
	}
	kw, kh := s.knobSize()
	knob := render.Rect{X: at - kw/2, Y: tr.Y + tr.H/2 - kh/2, W: kw, H: kh}
	if s.axis == Column {
		knob = render.Rect{X: tr.X + tr.W/2 - kw/2, Y: at - kh/2, W: kw, H: kh}
	}
	s.paintKnob(cv, knob, enabled)
	paintOutline(cv, v, s.bounds, radiusOr(v, 0))
}

// troughRadii are the trough node's corners over a pill's, for tr.
func (s *Slider) troughRadii(tr render.Rect) render.Corners {
	return radiusOr(s.trough.style(&s.trough), min(tr.W, tr.H)/2)
}

// paintTrough paints the unfilled trough over tr: the trough node's box
// over the text color at troughAlpha, fading further disabled.
func (s *Slider) paintTrough(cv *render.Canvas, tr render.Rect, enabled bool) {
	t, tv := Current(), s.trough.style(&s.trough)
	col := scaleAlpha(t.Text, troughAlpha)
	if !enabled {
		col = scaleAlpha(t.Text, troughAlpha*disabledFade)
	}
	paintBoxBehind(cv, tv, tr, s.troughRadii(tr), borderOf(tv), pickc(0, tv, style.PropBackgroundColor, col))
}

// paintFill paints the trough's highlight over r: the highlight node's
// box over the slider's accent, the derived disabled accent disabled.
func (s *Slider) paintFill(cv *render.Canvas, r render.Rect, enabled bool) {
	if r.Empty() {
		return
	}
	t := Current()
	col := t.Accent
	if !enabled {
		col = t.DisabledAccent()
	}
	hv := s.trough.highlight.style(&s.trough.highlight)
	radius := s.troughRadii(r).TopLeft
	paintBoxBehind(cv, hv, r, radiusOr(hv, radius), borderOf(hv), pickc(0, hv, style.PropBackgroundColor, accentOf(s.style(s), col)))
}

// paintKnob paints the knob over r, the knob node's box over the text
// color; disabled, the whole knob fades, keeping its colors, as the
// trough does.
func (s *Slider) paintKnob(cv *render.Canvas, r render.Rect, enabled bool) {
	if !enabled {
		defer cv.PopAlpha(cv.PushAlpha(disabledFade))
	}
	kv := s.trough.knob.style(&s.trough.knob)
	radii := radiusOr(kv, min(r.W, r.H)/2)
	kfx := pushEffects(cv, kv, r, radii)
	paintBoxBehind(kfx.cv, kv, r, radii, borderOf(kv), pickc(0, kv, style.PropBackgroundColor, Current().Text))
	kfx.pop()
}

// highlightRect is the trough's highlight, from the minimum's end at
// origin to the knob at at; empty at the minimum.
func (s *Slider) highlightRect(tr render.Rect, origin, at int) render.Rect {
	lo, hi := min(origin, at), max(origin, at)
	if hi <= lo {
		return render.Rect{}
	}
	if s.axis == Column {
		return render.Rect{X: tr.X, Y: lo, W: tr.W, H: hi - lo}
	}
	return render.Rect{X: lo, Y: tr.Y, W: hi - lo, H: tr.H}
}

func (s *Slider) fraction() float64 {
	if s.max == s.min {
		return 0
	}
	return (s.value - s.min) / (s.max - s.min)
}

// Role implements Roleer.
func (s *Slider) Role() Role { return RoleSlider }

// HitTest returns the slider when p is inside its bounds.
func (s *Slider) HitTest(p Point) Widget {
	return s.HitLeaf(s, p)
}

// SetPressed implements PressSetter; the trough brightens while dragging.
func (s *Slider) SetPressed(on bool) {
	if s.Pressed == on {
		return
	}
	s.Pressed = on
	s.invalidateState(style.Active)
}

// PressAt implements PressAter: the value warps to the press, and a
// drag carries on from there (GTK's primary-button warp). Disabled
// sliders ignore it.
func (s *Slider) PressAt(p Point) {
	if IsEnabled(s) {
		s.SetValue(s.ValueAt(p))
	}
}

// DragMove sets the value from the pointer position while pressed.
// Disabled sliders ignore the drag.
func (s *Slider) DragMove(p Point) {
	if !IsEnabled(s) {
		return
	}
	s.SetValue(s.ValueAt(p))
}

// keyDirection is the value's direction for an arrow: the arrows
// along the axis move the knob the way they point (so the value's
// sign follows the orientation and inversion), the cross arrows keep
// up/right as more.
func (s *Slider) keyDirection(a KeyAction) float64 {
	d := 1.0
	if a == KeyLeft || a == KeyDown {
		d = -1
	}
	alongAxis := (s.axis == Row) == (a == KeyLeft || a == KeyRight)
	if !alongAxis {
		return d
	}
	if s.axis == Column {
		d = -d // down the screen is toward a vertical slider's far end
	}
	if s.inverted {
		d = -d
	}
	return d
}

// KeyAction implements KeyActionHandler: arrows nudge the value by one
// step (or a tenth of the range without one), Home and End jump.
// Disabled sliders ignore keys.
func (s *Slider) KeyAction(a KeyAction, mods Mods) {
	if !IsEnabled(s) {
		return
	}
	step := s.step
	if step <= 0 {
		step = (s.max - s.min) / 10
	}
	switch a {
	case KeyLeft, KeyRight, KeyUp, KeyDown:
		s.SetValue(s.value + float64(step*s.keyDirection(a)))
	case KeyHome:
		s.SetValue(s.min)
	case KeyEnd:
		s.SetValue(s.max)
	}
}

// SliderLook paints a slider's parts for a control drawn by hand (a
// range slider with two knobs, a threshold bar): the trough, its
// highlight and the knob exactly as Slider paints them, from the
// `scale trough`, `scale trough highlight` and `scale trough slider`
// rules as they cascade at its owner, enabled while the owner is.
type SliderLook struct {
	owner  Widget
	slider *Slider
}

// NewSliderLook returns the look of a slider styled at owner.
func NewSliderLook(owner Widget) *SliderLook {
	s := NewSlider(0, 1, 0, 0)
	SetStyleAnchor(s, owner)
	return &SliderLook{owner: owner, slider: s}
}

// PaintTrough paints the unfilled trough over r.
func (l *SliderLook) PaintTrough(cv *render.Canvas, r render.Rect) {
	l.slider.paintTrough(cv, r, IsEnabled(l.owner))
}

// PaintFill paints the trough's highlight over r, the span a value
// covers.
func (l *SliderLook) PaintFill(cv *render.Canvas, r render.Rect) {
	l.slider.paintFill(cv, r, IsEnabled(l.owner))
}

// PaintKnob paints a knob over r, KnobSize big.
func (l *SliderLook) PaintKnob(cv *render.Canvas, r render.Rect) {
	l.slider.paintKnob(cv, r, IsEnabled(l.owner))
}

// KnobSize is the knob's size.
func (l *SliderLook) KnobSize() Size {
	w, h := l.slider.knobSize()
	return Size{W: w, H: h}
}

// TroughThickness is the trough's thickness across a horizontal
// slider.
func (l *SliderLook) TroughThickness() int {
	return l.slider.troughThickness()
}
