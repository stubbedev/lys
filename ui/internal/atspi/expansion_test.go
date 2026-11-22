package atspi

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/ui/widget"
)

func TestExpansionStates(t *testing.T) {
	if stateExpandable != 9 || stateExpanded != 10 {
		t.Fatalf("expandable, expanded = %d, %d, want ATSPI_STATE_EXPANDABLE (9), ATSPI_STATE_EXPANDED (10)", stateExpandable, stateExpanded)
	}
	b := &Bridge{}
	tr := &tree{nodes: map[int32]*anode{
		1: {id: 1, st: widget.A11yState{Role: widget.RoleButton, Enabled: true, Expansion: widget.ExpansionExpanded}},
		2: {id: 2, st: widget.A11yState{Role: widget.RoleButton, Enabled: true, Expansion: widget.ExpansionCollapsed}},
		3: {id: 3, st: widget.A11yState{Role: widget.RoleButton, Enabled: true}},
	}}
	cases := []struct {
		id                   int32
		expandable, expanded bool
	}{
		{1, true, true},
		{2, true, false},
		{3, false, false},
	}
	for _, c := range cases {
		s := b.statesOf(tr, c.id)
		if got := slices.Contains(s, stateExpandable); got != c.expandable {
			t.Errorf("node %d expandable = %v, want %v (%v)", c.id, got, c.expandable, s)
		}
		if got := slices.Contains(s, stateExpanded); got != c.expanded {
			t.Errorf("node %d expanded = %v, want %v (%v)", c.id, got, c.expanded, s)
		}
	}
}

func stateEvents(old, cur *tree) [][2]any {
	var out [][2]any
	for _, e := range diff(old, cur, ":1.1") {
		if e.name == eventObject("StateChanged") {
			out = append(out, [2]any{e.args[0], e.args[1]})
		}
	}
	return out
}

func expansionTree(x widget.Expansion) *tree {
	return &tree{nodes: map[int32]*anode{
		1: {id: 1, st: widget.A11yState{Role: widget.RoleButton, Enabled: true, Expansion: x}},
	}}
}

func TestExpansionStateEvents(t *testing.T) {
	cases := []struct {
		name     string
		old, cur widget.Expansion
		want     [][2]any
	}{
		{"opening", widget.ExpansionCollapsed, widget.ExpansionExpanded, [][2]any{{"expanded", int32(1)}}},
		{"closing", widget.ExpansionExpanded, widget.ExpansionCollapsed, [][2]any{{"expanded", int32(0)}}},
		{"becoming a trigger", widget.ExpansionNone, widget.ExpansionCollapsed, [][2]any{{"expandable", int32(1)}}},
		{"ceasing to be an open trigger", widget.ExpansionExpanded, widget.ExpansionNone, [][2]any{{"expandable", int32(0)}, {"expanded", int32(0)}}},
		{"unchanged", widget.ExpansionExpanded, widget.ExpansionExpanded, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := stateEvents(expansionTree(c.old), expansionTree(c.cur)); !slices.Equal(got, c.want) {
				t.Errorf("state events = %v, want %v", got, c.want)
			}
		})
	}
}
