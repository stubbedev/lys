package settings

import (
	"testing"

	"github.com/stubbedev/lys/ui/widget"
)

// A list's first row offered to move up and its last to move down,
// arrows that did nothing, while the other reorderable lists dim them.
// Each row's arrows are enabled only toward a row to swap with, and the
// former last row's down arrow comes on once a row is added after it.
func TestAListsArrowsMoveOnlyWhereThereIsARow(t *testing.T) {
	stored := any([]any{"a", "b", "c"})
	s := slot{get: func() any { return stored }, set: func(v any) error { stored = v; return nil }, unset: func() {}}
	c := newListEditor(testKit(t, ""), s, stringListSpec())
	want := [][2]bool{{false, true}, {true, true}, {true, false}}
	for i, a := range c.arrows {
		if got := [2]bool{widget.IsEnabled(a[0]), widget.IsEnabled(a[1])}; got != want[i] {
			t.Errorf("row %d's arrows are enabled %v, want %v", i, got, want[i])
		}
	}
	c.add.OnClick()
	if len(c.arrows) != 4 || !widget.IsEnabled(c.arrows[2][1]) || widget.IsEnabled(c.arrows[3][1]) {
		t.Errorf("after an add the former last row moves down %v, the new last %v", widget.IsEnabled(c.arrows[2][1]), widget.IsEnabled(c.arrows[3][1]))
	}
}
