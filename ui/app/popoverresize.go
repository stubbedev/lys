package app

import (
	"errors"
	"fmt"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/third_party/wayland/xdg"

	"github.com/stubbedev/lys/ui/internal/popup"
	"github.com/stubbedev/lys/ui/internal/wlsession"
	"github.com/stubbedev/lys/ui/widget"
)

// PopoverResize makes a popover resizable by dragging its free edge, the
// one away from its anchor, or either corner of that edge. The edge
// facing the anchor stays put, and so does the side edge across from a
// dragged corner. While a drag is under way a press elsewhere does not
// close an auto-hiding popover.
type PopoverResize struct {
	// Min and Max bound the content size a drag asks for; the largest
	// output bounds it too.
	Min, Max widget.Size
	// Resizing hears each size a drag asks for, on the loop, for the
	// content to take: the popover measures its content within that
	// size from then on, so it grows only as far as its content does.
	Resizing func(widget.Size)
	// Resized hears the content size a drag ended at, on the loop; nil
	// for none.
	Resized func(widget.Size)
}

func (r *PopoverResize) validate() error {
	switch {
	case r.Resizing == nil:
		return errors.New("app: a resizable popover needs Resizing")
	case r.Min.W <= 0 || r.Min.H <= 0:
		return fmt.Errorf("app: a popover's minimum size %dx%d is not positive", r.Min.W, r.Min.H)
	case r.Max.W < r.Min.W || r.Max.H < r.Min.H:
		return fmt.Errorf("app: a popover's maximum size %dx%d is below its minimum %dx%d", r.Max.W, r.Max.H, r.Min.W, r.Min.H)
	}
	return nil
}

// layoutMover is content with a layout motion of its own.
type layoutMover interface {
	SetLayoutMotion(widget.LayoutMotion)
	LayoutMotion() widget.LayoutMotion
}

// freeEdge is the resize edge away from the anchor of a popover placed
// on side.
func freeEdge(side Gravity) uint32 {
	switch side {
	case GravityTop:
		return xdg.ToplevelResizeEdgeTop
	case GravityLeft:
		return xdg.ToplevelResizeEdgeLeft
	case GravityRight:
		return xdg.ToplevelResizeEdgeRight
	}
	return xdg.ToplevelResizeEdgeBottom
}

// holdFor is the cross-axis edge a drag of edges keeps in place on a
// popover placed on side: the one across from a dragged corner, and the
// start where the drag moves the free edge alone, so a popover a corner
// drag moved off centre stays where it is.
func holdFor(side Gravity, edges uint32) popup.Hold {
	far := uint32(xdg.ToplevelResizeEdgeLeft)
	if side == GravityLeft || side == GravityRight {
		far = xdg.ToplevelResizeEdgeTop
	}
	if edges&far != 0 {
		return popup.HoldEnd
	}
	return popup.HoldStart
}

// dragSize is the content size a drag of edges from from to at asks
// of content start large, within lo and hi.
func dragSize(edges uint32, from, at widget.Point, start, lo, hi widget.Size) widget.Size {
	w, h := start.W, start.H
	switch {
	case edges&xdg.ToplevelResizeEdgeLeft != 0:
		w -= at.X - from.X
	case edges&xdg.ToplevelResizeEdgeRight != 0:
		w += at.X - from.X
	}
	switch {
	case edges&xdg.ToplevelResizeEdgeTop != 0:
		h -= at.Y - from.Y
	case edges&xdg.ToplevelResizeEdgeBottom != 0:
		h += at.Y - from.Y
	}
	return widget.Size{W: min(max(w, lo.W), hi.W), H: min(max(h, lo.H), hi.H)}
}

// resizeSlack is the room a resize drag gives the surface beyond the
// content on each axis it drags, which takes input too: a pointer that
// outruns the content's edge for a frame stays on the surface, where a
// compositor would otherwise send it a leave that ends the drag.
const resizeSlack = 128

// dragSlack is the room a drag of edges adds to the surface across and
// down.
func dragSlack(edges uint32) (w, h int) {
	if edges&(xdg.ToplevelResizeEdgeLeft|xdg.ToplevelResizeEdgeRight) != 0 {
		w = resizeSlack
	}
	if edges&(xdg.ToplevelResizeEdgeTop|xdg.ToplevelResizeEdgeBottom) != 0 {
		h = resizeSlack
	}
	return w, h
}

// popoverGrip is the resize handle along a resizable popover's free
// edge: the popover's key root hit-tests it over the content's edge, and
// it turns a press there into a resize drag.
type popoverGrip struct {
	widget.Base
	op     *openPopover
	resize PopoverResize
	// outputs is the session's outputs, which bound the size.
	outputs func() []*wlsession.Output
	// hover is the edges under the pointer at the last hit test.
	hover uint32
	drag  *gripDrag
	// endX and endY place the content at the end of a surface larger
	// than it on that axis, against the edges a drag holds: the right
	// one, the bottom one.
	endX, endY bool
}

// gripDrag is one resize drag: the edges it moves, where it started in
// the parent's coordinates, the content's size then, and the content's
// layout motion, which the drag holds still.
type gripDrag struct {
	edges  uint32
	from   widget.Point
	start  widget.Size
	motion widget.LayoutMotion
}

