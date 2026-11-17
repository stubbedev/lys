package recorder

import (
	"testing"
	"time"
)

// Frames fall due every 20 ms of clock past the latency, less those
// already emitted; none before it.
func TestFramesDue(t *testing.T) {
	for _, c := range []struct {
		elapsed, latency time.Duration
		emitted, want    int
	}{
		{50 * time.Millisecond, 100 * time.Millisecond, 0, 0},
		{159 * time.Millisecond, 100 * time.Millisecond, 0, 2},
		{1100 * time.Millisecond, 100 * time.Millisecond, 10, 40},
		{1 * time.Second, 0, 50, 0},
		{1 * time.Second, 0, 60, 0},
	} {
		if got := framesDue(c.elapsed, c.latency, c.emitted); got != c.want {
			t.Errorf("framesDue(%v, %v, %d) = %d, want %d", c.elapsed, c.latency, c.emitted, got, c.want)
		}
	}
}

func TestFramesToEnd(t *testing.T) {
	for _, c := range []struct {
		elapsed       time.Duration
		emitted, want int
	}{
		{0, 0, 1},
		{17500 * time.Microsecond, 0, 1},
		{17501 * time.Microsecond, 0, 2},
		{339 * time.Millisecond, 0, 18},
		{339 * time.Millisecond, 15, 3},
		{339 * time.Millisecond, 18, 0},
		{339 * time.Millisecond, 20, 0},
	} {
		if got := framesToEnd(c.elapsed, c.emitted); got != c.want {
			t.Errorf("framesToEnd(%v, %d) = %d, want %d", c.elapsed, c.emitted, got, c.want)
		}
	}
}
