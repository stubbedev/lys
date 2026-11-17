package widget

import "testing"

func TestMoveFocusAmongStepsWrapsAndPassesOverStopsFocusCannotLandOn(t *testing.T) {
	stops := make([]*Button, 5)
	for i := range stops {
		stops[i] = NewButton(nil, 0, 0)
	}
	stops[0].SetEnabled(false)
	stops[3].SetVisible(false)
	for _, c := range []struct {
		name   string
		target int
		move   FocusMove
		want   int
	}{
		{"next", 1, FocusMoveNext, 2},
		{"next over a hidden stop", 2, FocusMoveNext, 4},
		{"next wraps over a disabled stop", 4, FocusMoveNext, 1},
		{"prev", 2, FocusMovePrev, 1},
		{"prev wraps over a disabled stop", 1, FocusMovePrev, 4},
		{"prev over a hidden stop", 4, FocusMovePrev, 2},
		{"first passes a disabled stop", 4, FocusMoveFirst, 1},
		{"last", 1, FocusMoveLast, 4},
	} {
		if got := MoveFocusAmong(stops, stops[c.target], c.move); got != Widget(stops[c.want]) {
			t.Errorf("%s: from stop %d landed on %v, want stop %d", c.name, c.target, got, c.want)
		}
	}
}

func TestMoveFocusAmongLeavesATargetItCannotRove(t *testing.T) {
	stops := []*Button{NewButton(nil, 0, 0), NewButton(nil, 0, 0)}
	stops[1].SetEnabled(false)
	if got := MoveFocusAmong(stops, stops[1], FocusMoveNext); got != nil {
		t.Errorf("from a disabled stop: %v, want nil", got)
	}
	if got := MoveFocusAmong(stops, NewButton(nil, 0, 0), FocusMoveNext); got != nil {
		t.Errorf("from a widget outside the stops: %v, want nil", got)
	}
	if got := MoveFocusAmong(stops[:0], stops[0], FocusMovePrev); got != nil {
		t.Errorf("among no stops: %v, want nil", got)
	}
	for _, move := range []FocusMove{FocusMovePrev, FocusMoveNext, FocusMoveFirst, FocusMoveLast} {
		if got := MoveFocusAmong(stops, stops[0], move); got != Widget(stops[0]) {
			t.Errorf("move %d from the one stop focus can land on: %v, want it", move, got)
		}
	}
}
