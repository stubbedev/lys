package settings

import (
	"fmt"
	"maps"
	"math"
	"slices"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	thresholdBarH     = 44
	thresholdTrackH   = 10
	thresholdStopR    = 7
	thresholdScaleMax = 100.0
	thresholdStep     = 1.0
	thresholdBigStep  = 10.0
)

var thresholdColorKeys = []string{"icon-color", "label-color", "button-bg-color", "icon-bg-color", "border-color"}

func thresholdNum(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int64:
		return float64(n), true
	}
	return 0, false
}

type thresholdStop struct {
	widget.Base
	bar   *thresholdBar
	entry int
	key   string
}

type thresholdBar struct {
	widget.Base
	k      *kit
	slot   slot
	items  []map[string]any
	stops  []*thresholdStop
	max    float64
	drag   *thresholdStop
	levels *levelBands
}

func newThresholdBar(k *kit, s slot, levels *levelBands) *thresholdBar {
	b := &thresholdBar{Base: widget.NewBase(), k: k, slot: s, max: thresholdScaleMax, levels: levels}
	b.AddClass("threshold-bar")
	b.refresh()
	return b
}

func (b *thresholdBar) refresh() {
	raw, _ := b.slot.get().([]any)
	b.items = b.items[:0]
	b.max = thresholdScaleMax
	for _, it := range raw {
		m, _ := it.(map[string]any)
		b.items = append(b.items, m)
		for _, key := range []string{"above", "below"} {
			if v, ok := thresholdNum(m[key]); ok && v > b.max {
				b.max = math.Ceil(float64(v*1.2)/10) * 10
			}
		}
	}
	b.stops = b.stops[:0]
	t := locale.T()
	for i, m := range b.items {
		for _, key := range []string{"above", "below"} {
			if v, ok := thresholdNum(m[key]); ok {
				s := &thresholdStop{Base: widget.NewBase(), bar: b, entry: i, key: key}
				s.SetAccessibleName(fmt.Sprintf("%s %d: %s %g", t.Get("settings-threshold-card-title"), i+1, t.Get("settings-threshold-"+key), v))
				b.stops = append(b.stops, s)
			}
		}
	}
	if b.levels != nil {
		b.levels.refresh()
	}
	widget.SetParents(b, b.Children()...)
	b.InvalidateLayout()
	b.Invalidate()
}

func (b *thresholdBar) track() render.Rect {
	r := b.Bounds()
	pad := thresholdStopR + 2
	left := pad
	if b.levels != nil {
		r.Y += levelRowH
		r.H -= levelRowH
		left += b.levels.leftPad()
	}
	return render.Rect{X: r.X + left, Y: r.Y + (r.H-thresholdTrackH)/2, W: max(1, r.W-left-pad), H: thresholdTrackH}
}

func (b *thresholdBar) xOf(v float64) int {
	t := b.track()
	return t.X + int(math.Round(float64(float64(t.W)*min(max(v, 0), b.max)/b.max)))
}

func (b *thresholdBar) valueAt(x int) float64 {
	t := b.track()
	return math.Round(min(max(float64(x-t.X)/float64(t.W), 0), 1) * b.max)
}

func (b *thresholdBar) colorOf(m map[string]any) render.Color {
	th := widget.Current()
	p := b.k.palette()
	for _, key := range thresholdColorKeys {
		s, _ := m[key].(string)
		cv, err := config.ParseColorValue(s)
		if s == "" || err != nil {
			continue
		}
		switch cv.Kind {
		case config.ColorToken:
			if c, ok := p.Token(cv.Token); ok {
				return c
			}
		case config.ColorCustom:
			if c, err := widget.ParseColor(cv.Hex); err == nil {
				return c
			}
		case config.ColorAuto, config.ColorTransparent:
		}
	}
	return th.Accent
}

func (b *thresholdBar) Children() []widget.Widget {
	out := make([]widget.Widget, len(b.stops))
	for i, s := range b.stops {
		out[i] = s
	}
	if b.levels != nil {
		out = append(out, b.levels.widgets()...)
	}
	return out
}

func (b *thresholdBar) Measure(axis widget.Axis, _ int) widget.Sizes {
	if axis == widget.Row {
		return widget.Sizes{Min: 160, Nat: 320}
	}
	if b.levels != nil {
		return widget.FixedSizes(thresholdBarH + levelRowH)
	}
	return widget.FixedSizes(thresholdBarH)
}

