package settings

import (
	"cmp"
	"math"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/displays"
)

const (
	canvasHeight = 240
	canvasMinW   = 240
	canvasPad    = 16
	snapPx       = 10
	tileRadius   = 6
	tileGap      = 2
	focusRing    = 2
)

type displaysCanvas struct {
	widget.Base
	p *displaysPanel
	arrangement
	rects   []displays.Rect
	index   []int
	handles []*outputHandle
	labels  map[string]string
	drag    int
	grabX   float64
	grabY   float64
	dragged displays.Rect
}

type outputHandle struct {
	widget.Base
	c  *displaysCanvas
	at int
}

func newDisplaysCanvas(p *displaysPanel) *displaysCanvas {
	return &displaysCanvas{Base: widget.NewBase(), p: p, drag: -1}
}

func (c *displaysCanvas) layout() {
	c.rects, c.index = c.p.enabledRects()
	for len(c.handles) < len(c.rects) {
		h := &outputHandle{Base: widget.NewBase(), c: c, at: len(c.handles)}
		c.handles = append(c.handles, h)
		widget.SetParents(c, h)
	}
	c.handles = c.handles[:len(c.rects)]
	c.labels = headLabels(c.p.heads)
	for i, h := range c.handles {
		name := c.p.draft[c.index[i]].Name
		h.SetAccessibleName(cmp.Or(c.labels[name], name) + " (" + name + ")")
	}
	c.InvalidateLayout()
	c.Invalidate()
}

func (c *displaysCanvas) Children() []widget.Widget {
	out := make([]widget.Widget, len(c.handles))
	for i, h := range c.handles {
		out[i] = h
	}
	return out
}

func (c *displaysCanvas) Measure(axis widget.Axis, _ int) widget.Sizes {
	if axis == widget.Row {
		return widget.Sizes{Min: canvasMinW, Nat: canvasMinW}
	}
	return widget.FixedSizes(canvasHeight)
}

func (c *displaysCanvas) Arrange(r render.Rect) {
	c.ArrangeSelf(r)
	c.fit(c.rects, r)
	for i, h := range c.handles {
		h.Arrange(c.screen(c.rects[i]))
	}
}

func (c *displaysCanvas) Paint(cv *render.Canvas) {
	th := widget.Current()
	cv.RoundedRect(c.Bounds(), tileRadius, th.MutedColor())
	for i, r := range c.rects {
		if i == c.drag {
			r = c.dragged
		}
		tile := c.screen(r)
		selected := c.index[i] == c.p.selected
		fill, ink := th.CardColor(), th.Text
		if selected {
			fill, ink = th.Accent, th.OnAccent
		}
		cv.RoundedRect(tile, tileRadius, fill)
		if widget.FocusVisible(c.handles[i]) {
			cv.RoundedBorder(tile, render.UniformCorners(tileRadius), render.UniformInsets(focusRing), th.Text)
		}
		name := c.p.draft[c.index[i]].Name
		c.p.k.face.DrawAligned(cv, cmp.Or(c.labels[name], name), tile, c.p.k.textPx(), ink, render.AlignCenter)
	}
}

func (c *displaysCanvas) HitTest(p widget.Point) widget.Widget { return c.HitLeaf(c, p) }

func (c *displaysCanvas) PressAt(p widget.Point) {
	x, y := c.logical(p)
	if i := rectAt(c.rects, x, y); i >= 0 {
		r := c.rects[i]
		c.p.selectOutput(c.index[i])
		c.drag, c.dragged = i, r
		c.grabX, c.grabY = x-float64(r.X), y-float64(r.Y)
		c.Invalidate()
	}
}

func (c *displaysCanvas) DragMove(p widget.Point) {
	if c.drag < 0 {
		return
	}
	x, y := c.logical(p)
	moving := append([]displays.Rect(nil), c.rects...)
	moving[c.drag] = displays.Rect{X: int(math.Round(x - c.grabX)), Y: int(math.Round(y - c.grabY)), W: c.dragged.W, H: c.dragged.H}
	c.dragged = displays.Snap(moving, c.drag, int(math.Ceil(snapPx/c.scale)))
	c.Invalidate()
}

func (c *displaysCanvas) PressEnd() {
	if c.drag < 0 {
		return
	}
	i := c.drag
	c.drag = -1
	rects := append([]displays.Rect(nil), c.rects...)
	rects[i] = c.dragged
	c.p.moved(c.index, displays.Settle(rects, i))
}

func (c *displaysCanvas) FocusMoved(_, to widget.Widget) {
	if h, ok := to.(*outputHandle); ok && h.c == c && h.at < len(c.index) {
		c.p.selectOutput(c.index[h.at])
	}
	c.Invalidate()
}

func (c *displaysCanvas) nudge(i, dx, dy int, toEdge bool) {
	rects := append([]displays.Rect(nil), c.rects...)
	if toEdge {
		next, ok := displays.NextEdge(rects, i, dx, dy)
		if !ok {
			return
		}
		rects[i] = next
	} else {
		rects[i].X += dx
		rects[i].Y += dy
	}
	c.p.moved(c.index, displays.Settle(rects, i))
}

func (h *outputHandle) Measure(widget.Axis, int) widget.Sizes { return widget.FixedSizes(0) }

func (h *outputHandle) Paint(*render.Canvas) {}

func (h *outputHandle) HitTest(widget.Point) widget.Widget { return nil }

func (h *outputHandle) Role() widget.Role { return widget.RoleButton }

func (h *outputHandle) KeyAction(a widget.KeyAction, mods widget.Mods) {
	toEdge := mods&widget.ModShift != 0
	switch a {
	case widget.KeyLeft:
		h.c.nudge(h.at, -1, 0, toEdge)
	case widget.KeyRight:
		h.c.nudge(h.at, 1, 0, toEdge)
	case widget.KeyUp:
		h.c.nudge(h.at, 0, -1, toEdge)
	case widget.KeyDown:
		h.c.nudge(h.at, 0, 1, toEdge)
	}
}
