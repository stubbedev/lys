// Damage tracking turns per-widget invalidation flags into the repaint
// rects one frame needs. Widgets mark themselves through Invalidate,
// InvalidateRect, and InvalidateLayout; a DamageCollector drains the flags
// into a rect list the app clips painting to and sends as
// wl_surface.damage_buffer.
package widget

import (
	"sync/atomic"

	"github.com/stubbedev/lys/lib/debug"
	"github.com/stubbedev/lys/lib/icons"
	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/ui/internal/style"
)

// maxDamageRects caps the rect list handed to the frame. Past the cap
// the remaining rects collapse into the running bounding box: more wire
// rects than this stops paying for itself.
const maxDamageRects = 32

type damageClipper interface {
	damageClip() (render.Rect, bool)
}

// damageNoder is node's drain hook; every widget that embeds node has it.
type damageNoder interface {
	takeDamage() (bounds render.Rect, extra []render.Rect, dirty bool)
}

// scrollDrainer is a viewport that drains its scroll into the walk
// before its damage: a move the frame can copy, or the view it owes.
type scrollDrainer interface {
	drainScroll(c *DamageCollector)
}

// childser exposes a widget's children for tree walks (the same
// interface focus traversal uses).
type childser interface {
	Children() []Widget
}

// childBuf is the damage walk's child source: the same snapshot
// Children() returns, appended into a buffer the walk reuses across
// the whole traversal instead of a fresh slice per container per
// frame. Implementations must match Children() in order and membership;
// TestChildWalkMatchesChildren pins that over every container.
type childBuf interface {
	appendChildren(buf []Widget) []Widget
}

// damageGen counts every mark that can turn a tree dirty: a widget
// invalidation, a style or theme generation move, a stale cascade.
// Draw owners compare it against the generation they last drained at
// and skip the walk when nothing anywhere was marked since, so pump
// work that touched no tree costs no CollectDamage pass. The +1 in
// DamageGeneration keeps zero meaning "never walked".
var damageGen atomic.Uint64

// damageWalks counts CollectDamage calls; tests use it to pin that a
// clean tree's walk is skipped.
var damageWalks atomic.Uint64

// DamageGeneration returns the current damage generation; it is never
// zero.
func DamageGeneration() uint64 { return damageGen.Load() + 1 }

// DamageWalks counts the damage walks the process has run; tests use
// it to pin the idle skip.
func DamageWalks() uint64 { return damageWalks.Load() }

// markDamage bumps the generation every drain-source flip owes, so no
// window can skip past a dirty tree.
func markDamage() { damageGen.Add(1) }

var keptLayerGen atomic.Uint64

// RepaintKeptLayers makes every layer a widget keeps across frames
// repaint at its next frame instead of compositing again: a host calls
// it after a frame that failed or went out incomplete, whose layers may
// not hold what was recorded into them.
func RepaintKeptLayers() { keptLayerGen.Add(1) }

// DamageCollector drains a widget tree's pending invalidations into the
// repaint rects one frame owes. It keeps the walk's state and buffers,
// reused by every collection, so a surface that keeps its collector
// drains without allocating once they have grown. One collector serves
// one loop: two trees drained at once need two collectors. The zero
// value is ready to use.
type DamageCollector struct {
	rects        []render.Rect
	moves        []ScrollMove
	kids         kidStack
	backdrop     []render.Rect
	plateCorners []render.Rect
	icons        uint64
	takesMoves   bool
	clip         render.Rect
	clipped      bool
	shift        viewShift
	shiftNext    viewShift
}

// CollectDamage drains every pending invalidation in the tree with a
// collector of its own and returns the repaint rects in root
// coordinates plus whether anything is owed. A frame loop keeps a
// DamageCollector instead, so its walks reuse their buffers.
func CollectDamage(root Widget) (rects []render.Rect, any bool) {
	var c DamageCollector
	return c.Collect(root)
}

// Collect drains every pending invalidation in the tree and returns the
// repaint rects in root coordinates plus whether anything is owed.
// Rects may overlap; the caller can clip painting to their bounding box
// and damage each rect on the wire. Draining clears the flags, so an
// empty result means the tree paints identically to the last frame.
// The rects are the collector's storage, valid until its next
// collection.
func (c *DamageCollector) Collect(root Widget) (rects []render.Rect, any bool) {
	c.takesMoves = false
	c.collect(root)
	return c.rects, len(c.rects) > 0
}

// CollectFrame is Collect for a frame that can copy pixels: a viewport
// that only scrolled reports a ScrollMove and owes only what the move
// exposed. The moves are the collector's storage too, valid until its
// next collection.
func (c *DamageCollector) CollectFrame(root Widget) (rects []render.Rect, moves []ScrollMove, any bool) {
	c.takesMoves = true
	c.collect(root)
	return c.rects, c.moves, len(c.rects) > 0 || len(c.moves) > 0
}

func (c *DamageCollector) collect(root Widget) {
	c.rects, c.moves = c.rects[:0], c.moves[:0]
	if root == nil {
		return
	}
	damageWalks.Add(1)
	c.backdrop = c.backdrop[:0]
	c.icons = icons.Default().Generation()
	c.clip, c.clipped, c.shift, c.shiftNext = render.Rect{}, false, viewShift{}, viewShift{}
	bbox := c.collectInto(root)
	bbox = bbox.Union(c.widenForBackdrops())
	if len(c.rects) > maxDamageRects {
		c.rects = c.rects[:0]
		if !bbox.Empty() {
			c.rects = append(c.rects, bbox)
		}
	}
}

func markUnlinked(w, kid Widget) {
	n, k := nodeOf(w), nodeOf(kid)
	if n != nil && (k == nil || k.parent != outerOf(w)) {
		n.markBranchStale()
	}
}

