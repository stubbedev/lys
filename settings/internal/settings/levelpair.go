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

const levelPairH = 60

// levelScale is what a pair of alarm levels is measured on: its top, its
// unit's message, and whether the alarm is at or below the levels (a
// battery running low) rather than at or above them.
type levelScale struct {
	top   float64
	unit  string
	below bool
}

var (
	percentLevels = levelScale{top: 100, unit: "settings-level-unit-percent"}
	celsiusLevels = levelScale{top: 150, unit: "settings-level-unit-celsius"}
	chargeLevels  = levelScale{top: 100, unit: "settings-level-unit-percent", below: true}
)

// levelPairRow edits two alarm levels on one bar: the milder at lower and
// the graver at upper, or the other way round on a scale that alarms
// below; the two stops cannot cross.
func levelPairRow(lower, upper, key string, scale levelScale, names [2]string) rowSpec {
	return field(lower, withKey(key), func(r *rowSpec) {
		r.fullWidth = true
		r.also = []string{upper}
		r.editor = func(k *kit, s slot, _ config.FieldMeta) control {
			return newLevelPair(k, [2]slot{s, k.store.slot(upper)}, scale, names)
		}
	})
}

type levelPair struct {
	widget.Base
	k     *kit
	slots [2]slot
	scale levelScale
	stops [2]*levelStop
	drag  *levelStop
}

type levelStop struct {
	widget.Base
	pair *levelPair
	at   int
	name string
}

func newLevelPair(k *kit, slots [2]slot, scale levelScale, names [2]string) *levelPair {
	p := &levelPair{Base: widget.NewBase(), k: k, slots: slots, scale: scale}
	p.AddClass("level-pair")
	for i := range p.stops {
		p.stops[i] = &levelStop{Base: widget.NewBase(), pair: p, at: i, name: locale.T().Get(names[i])}
		widget.SetParents(p, p.stops[i])
	}
	p.refresh()
	return p
}

func (p *levelPair) value(i int) float64 {
	v, _ := thresholdNum(p.slots[i].get())
	return v
}

func (p *levelPair) refresh() {
	for _, s := range p.stops {
		s.SetAccessibleName(fmt.Sprintf("%s: %s", s.name, p.reading(s.at)))
	}
	p.InvalidateLayout()
	p.Invalidate()
}

func (p *levelPair) reading(i int) string {
	return locale.T().Get(p.scale.unit, i18n.Float("n", p.value(i)))
}

// bounds is where stop i may go: below the upper stop, above the lower.
func (p *levelPair) bounds(i int) (lo, hi float64) {
	if i == 0 {
		return 0, p.value(1) - thresholdStep
	}
	return p.value(0) + thresholdStep, p.scale.top
}

func (p *levelPair) move(i int, v float64) {
	lo, hi := p.bounds(i)
	v = min(max(math.Round(v), lo), hi)
	if v == p.value(i) {
		return
	}
	if err := p.slots[i].set(v); err != nil {
		return
	}
	p.refresh()
}

func (p *levelPair) track() render.Rect {
	r := p.Bounds()
	pad := thresholdStopR + 2
	return render.Rect{X: r.X + pad, Y: r.Y + (r.H-thresholdTrackH)/2, W: max(1, r.W-2*pad), H: thresholdTrackH}
}

func (p *levelPair) xOf(v float64) int {
	t := p.track()
	return t.X + int(math.Round(float64(t.W)*min(max(v, 0), p.scale.top)/p.scale.top))
}

func (p *levelPair) valueAt(x int) float64 {
	t := p.track()
	return min(max(float64(x-t.X)/float64(t.W), 0), 1) * p.scale.top
}

func (p *levelPair) Children() []widget.Widget { return []widget.Widget{p.stops[0], p.stops[1]} }

func (p *levelPair) Measure(axis widget.Axis, _ int) widget.Sizes {
	if axis == widget.Row {
		return widget.Sizes{Min: 160, Nat: 320}
	}
	return widget.FixedSizes(levelPairH)
}