func (b *thresholdBar) stopValue(s *thresholdStop) float64 {
	v, _ := thresholdNum(b.items[s.entry][s.key])
	return v
}

func (b *thresholdBar) Arrange(r render.Rect) {
	b.ArrangeSelf(r)
	t := b.track()
	for _, s := range b.stops {
		x := b.xOf(b.stopValue(s))
		s.Arrange(render.Rect{X: x - thresholdStopR, Y: t.Y + t.H/2 - thresholdStopR, W: 2 * thresholdStopR, H: 2 * thresholdStopR})
	}
	if b.levels != nil {
		b.levels.arrange(b)
	}
}

func (b *thresholdBar) Paint(cv *render.Canvas) {
	th := widget.Current()
	t := b.track()
	cv.RoundedRect(t, t.H/2, th.MutedColor())
	for _, m := range b.items {
		lo, hasLo := thresholdNum(m["above"])
		hi, hasHi := thresholdNum(m["below"])
		if !hasLo {
			lo = 0
		}
		if !hasHi {
			hi = b.max
		}
		if !hasLo && !hasHi {
			continue
		}
		x0, x1 := b.xOf(lo), b.xOf(hi)
		cv.RoundedRect(render.Rect{X: x0, Y: t.Y, W: max(1, x1-x0), H: t.H}, t.H/2, b.colorOf(m))
	}
	if b.levels != nil {
		b.levels.paint(b, cv)
	}
	for _, s := range b.stops {
		r := s.Bounds()
		cv.RoundedRect(r, thresholdStopR, th.CardColor())
		cv.RoundedBorder(r, render.UniformCorners(thresholdStopR), render.UniformInsets(2), b.colorOf(b.items[s.entry]))
		if widget.FocusVisible(s) {
			cv.RoundedBorder(r, render.UniformCorners(thresholdStopR), render.UniformInsets(focusRing), th.Text)
		}
	}
	px := b.k.textPx() * 0.85
	b.k.face.DrawAligned(cv, "0", render.Rect{X: t.X, Y: t.Y + t.H + 2, W: t.W, H: int(px * 1.4)}, px, th.Text, render.AlignStart)
	b.k.face.DrawAligned(cv, fmt.Sprintf("%g", b.max), render.Rect{X: t.X, Y: t.Y + t.H + 2, W: t.W, H: int(px * 1.4)}, px, th.Text, render.AlignEnd)
}

func (b *thresholdBar) HitTest(p widget.Point) widget.Widget { return b.HitLeaf(b, p) }

func (b *thresholdBar) PressAt(p widget.Point) {
	b.drag = nil
	best := math.MaxInt
	for _, s := range b.stops {
		c := s.Bounds()
		if d := abs(p.X - (c.X + c.W/2)); d <= thresholdStopR*2 && d < best {
			best, b.drag = d, s
		}
	}
}

func (b *thresholdBar) DragMove(p widget.Point) {
	if b.drag != nil {
		b.move(b.drag, b.valueAt(p.X))
	}
}

func (b *thresholdBar) PressEnd() { b.drag = nil }

func (b *thresholdBar) FocusMoved(_, _ widget.Widget) { b.Invalidate() }

func (b *thresholdBar) move(s *thresholdStop, v float64) {
	if v == b.stopValue(s) {
		return
	}
	raw, _ := b.slot.get().([]any)
	out := slices.Clone(raw)
	m, _ := out[s.entry].(map[string]any)
	c := maps.Clone(m)
	c[s.key] = v
	out[s.entry] = c
	if err := b.slot.set(out); err != nil {
		return
	}
	b.items[s.entry] = c
	b.InvalidateLayout()
	b.Invalidate()
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func (s *thresholdStop) Measure(widget.Axis, int) widget.Sizes { return widget.FixedSizes(0) }

func (s *thresholdStop) Paint(*render.Canvas) {}

func (s *thresholdStop) HitTest(widget.Point) widget.Widget { return nil }

func (s *thresholdStop) Role() widget.Role { return widget.RoleButton }

func (s *thresholdStop) KeyAction(a widget.KeyAction, mods widget.Mods) {
	step := thresholdStep
	if mods&widget.ModShift != 0 {
		step = thresholdBigStep
	}
	switch a {
	case widget.KeyLeft:
		s.bar.move(s, max(0, s.bar.stopValue(s)-step))
	case widget.KeyRight:
		s.bar.move(s, min(s.bar.max, s.bar.stopValue(s)+step))
	}
}
