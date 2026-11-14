package widget

import (
	"testing"
)

type visibilityCountingBox struct {
	*Box
	checks *int
}

func (b *visibilityCountingBox) Visible() bool {
	*b.checks++
	return b.Box.Visible()
}

func TestATabStepTestsEachContainersVisibilityOnceWhateverItsDepth(t *testing.T) {
	face := testFace(t)
	const depth = 40
	checks := 0
	root := NewBox(Column, 0, 0)
	parent := root
	for range depth {
		level := &visibilityCountingBox{Box: NewBox(Column, 0, 0), checks: &checks}
		level.Append(NewButton(NewLabel(face, 14, "x", 0), 4, 2), false)
		parent.Append(level, false)
		parent = level.Box
	}
	hidden := &visibilityCountingBox{Box: NewBox(Column, 0, 0), checks: &checks}
	for range depth {
		hidden.Append(NewButton(NewLabel(face, 14, "hidden", 0), 4, 2), false)
	}
	hidden.SetVisible(false)
	root.Append(hidden, false)
	arrangeTree(t, root, 400, depth*40)
	r := &Router{Root: root}
	r.FocusNext()
	for step := range depth {
		checks = 0
		r.FocusNext()
		if checks > 2*depth+2 {
			t.Fatalf("Tab step %d tested container visibility %d times on a %d-deep tree, want at most %d: once per container, plus the focus's own ancestors", step, checks, depth, 2*depth+2)
		}
		if _, ok := r.Focused().(*Button); !ok {
			t.Fatalf("Tab step %d focused %T, want a button", step, r.Focused())
		}
	}
	for _, b := range hidden.Children() {
		if r.Focused() == b {
			t.Fatal("Tab landed in a hidden container")
		}
	}
}
