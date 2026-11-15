package widget

import (
	"fmt"
	"slices"
	"sync"
	"testing"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/testbuild"
)

type walkedTree struct {
	root, glass   *Box
	label         *Label
	labelOwes     []render.Rect
	glassOwes     []render.Rect
	labelInScroll render.Rect
}

func newWalkedTree(t *testing.T, at render.Rect) walkedTree {
	t.Helper()
	face := entryFace(t)
	col := NewBox(Column, 0, 0)
	for i := range 40 {
		col.Append(NewLabel(face, 14, fmt.Sprintf("row %d", i), 0), false)
	}
	label := col.Children()[0].(*Label)
	sc := NewScroll(col)
	glass := NewBox(Column, 0, 0)
	glass.AddClass("glass")
	glass.SetSizeRequest(at.W, 40)
	root := NewBox(Column, 0, 0).Append(sc, true).Append(glass, false)
	SizeWithin(root, Size{W: at.W, H: at.H})
	root.Arrange(at)
	tr := walkedTree{root: root, glass: glass, label: label, labelInScroll: label.Bounds().Intersect(sc.Bounds())}
	CollectDamage(root)
	label.Invalidate()
	tr.labelOwes, _ = CollectDamage(root)
	glass.Invalidate()
	tr.glassOwes, _ = CollectDamage(root)
	return tr
}

func TestDamageWalksOfTwoTreesRunAtOnce(t *testing.T) {
	loadCSS(t, `.glass { backdrop-filter: blur(8px); border-radius: 0; }`)
	trees := [2]walkedTree{
		newWalkedTree(t, render.Rect{W: 200, H: 300}),
		newWalkedTree(t, render.Rect{X: 1000, W: 200, H: 300}),
	}
	for i, tr := range trees {
		if len(tr.labelOwes) != 1 || tr.labelOwes[0].Intersect(tr.labelInScroll) != tr.labelInScroll {
			t.Fatalf("tree %d: an invalidated label owes %v, want one rect over %+v", i, tr.labelOwes, tr.labelInScroll)
		}
		if got, glass := render.UnionAll(tr.glassOwes), tr.glass.Bounds(); got.Intersect(glass) != glass || got == glass {
			t.Fatalf("tree %d: an invalidated backdrop box owes %v, want its reach around %+v", i, tr.glassOwes, glass)
		}
	}
	const rounds = 300
	errs := make(chan error, 3*2*rounds)
	var wg sync.WaitGroup
	for i, tr := range trees {
		wg.Go(func() {
			var damage DamageCollector
			var buf []render.Rect
			var any bool
			for range rounds {
				tr.label.Invalidate()
				if buf, any = damage.Collect(tr.root); !any || !slices.Equal(buf, tr.labelOwes) {
					errs <- fmt.Errorf("tree %d: an invalidated label drained %v, want %v", i, buf, tr.labelOwes)
				}
				tr.glass.Invalidate()
				if buf, any = damage.Collect(tr.root); !any || !slices.Equal(buf, tr.glassOwes) {
					errs <- fmt.Errorf("tree %d: an invalidated backdrop box drained %v, want %v", i, buf, tr.glassOwes)
				}
				if buf, any = damage.Collect(tr.root); any {
					errs <- fmt.Errorf("tree %d: a tree nothing marked drained %v", i, buf)
				}
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestAKeptDamageCollectorDrainsWithoutAllocating(t *testing.T) {
	testbuild.SkipAllocBudget(t)
	loadCSS(t, `.glass { backdrop-filter: blur(8px); border-radius: 0; }`)
	tr := newWalkedTree(t, render.Rect{W: 200, H: 300})
	var damage DamageCollector
	drain := func() {
		tr.label.Invalidate()
		tr.glass.Invalidate()
		damage.Collect(tr.root)
	}
	drain()
	if rects, _ := damage.Collect(tr.root); len(rects) != 0 {
		t.Fatalf("a drained tree owes %v", rects)
	}
	const budget = 0
	if allocs := testing.AllocsPerRun(100, drain); allocs > budget {
		t.Errorf("a kept collector's drain allocates %v, budget %d", allocs, budget)
	}
}

func BenchmarkDamageWalk(b *testing.B) {
	b.Run("invalidate", func(b *testing.B) {
		const w, h = 1605, 1391
		root := settingsTree(b)
		cv := render.New(make([]byte, render.Stride(w)*h), render.Stride(w), w, h)
		settingsFrame(root, cv, w, h)
		leaf := nodeOf(lastLeaf(root))
		var damage DamageCollector
		b.ReportAllocs()
		for b.Loop() {
			leaf.Invalidate()
			damage.Collect(root)
		}
	})
	b.Run("scroll", func(b *testing.B) {
		sc, _ := longScroll(b, 2000)
		r := render.Rect{W: 800, H: 600}
		SizeWithin(sc, Size{W: 800, H: 600})
		sc.Arrange(r)
		var damage DamageCollector
		damage.CollectFrame(sc)
		y := 0
		b.ReportAllocs()
		for b.Loop() {
			y = (y + 7) % 500
			sc.SetOffset(0, y)
			sc.Arrange(r)
			damage.CollectFrame(sc)
		}
	})
}

func lastLeaf(w Widget) Widget {
	for {
		kids := appendKids(w, nil)
		if len(kids) == 0 {
			return w
		}
		w = kids[len(kids)-1]
	}
}
