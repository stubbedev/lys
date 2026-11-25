package settings

import (
	"math"
	"slices"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

const (
	handsWidth   = 360
	handsHeight  = 200
	handUnitW    = 1.0
	handUnitH    = 1.2
	handGapUnits = 0.3
	fingerUnitW  = 0.16
	tipRingWidth = 3
)

type unitRect struct{ x, y, w, h float64 }

var leftHandFingers = [5]unitRect{
	{0.06, 0.30, fingerUnitW, 0.40},
	{0.26, 0.12, fingerUnitW, 0.55},
	{0.46, 0.04, fingerUnitW, 0.60},
	{0.66, 0.12, fingerUnitW, 0.55},
	{0.86, 0.58, fingerUnitW, 0.36},
}

var leftPalm = unitRect{0.04, 0.55, 0.82, 0.62}

func mirrored(r unitRect) unitRect { return unitRect{handUnitW - r.x - r.w, r.y, r.w, r.h} }

type fingerHands struct {
	widget.Base
	k         *kit
	enrolled  []string
	enrolling string
	progress  float64
	busy      bool
	handles   []*spot[string]
	scale     float64
	ox, oy    float64
	onPick    func(finger string)
}

func newFingerHands(k *kit, onPick func(finger string)) *fingerHands {
	h := &fingerHands{Base: widget.NewBase(), k: k, onPick: onPick}
	for _, f := range fingers {
		fh := newSpot(f, h.pick)
		fh.SetAccessibleName(fingerName(f))
		h.handles = append(h.handles, fh)
	}
	for _, fh := range h.handles {
		widget.SetParents(h, fh)
	}
	h.AddClass("fingerprint-hands")
	return h
}

func (h *fingerHands) set(enrolled []string, enrolling string, progress float64, busy bool) {
	h.enrolled, h.enrolling, h.progress, h.busy = enrolled, enrolling, progress, busy
	h.Invalidate()
}

func (h *fingerHands) offers(finger string) bool {
	return !h.busy && !slices.Contains(h.enrolled, finger)
}

func (h *fingerHands) pick(finger string) {
	if h.offers(finger) && h.onPick != nil {
		h.onPick(finger)
	}
}

func fingerUnit(i int) unitRect {
	if i < 5 {
		return leftHandFingers[i]
	}
	r := mirrored(leftHandFingers[9-i])
	r.x += handUnitW + handGapUnits
	return r
}

func palmUnit(right bool) unitRect {
	if !right {
		return leftPalm
	}
	r := mirrored(leftPalm)
	r.x += handUnitW + handGapUnits
	return r
}

func (h *fingerHands) screen(u unitRect) render.Rect {
	return render.Rect{
		X: int(math.Round(h.ox + float64(u.x*h.scale))), Y: int(math.Round(h.oy + float64(u.y*h.scale))),
		W: int(math.Round(u.w * h.scale)), H: int(math.Round(u.h * h.scale)),
	}
}

func (h *fingerHands) Children() []widget.Widget {
	out := make([]widget.Widget, len(h.handles))
	for i, fh := range h.handles {
		out[i] = fh
	}
	return out
}

func (h *fingerHands) Measure(axis widget.Axis, _ int) widget.Sizes {
	if axis == widget.Row {
		return widget.Sizes{Min: handsWidth / 2, Nat: handsWidth}
	}
	return widget.FixedSizes(handsHeight)
}

func (h *fingerHands) Arrange(r render.Rect) {
	h.ArrangeSelf(r)
	w, ht := 2*handUnitW+handGapUnits, handUnitH
	h.scale = min(float64(r.W)/w, float64(r.H)/ht)
	h.ox = float64(r.X) + float64((float64(r.W)-float64(w*h.scale))/2)
	h.oy = float64(r.Y) + float64((float64(r.H)-float64(ht*h.scale))/2)
	for i, fh := range h.handles {
		fh.Arrange(h.screen(fingerUnit(i)))
	}
}

func (h *fingerHands) Paint(cv *render.Canvas) {
	th := widget.Current()
	ink := handsInk(th)
	for _, right := range []bool{false, true} {
		palm := h.screen(palmUnit(right))
		cv.RoundedRect(palm, palm.W/4, ink.palm)
	}
	for i, f := range fingers {
		r := h.screen(fingerUnit(i))
		radius := r.W / 2
		fill := th.CardColor()
		switch {
		case slices.Contains(h.enrolled, f):
			fill = th.Accent
		case f == h.enrolling:
			fill = ink.palm
		}
		cv.RoundedRect(r, radius, fill)
		cv.RoundedBorder(r, render.UniformCorners(radius), render.UniformInsets(1), ink.outline)
		if widget.FocusVisible(h.handles[i]) {
			cv.RoundedBorder(r, render.UniformCorners(radius), render.UniformInsets(focusRing), th.Text)
		}
		if f == h.enrolling {
			cx, cy, ring := float64(r.X+radius), float64(r.Y+radius), float64(radius+2*tipRingWidth)
			cv.Arc(cx, cy, ring, tipRingWidth, 0, 2*math.Pi, ink.palm)
			cv.Arc(cx, cy, ring, tipRingWidth, -math.Pi/2, 2*math.Pi*h.progress, th.Accent)
		}
	}
}

// handsColors are what the hands picture is drawn in besides the
// accent: the palms and the finger being enrolled in the fill of a field
// on a card, and every finger's outline in the muted text color, so the
// hands stand off the card they are drawn on.
type handsColors struct{ palm, outline render.Color }

func handsInk(th *widget.Theme) handsColors {
	return handsColors{palm: th.InputColor(), outline: th.TextMuted}
}

func (h *fingerHands) HitTest(p widget.Point) widget.Widget { return h.HitLeaf(h, p) }

func (h *fingerHands) PressAt(p widget.Point) {
	for i, fh := range h.handles {
		if fh.Bounds().Contains(p.X, p.Y) {
			h.pick(fingers[i])
			return
		}
	}
}

func (h *fingerHands) FocusMoved(_, _ widget.Widget) { h.Invalidate() }
