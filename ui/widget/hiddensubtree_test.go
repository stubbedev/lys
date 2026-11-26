package widget

import (
	"slices"
	"strconv"
	"testing"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/testbuild"
)

const hiddenSubtreeCSS = `
box { --leaf-size: 12px; --leaf-family: "Inter", sans-serif; }
label:first-child { font-size: var(--leaf-size); font-family: var(--leaf-family); }
box > label { font-size: var(--leaf-size); }
.leaf + .leaf { min-width: 3px; }
`

const (
	hiddenFrameW = 320
	hiddenFrameH = 480
)

type hiddenFrameTree struct {
	root    *Box
	counter *Label
	toggled *Label
	subtree *Box
	nodes   []Widget
	damage  DamageCollector
}

func newHiddenFrameTree(tb testing.TB, withSubtree bool) *hiddenFrameTree {
	tb.Helper()
	face := entryFace(tb)
	t := &hiddenFrameTree{root: NewBox(Column, 4, 4)}
	t.counter = NewLabel(face, 14, "0", 0)
	t.toggled = NewLabel(face, 14, "toggled", 0)
	t.root.Append(t.counter, false)
	t.root.Append(t.toggled, false)
	t.subtree = NewBox(Column, 2, 0)
	if withSubtree {
		for i := range 16 {
			row := NewBox(Row, 2, 0)
			for j := range 8 {
				l := NewLabel(face, 12, strconv.Itoa(i*8+j), 0)
				l.AddClass("leaf")
				row.Append(l, false)
				t.nodes = append(t.nodes, l)
			}
			t.subtree.Append(row, false)
			t.nodes = append(t.nodes, row)
		}
	}
	t.subtree.SetVisible(false)
	t.root.Append(t.subtree, true)
	t.nodes = append(t.nodes, t.subtree)
	return t
}

func (t *hiddenFrameTree) frame(cv *render.Canvas, i int) {
	t.counter.SetText(strconv.Itoa(i % 10))
	t.toggled.SetVisible(i%2 == 0)
	t.paint(cv)
}

func (t *hiddenFrameTree) paint(cv *render.Canvas) []render.Rect {
	t.root.Arrange(render.Rect{W: hiddenFrameW, H: hiddenFrameH})
	rects, _ := t.damage.Collect(t.root)
	region := render.UnionAll(rects)
	prev := cv.PushClip(region)
	cv.Clear(region, Current().Bg)
	t.root.Paint(cv)
	cv.PopClip(prev)
	return rects
}

func hiddenFrameAllocs(t *testing.T, withSubtree bool) float64 {
	t.Helper()
	tree := newHiddenFrameTree(t, withSubtree)
	arrangeTree(t, tree.root, hiddenFrameW, hiddenFrameH)
	_, cv := hiddenFrameCanvas()
	i := 0
	for range 20 {
		tree.frame(cv, i)
		i++
	}
	return testing.AllocsPerRun(50, func() {
		tree.frame(cv, i)
		i++
	})
}

func TestAHiddenSubtreeCostsAFrameNoAllocations(t *testing.T) {
	testbuild.SkipAllocBudget(t)
	LoadStylesheet(hiddenSubtreeCSS)
	t.Cleanup(func() { LoadStylesheet("") })
	without := hiddenFrameAllocs(t, false)
	with := hiddenFrameAllocs(t, true)
	if with != without {
		t.Errorf("a frame allocates %v times beside a hidden box holding %d widgets, %v beside an empty one", with, len(newHiddenFrameTree(t, true).nodes)-1, without)
	}
}

func TestAHiddenSubtreeTakesNoPartInAFrame(t *testing.T) {
	LoadStylesheet(hiddenSubtreeCSS)
	t.Cleanup(func() { LoadStylesheet("") })
	tree := newHiddenFrameTree(t, true)
	arrangeTree(t, tree.root, hiddenFrameW, hiddenFrameH)
	_, cv := hiddenFrameCanvas()
	tree.frame(cv, 0)
	type counts struct{ arranges, restyles, measures int }
	before := make([]counts, len(tree.nodes))
	for i, w := range tree.nodes {
		n := nodeOf(w)
		before[i] = counts{n.arrangeCount(), n.restyleCount(), n.measureMissCount()}
	}
	for i := 1; i <= 6; i++ {
		tree.frame(cv, i)
	}
	for i, w := range tree.nodes {
		n := nodeOf(w)
		if got := (counts{n.arrangeCount(), n.restyleCount(), n.measureMissCount()}); got != before[i] {
			t.Fatalf("hidden %T %v: arranges, restyles, measures went from %+v to %+v over frames that hid and showed its sibling", w, n.classes, before[i], got)
		}
		if !n.bounds.Empty() {
			t.Fatalf("hidden %T %v has bounds %v", w, n.classes, n.bounds)
		}
	}
}

func TestShowingAHiddenSubtreeLaysItOutAndHidingItAgainRepaintsWhereItWas(t *testing.T) {
	LoadStylesheet(hiddenSubtreeCSS)
	t.Cleanup(func() { LoadStylesheet("") })
	tree := newHiddenFrameTree(t, true)
	arrangeTree(t, tree.root, hiddenFrameW, hiddenFrameH)
	data, cv := hiddenFrameCanvas()
	tree.frame(cv, 0)
	tree.frame(cv, 1)

	tree.subtree.SetVisible(true)
	shownDamage := slices.Clone(tree.paint(cv))
	area := tree.subtree.Bounds()
	if area.Empty() || area.Intersect(render.Rect{W: hiddenFrameW, H: hiddenFrameH}) != area {
		t.Fatalf("the shown subtree lies at %v, not inside the frame", area)
	}
	if !render.UnionAll(shownDamage).Contains(area.X, area.Y) {
		t.Errorf("showing the subtree at %v damaged only %v", area, shownDamage)
	}
	for _, w := range tree.nodes {
		if nodeOf(w).bounds.Empty() {
			t.Fatalf("shown %T %v has no bounds", w, nodeOf(w).classes)
		}
	}
	if paintedIn(data, area) == 0 {
		t.Errorf("the shown subtree painted nothing in %v", area)
	}

	tree.counter.SetText("changed while shown")
	tree.subtree.SetVisible(false)
	hiddenDamage := tree.paint(cv)
	if !render.UnionAll(hiddenDamage).Contains(area.X, area.Y) {
		t.Errorf("hiding the subtree at %v damaged only %v", area, hiddenDamage)
	}
	if n := paintedIn(data, area); n != 0 {
		t.Errorf("%d pixels of the hidden subtree stayed painted in %v", n, area)
	}
	for _, w := range tree.nodes {
		if !nodeOf(w).bounds.Empty() {
			t.Fatalf("hidden again, %T %v keeps bounds %v", w, nodeOf(w).classes, nodeOf(w).bounds)
		}
	}
}

func hiddenFrameCanvas() ([]byte, *render.Canvas) {
	data := make([]byte, render.Stride(hiddenFrameW)*hiddenFrameH)
	return data, render.New(data, render.Stride(hiddenFrameW), hiddenFrameW, hiddenFrameH)
}

func paintedIn(data []byte, r render.Rect) int {
	bg := Current().Bg
	n := 0
	for y := r.Y; y < r.Y+r.H; y++ {
		for x := r.X; x < r.X+r.W; x++ {
			if render.ColorFromBytes(data[y*render.Stride(hiddenFrameW)+x*4:]) != bg {
				n++
			}
		}
	}
	return n
}
