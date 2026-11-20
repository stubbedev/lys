package notifymodel

import "testing"

func TestUrgencyFromByte(t *testing.T) {
	for in, want := range map[byte]Urgency{0: UrgencyLow, 1: UrgencyNormal, 2: UrgencyCritical, 9: UrgencyNormal} {
		if got := UrgencyFromByte(in); got != want {
			t.Errorf("UrgencyFromByte(%d) = %v, want %v", in, got, want)
		}
	}
}
