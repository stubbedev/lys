package wm

import "testing"

func TestABlockedWindowStaysOutOfEveryCaptureItIsBlockedFrom(t *testing.T) {
	for _, c := range []struct {
		from       BlockFrom
		screencast bool
		screenshot bool
	}{
		{BlockNone, false, false},
		{BlockScreencast, true, false},
		{BlockCapture, true, true},
	} {
		if got := c.from.Blocks(Screencast); got != c.screencast {
			t.Errorf("%v blocks a screencast: %v, want %v", c.from, got, c.screencast)
		}
		if got := c.from.Blocks(Screenshot); got != c.screenshot {
			t.Errorf("%v blocks a screenshot: %v, want %v", c.from, got, c.screenshot)
		}
	}
}

func TestACaptureIsAScreencastUnlessMarkedAScreenshot(t *testing.T) {
	var kind CaptureKind
	if kind != Screencast {
		t.Fatalf("an unmarked capture is %v, want a screencast", kind)
	}
}
