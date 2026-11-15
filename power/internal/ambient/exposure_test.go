package ambient

import "testing"

func mustBracket(t *testing.T) Bracket {
	t.Helper()
	b, err := NewBracket(3, 2000, 30, 110, 210)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestBracketSettlesInsideTheWindowOrAtABound(t *testing.T) {
	b := mustBracket(t)
	for _, tc := range []struct {
		exposure int32
		luma     float64
		next     int32
		settled  bool
	}{
		{100, 110, 100, true},
		{100, 30, 100, true},
		{100, 210, 100, true},
		{100, 250, 44, false},
		{100, 10, 1100, false},
		{100, 0, 2000, false},
		{3, 255, 3, true},
		{2000, 2, 2000, true},
		{4, 255, 3, false},
		{1990, 20, 2000, false},
	} {
		next, settled := b.Next(tc.exposure, tc.luma)
		if next != tc.next || settled != tc.settled {
			t.Errorf("Next(%d, %v) = %d, %v, want %d, %v", tc.exposure, tc.luma, next, settled, tc.next, tc.settled)
		}
	}
}

func TestBracketConvergesOnALinearCamera(t *testing.T) {
	b := mustBracket(t)
	for _, scene := range []float64{0.02, 1, 40, 200} {
		exposure := int32(156)
		settled := false
		for range 6 {
			luma := min(scene*float64(exposure), 255)
			if exposure, settled = b.Next(exposure, luma); settled {
				break
			}
		}
		if !settled {
			t.Errorf("a scene of %v luma per exposure unit did not settle", scene)
		}
	}
}

func TestNewBracketRefusesAnInvalidWindow(t *testing.T) {
	for _, tc := range []struct {
		lo, hi            int32
		low, target, high float64
	}{
		{0, 100, 30, 110, 210},
		{100, 10, 30, 110, 210},
		{1, 100, 110, 30, 210},
		{1, 100, 30, 110, 255},
		{1, 100, 0, 110, 210},
	} {
		if _, err := NewBracket(tc.lo, tc.hi, tc.low, tc.target, tc.high); err == nil {
			t.Errorf("NewBracket(%d, %d, %v, %v, %v) accepted", tc.lo, tc.hi, tc.low, tc.target, tc.high)
		}
	}
}

func TestCameraLuxIsLumaOverExposure(t *testing.T) {
	if got := CameraLux(900, 110, 330); got != 300 {
		t.Errorf("CameraLux(900, 110, 330) = %v, want 300", got)
	}
	if CameraLux(900, 110, 33) <= CameraLux(900, 110, 330) {
		t.Error("the same luma at a shorter exposure is not a brighter room")
	}
}
