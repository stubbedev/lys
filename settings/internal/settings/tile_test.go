package settings

import (
	"testing"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

type testTile struct {
	tile
}

func (t *testTile) Measure(widget.Axis, int) widget.Sizes { return widget.FixedSizes(40) }

func (t *testTile) Paint(*render.Canvas) {}

// pressKey drives w through a router: focused, then the key a.
func pressKey(t *testing.T, root, w widget.Widget, a widget.KeyAction) {
	t.Helper()
	r := &widget.Router{Root: root}
	if err := r.SetFocus(w); err != nil {
		t.Fatal(err)
	}
	r.KeyAction(a, 0)
}

// Both kinds of tile, a picture that is its own button and a spot a
// picture draws, take the focus as buttons: Enter and Space activate
// them, another key does not; a press on a tile activates it and its
// hit test finds the widget embedding it, while a spot takes no pointer.
func TestEveryKindOfTileIsAButtonEnterAndSpaceActivate(t *testing.T) {
	ran := 0
	tl := &testTile{}
	tl.tile = newTile(tl, func() { ran++ })
	tl.Arrange(render.Rect{W: 40, H: 40})
	var picked []int
	sp := newSpot(3, func(i int) { picked = append(picked, i) })
	box := gapBox(widget.Row)
	box.Append(tl, false)
	box.Append(sp, false)
	box.Arrange(render.Rect{W: 200, H: 40})
	if tl.Role() != widget.RoleButton || sp.Role() != widget.RoleButton {
		t.Fatal("a tile reads as no button")
	}
	for _, a := range []widget.KeyAction{widget.KeyEnter, widget.KeySpace} {
		pressKey(t, box, tl, a)
		pressKey(t, box, sp, a)
	}
	pressKey(t, box, tl, widget.KeyDown)
	pressKey(t, box, sp, widget.KeyDown)
	if ran != 2 || len(picked) != 2 || picked[0] != 3 {
		t.Fatalf("Enter and Space ran the tile %d times and the spot %v, want twice each and no other key", ran, picked)
	}
	b := tl.Bounds()
	if got := tl.HitTest(widget.Point{X: b.X + 1, Y: b.Y + 1}); got != widget.Widget(tl) {
		t.Errorf("the tile's hit test found %T, want the widget embedding it", got)
	}
	if tl.HitTest(widget.Point{X: b.X + b.W + 5, Y: b.Y}) != nil || sp.HitTest(widget.Point{}) != nil {
		t.Error("a hit test found a tile outside it, or a spot at all")
	}
	tl.PressAt(widget.Point{})
	if ran != 3 {
		t.Fatalf("a press ran the tile %d times in all, want 3", ran)
	}
	if sp.Measure(widget.Row, 0) != widget.FixedSizes(0) {
		t.Error("a spot takes room")
	}
}
