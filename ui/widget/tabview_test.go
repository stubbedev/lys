package widget

import (
	"runtime"
	"slices"
	"strconv"
	"testing"

	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/ui/anim"
	"github.com/stubbedev/lys/ui/internal/waittest"
)

func titles(v *TabView) []string {
	var out []string
	for _, p := range v.Pages() {
		out = append(out, p.Title())
	}
	return out
}

func TestTabViewPagesSelectionPinningAndClose(t *testing.T) {
	v := NewTabView()
	a := v.Append(NewSpacer(10, 10), "a")
	b := v.Append(NewSpacer(10, 10), "b")
	c := v.Append(NewSpacer(10, 10), "c")
	if v.Selected() != a {
		t.Fatal("the first page is not selected")
	}
	v.SetPinned(c, true)
	if !slices.Equal(titles(v), []string{"c", "a", "b"}) || v.NPinned() != 1 {
		t.Errorf("after pinning c: %v pinned %d", titles(v), v.NPinned())
	}
	v.Reorder(b, 0)
	if !slices.Equal(titles(v), []string{"c", "b", "a"}) {
		t.Errorf("an unpinned page moved into the pinned run: %v", titles(v))
	}
	v.SelectPrevious()
	if v.Selected() != b {
		t.Errorf("previous from a selected %q", v.Selected().Title())
	}
	b.SetNeedsAttention(true)
	v.SelectNext()
	v.Select(b)
	if b.NeedsAttention() {
		t.Error("selecting a page did not clear its attention mark")
	}
	v.OnClosePage = func(p *TabPage) bool { return p != a }
	if v.ClosePage(a) || v.NPages() != 3 {
		t.Error("a vetoed close closed")
	}
	if !v.ClosePage(b) || v.Selected() != a || b.View() != nil {
		t.Errorf("closing the selected b: selected %v", v.Selected().Title())
	}
}

func TestTabTransferMovesThePageBetweenViews(t *testing.T) {
	src, dst := NewTabView(), NewTabView()
	child := NewSpacer(10, 10)
	p := src.Append(child, "moved")
	src.Append(NewSpacer(10, 10), "stays")
	dst.Append(NewSpacer(10, 10), "there")
	var detached, attached int
	src.OnPageDetached = func(*TabPage) { detached++ }
	dst.OnPageAttached = func(*TabPage) { attached++ }
	dst.TransferPage(p, 0)
	if p.View() != dst || dst.Selected() != p || p.Child() != child || detached != 1 || attached != 1 {
		t.Errorf("transfer: view ok=%v selected ok=%v detached=%d attached=%d", p.View() == dst, dst.Selected() == p, detached, attached)
	}
	if !slices.Equal(titles(src), []string{"stays"}) || !slices.Equal(titles(dst), []string{"moved", "there"}) {
		t.Errorf("src %v dst %v", titles(src), titles(dst))
	}
}

func TestTabBarFollowsItsViewAndDropsMovePages(t *testing.T) {
	face := testFace(t)
	v := NewTabView()
	a := v.Append(NewSpacer(10, 10), "a")
	b := v.Append(NewSpacer(10, 10), "b")
	bar := NewTabBar(face, 12, v)
	SizeWithin(bar, Size{W: 400, H: 40})
	bar.Arrange(render.Rect{W: 400, H: 40})
	tabs := bar.Tabs()
	if len(tabs) != 2 || !tabs[0].(*tabWidget).HasState(StateSelected) {
		t.Fatalf("bar shows %d tabs, first selected=%v", len(tabs), len(tabs) > 0 && tabs[0].(*tabWidget).HasState(StateSelected))
	}
	tabs[1].(*tabWidget).OnClick()
	if v.Selected() != b {
		t.Error("clicking a tab did not select it")
	}
	drag := tabs[1].(*tabWidget).DragContent()
	data, err := drag.Bytes(TabMime)
	if err != nil || string(data) != strconv.FormatUint(b.id, 10) {
		t.Fatalf("drag payload %q, %v", data, err)
	}
	if bar.DragEnter([]string{"text/plain", TabMime}, Point{}) != TabMime {
		t.Error("the bar rejected a tab drag")
	}
	bar.Drop(TabMime, data, Point{X: 0, Y: 5})
	if !slices.Equal(titles(v), []string{"b", "a"}) {
		t.Errorf("drop at the left edge reordered to %v", titles(v))
	}
	v.SetPinned(a, true)
	if got := len(bar.Tabs()); got != 2 || !bar.Tabs()[0].(*tabWidget).page.Pinned() {
		t.Error("the bar did not rebuild for a pinned page first")
	}
}

func TestTabOverviewSnapshotsPagesAndSelects(t *testing.T) {
	face := testFace(t)
	v := NewTabView()
	v.Append(NewSpacer(50, 50), "one")
	two := v.Append(NewSpacer(50, 50), "two")
	o := NewTabOverview(face, 12, v, v)
	SizeWithin(o, Size{W: 640, H: 480})
	o.Arrange(render.Rect{W: 640, H: 480})
	o.SetOpen(true)
	if !o.Open() || o.grid.Len() != 2 {
		t.Fatalf("open=%v thumbnails=%d", o.Open(), o.grid.Len())
	}
	o.grid.ChildAt(1).Child().(*Button).OnClick()
	if v.Selected() != two || o.Open() {
		t.Errorf("clicking a thumbnail: selected two=%v, open=%v", v.Selected() == two, o.Open())
	}
}

func tabPageListed(id uint64) bool {
	tabPages.mu.Lock()
	defer tabPages.mu.Unlock()
	_, ok := tabPages.pages[id]
	return ok
}

func TestADroppedTabViewsPagesLeaveTheRegistry(t *testing.T) {
	dropped := func() []uint64 {
		v := NewTabView()
		return []uint64{v.Append(NewSpacer(1, 1), "a").id, v.Append(NewSpacer(1, 1), "b").id}
	}()
	kept := NewTabView()
	held := kept.Append(NewSpacer(1, 1), "held")
	waittest.Until(t, func() bool {
		runtime.GC()
		return !slices.ContainsFunc(dropped, tabPageListed)
	}, "a dropped view's pages to leave the registry")
	if tabPageByID(held.id) != held {
		t.Error("a page its view still holds no longer resolves by its id")
	}
	runtime.KeepAlive(kept)
}

func TestATabDropsOnlyIntoAViewOfItsOwnApplication(t *testing.T) {
	face := testFace(t)
	var mine, theirs anim.Clock
	src, sibling, foreign := NewTabView(), NewTabView(), NewTabView()
	t.Cleanup(Host(src, &mine))
	t.Cleanup(Host(sibling, &mine))
	t.Cleanup(Host(foreign, &theirs))
	src.Append(NewSpacer(10, 10), "stays")
	moving := src.Append(NewSpacer(10, 10), "moving")
	payload := []byte(strconv.FormatUint(moving.id, 10))

	foreignBar := NewTabBar(face, 12, foreign)
	foreignBar.Drop(TabMime, payload, Point{})
	if moving.View() != src || foreign.NPages() != 0 {
		t.Errorf("a tab dropped into another application's view moved there: view is src %v, foreign has %d pages", moving.View() == src, foreign.NPages())
	}

	siblingBar := NewTabBar(face, 12, sibling)
	siblingBar.Drop(TabMime, payload, Point{})
	if moving.View() != sibling || sibling.Selected() != moving {
		t.Errorf("a tab dropped into a view of its own application did not move: view is sibling %v", moving.View() == sibling)
	}
}
