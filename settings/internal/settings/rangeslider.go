package settings

import (
	"fmt"
	"math"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	rangeSliderH     = 60
	rangeFineShare   = 0.01
	rangeCoarseShare = 0.1
	rangeCoarseSteps = 10
)

type rangeAxis struct {
	lo, hi, step float64
	log          bool
}

func linearAxis(lo, hi, step float64) rangeAxis {
	if !(lo < hi) || !(step > 0) {
		panic(fmt.Sprintf("settings: linear axis %v-%v by %v", lo, hi, step))
	}
	return rangeAxis{lo: lo, hi: hi, step: step}
}

func logAxis(lo, hi, step float64) rangeAxis {
	if !(lo > 0) || !(lo < hi) || !(step > 0) {
		panic(fmt.Sprintf("settings: logarithmic axis %v-%v by %v", lo, hi, step))
	}
	return rangeAxis{lo: lo, hi: hi, step: step, log: true}
}

func (a rangeAxis) snap(v float64) float64 { return math.Round(v/a.step) * a.step }

func (a rangeAxis) fraction(v, top float64) float64 {
	v = min(max(v, a.lo), top)
	if a.log {
		return math.Log(v/a.lo) / math.Log(top/a.lo)
	}
	return (v - a.lo) / (top - a.lo)
}

func (a rangeAxis) at(f, top float64) float64 {
	f = min(max(f, 0), 1)
	if a.log {
		return a.snap(a.lo * math.Pow(top/a.lo, f))
	}
	return a.snap(a.lo + float64(f*(top-a.lo)))
}

func (a rangeAxis) nudge(v, top float64, dir int, coarse bool) float64 {
	if !a.log {
		step := a.step
		if coarse {
			step *= rangeCoarseSteps
		}
		return v + float64(float64(dir)*step)
	}
	share := rangeFineShare
	if coarse {
		share = rangeCoarseShare
	}
	next := a.at(a.fraction(v, top)+float64(float64(dir)*share), top)
	if next == v {
		return v + float64(float64(dir)*a.step)
	}
	return next
}

type rangeTone int

const (
	toneAccent rangeTone = iota
	toneWarning
	toneError
)

type rangeBand struct {
	from, to float64
	tone     rangeTone
}

type rangeSpec struct {
	axis   rangeAxis
	minGap float64
	unit   string
	names  [2]string
	bands  func(lo, hi, top float64) []rangeBand
}

func betweenThumbs(lo, hi, _ float64) []rangeBand {
	return []rangeBand{{from: lo, to: hi, tone: toneAccent}}
}

func rangeRow(low, high, key string, spec rangeSpec) rowSpec {
	return field(low, withKey(key), func(r *rowSpec) {
		r.fullWidth = true
		r.also = []string{high}
		r.editor = func(k *kit, s slot, _ config.FieldMeta) control {
			return newRangeSlider(k, [2]slot{s, k.store.slot(high)}, spec)
		}
	})
}

type rangeSlider struct {
	widget.Base
	k      *kit
	slots  [2]slot
	spec   rangeSpec
	thumbs [2]*rangeThumb
	drag   *rangeThumb
}

type rangeThumb struct {
	widget.Base
	slider *rangeSlider
	at     int
	name   string
}

func newRangeSlider(k *kit, slots [2]slot, spec rangeSpec) *rangeSlider {
	if spec.bands == nil {
		spec.bands = betweenThumbs
	}
	s := &rangeSlider{Base: widget.NewBase(), k: k, slots: slots, spec: spec}
	s.AddClass("range-slider")
	for i := range s.thumbs {
		s.thumbs[i] = &rangeThumb{Base: widget.NewBase(), slider: s, at: i, name: locale.T().Get(spec.names[i])}
		widget.SetParents(s, s.thumbs[i])
	}
	s.refresh()
	return s
}

func (s *rangeSlider) value(i int) float64 {
	v, _ := numeric(s.slots[i].get())
	return v
}

func (s *rangeSlider) top() float64 { return max(s.spec.axis.hi, s.value(1)) }

func (s *rangeSlider) refresh() {
	for _, t := range s.thumbs {
		t.SetAccessibleName(fmt.Sprintf("%s: %s", t.name, s.reading(s.value(t.at))))
	}
	s.InvalidateLayout()
	s.Invalidate()
}

func (s *rangeSlider) reading(v float64) string {
	return locale.T().Get(s.spec.unit, i18n.Float("n", v))
}

func (s *rangeSlider) bounds(i int) (lo, hi float64) {
	if i == 0 {
		return s.spec.axis.lo, s.value(1) - s.spec.minGap
	}
	return s.value(0) + s.spec.minGap, s.top()
}

func (s *rangeSlider) move(i int, v float64) {
	lo, hi := s.bounds(i)
	v = min(max(s.spec.axis.snap(v), lo), hi)
	if v == s.value(i) {
		return
	}
	var stored any = v
	if _, whole := s.slots[i].get().(int64); whole {
		stored = int64(v)
	}
	if err := s.slots[i].set(stored); err != nil {
		return
	}
	s.refresh()
}

func (s *rangeSlider) track() render.Rect {
	r := s.Bounds()
	pad := thresholdStopR + 2
	return render.Rect{X: r.X + pad, Y: r.Y + (r.H-thresholdTrackH)/2, W: max(1, r.W-2*pad), H: thresholdTrackH}
}

func (s *rangeSlider) xOf(v float64) int {
	t := s.track()
	return t.X + int(math.Round(float64(t.W)*s.spec.axis.fraction(v, s.top())))
}

