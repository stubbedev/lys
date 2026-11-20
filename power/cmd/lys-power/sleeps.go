package main

import (
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/stubbedev/lys/power/sleep"
)

func appendSleeps(out []byte, reports []sleep.Report, err error, loc *time.Location) []byte {
	switch {
	case err != nil:
		return fmt.Appendf(out, "sleeps:      %v\n", err)
	case len(reports) == 0:
		return fmt.Appendf(out, "sleeps:      none measured yet\n")
	}
	out = fmt.Appendf(out, "sleeps:      the last %d, newest first\n", len(reports))
	for _, r := range slices.Backward(reports) {
		out = appendSleep(out, r, loc)
	}
	return out
}

func appendSleep(out []byte, r sleep.Report, loc *time.Location) []byte {
	depth := r.Depth.String()
	if r.Depth != sleep.DepthUnmeasured {
		depth = fmt.Sprintf("%s, %.0f%% in the deepest state", depth, math.Floor(r.DeepShare*100))
	}
	out = fmt.Appendf(out, "  %s for %s: %s", r.Slept.In(loc).Format("2006-01-02 15:04"), r.Asleep().Round(time.Minute), depth)
	if mw, ok := r.Drain.Milliwatts(); ok {
		out = fmt.Appendf(out, ", drew %d mW", mw)
	}
	out = append(out, '\n')
	if len(r.Blockers) > 0 {
		out = fmt.Appendf(out, "    kept shallow by %s\n", strings.Join(r.Blockers, ", "))
	}
	if len(r.WokenBy) > 0 {
		out = fmt.Appendf(out, "    woken by %s\n", strings.Join(r.WokenBy, ", "))
	}
	return out
}
