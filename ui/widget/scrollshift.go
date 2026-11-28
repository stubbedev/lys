package widget

import (
	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/ui/internal/style"
)

const scrollOverscan = 64

// deviceGrid is the step a scroll offset moves in, in logical pixels:
// the least whole logical distance that is whole in device pixels too at
// the tree's device scale, so every step can move pixels rather than
// repaint the view (two at 1.5, four at 1.25, one at a whole scale).
func (s *Scroll) deviceGrid() int {
	num, denom := DeviceScaleOf(s)
	if num <= 0 {
		return 1
	}
	a, b := num, denom
	for b != 0 {
		a, b = b, a%b
	}
	return denom / a
}

// onGrid is v at the nearest multiple of g, but limit, the offset's
// end, which stays reachable wherever it falls.
func onGrid(v, g, limit int) int {
	if g == 1 || v == limit {
		return v
	}
	return min(limit, (v+g/2)/g*g)
}

// ScrollMove is a viewport whose pixels moved by DX, DY as they stand:
// a frame can copy them there instead of painting them, and owes only
// what the move exposed, which the damage holds. All in logical pixels.
type ScrollMove struct {
	View   render.Rect
	DX, DY int
}

type viewShift struct{ dx, dy int }

func (s viewShift) moved() bool { return s.dx != 0 || s.dy != 0 }

func (t *treeState) beginShift(view render.Rect, culling bool) shiftState {
	st := t.shift
	t.shift.shifted, t.shiftBroken = true, false
	if !culling {
		return st
	}
	cull := render.UniformInsets(scrollOverscan).Grow(view)
	if t.shift.culling {
		cull = cull.Intersect(t.shift.cull)
	}
	t.shift.culling, t.shift.cull = true, cull
	return st
}

func (t *treeState) endShift(st shiftState) (broken bool) {
	broken = t.shiftBroken
	t.shift = st
	t.shiftBroken = st.shifted && broken
	return broken
}

func shifted(r render.Rect, dx, dy int) render.Rect {
	r.X += dx
	r.Y += dy
	return r
}

func (n *node) cullBy(dx, dy int) {
	n.bounds = shifted(n.bounds, dx, dy)
	n.inkBox = shifted(n.inkBox, dx, dy)
	n.solidBox = shifted(n.solidBox, dx, dy)
	if !n.culled {
		n.culled = true
		n.scratch().culled++
	}
}

func (n *node) settleCulled() {
	var top *node
	for a := n; a != nil; a = parentNodeOf(a) {
		if a.culled {
			top = a
		}
	}
	if top == nil || top.self == nil {
		return
	}
	t := top.scratch()
	st := t.beginShift(render.Rect{}, false)
	wasCulling := t.shift.culling
	t.shift.culling = false
	top.self.Arrange(top.slot)
	t.shift.culling = wasCulling
	t.endShift(st)
}

// settleShiftable records whether a move can follow the arrangement:
// every shown child placed in flow at a slot that moves with the box. An
// empty box lays its in-flow children at the origin, wherever it stands,
// so a move leaves them where they are.
func (b *Box) settleShiftable(inner render.Rect) {
	b.shiftable = len(b.departing) == 0
	for _, c := range b.child {
		n := nodeOf(c.w)
		switch {
		case n == nil:
			b.shiftable = false
		case !ownVisible(c.w):
			n.slotSet = false
		case flowOf(c.w) == flowOut:
			b.shiftable = false
		case inner.Empty():
			n.slotSet = false
		case !n.slotSet:
			b.shiftable = false
		}
	}
}

// noteEscapes records, after a clipping view painted its content under
// the clip prev was the device clip before, whether the content deferred
// top-layer paints past deferred, which its clip does not hold: a view
// whose content paints only inside its clip may move its pixels. A paint
// clipped short of the view may have skipped such content, so it keeps
// an escape it saw before.
func (n *node) noteEscapes(cv *render.Canvas, view, prev render.Rect, deferred int) {
	dev := cv.MapRect(view)
	n.escapes = cv.Overlays() > deferred || n.escapes && dev.Intersect(prev) != dev
}