func (p *levelPair) Arrange(r render.Rect) {
	p.ArrangeSelf(r)
	t := p.track()
	for _, s := range p.stops {
		x := p.xOf(p.value(s.at))
		s.Arrange(render.Rect{X: x - thresholdStopR, Y: t.Y + t.H/2 - thresholdStopR, W: 2 * thresholdStopR, H: 2 * thresholdStopR})
	}
}

// bands are the bar's two alarm stretches, milder first.
func (p *levelPair) bands() [2][2]float64 {
	lo, hi := p.value(0), p.value(1)
	if p.scale.below {
		return [2][2]float64{{lo, hi}, {0, lo}}
	}
	return [2][2]float64{{lo, hi}, {hi, p.scale.top}}
}

func (p *levelPair) Paint(cv *render.Canvas) {
	th := widget.Current()
	t := p.track()
	cv.RoundedRect(t, t.H/2, th.MutedColor())
	colors := [2]render.Color{th.Warning, th.Error}
	for i, b := range p.bands() {
		x0, x1 := p.xOf(b[0]), p.xOf(b[1])
		cv.RoundedRect(render.Rect{X: x0, Y: t.Y, W: max(1, x1-x0), H: t.H}, t.H/2, colors[i])
	}
	px := p.k.textPx() * 0.85
	lineH := int(px * 1.4)
	for _, s := range p.stops {
		r := s.Bounds()
		cv.RoundedRect(r, thresholdStopR, th.CardColor())
		cv.RoundedBorder(r, render.UniformCorners(thresholdStopR), render.UniformInsets(2), th.Text)
		if widget.FocusVisible(s) {
			cv.RoundedBorder(r, render.UniformCorners(thresholdStopR), render.UniformInsets(focusRing), th.Text)
		}
		label := render.Rect{X: r.X - 3*r.W, Y: t.Y - thresholdStopR - lineH, W: 7 * r.W, H: lineH}
		p.k.face.DrawAligned(cv, p.reading(s.at), label, px, th.Text, render.AlignCenter)
	}
	scale := render.Rect{X: t.X, Y: t.Y + t.H + thresholdStopR/2, W: t.W, H: lineH}
	p.k.face.DrawAligned(cv, "0", scale, px, th.Text, render.AlignStart)
	p.k.face.DrawAligned(cv, fmt.Sprintf("%g", p.scale.top), scale, px, th.Text, render.AlignEnd)
}

func (p *levelPair) HitTest(pt widget.Point) widget.Widget { return p.HitLeaf(p, pt) }

func (p *levelPair) PressAt(pt widget.Point) {
	p.drag = nil
	best := math.MaxInt
	for _, s := range p.stops {
		c := s.Bounds()
		if d := abs(pt.X - (c.X + c.W/2)); d <= thresholdStopR*2 && d < best {
			best, p.drag = d, s
		}
	}
}

func (p *levelPair) DragMove(pt widget.Point) {
	if p.drag != nil {
		p.move(p.drag.at, p.valueAt(pt.X))
	}
}

func (p *levelPair) PressEnd() { p.drag = nil }

func (p *levelPair) FocusMoved(_, _ widget.Widget) { p.Invalidate() }

func (s *levelStop) Measure(widget.Axis, int) widget.Sizes { return widget.FixedSizes(0) }

func (s *levelStop) Paint(*render.Canvas) {}

func (s *levelStop) HitTest(widget.Point) widget.Widget { return nil }

func (s *levelStop) Role() widget.Role { return widget.RoleSlider }

func (s *levelStop) KeyAction(a widget.KeyAction, mods widget.Mods) {
	step := thresholdStep
	if mods&widget.ModShift != 0 {
		step = thresholdBigStep
	}
	switch a {
	case widget.KeyLeft:
		s.pair.move(s.at, s.pair.value(s.at)-step)
	case widget.KeyRight:
		s.pair.move(s.at, s.pair.value(s.at)+step)
	}
}
