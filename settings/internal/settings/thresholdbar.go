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
	thresholdTrackH   = 10
	thresholdStopR    = 7
	thresholdScaleMax = 100.0
	thresholdStep     = 1.0
	thresholdBigStep  = 10.0
)

var thresholdColorKeys = []string{"icon-color", "label-color", "button-bg-color", "icon-bg-color", "border-color"}

func numeric(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int64:
		return float64(n), true
	case int:
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
	look   *widget.SliderLook
}

func newThresholdBar(k *kit, s slot, levels *levelBands) *thresholdBar {
	b := &thresholdBar{Base: widget.NewBase(), k: k, slot: s, max: thresholdScaleMax, levels: levels}
	b.look = widget.NewSliderLook(b)
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
			if v, ok := numeric(m[key]); ok && v > b.max {
				b.max = math.Ceil(float64(v*1.2)/10) * 10
			}
		}
	}
	b.stops = b.stops[:0]
	t := locale.T()
	for i, m := range b.items {
		for _, key := range []string{"above", "below"} {
			if v, ok := numeric(m[key]); ok {
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
		left += b.levels.leftPad()
	}
	return render.Rect{X: r.X + left, Y: r.Y + thresholdStopR + 1 - thresholdTrackH/2, W: max(1, r.W-left-pad), H: thresholdTrackH}
}

// barH is the bar's height below its level icons: the track with room
// for its stops, then its marks.
func (b *thresholdBar) barH() int { return 2*(thresholdStopR+1) + markTop + b.markH() }

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
		return widget.FixedSizes(b.barH() + levelRowH)
	}
	return widget.FixedSizes(b.barH())
}

func (b *thresholdBar) stopValue(s *thresholdStop) float64 {
	v, _ := numeric(b.items[s.entry][s.key])
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
	trough := sliderTrough(b.look, t)
	b.look.PaintTrough(cv, trough)
	for _, m := range b.items {
		lo, hasLo := numeric(m["above"])
		hi, hasHi := numeric(m["below"])
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
		cv.RoundedRect(render.Rect{X: x0, Y: trough.Y, W: max(1, x1-x0), H: trough.H}, trough.H/2, b.colorOf(m))
	}
	if b.levels != nil {
		b.levels.paint(b, cv)
	}
	for _, s := range b.stops {
		paintSliderKnob(cv, b.look, s.Bounds(), widget.FocusVisible(s))
	}
	px := b.markPx()
	for _, m := range b.marks() {
		b.k.face.DrawAligned(cv, m.text, m.box, px, th.Text, render.AlignStart)
	}
}

// scaleMark is a value read under the track: its text and where it is
// drawn.
type scaleMark struct {
	text string
	box  render.Rect
}

// markTop is the room between the track's stops and the marks.
const markTop = 2

// markGap is the least room between two marks under the track, the
// theme's spacing, so two marks never read as one ("0min 5").
func markGap() int { return widget.Current().Spacing }

func (b *thresholdBar) markPx() float64 { return b.k.textPx() * 0.85 }

func (b *thresholdBar) markH() int { return int(b.markPx() * 1.4) }

// marks are the values read under the track: what the level bands mark
// on it, each centred under its value, then its ends, a mark that would
// run into one already placed left out, so an end gives way to the
// floor or a boundary beside it.
func (b *thresholdBar) marks() []scaleMark {
	t := b.track()
	px := b.markPx()
	h := b.markH()
	var placed []scaleMark
	place := func(text string, x int, anchor render.Alignment) {
		w := int(math.Ceil(b.k.face.Shape(text, px).Advance()))
		switch anchor {
		case render.AlignCenter:
			x -= w / 2
		case render.AlignEnd:
			x -= w
		}
		x = min(max(x, t.X), t.X+t.W-w)
		box := render.Rect{X: x, Y: t.Y + t.H/2 + thresholdStopR + 1 + markTop, W: w, H: h}
		gap := markGap()
		for _, m := range placed {
			if box.X < m.box.X+m.box.W+gap && m.box.X < box.X+box.W+gap {
				return
			}
		}
		placed = append(placed, scaleMark{text: text, box: box})
	}
	if b.levels != nil {
		for _, m := range b.levels.marks() {
			place(m.text, b.xOf(m.value), render.AlignCenter)
		}
	}
	place("0", t.X, render.AlignStart)
	place(fmt.Sprintf("%g", b.max), t.X+t.W, render.AlignEnd)
	return placed
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