// shifter is a scrolled child that can follow its viewport's offset by
// moving what it laid out, rather than laying it out again.
type shifter interface{ canShift() bool }

// canShift lets a clamp pass the move to its root: its own layout reads
// only the width it is given, which a move keeps.
func (c *Clamp) canShift() bool {
	s, ok := c.root.(shifter)
	return ok && s.canShift()
}

func (b *Box) canShift() bool {
	s := b.arranged
	return !b.relayout && b.shiftable && !s.volatile && s.style == styleGen.Load() && s.theme == themeGen.Load()
}

func (b *Box) shiftTo(t *treeState, r render.Rect) {
	dx, dy := r.X-b.arranged.rect.X, r.Y-b.arranged.rect.Y
	b.arranged.rect = r
	border, _ := boxRects(b.box(b.style(b)), r)
	b.ArrangeRoot(border)
	for _, c := range b.child {
		n := nodeOf(c.w)
		if !n.slotSet {
			continue
		}
		if n.sticky {
			restick(c.w, n, dx, dy)
			continue
		}
		n.slot = shifted(n.slot, dx, dy)
		if t.shift.culling && n.inkClosed && shifted(n.inkBox, dx, dy).Intersect(t.shift.cull).Empty() {
			n.cullBy(dx, dy)
			n.bubbleInk()
			continue
		}
		c.w.Arrange(n.slot)
	}
}

// restick places a sticky widget again after its parent moved by dx, dy:
// its flow rect moves with the content, the constraint holds it against
// the view, and it owes its place before and after, which the moved
// view's damage also owes where the copy carried them.
func restick(w Widget, n *node, dx, dy int) {
	before := n.bounds
	n.scratch().stickiness++
	arrangeAt(w, n.style(w), shifted(n.stickyFlow, dx, dy))
	if n.bounds == shifted(before, dx, dy) {
		return
	}
	n.InvalidateRect(before)
	n.Invalidate()
}

func (n *node) drainShift(c *DamageCollector, sh *viewShift, view, area render.Rect, blit bool) {
	moved := *sh
	*sh = viewShift{}
	if !moved.moved() || n.invalid {
		return
	}
	if !blit || !c.takesMoves || max(moved.dx, -moved.dx) >= area.W || max(moved.dy, -moved.dy) >= area.H {
		n.extras = append(n.extras, view)
		return
	}
	c.moves = append(c.moves, ScrollMove{View: area, DX: moved.dx, DY: moved.dy})
	c.shiftNext = moved
	switch {
	case moved.dy < 0:
		n.extras = append(n.extras, render.Rect{X: area.X, Y: area.Y + area.H + moved.dy, W: area.W, H: -moved.dy})
	case moved.dy > 0:
		n.extras = append(n.extras, render.Rect{X: area.X, Y: area.Y, W: area.W, H: moved.dy})
	}
	switch {
	case moved.dx < 0:
		n.extras = append(n.extras, render.Rect{X: area.X + area.W + moved.dx, Y: area.Y, W: -moved.dx, H: area.H})
	case moved.dx > 0:
		n.extras = append(n.extras, render.Rect{X: area.X, Y: area.Y, W: moved.dx, H: area.H})
	}
	if side := view.W - area.W; side > 0 {
		n.extras = append(n.extras, render.Rect{X: area.X + area.W, Y: view.Y, W: side, H: view.H})
	}
	if below := view.H - area.H; below > 0 {
		n.extras = append(n.extras, render.Rect{X: view.X, Y: area.Y + area.H, W: view.W, H: below})
	}
}

