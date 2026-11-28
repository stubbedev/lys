package widget

import (
	"fmt"
	"testing"

	"github.com/stubbedev/lys/lib/render"
)

// itemScroll is a scrolled column of items, each with a description
// that wraps beside a trailing switch, the shape of a settings page.
func itemScroll(t *testing.T, rows int) (*Scroll, []*Item) {
	t.Helper()
	face := entryFace(t)
	col := NewBox(Column, 0, 0)
	items := make([]*Item, 0, rows)
	for i := range rows {
		it := NewItem(face, fmt.Sprintf("Option %d", i), "What this option changes, in a sentence long enough to wrap beside the switch.")
		it.AddTrailing(NewSwitch(i%2 == 0))
		items = append(items, it)
		col.Append(it, false)
	}
	sc := NewScroll(col)
	sc.VerticalOnly = true
	return sc, items
}

func descMisses(items []*Item) int {
	n := 0
	for _, it := range items {
		n += it.desc.measureMisses
	}
	return n
}

func TestScrollingPastItemsWrapsNoDescriptionAgain(t *testing.T) {
	sc, items := itemScroll(t, 200)
	r := render.Rect{W: 300, H: 400}
	for range 2 {
		SizeWithin(sc, Size{W: r.W, H: r.H})
		sc.Arrange(r)
		CollectDamage(sc)
	}
	for step := 1; step <= 5; step++ {
		sc.SetOffset(0, step*12)
		before := descMisses(items)
		sc.Arrange(r)
		if got := descMisses(items) - before; got != 0 {
			t.Fatalf("step %d measured %d descriptions afresh: moving an item wrapped its description again", step, got)
		}
		CollectDamage(sc)
	}
}

func TestAnItemLaysOutAgainWhenAPartChanges(t *testing.T) {
	sc, items := itemScroll(t, 3)
	r := render.Rect{W: 300, H: 400}
	SizeWithin(sc, Size{W: r.W, H: r.H})
	sc.Arrange(r)
	CollectDamage(sc)
	desc := items[1].desc
	before := desc.Bounds().H
	items[1].SetDescription("Short.")
	SizeWithin(sc, Size{W: r.W, H: r.H})
	sc.Arrange(r)
	if after := desc.Bounds().H; after >= before {
		t.Errorf("a shorter description kept its height %d (was %d): the item reused a stale layout", after, before)
	}
	SizeWithin(sc, Size{W: 200, H: r.H})
	sc.Arrange(render.Rect{W: 200, H: r.H})
	if got := desc.Bounds().W; got > 200 {
		t.Errorf("at a narrower width the description is %d wide: the item reused the wider layout", got)
	}
}
