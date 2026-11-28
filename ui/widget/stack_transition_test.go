package widget

import (
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/render"
)

// stackAt is a 40x20 stack at (20, 20) with a red "a" and a blue "b"
// page.
func stackAt(t *testing.T, c *animClock, tr StackTransition) *Stack {
	t.Helper()
	s := NewStack()
	c.host(t, s)
	s.Add("a", &solidLeaf{sz: Size{W: 40, H: 20}, col: render.RGB(255, 0, 0)})
	s.Add("b", &solidLeaf{sz: Size{W: 40, H: 20}, col: render.RGB(0, 0, 255)})
	s.SetTransition(tr, 100*time.Millisecond)
	SizeWithin(s, Size{W: 80, H: 60})
	s.Arrange(render.Rect{X: 20, Y: 20, W: 40, H: 20})
	return s
}

func paintStack(s *Stack) shot {
	data := make([]byte, render.Stride(80)*60)
	cv := render.New(data, render.Stride(80), 80, 60)
	cv.Clear(cv.Rect(), revealBG)
	s.Paint(cv)
	return data
}

func blueAt(data shot, x, y int) uint8 { return data[y*render.Stride(80)+x*4] }

func TestStackWithoutTransitionSwitchesAtOnce(t *testing.T) {
	c := pinAnimClock(t)
	s := stackAt(t, c, StackNone)
	s.Show("b")
	if s.Switching() || c.step() {
		t.Error("a plain stack animated")
	}
	if px := paintStack(s); blueAt(px, 30, 30) != 255 || redAt(px, 30, 30) != 0 {
		t.Error("the new page is not shown at once")
	}
}

func TestStackCrossfade(t *testing.T) {
	c := pinAnimClock(t)
	s := stackAt(t, c, StackCrossfade)
	s.Show("b")
	if !s.Switching() {
		t.Fatal("no switch running")
	}
	advance(c, 30*time.Millisecond)
	px := paintStack(s)
	if r, b := redAt(px, 30, 30), blueAt(px, 30, 30); r == 0 || b == 0 || int(r)+int(b) < 250 {
		t.Errorf("mid crossfade red %d blue %d: want both, summing to full", r, b)
	}
	c.drive()
	if s.Switching() {
		t.Error("still switching after the duration")
	}
	if px := paintStack(s); blueAt(px, 30, 30) != 255 || redAt(px, 30, 30) != 0 {
		t.Error("the crossfade did not land on the new page")
	}
}

func TestStackSlideFollowsPageOrder(t *testing.T) {
	c := pinAnimClock(t)
	s := stackAt(t, c, StackSlideLeftRight)
	s.Show("b") // a later page comes in from the right
	advance(c, 30*time.Millisecond)
	px := paintStack(s)
	if redAt(px, 21, 30) != 255 || blueAt(px, 58, 30) != 255 {
		t.Errorf("forward slide: left red %d, right blue %d", redAt(px, 21, 30), blueAt(px, 58, 30))
	}
	if got := lit(px, render.Rect{X: 60, Y: 20, W: 20, H: 20}); got != 0 {
		t.Errorf("%d px painted outside the stack", got)
	}
	c.drive()
	s.Show("a") // an earlier page comes in from the left
	advance(c, 30*time.Millisecond)
	px = paintStack(s)
	if redAt(px, 21, 30) != 255 || blueAt(px, 58, 30) != 255 {
		t.Errorf("backward slide: left red %d, right blue %d", redAt(px, 21, 30), blueAt(px, 58, 30))
	}
	s.Remove("b")
	if s.Switching() {
		t.Error("removing the leaving page left the switch running on nothing")
	}
}

// TestAStackSwitchPaintsEachPageOnce pins that a running switch keeps
// what it painted: a page paints into its layer once, again only after
// it owed damage, and the frames between composite the layers.
func TestAStackSwitchPaintsEachPageOnce(t *testing.T) {
	c := pinAnimClock(t)
	s := stackAt(t, c, StackCrossfade)
	a, b := s.Page("a").(*solidLeaf), s.Page("b").(*solidLeaf)
	var damage DamageCollector
	frame := func() shot {
		damage.Collect(s)
		return paintStack(s)
	}
	frame()
	s.Show("b")
	advance(c, 10*time.Millisecond)
	frame()
	if a.paints != 2 || b.paints != 1 {
		t.Fatalf("the first switch frame painted a %d and b %d times, want a at rest and in its layer, b once", a.paints, b.paints)
	}
	for range 3 {
		advance(c, 20*time.Millisecond)
		mid := frame()
		if r, bl := redAt(mid, 30, 30), blueAt(mid, 30, 30); r == 0 || bl == 0 {
			t.Fatalf("mid switch red %d blue %d, want both pages showing", r, bl)
		}
	}
	if a.paints != 2 || b.paints != 1 {
		t.Errorf("frames that changed no page painted a %d and b %d times, want neither again", a.paints-2, b.paints-1)
	}
	b.col = render.RGB(0, 255, 0)
	b.Invalidate()
	frame()
	if a.paints != 2 || b.paints != 2 {
		t.Errorf("a change on the coming page painted a %d and b %d times again, want only b once", a.paints-2, b.paints-1)
	}
	a.col = render.RGB(0, 0, 0)
	a.Invalidate()
	got := frame()
	if a.paints != 3 {
		t.Errorf("a change on the leaving page painted it %d times again, want once", a.paints-2)
	}
	if r := redAt(got, 30, 30); r != 0 {
		t.Errorf("red %d after the leaving page turned black: its change does not show", r)
	}
}
