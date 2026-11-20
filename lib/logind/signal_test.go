package logind

import (
	"slices"
	"testing"
)

func TestALidChangeSentBeforeTheFirstReadIsNotHandedOn(t *testing.T) {
	var got []bool
	record := func(closed bool) { got = append(got, closed) }
	var f lidFollow
	f.changed(lidChange{closed: true, seq: 4}, record)
	f.changed(lidChange{closed: false, seq: 6}, record)
	if len(got) != 0 {
		t.Fatalf("changes reached the follower before the first read was answered: %v", got)
	}
	f.answer(5, record)
	if !slices.Equal(got, []bool{false}) {
		t.Fatalf("after a read answered at 5 the follower saw %v, want only the change sent at 6", got)
	}
	f.changed(lidChange{closed: true, seq: 3}, record)
	f.changed(lidChange{closed: true, seq: 7}, record)
	if !slices.Equal(got, []bool{false, true}) {
		t.Errorf("after the read the follower saw %v, want the change sent at 7 and not the one at 3", got)
	}
}
