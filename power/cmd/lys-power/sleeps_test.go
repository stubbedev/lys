package main

import (
	"errors"
	"testing"
	"time"

	"github.com/stubbedev/lys/power/sleep"
)

func TestStatusListsTheLastSleepsNewestFirst(t *testing.T) {
	slept := time.Date(2026, 11, 20, 23, 0, 0, 0, time.UTC)
	reports := []sleep.Report{
		{Slept: slept.Add(-24 * time.Hour), Woke: slept.Add(-23 * time.Hour), Depth: sleep.DepthUnmeasured},
		{
			Slept: slept, Woke: slept.Add(6*time.Hour + 20*time.Second), Depth: sleep.DepthShallow, DeepShare: 0.257, Drain: sleep.Drew(1400),
			Blockers: []string{"USB2PLL_OFF_STS", "ISH_VNNAON_REQ"}, WokenBy: []string{"PNP0C0D:00"},
		},
	}
	want := "sleeps:      the last 2, newest first\n" +
		"  2026-11-20 23:00 for 6h0m0s: shallow, 25% in the deepest state, drew 1400 mW\n" +
		"    kept shallow by USB2PLL_OFF_STS, ISH_VNNAON_REQ\n" +
		"    woken by PNP0C0D:00\n" +
		"  2026-11-19 23:00 for 1h0m0s: unmeasured\n"
	if got := string(appendSleeps(nil, reports, nil, time.UTC)); got != want {
		t.Errorf("status prints\n%s\nwant\n%s", got, want)
	}
	if got := string(appendSleeps(nil, nil, nil, time.UTC)); got != "sleeps:      none measured yet\n" {
		t.Errorf("no sleeps print %q", got)
	}
	if got := string(appendSleeps(nil, nil, errors.New("parsing the sleep reports: bad"), time.UTC)); got != "sleeps:      parsing the sleep reports: bad\n" {
		t.Errorf("unreadable sleeps print %q", got)
	}
}
