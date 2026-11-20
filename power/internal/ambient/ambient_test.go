package ambient

import (
	"math"
	"testing"
	"time"

	"github.com/stubbedev/lys/power/autobrightness"
)

func mustCurve(t *testing.T) Curve {
	t.Helper()
	c, err := NewCurve(1, 10000, 5, 100)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func near(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestCurveMapsLogLuxBetweenFloorAndCeiling(t *testing.T) {
	c := mustCurve(t)
	for _, tc := range []struct {
		lux  Lux
		want float64
	}{
		{0, 5},
		{0.5, 5},
		{1, 5},
		{10, 28.75},
		{100, 52.5},
		{1000, 76.25},
		{10000, 100},
		{1e6, 100},
	} {
		if got := c.Percent(tc.lux); !near(got, tc.want) {
			t.Errorf("Percent(%v) = %v, want %v", tc.lux, got, tc.want)
		}
	}
}

func TestCurveIsMonotonic(t *testing.T) {
	c := mustCurve(t)
	prev := c.Percent(0)
	for l := Lux(0.01); l < 1e5; l *= 1.07 {
		got := c.Percent(l)
		if got < prev {
			t.Fatalf("Percent(%v) = %v falls below %v", l, got, prev)
		}
		if got < 5 || got > 100 {
			t.Fatalf("Percent(%v) = %v leaves 5-100", l, got)
		}
		prev = got
	}
}

func TestNewCurveRefusesAnInvalidCurve(t *testing.T) {
	for _, tc := range []struct {
		dark, bright   Lux
		floor, ceiling float64
	}{
		{0, 100, 5, 100},
		{100, 10, 5, 100},
		{10, 10, 5, 100},
		{1, 100, -1, 100},
		{1, 100, 50, 50},
		{1, 100, 5, 101},
		{1, 100, math.NaN(), 100},
	} {
		if _, err := NewCurve(tc.dark, tc.bright, tc.floor, tc.ceiling); err == nil {
			t.Errorf("NewCurve(%v, %v, %v, %v) accepted", tc.dark, tc.bright, tc.floor, tc.ceiling)
		}
	}
}

func TestSmootherConvergesOnTheLogOfASteadyLight(t *testing.T) {
	s, err := NewSmoother(30*time.Second, 0.1)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Lux(); ok {
		t.Fatal("an empty smoother has an average")
	}
	at := time.Date(2026, 11, 15, 12, 0, 0, 0, time.UTC)
	s = s.Add(10, at)
	if got, _ := s.Lux(); !near(float64(got), 10) {
		t.Fatalf("the first sample averages to %v, want 10", got)
	}
	at = at.Add(30 * time.Second)
	s = s.Add(1000, at)
	got, _ := s.Lux()
	want := math.Exp(math.Log(10) + (1-math.Exp(-1))*(math.Log(1000)-math.Log(10)))
	if !near(float64(got), want) {
		t.Errorf("one time constant later the average is %v, want %v", got, want)
	}
	for range 20 {
		at = at.Add(30 * time.Second)
		s = s.Add(1000, at)
	}
	if got, _ := s.Lux(); math.Abs(float64(got)-1000) > 0.01 {
		t.Errorf("after twenty time constants the average is %v, want 1000", got)
	}
}

func TestSmootherHoldsTheDarkFinite(t *testing.T) {
	s, err := NewSmoother(time.Minute, 0.1)
	if err != nil {
		t.Fatal(err)
	}
	s = s.Add(0, time.Unix(0, 0))
	if got, _ := s.Lux(); !near(float64(got), 0.1) {
		t.Errorf("darkness averages to %v, want the 0.1 lux floor", got)
	}
	s = s.Add(0, time.Unix(0, 0))
	if got, _ := s.Lux(); !near(float64(got), 0.1) {
		t.Errorf("a sample at the same instant moved the average to %v", got)
	}
}

func TestNewSmootherRefusesNonPositiveParameters(t *testing.T) {
	if _, err := NewSmoother(0, 1); err == nil {
		t.Error("a zero time constant accepted")
	}
	if _, err := NewSmoother(time.Second, 0); err == nil {
		t.Error("a zero lux floor accepted")
	}
}

func TestOutsideHoldsInsideTheBandAndMovesBeyondIt(t *testing.T) {
	for _, tc := range []struct {
		current, target float64
		want            bool
	}{
		{50, 50, false},
		{50, 55.9, false},
		{50, 44.1, false},
		{50, 56, false},
		{50, 56.1, true},
		{50, 43.9, true},
		{5, 100, true},
	} {
		if got := Outside(tc.current, tc.target, 6); got != tc.want {
			t.Errorf("Outside(%v, %v, 6) = %v, want %v", tc.current, tc.target, got, tc.want)
		}
	}
}

func TestStepsAreEvenAndEndAtTheTarget(t *testing.T) {
	got := Steps(20, 50, 3)
	want := []float64{30, 40, 50}
	if len(got) != len(want) {
		t.Fatalf("Steps(20, 50, 3) = %v", got)
	}
	for i := range want {
		if !near(got[i], want[i]) {
			t.Errorf("Steps(20, 50, 3) = %v, want %v", got, want)
		}
	}
	down := Steps(80, 10, 7)
	for i := 1; i < len(down); i++ {
		if down[i] >= down[i-1] {
			t.Errorf("Steps(80, 10, 7) = %v does not fall", down)
		}
	}
	if down[len(down)-1] != 10 {
		t.Errorf("Steps(80, 10, 7) ends at %v", down[len(down)-1])
	}
	if one := Steps(10, 90, 0); len(one) != 1 || one[0] != 90 {
		t.Errorf("Steps with no count = %v, want a jump to 90", one)
	}
}

func TestTheRoomTurnsBrightAndDimAcrossAGap(t *testing.T) {
	band, err := NewRoomBand(250, 500)
	if err != nil {
		t.Fatal(err)
	}
	room := autobrightness.RoomUnknown
	for i, step := range []struct {
		lux  Lux
		want autobrightness.Room
	}{
		{300, autobrightness.RoomDim},
		{500, autobrightness.RoomBright},
		{300, autobrightness.RoomBright},
		{249, autobrightness.RoomDim},
		{499, autobrightness.RoomDim},
	} {
		room = band.After(room, step.lux)
		if room != step.want {
			t.Errorf("step %d at %v lux: %v, want %v", i, step.lux, room, step.want)
		}
	}
	for _, b := range [][2]Lux{{0, 500}, {500, 500}, {600, 500}} {
		if _, err := NewRoomBand(b[0], b[1]); err == nil {
			t.Errorf("NewRoomBand(%v, %v) accepted", b[0], b[1])
		}
	}
}
