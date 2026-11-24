package widget

import (
	"testing"

	"github.com/stubbedev/lys/lib/render"
)

func TestWidgetsWithEqualComputedStylesShareOne(t *testing.T) {
	face := goldenFace(t)
	loadCSS(t, `label { color: #101010; } label.loud { color: #f00000; }`)
	a, b := NewLabel(face, 14, "a", 0), NewLabel(face, 14, "b", 0)
	loud := NewLabel(face, 14, "c", 0)
	loud.AddClass("loud")
	root := NewBox(Column, 0, 0)
	root.Append(a, false)
	root.Append(b, false)
	root.Append(loud, false)
	arrangeTree(t, root, 200, 100)
	if a.style(a) != b.style(b) {
		t.Error("two labels computing the same style hold a copy each")
	}
	if loud.style(loud) == a.style(a) {
		t.Fatal("a label of another class shares the plain labels' style")
	}
	if got := loud.style(loud).Color; got != render.RGB(0xf0, 0, 0) {
		t.Errorf("the loud label's colour is %08x", uint32(got))
	}
	b.AddClass("loud")
	CollectDamage(root)
	if b.style(b) != loud.style(loud) || a.style(a).Color != render.RGB(0x10, 0x10, 0x10) {
		t.Error("a label that took the class did not move to that class's shared style, or moved its old neighbour with it")
	}
}

func TestATransitionWritesOnlyItsOwnWidgetsStyle(t *testing.T) {
	c := pinAnimClock(t)
	loadCSS(t, `button { background-color: #000000; transition: background-color 200ms; } button:hover { background-color: #ffffff; }`)
	moving, still := NewButton(NewSpacer(0, 0), 0, 0), NewButton(NewSpacer(0, 0), 0, 0)
	settled := NewButton(NewSpacer(0, 0), 0, 0)
	settled.SetHovered(true)
	root := NewBox(Row, 0, 0)
	root.Append(moving, true)
	root.Append(still, true)
	root.Append(settled, true)
	c.arrangeTree(t, root, 300, 40)
	if got := settled.style(settled).Background; got != render.RGB(255, 255, 255) {
		t.Fatalf("a button hovered from its first style is %#08x, want white at once", uint32(got))
	}
	moving.SetHovered(true)
	_ = moving.style(moving)
	c.step()
	mid := moving.style(moving).Background
	if mid == render.RGB(0, 0, 0) || mid == render.RGB(255, 255, 255) {
		t.Fatalf("the hovered button's background %#08x mid-flight, want it between the ends", uint32(mid))
	}
	if got := still.style(still).Background; got != render.RGB(0, 0, 0) {
		t.Errorf("the resting button's background moved with the transition: %#08x", uint32(got))
	}
	if got := settled.style(settled).Background; got != render.RGB(255, 255, 255) {
		t.Errorf("the button already hovered moved with the other's transition: %#08x", uint32(got))
	}
	fresh := NewButton(NewSpacer(0, 0), 0, 0)
	root.Append(fresh, true)
	c.arrangeTree(t, root, 400, 40)
	if got := fresh.style(fresh).Background; got != render.RGB(0, 0, 0) {
		t.Errorf("a button added mid-flight starts at %#08x, want black", uint32(got))
	}
}
