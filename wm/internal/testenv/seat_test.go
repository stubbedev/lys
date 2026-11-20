package testenv

import "testing"

func ranOwningTheSeat(t *testing.T, marker string) (ran, skipped bool) {
	t.Helper()
	t.Setenv(SeatEnv, marker)
	t.Run("seat", func(t *testing.T) {
		defer func() { skipped = t.Skipped() }()
		OwnSeat(t, "own the seat")
		ran = true
	})
	return ran, skipped
}

func TestATestOwnsTheSeatOnlyWhereTheMarkerSaysSo(t *testing.T) {
	for _, marker := range []string{"", "0", "yes"} {
		if ran, skipped := ranOwningTheSeat(t, marker); ran || !skipped {
			t.Errorf("with %s=%q the test went on (%v) or did not skip (%v)", SeatEnv, marker, ran, !skipped)
		}
	}
	if ran, skipped := ranOwningTheSeat(t, "1"); !ran || skipped {
		t.Errorf("with %s=1 the test stopped (%v) or skipped (%v)", SeatEnv, !ran, skipped)
	}
}