// collectInto walks w, appending drained rects and returning the
// bounding box of everything it appended (empty when nothing). The
// children iterate from the walk's own snapshot buffer, so a
// container that mutates mid-drain changes the next frame, not this
// walk.
func (c *DamageCollector) collectInto(w Widget) render.Rect {
	if n := nodeOf(w); n != nil {
		if !n.damageStale && !n.damageVolatile && !n.styleDirty && n.themeSeen == themeGen.Load() && n.styleDmg == styleGen.Load() && n.damageIcons == c.icons {
			return render.Rect{}
		}
		n.damageStale, n.damageIcons = false, c.icons
		reach := len(c.backdrop)
		defer func() { n.damageVolatile = len(c.backdrop) != reach }()
	}
	// A style-marked widget recomputes before it drains: pre-order, so
	// a parent's inherited change marks and damages its subtree within
	// this same walk, and a class toggle lands in one frame.
	if n := nodeOf(w); n != nil && n.styleDirty {
		n.restyle(w)
	}
	bbox := render.Rect{}
	if n := nodeOf(w); n != nil {
		if v := n.style(w); v.Has(style.PropBackdropFilter) && !v.Backdrop.Empty() {
			if bs, ok := w.(Boundser); ok {
				reach := v.Backdrop.Extent().Grow(bs.Bounds())
				if c.clipped {
					reach = reach.Intersect(c.clip)
				}
				if !reach.Empty() {
					c.backdrop = append(c.backdrop, reach)
				}
			}
		}
	}
	savedClip, savedClipped, savedShift := c.clip, c.clipped, c.shift
	defer func() { c.clip, c.clipped, c.shift = savedClip, savedClipped, savedShift }()
	n := nodeOf(w)
	clip := c.clipped && n != nil && n.ownInkBounded(w)
	if sd, ok := w.(scrollDrainer); ok {
		sd.drainScroll(c)
	}
	if dn, ok := w.(damageNoder); ok {
		var why string
		if debug.Enabled {
			why = damageReasons(nodeOf(w))
		}
		bounds, extras, dirty := dn.takeDamage()
		if debug.Enabled && dirty && !bounds.Empty() {
			noteDamage(w, bounds, why)
		}
		if dirty {
			bbox = bbox.Union(c.emit(bounds, clip))
			for _, r := range extras {
				bbox = bbox.Union(c.emit(r, clip))
			}
		}
	}
	if c.shiftNext.moved() {
		c.shift.dx += c.shiftNext.dx
		c.shift.dy += c.shiftNext.dy
		c.shiftNext = viewShift{}
	}
	if dc, ok := w.(damageClipper); ok {
		if clip, clips := dc.damageClip(); clips {
			if c.clipped {
				clip = clip.Intersect(c.clip)
			}
			c.clip, c.clipped = clip, true
		}
	}
	if n != nil && n.culled {
		c.clip, c.clipped = render.Rect{}, true
	}
	c.kids.each(func(kids []Widget) []Widget { return appendKids(w, kids) }, func(k Widget) {
		if k != nil {
			markUnlinked(w, k)
			bbox = bbox.Union(c.collectInto(k))
		}
	})
	return bbox
}

func (c *DamageCollector) emit(r render.Rect, clip bool) render.Rect {
	box := render.Rect{}
	for _, d := range [2]render.Rect{r, shifted(r, c.shift.dx, c.shift.dy)} {
		if clip {
			d = d.Intersect(c.clip)
		}
		if !d.Empty() {
			c.rects = append(c.rects, d)
			box = box.Union(d)
		}
		if !c.shift.moved() {
			break
		}
	}
	return box
}

// widenForBackdrops adds every backdrop reach the damage touches: a
// backdrop filter reads the backdrop around its box, and outside the
// damage the canvas still holds last frame's filtered pixels, so a box
// repaints whole (and the reach around it) whenever any of it does.
// Added reaches can touch further boxes, so it runs to a fixed point.
// It returns the bounding box of what it added.
func (c *DamageCollector) widenForBackdrops() render.Rect {
	added := render.Rect{}
	for grew := true; grew; {
		grew = false
		for i := 0; i < len(c.backdrop); {
			reach := c.backdrop[i]
			if !touches(c.rects, reach) {
				i++
				continue
			}
			c.rects = append(c.rects, reach)
			added = added.Union(reach)
			c.backdrop[i] = c.backdrop[len(c.backdrop)-1]
			c.backdrop = c.backdrop[:len(c.backdrop)-1]
			grew = true
		}
	}
	return added
}

// touches reports whether any rect overlaps r.
func touches(rects []render.Rect, r render.Rect) bool {
	for _, d := range rects {
		if !d.Intersect(r).Empty() {
			return true
		}
	}
	return false
}

// InkRect is the rect w paints over: the rect it is shown at grown by
// what its paint overflows it with, a shadow or an outline, and by the
// extent a widget painting past its ink declares (a toast's shadow ring
// and entrance slide).
func InkRect(w Widget) render.Rect {
	r := ShownRect(w)
	if n := nodeOf(w); n != nil {
		r = n.ink.Grow(r)
	}
	if pe, ok := w.(paintExtenter); ok {
		r = r.Union(pe.paintExtent())
	}
	return r
}

// paintExtenter is a widget whose paint can reach past its ink.
type paintExtenter interface{ paintExtent() render.Rect }

// DamageArea returns the total pixel area the rects cover, counting
// overlaps once per rect. Tests use it to pin how much of a frame a
// damage-restricted repaint touches.
func DamageArea(rects []render.Rect) int {
	area := 0
	for _, r := range rects {
		if !r.Empty() {
			area += r.W * r.H
		}
	}
	return area
}
