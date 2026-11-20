package compositor

import (
	"testing"
	"time"
)

func TestADeclaredFrameIntervalSnapsToAVideoRate(t *testing.T) {
	for _, tc := range []struct {
		interval time.Duration
		hz       int
	}{
		{time.Second / 24, 24},
		{time.Second * 1001 / 24000, 24},
		{time.Second / 25, 25},
		{time.Second * 1001 / 30000, 30},
		{time.Second / 48, 48},
		{time.Second / 50, 50},
		{time.Second * 1001 / 60000, 60},
		{time.Second / 27, 0},
		{time.Second / 120, 0},
		{time.Second, 0},
		{0, 0},
	} {
		if got := videoRate(tc.interval); got != tc.hz {
			t.Errorf("a frame every %v snapped to %d Hz, want %d", tc.interval, got, tc.hz)
		}
	}
}