// dragging is the edges a drag under way moves, 0 without one.
func (g *popoverGrip) dragging() uint32 {
	if g == nil || g.drag == nil {
		return 0
	}
	return g.drag.edges
}

// align places content, size large, inside rect against the edges the
// last drag held.
func (g *popoverGrip) align(rect render.Rect, size widget.Size) render.Rect {
	out := render.Rect{X: rect.X, Y: rect.Y, W: size.W, H: size.H}
	if g != nil && g.endX {
		out.X = rect.X + rect.W - size.W
	}
	if g != nil && g.endY {
		out.Y = rect.Y + rect.H - size.H
	}
	return out
}

func newPopoverGrip(op *openPopover, resize PopoverResize, outputs func() []*wlsession.Output) *popoverGrip {
	g := &popoverGrip{op: op, resize: resize, outputs: outputs}
	g.SetFocusOnClick(false)
	widget.SetParents(op.keyRoot, g)
	return g
}

// edgesAt is the resize edges under p, in the popover's surface
// coordinates: the free edge or one of its corners, 0 elsewhere.
func (g *popoverGrip) edgesAt(p widget.Point) uint32 {
	b := g.op.keyRoot.contentRect
	if !b.Contains(p.X, p.Y) {
		return 0
	}
	edges := resizeEdge(b.W, b.H, float64(p.X-b.X), float64(p.Y-b.Y))
	if edges&freeEdge(g.op.pop.Side()) == 0 {
		return 0
	}
	return edges
}

// hit reports whether the grip takes the pointer at p: over an edge, or
// anywhere while it drags.
func (g *popoverGrip) hit(p widget.Point) bool {
	if g.drag != nil {
		return true
	}
	g.hover = g.edgesAt(p)
	return g.hover != 0
}

// inParent is p, in the surface's coordinates, in its parent's: the
// surface moves while a drag holds the far edge of a popover above or
// left of its anchor.
func (g *popoverGrip) inParent(p widget.Point) widget.Point {
	x, y, _ := g.op.pop.Position()
	return widget.Point{X: x + p.X, Y: y + p.Y}
}

// limit is the largest size a drag reaches: Max, within the largest
// output.
func (g *popoverGrip) limit() widget.Size {
	bound := popoverCeiling(g.outputs(), 0, 0)
	return widget.Size{W: min(g.resize.Max.W, bound.W), H: min(g.resize.Max.H, bound.H)}
}

func (g *popoverGrip) Measure(widget.Axis, int) widget.Sizes { return widget.Sizes{} }

func (g *popoverGrip) Arrange(r render.Rect) { g.ArrangeSelf(r) }

func (g *popoverGrip) Paint(*render.Canvas) {}

func (g *popoverGrip) HitTest(widget.Point) widget.Widget { return g }

// CursorName is the resize shape of the edges dragged or under the
// pointer.
func (g *popoverGrip) CursorName() string {
	if g.drag != nil {
		return resizeCursor(g.drag.edges)
	}
	return resizeCursor(g.hover)
}

// PressAt starts a drag of the edges under p.
func (g *popoverGrip) PressAt(p widget.Point) {
	edges := g.edgesAt(p)
	if edges == 0 {
		return
	}
	b := g.op.keyRoot.contentRect
	g.drag = &gripDrag{edges: edges, from: g.inParent(p), start: widget.Size{W: b.W, H: b.H}}
	if m, ok := g.op.keyRoot.content.(layoutMover); ok {
		g.drag.motion = m.LayoutMotion()
		m.SetLayoutMotion(widget.LayoutMotionNone)
	}
	g.op.popover.Root().resizing = true
	side := g.op.pop.Side()
	hold := holdFor(side, edges)
	g.op.pop.Hold(hold)
	switch side {
	case GravityTop, GravityBottom:
		g.endX, g.endY = hold == popup.HoldEnd, side == GravityTop
	default:
		g.endX, g.endY = side == GravityLeft, hold == popup.HoldEnd
	}
	g.op.pop.CatchAll(true)
}

// DragMove resizes the popover to the size the drag asks for.
func (g *popoverGrip) DragMove(p widget.Point) {
	if g.drag == nil {
		return
	}
	size := dragSize(g.drag.edges, g.drag.from, g.inParent(p), g.drag.start, g.resize.Min, g.limit())
	g.op.keyRoot.maxW, g.op.keyRoot.maxH = size.W, size.H
	g.resize.Resizing(size)
	if l, ok := g.op.keyRoot.content.(interface{ InvalidateLayout() }); ok {
		l.InvalidateLayout()
	}
	g.op.fitContent(g.outputs())
	g.op.pop.MarkFrame()
}

// PressEnd ends the drag and reports the size the content took.
func (g *popoverGrip) PressEnd() {
	d := g.drag
	if d == nil {
		return
	}
	g.drag = nil
	if m, ok := g.op.keyRoot.content.(layoutMover); ok {
		m.SetLayoutMotion(d.motion)
	}
	g.op.popover.Root().resizing = false
	g.op.pop.CatchAll(false)
	g.op.fitContent(g.outputs())
	g.op.pop.MarkFrame()
	if g.resize.Resized != nil {
		g.resize.Resized(g.op.contentSize(g.outputs()))
	}
}
