package widget

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/ui/anim"
)

type backdropProbe struct {
	node
	shape []render.Rect
}

func (b *backdropProbe) BackdropRegion() []render.Rect { return b.shape }
func (b *backdropProbe) HitTest(Point) Widget          { return nil }
func (b *backdropProbe) Paint(*render.Canvas)          {}
func (b *backdropProbe) Measure(Axis, int) Sizes       { return Sizes{} }
func (b *backdropProbe) Arrange(r render.Rect)         { b.node.Arrange(r) }

func extent(rects []render.Rect) render.Rect {
	var u render.Rect
	for _, r := range rects {
		u = u.Union(r)
	}
	return u
}

// The blur region never leads the shape it backs: hidden there is none,
// it grows with the unrolling clip and the scale, and at rest it is the
// content's exactly.
func TestMorphBackdropRegionFollowsTheReveal(t *testing.T) {
	probe := &backdropProbe{shape: []render.Rect{{X: 10, Y: 10, W: 100, H: 200}}}
	m := NewMorph(probe)
	m.SetOrigin(0.5, 0)
	m.SetPlate(10, nil)
	m.Arrange(render.Rect{W: 120, H: 220})

	m.SetReveal(0, anim.VisualGrow)
	if got := m.BackdropRegion(); len(got) != 0 {
		t.Fatalf("hidden: region %v, want none", got)
	}
	m.SetReveal(0.05, anim.VisualGrow)
	early := extent(m.BackdropRegion())
	if early.H >= 100 || early.W >= 100 || early.Y != 10 {
		t.Fatalf("early in the motion: region %+v, want a short, scaled one from the top edge", early)
	}
	m.SetReveal(0.5, anim.VisualGrow)
	mid := extent(m.BackdropRegion())
	if mid.H <= early.H || mid.W <= early.W || mid.H >= 200 {
		t.Fatalf("mid motion: region %+v after %+v, want it grown but not whole", mid, early)
	}
	m.SetReveal(1, anim.VisualGrow)
	if got := m.BackdropRegion(); !slices.Equal(got, probe.shape) {
		t.Fatalf("at rest: region %v, want the content's %v", got, probe.shape)
	}
	m.SetReveal(0.5, anim.VisualFade)
	if got := m.BackdropRegion(); !slices.Equal(got, probe.shape) {
		t.Fatalf("a plain fade: region %v, want the content's %v", got, probe.shape)
	}
}

// The motion cuts the content's own shape rather than replacing it: a
// transparent surface holding one small glass card frosts the card alone.
func TestMorphBackdropRegionKeepsTheContentsShape(t *testing.T) {
	probe := &backdropProbe{shape: []render.Rect{{X: 40, Y: 20, W: 20, H: 20}}}
	m := NewMorph(probe)
	m.SetOrigin(0.5, 0)
	m.Arrange(render.Rect{W: 100, H: 100})
	m.SetReveal(0.8, anim.VisualGrow)
	got := extent(m.BackdropRegion())
	if got.W > 21 || got.H > 21 || got.X < 38 {
		t.Fatalf("region %+v, want the card, scaled", got)
	}
}

func TestMorphFadeLeadsTheMotion(t *testing.T) {
	m := NewMorph(&backdropProbe{})
	for _, c := range []struct {
		reveal float64
		visual anim.Visual
		want   float64
	}{
		{0, anim.VisualGrow, 0},
		{0.5, anim.VisualGrow, 0.8},
		{1, anim.VisualGrow, 1},
		{1.05, anim.VisualGrow, 1},
		{-0.1, anim.VisualGrow, 0},
		{0.5, anim.VisualFade, 0.5},
		{0.5, anim.VisualLift, 0.5},
		{1, anim.VisualLift, 1},
		{0, anim.VisualLift, 0},
	} {
		m.SetReveal(c.reveal, c.visual)
		if got := m.Fade(); got != c.want {
			t.Errorf("reveal %v visual %v: fade %v, want %v", c.reveal, c.visual, got, c.want)
		}
	}
}

// A lifting surface's blur region is the whole content scaled up about
// its center by the paint's own scale, LiftScale when hidden, and the
// content's exactly at rest.
func TestMorphBackdropRegionLiftsAboutTheCenter(t *testing.T) {
	probe := &backdropProbe{shape: []render.Rect{{X: 20, Y: 20, W: 100, H: 200}}}
	m := NewMorph(probe)
	m.SetOrigin(0.5, 0)
	m.SetPlate(20, nil)
	m.Arrange(render.Rect{W: 140, H: 240})

	m.SetReveal(0.5, anim.VisualLift)
	mid := extent(m.BackdropRegion())
	if mid.W <= 100 || mid.H <= 200 || mid.X+mid.W/2 != 70 || mid.Y+mid.H/2 != 120 {
		t.Fatalf("mid lift: region %+v, want the content grown about its center", mid)
	}
	m.SetReveal(0.1, anim.VisualLift)
	if late := extent(m.BackdropRegion()); late.W <= mid.W || late.H <= mid.H || float64(late.W) > 100*LiftScale+1 {
		t.Fatalf("late in the exit: region %+v after %+v, want it grown further, at most LiftScale", late, mid)
	}
	m.SetReveal(1, anim.VisualLift)
	if got := m.BackdropRegion(); !slices.Equal(got, probe.shape) {
		t.Fatalf("at rest: region %v, want the content's %v", got, probe.shape)
	}
}