func (s *rangeSlider) valueAt(x int) float64 {
	t := s.track()
	return s.spec.axis.at(float64(x-t.X)/float64(t.W), s.top())
}

func (s *rangeSlider) Children() []widget.Widget {
	return []widget.Widget{s.thumbs[0], s.thumbs[1]}
}

func (s *rangeSlider) Measure(axis widget.Axis, _ int) widget.Sizes {
	if axis == widget.Row {
		return widget.Sizes{Min: 160, Nat: 320}
	}
	return widget.FixedSizes(rangeSliderH)
}

func (s *rangeSlider) Arrange(r render.Rect) {
	s.ArrangeSelf(r)
	t := s.track()
	for _, th := range s.thumbs {
		x := s.xOf(s.value(th.at))
		th.Arrange(render.Rect{X: x - thresholdStopR, Y: t.Y + t.H/2 - thresholdStopR, W: 2 * thresholdStopR, H: 2 * thresholdStopR})
	}
}

func (s *rangeSlider) bandsNow() []rangeBand {
	return s.spec.bands(s.value(0), s.value(1), s.top())
}

func toneColor(th *widget.Theme, tone rangeTone) render.Color {
	switch tone {
	case toneWarning:
		return th.Warning
	case toneError:
		return th.Error
	case toneAccent:
	}
	return th.Accent
}

func (s *rangeSlider) Paint(cv *render.Canvas) {
	th := widget.Current()
	t := s.track()
	cv.RoundedRect(t, t.H/2, th.MutedColor())
	for _, b := range s.bandsNow() {
		x0, x1 := s.xOf(b.from), s.xOf(b.to)
		cv.RoundedRect(render.Rect{X: x0, Y: t.Y, W: max(1, x1-x0), H: t.H}, t.H/2, toneColor(th, b.tone))
	}
	px := s.k.textPx() * 0.85
	lineH := int(px * 1.4)
	for _, thumb := range s.thumbs {
		r := thumb.Bounds()
		cv.RoundedRect(r, thresholdStopR, th.CardColor())
		cv.RoundedBorder(r, render.UniformCorners(thresholdStopR), render.UniformInsets(2), th.Text)
		if widget.FocusVisible(thumb) {
			cv.RoundedBorder(r, render.UniformCorners(thresholdStopR), render.UniformInsets(focusRing), th.Text)
		}
		label := render.Rect{X: r.X - 3*r.W, Y: t.Y - thresholdStopR - lineH, W: 7 * r.W, H: lineH}
		s.k.face.DrawAligned(cv, s.reading(s.value(thumb.at)), label, px, th.Text, render.AlignCenter)
	}
	scale := render.Rect{X: t.X, Y: t.Y + t.H + thresholdStopR/2, W: t.W, H: lineH}
	s.k.face.DrawAligned(cv, s.reading(s.spec.axis.lo), scale, px, th.Text, render.AlignStart)
	s.k.face.DrawAligned(cv, s.reading(s.top()), scale, px, th.Text, render.AlignEnd)
}

func (s *rangeSlider) HitTest(pt widget.Point) widget.Widget { return s.HitLeaf(s, pt) }

func (s *rangeSlider) PressAt(pt widget.Point) {
	s.drag = nil
	best := math.MaxInt
	for _, th := range s.thumbs {
		c := th.Bounds()
		if d := abs(pt.X - (c.X + c.W/2)); d <= thresholdStopR*2 && d < best {
			best, s.drag = d, th
		}
	}
}

func (s *rangeSlider) DragMove(pt widget.Point) {
	if s.drag != nil {
		s.move(s.drag.at, s.valueAt(pt.X))
	}
}

func (s *rangeSlider) PressEnd() { s.drag = nil }

func (s *rangeSlider) FocusMoved(_, _ widget.Widget) { s.Invalidate() }

func (t *rangeThumb) Measure(widget.Axis, int) widget.Sizes { return widget.FixedSizes(0) }

func (t *rangeThumb) Paint(*render.Canvas) {}

func (t *rangeThumb) HitTest(widget.Point) widget.Widget { return nil }

func (t *rangeThumb) Role() widget.Role { return widget.RoleSlider }

func (t *rangeThumb) KeyAction(a widget.KeyAction, mods widget.Mods) {
	s := t.slider
	coarse := mods&widget.ModShift != 0
	v := s.value(t.at)
	switch a {
	case widget.KeyLeft, widget.KeyDown:
		s.move(t.at, s.spec.axis.nudge(v, s.top(), -1, coarse))
	case widget.KeyRight, widget.KeyUp:
		s.move(t.at, s.spec.axis.nudge(v, s.top(), 1, coarse))
	case widget.KeyHome:
		lo, _ := s.bounds(t.at)
		s.move(t.at, lo)
	case widget.KeyEnd:
		_, hi := s.bounds(t.at)
		s.move(t.at, hi)
	}
}

var celsiusLevels = linearAxis(0, 150, thresholdStep)

func levelRange(axis rangeAxis, unit string, names [2]string, bands func(lo, hi, top float64) []rangeBand) rangeSpec {
	return rangeSpec{axis: axis, minGap: thresholdStep, unit: unit, names: names, bands: bands}
}

func risingLevels(lo, hi, top float64) []rangeBand {
	return []rangeBand{{from: lo, to: hi, tone: toneWarning}, {from: hi, to: top, tone: toneError}}
}