// takeShift drains a view's shift as drainShift does, and owes the
// rects a copied view carried over.
func (n *node) takeShift(c *DamageCollector, sh *viewShift, view, area render.Rect, blit bool) {
	moved := *sh
	n.drainShift(c, sh, view, area, blit)
	if !blit || !moved.moved() {
		return
	}
	for _, cover := range c.carriedRects {
		n.extras = append(n.extras, cover)
		if to := shifted(cover, moved.dx, moved.dy).Intersect(area); !to.Empty() {
			n.extras = append(n.extras, to)
		}
	}
	c.carriedRects = c.carriedRects[:0]
}

func flatBehind(n *node, w Widget, area render.Rect) bool {
	v := n.style(w)
	if n.sharedTag != "" || n.motion != nil || n.anims != nil || n.trCancel != nil || n.pseudo != nil {
		return false
	}
	if effectsHeavy(v) || v.Has(style.PropTransform) || v.Has(style.PropBackdropFilter) || v.Bg.Painted() || v.Has(style.PropBorderImageSource) {
		return false
	}
	for _, sh := range v.Shadow.List() {
		if sh.Inset {
			return false
		}
	}
	if pickc(0, v, style.PropBackgroundColor, 0) == 0 && borderOf(v).Zero() {
		return true
	}
	c := radiusOr(v, 0)
	inner := render.UniformInsets(max(c.TopLeft, c.TopRight, c.BottomRight, c.BottomLeft)).Shrink(borderOf(v).Shrink(n.bounds))
	return area.Intersect(inner) == area
}

// shiftHost is a container a scrolled view inside it can move its
// pixels through: clearOver reports whether, besides child, it paints
// nothing over area but what bounds itself, and appends what does to the
// collector's carried rects, owed where it stands and where a copy
// carries it.
type shiftHost interface {
	clearOver(c *DamageCollector, child Widget, area render.Rect) bool
}

// clearOfSiblings reports whether nothing p paints besides self covers
// area but what the collector's carried rects then hold. A container that is no
// shiftHost may paint anything, so it refuses.
func clearOfSiblings(c *DamageCollector, p Widget, self Widget, area render.Rect) bool {
	h, ok := p.(shiftHost)
	return ok && h.clearOver(c, self, area)
}

// coverSiblings is clearOver for children painted in one plane: each
// shown child but self whose paint meets area is a cover.
func coverSiblings(c *DamageCollector, kids []Widget, self Widget, area render.Rect) bool {
	at := nodeOf(self)
	for _, k := range kids {
		if nodeOf(k) != at && !coverSibling(c, k, area) {
			return false
		}
	}
	return true
}

func coverSibling(c *DamageCollector, k Widget, area render.Rect) bool {
	if !ownVisible(k) {
		return true
	}
	n := nodeOf(k)
	if n == nil {
		return false
	}
	box := n.coverBox()
	if cover := box.Intersect(area); !cover.Empty() {
		c.carriedRects = append(c.carriedRects, cover)
	}
	return true
}

// coverBox is what a widget paints, as far as a move beside it knows:
// its audited ink, or else what it repaints when it changes, its bounds
// grown by its CSS ink, and its focus ring while it holds the focus.
func (n *node) coverBox() render.Rect {
	switch {
	case n.inkClosed && n.focusWithin > 0:
		return n.inkBox
	case n.inkClosed:
		return n.solidBox
	case n.focusWithin > 0:
		return focusInk.Grow(n.ink.Grow(n.bounds))
	}
	return n.ink.Grow(n.bounds)
}

func (b *Box) clearOver(c *DamageCollector, self Widget, area render.Rect) bool {
	at := nodeOf(self)
	for _, k := range b.child {
		if nodeOf(k.w) != at && !coverSibling(c, k.w, area) {
			return false
		}
	}
	return true
}

func (o *Overlay) clearOver(c *DamageCollector, self Widget, area render.Rect) bool {
	return coverSiblings(c, o.kids, self, area)
}

func (*Scroll) clearOver(*DamageCollector, Widget, render.Rect) bool { return true }

