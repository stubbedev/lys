package widget

import "testing"

func newRadioRowBetweenStops(t *testing.T) (root *Box, first, before, after *focusTarget, g *ChoiceChips) {
	t.Helper()
	g, _ = newChoiceChips(t, ChooseOne, "a", "b", "c")
	first, before, after = newFocusTarget(), newFocusTarget(), newFocusTarget()
	root = NewBox(Column, 0, 0)
	root.Append(first, false).Append(before, false).Append(g, false).Append(after, false)
	arrangeTree(t, root, 400, 200)
	return root, first, before, after, g
}

func focusedAfter(t *testing.T, root Widget, from Widget, step func(*Router)) Widget {
	t.Helper()
	r := &Router{Root: root}
	if err := r.SetFocus(from); err != nil {
		t.Fatal(err)
	}
	step(r)
	return r.Focused()
}

func TestTabFromAFocusedChipTabPassesByLeavesTheRow(t *testing.T) {
	root, _, before, after, g := newRadioRowBetweenStops(t)
	leaves := func(state string, chips ...int) {
		t.Helper()
		for _, i := range chips {
			if !skipsTab(g.Chip(i)) {
				t.Fatalf("%s chip %d is a Tab stop, want one Tab passes by", state, i)
			}
			if got := focusedAfter(t, root, g.Chip(i), (*Router).FocusNext); got != Widget(after) {
				t.Errorf("%s Tab from chip %d went to %p, want the stop after the row %p", state, i, got, after)
			}
			if got := focusedAfter(t, root, g.Chip(i), (*Router).FocusPrev); got != Widget(before) {
				t.Errorf("%s Shift+Tab from chip %d went to %p, want the stop before the row %p", state, i, got, before)
			}
		}
	}
	leaves("with none checked", 1, 2)
	g.SetChecked(2, true)
	leaves("with chip 2 checked", 0, 1)
}

func TestTabFromAFocusedChipTabPassesByMovesAsFromTheRowsStop(t *testing.T) {
	root, _, _, _, g := newRadioRowBetweenStops(t)
	trailing := newFocusTarget()
	g.SetTrailing(trailing)
	g.SetChecked(1, true)
	arrangeTree(t, root, 400, 200)
	from := focusedAfter(t, root, g.Chip(1), (*Router).FocusNext)
	if from != Widget(trailing) {
		t.Fatalf("Tab from the row's stop went to %p, want the trailing widget %p", from, trailing)
	}
	for _, i := range []int{0, 2} {
		if got := focusedAfter(t, root, g.Chip(i), (*Router).FocusNext); got != from {
			t.Errorf("Tab from chip %d went to %p, want %p as from the row's stop", i, got, from)
		}
	}
}

func TestTabFromAStopMovesToItsNeighbors(t *testing.T) {
	root, first, before, after, g := newRadioRowBetweenStops(t)
	cases := []struct {
		name       string
		from       Widget
		next, prev Widget
	}{
		{"the row's stop", g.Chip(0), after, before},
		{"the stop before the row", before, g.Chip(0), first},
		{"the first stop", first, before, after},
		{"the last stop", after, first, g.Chip(0)},
	}
	for _, c := range cases {
		if got := focusedAfter(t, root, c.from, (*Router).FocusNext); got != c.next {
			t.Errorf("Tab from %s went to %p, want %p", c.name, got, c.next)
		}
		if got := focusedAfter(t, root, c.from, (*Router).FocusPrev); got != c.prev {
			t.Errorf("Shift+Tab from %s went to %p, want %p", c.name, got, c.prev)
		}
	}
}

func TestTabWithNoFocusTakesTheFirstStopAndShiftTabTheLast(t *testing.T) {
	root, first, _, after, _ := newRadioRowBetweenStops(t)
	r := &Router{Root: root}
	r.FocusNext()
	if r.Focused() != Widget(first) {
		t.Errorf("Tab with no focus went to %p, want the first stop %p", r.Focused(), first)
	}
	r = &Router{Root: root}
	r.FocusPrev()
	if r.Focused() != Widget(after) {
		t.Errorf("Shift+Tab with no focus went to %p, want the last stop %p", r.Focused(), after)
	}
}
