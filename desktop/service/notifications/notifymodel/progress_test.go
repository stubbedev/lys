package notifymodel

import "testing"

func TestProgressClampsToAPercentAndKnowsWhenItIsNone(t *testing.T) {
	for in, want := range map[int64]uint8{-5: 0, 0: 0, 42: 42, 100: 100, 250: 100} {
		if got, set := NewProgress(in).Percent(); !set || got != want {
			t.Errorf("NewProgress(%d) = %d set %v, want %d", in, got, set, want)
		}
	}
	if _, set := (Progress{}).Percent(); set {
		t.Error("no progress reads as a value")
	}
}
