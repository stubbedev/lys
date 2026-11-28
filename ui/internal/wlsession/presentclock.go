package wlsession

import (
	"time"

	"github.com/stubbedev/lys/lib/intconv"
	"github.com/stubbedev/lys/lib/logutil"
	"github.com/stubbedev/lys/lib/wlr"
	"golang.org/x/sys/unix"
)

// presentClock maps wp_presentation timestamps onto the local clock. It
// knows the mapping only for CLOCK_MONOTONIC, the clock Go's monotonic
// readings come from: base and baseMono are one instant on both.
type presentClock struct {
	monotonic bool
	base      time.Time
	baseMono  time.Duration
}

// HandleWpPresentationClockId implements wlr.WpPresentationClockIdHandler.
func (c *presentClock) HandleWpPresentationClockId(ev wlr.WpPresentationClockIdEvent) {
	c.monotonic = false
	if ev.ClkId != unix.CLOCK_MONOTONIC {
		logutil.L().Debug("wlsession: presentation clock is not CLOCK_MONOTONIC; frames are not paced by presentation time", "clock", ev.ClkId)
		return
	}
	var ts unix.Timespec
	base := time.Now()
	if err := unix.ClockGettime(unix.CLOCK_MONOTONIC, &ts); err != nil {
		logutil.L().Error("wlsession: read CLOCK_MONOTONIC for presentation times; frames are not paced by presentation time", "err", err)
		return
	}
	c.monotonic, c.base, c.baseMono = true, base, time.Duration(ts.Nano())
}

// PresentationTime is when a wp_presentation timestamp happened on the
// local clock, false while the compositor's presentation clock is not
// CLOCK_MONOTONIC or not announced yet.
func (s *Session) PresentationTime(secHi, secLo, nsec uint32) (time.Time, bool) {
	c := &s.presentClock
	if !c.monotonic {
		return time.Time{}, false
	}
	at := intconv.Duration(uint64(secHi)<<32|uint64(secLo), time.Second) + time.Duration(nsec)
	return c.base.Add(at - c.baseMono), true
}
