package widget

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stubbedev/lys/lib/render"
)

const (
	laidRows  = 200
	tallRowH  = 60
	shiftedAt = 5
)

type laidTree struct {
	at        render.Rect
	root      *Box
	sc        *Scroll
	rows      []*Label
	sheet     *Stylesheet
	pads      [2]int
	rowH      int
	shiftedAt int
}

func padSheet(pad int) string {
	return fmt.Sprintf(`.pad { padding: %dpx; container-type: inline-size; } .tall { min-height: %dpx; } @container (min-width: 1px) { .tall { color: #123; } }`, pad, tallRowH)
}

func newLaidTree(t *testing.T, at render.Rect, pads [2]int) *laidTree {
	t.Helper()
	face := entryFace(t)
	col := NewBox(Column, 0, 0)
	rows := make([]*Label, 0, laidRows)
	for i := range laidRows {
		l := NewLabel(face, 14, fmt.Sprintf("row %d", i), 0)
		rows = append(rows, l)
		col.Append(l, false)
	}
	sc := NewScroll(col)
	root := NewBox(Column, 0, 0).Append(sc, true)
	root.AddClass("pad")
	sheet := NewStylesheet(padSheet(pads[0]), StylePriorityApplication)
	root.AttachStylesheet(sheet)
	tr := &laidTree{at: at, root: root, sc: sc, rows: rows, sheet: sheet, pads: pads}
	var damage DamageCollector
	tr.pass(&damage)
	tr.rowH = rows[0].Bounds().H
	tr.shiftedAt = rows[shiftedAt].Bounds().Y - sc.Bounds().Y
	if tr.rowH <= 0 || tr.rowH >= tallRowH || tr.shiftedAt != shiftedAt*tr.rowH {
		t.Fatalf("a laid tree's rows are %d tall with row %d at %d, want rows under %d with row %d at %d", tr.rowH, shiftedAt, tr.shiftedAt, tallRowH, shiftedAt, shiftedAt*tr.rowH)
	}
	return tr
}

func (tr *laidTree) pass(damage *DamageCollector) {
	SizeWithin(tr.root, Size{W: tr.at.W, H: tr.at.H})
	tr.root.Arrange(tr.at)
	damage.Collect(tr.root)
}

func (tr *laidTree) round(i int, damage *DamageCollector) error {
	pad := tr.pads[i%2]
	tr.sheet.Load(padSheet(pad))
	last := tr.rows[laidRows-1]
	tall := i%3 == 0
	if tall {
		last.AddClass("tall")
	} else {
		last.RemoveClass("tall")
	}
	offset := (i%7 + 1) * tr.rowH
	tr.sc.SetOffset(0, offset)
	tr.pass(damage)
	if got, want := tr.sc.Bounds().X, tr.at.X+pad; got != want {
		return fmt.Errorf("round %d: the scroll sits at x %d under its tree's %dpx padding, want %d", i, got, pad, want)
	}
	if got, want := tr.rows[shiftedAt].Bounds().Y, tr.sc.Bounds().Y+tr.shiftedAt-offset; got != want {
		return fmt.Errorf("round %d: row %d sits at y %d scrolled by %d, want %d", i, shiftedAt, got, offset, want)
	}
	want := tr.rowH
	if tall {
		want = tallRowH
	}
	if got := last.Bounds().H; got != want {
		return fmt.Errorf("round %d: the last row is %d tall with .tall %v, want %d", i, got, tall, want)
	}
	walked := 0
	walkTree(tr.root, func(Widget, int) { walked++ })
	if walked != laidRows+3 {
		return fmt.Errorf("round %d: a walk visited %d widgets, want %d", i, walked, laidRows+3)
	}
	return nil
}

func TestTwoTreesArrangeAndRestyleAtOnce(t *testing.T) {
	trees := [2]*laidTree{
		newLaidTree(t, render.Rect{W: 300, H: 240}, [2]int{3, 7}),
		newLaidTree(t, render.Rect{X: 1000, W: 300, H: 240}, [2]int{11, 13}),
	}
	const rounds = 120
	atOnce(t, len(trees), func(w int) error {
		var damage DamageCollector
		var errs []error
		for i := range rounds {
			errs = append(errs, trees[w].round(i, &damage))
		}
		return errors.Join(errs...)
	})
}

func TestATreeKeepsItsScratchUntilItMoves(t *testing.T) {
	a := NewBox(Column, 0, 0)
	b := NewBox(Column, 0, 0)
	moved := NewBox(Column, 0, 0)
	leaf := NewSpacer(4, 4)
	moved.Append(leaf, false)
	a.Append(moved, false)
	arrangeTree(t, a, 100, 100)
	arrangeTree(t, b, 100, 100)
	ta, tb := nodeOf(a).scratch(), nodeOf(b).scratch()
	if ta == tb {
		t.Fatal("two separate trees share one scratch")
	}
	if got := nodeOf(leaf).scratch(); got != ta {
		t.Fatal("a leaf does not use its tree's scratch")
	}
	a.Remove(moved)
	b.Append(moved, false)
	arrangeTree(t, b, 100, 100)
	if got := nodeOf(leaf).scratch(); got != tb {
		t.Error("a leaf moved into another tree kept the first tree's scratch")
	}
	if got := nodeOf(a).scratch(); got != ta {
		t.Error("the tree a subtree left lost its own scratch")
	}
}

type bumpingMeasure struct {
	Base
	bump bool
}

func (b *bumpingMeasure) Measure(axis Axis, forSize int) Sizes {
	if s, ok := b.measureHit(axis, forSize); ok {
		return s
	}
	if b.bump {
		styleGen.Add(1)
	}
	return b.measureStore(axis, forSize, FixedSizes(10))
}

func TestAMeasureAStyleBumpOverlapsStaysStale(t *testing.T) {
	for _, bump := range []bool{false, true} {
		w := &bumpingMeasure{bump: bump}
		w.Measure(Row, Unknown)
		if _, hit := w.measureHit(Row, Unknown); hit == bump {
			t.Errorf("a measure with a style bump during it %v: the next ask hits the cache %v, want %v", bump, hit, !bump)
		}
	}
}

func TestARestyleKeepsTheGenerationItStartedUnder(t *testing.T) {
	root := NewBox(Column, 0, 0)
	leaf := NewSpacer(4, 4)
	root.Append(leaf, false)
	arrangeTree(t, root, 40, 40)
	n := &leaf.node
	before := styleGen.Load()
	t1 := n.scratch()
	t1.pinStyleGen()
	styleGen.Add(1)
	n.markStyleDirty()
	n.style(leaf)
	t1.unpinStyleGen()
	if n.csGen != before {
		t.Errorf("a restyle under a pinned generation stamped %d, want the pinned %d", n.csGen, before)
	}
	n.style(leaf)
	if n.csGen != styleGen.Load() {
		t.Errorf("the next restyle stamped %d, want the current %d", n.csGen, styleGen.Load())
	}
}
