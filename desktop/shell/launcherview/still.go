package launcherview

import (
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

// Still is a session's panel resting where its look puts it in the
// still's own bounds, the output it stands for: the place and size the
// shell springs a panel to, without the springs or a surface.
type Still struct {
	widget.Base
	session *Session
	panel   *widget.Box
	kids    [1]widget.Widget
}

// NewStill rests s's panel in the still, its content clipped to it: a
// still caps the panel to its place, where the shell's surface would cut
// a list longer than it.
func NewStill(s *Session) *Still {
	st := &Still{session: s, panel: NewPanelBox()}
	st.panel.SetInlineStyle("overflow: hidden;")
	st.panel.Append(s.Content, true)
	return st
}

// Panel is the panel box.
func (st *Still) Panel() *widget.Box { return st.panel }

// Measure wants nothing: the still is whatever output it is given.
func (st *Still) Measure(widget.Axis, int) widget.Sizes { return widget.Sizes{} }

// Arrange lays the panel at its resting place.
func (st *Still) Arrange(r render.Rect) {
	st.ArrangeSelf(r)
	w, h := st.session.TargetIn(r, 0)
	st.panel.Arrange(st.session.PlaceIn(r, w, h))
	st.kids[0] = st.panel
	widget.SetParents(st, st.kids[:]...)
}

// ArrangeRoot mirrors Arrange for the root path.
func (st *Still) ArrangeRoot(r render.Rect) { st.Arrange(r) }

// Paint paints the panel inside the still.
func (st *Still) Paint(cv *render.Canvas) {
	prev := cv.PushClip(st.Bounds())
	widget.PaintChild(cv, st.panel)
	cv.PopClip(prev)
}

// Children is the panel.
func (st *Still) Children() []widget.Widget { st.kids[0] = st.panel; return st.kids[:] }

// HitTest descends into the panel.
func (st *Still) HitTest(p widget.Point) widget.Widget {
	b := st.Bounds()
	if p.X < b.X || p.Y < b.Y || p.X >= b.X+b.W || p.Y >= b.Y+b.H {
		return nil
	}
	return st.panel.HitTest(p)
}