// clearOver lets a stack at rest pass a move: it paints only its visible
// page, while a switch paints both through layers.
func (s *Stack) clearOver(*DamageCollector, Widget, render.Rect) bool { return s.prev == "" }

func (*Clamp) clearOver(*DamageCollector, Widget, render.Rect) bool { return true }

func (*Sidebar) clearOver(*DamageCollector, Widget, render.Rect) bool { return true }

// clearOver lets a closed sheet pass a move from its content: an open one
// shades it and paints its panel over it.
func (s *Sheet) clearOver(_ *DamageCollector, self Widget, _ render.Rect) bool {
	return !s.showing() && nodeOf(self) == nodeOf(s.root)
}

func (m *Morph) clearOver(c *DamageCollector, _ Widget, area render.Rect) bool {
	if m.reveal < 1 {
		return false
	}
	var ok bool
	c.carriedRects, ok = m.plateCorners(area, c.carriedRects)
	return ok
}

// blittable reports whether a clipping view's pixels in area may move:
// its content paints only inside its clip, its backdrop is one flat
// colour under area, and every container above it passes the move.
func (n *node) blittable(c *DamageCollector, self Widget, area render.Rect) bool {
	if n.escapes || !flatBehind(n, self, area) {
		return false
	}
	at := self
	for p := parentOf(at); p != nil; at, p = p, parentOf(p) {
		pn := nodeOf(p)
		if pn == nil || !clearOfSiblings(c, p, at, area) || !flatBehind(pn, p, area) {
			return false
		}
	}
	return true
}

func (s *Scroll) contentArea() render.Rect {
	return render.Rect{X: s.view.X, Y: s.view.Y, W: s.viewW, H: s.viewH}
}

func (s *Scroll) arrangeContent(to render.Rect) {
	dx, dy := to.X-s.childAt.X, to.Y-s.childAt.Y
	offsetMoved := s.offX != s.arrOffX || s.offY != s.arrOffY
	same := s.viewAt == s.view && to.W == s.childAt.W && to.H == s.childAt.H && (dx != 0 || dy != 0)
	s.viewAt, s.childAt = s.view, to
	if sh, ok := s.child.(shifter); ok && same && offsetMoved && sh.canShift() {
		t := s.scratch()
		st := t.beginShift(s.view, true)
		arrangeChild(s.child, to)
		if t.endShift(st) {
			s.Invalidate()
			return
		}
		s.shift.dx += dx
		s.shift.dy += dy
		return
	}
	arrangeChild(s.child, to)
	if offsetMoved {
		s.Invalidate()
	}
}

func (s *Scroll) drainScroll(c *DamageCollector) {
	if s.child != nil && (s.offX != s.arrOffX || s.offY != s.arrOffY) {
		s.invalid = true
	}
	_, both := s.gutterStable()
	area := s.contentArea()
	c.carriedRects = c.carriedRects[:0]
	blit := s.shift.moved() && !both && s.overshoot == [2]float64{} && s.blittable(c, s, area)
	s.takeShift(c, &s.shift, s.view, area, blit)
}

func (s *Scroll) damageClip() (render.Rect, bool) { return s.view, true }

func (l *List) damageClip() (render.Rect, bool) { return l.view, true }

func (b *Box) damageClip() (render.Rect, bool) {
	if !b.inkClips {
		return render.Rect{}, false
	}
	return b.overflowClip(b.style(b))
}

func (l *List) drainScroll(c *DamageCollector) {
	c.carriedRects = c.carriedRects[:0]
	blit := l.shift.moved() && l.blittable(c, l, l.view)
	l.takeShift(c, &l.shift, l.view, l.view, blit)
}

func (n *node) ownInkBounded(w Widget) bool {
	b, ok := w.(inkBounder)
	if !ok || n.opensInk(w) {
		return false
	}
	owner, _, closed := b.ownInk()
	return closed && owner == w
}
