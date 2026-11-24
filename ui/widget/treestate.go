package widget

import (
	"sync"

	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/ui/internal/style"
)

type treeState struct {
	shift       shiftState
	shiftBroken bool
	culled      int
	volatility  uint64
	stickiness  uint64

	boxShares   depthStack[int]
	boxReqs     depthStack[Sizes]
	boxExpands  depthStack[bool]
	boxFlows    depthStack[flow]
	boxArranged depthStack[int]
	cssItems    depthStack[cssItem]
	flexLines   depthStack[flexLine]
	gridTracks  depthStack[gridTrack]
	gridCells   depthStack[uint64]
	gridCursors depthStack[int]
	order       []int

	layers    []style.Layer
	root      *node
	work      *styleWork
	styleBusy bool
	styling   int
	pinnedGen uint64
}

type styleWork struct {
	scratch style.Scratch
	self    styleTargeter
	next    style.Values
	pseudo  [len(pseudoNames)]style.Values
}

var styleWorks = sync.Pool{New: func() any { return new(styleWork) }}

type shiftState struct {
	shifted, culling bool
	cull             render.Rect
}

func (t *treeState) pinStyleGen() {
	if t.styling == 0 {
		t.pinnedGen = styleGen.Load()
	}
	t.styling++
}

func (t *treeState) unpinStyleGen() { t.styling-- }

func (t *treeState) styleGenNow() uint64 {
	if t.styling > 0 {
		return t.pinnedGen
	}
	return styleGen.Load()
}

func (n *node) scratch() *treeState {
	if t := n.knownTree(); t != nil {
		return t
	}
	root := n
	for p := parentNodeOf(root); p != nil; p = parentNodeOf(p) {
		root = p
	}
	root.tree = &treeState{root: root}
	return n.knownTree()
}

func (t *treeState) styleWork() *styleWork {
	if t.work == nil {
		t.work, _ = styleWorks.Get().(*styleWork)
	}
	return t.work
}

func (t *treeState) endCompute() {
	t.styleBusy = false
	if t.root != nil && t.root.parent == nil && t.root.bounds.Empty() {
		t.releaseWork()
	}
}

func (t *treeState) releaseWork() {
	if t.work != nil {
		styleWorks.Put(t.work)
		t.work = nil
	}
}

func (n *node) knownTree() *treeState {
	if n.tree != nil {
		return n.tree
	}
	p := parentNodeOf(n)
	if p == nil {
		return nil
	}
	n.tree = p.knownTree()
	return n.tree
}

func (n *node) leaveTree() {
	old := n.tree
	if old == nil {
		return
	}
	n.tree = nil
	if old.root == n && !old.styleBusy {
		old.releaseWork()
	}
	if n.culled {
		n.culled = false
		old.culled--
	}
	if n.self == nil {
		return
	}
	withKidStack(func(s *kidStack) { leaveTreeBelow(s, n.self, old) })
}

func leaveTreeBelow(s *kidStack, w Widget, old *treeState) {
	s.each(func(kids []Widget) []Widget { return appendStyleKids(w, kids) }, func(k Widget) {
		kn := nodeOf(k)
		if kn == nil || kn.tree != old {
			return
		}
		kn.tree = nil
		if kn.culled {
			kn.culled = false
			old.culled--
		}
		leaveTreeBelow(s, k, old)
	})
}

var kidStacks = sync.Pool{New: func() any { return new(kidStack) }}

func withKidStack(walk func(s *kidStack)) {
	s, _ := kidStacks.Get().(*kidStack)
	defer kidStacks.Put(s)
	walk(s)
}
