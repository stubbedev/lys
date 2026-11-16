package wallclock

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/lib/testhome"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/lib/wallclock/wallclocktest"
)

func TestMain(m *testing.M) { testhome.Run(m) }

func TestTheTestsReachNoHostSession(t *testing.T) { testhome.RequireNoHostSession(t) }

func onLoop(calls chan func()) func(func()) {
	return func(fn func()) { calls <- fn }
}

func startClock(t *testing.T, path string) (*SystemClock, chan func()) {
	t.Helper()
	calls := make(chan func(), 4)
	c, err := NewSystemClock(onLoop(calls), path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := c.Close(); err != nil {
			t.Error(err)
		}
	})
	return c, calls
}

func unsetTZ(t *testing.T) {
	t.Helper()
	t.Setenv("TZ", "")
	if err := os.Unsetenv("TZ"); err != nil {
		t.Fatal(err)
	}
}

func TestTheSystemClockFiresOnTheWallClock(t *testing.T) {
	c, calls := startClock(t, filepath.Join(t.TempDir(), "localtime"))
	fired := false
	c.Arm(c.Now().Add(10*time.Millisecond), func() { fired = true })
	waittest.For(t, calls, "the timer to fire")()
	if !fired {
		t.Error("what was armed did not run")
	}
	c.Arm(c.Now().Add(time.Hour), func() { t.Error("a disarmed timer ran") })
	c.Disarm()
	c.fire()
}

func TestTheSystemClockFollowsAChangeOfTimeZone(t *testing.T) {
	unsetTZ(t)
	path := filepath.Join(t.TempDir(), "localtime")
	c, calls := startClock(t, path)
	fired := false
	c.Arm(c.Now().Add(time.Hour), func() { fired = true })
	const offset = 5*3600 + 1800
	wallclocktest.SetZone(t, path, "XST", offset)
	waittest.For(t, calls, "the change of time zone to wake the timer")()
	if !fired {
		t.Error("what was armed did not run on the change")
	}
	if _, got := c.Now().Zone(); got != offset {
		t.Errorf("the clock is at offset %d after the change, want %d", got, offset)
	}
}

func TestTheSystemClockKeepsTheZoneTZSets(t *testing.T) {
	t.Setenv("TZ", "UTC")
	path := filepath.Join(t.TempDir(), "localtime")
	c, calls := startClock(t, path)
	_, before := c.Now().Zone()
	wallclocktest.SetZone(t, path, "XST", int32(before+5*3600+1800))
	c.Arm(c.Now().Add(50*time.Millisecond), func() {})
	waittest.For(t, calls, "the timer armed after the change of the zone file")()
	if _, after := c.Now().Zone(); after != before {
		t.Errorf("with TZ set the clock moved from offset %d to %d on a change of the zone file", before, after)
	}
}

func TestSettingTheClockWakesTheTimer(t *testing.T) {
	if os.Getenv("LYS_WALLCLOCK_VM") == "" {
		t.Skip("LYS_WALLCLOCK_VM is unset: this sets the clock, so only the wallclock VM scenario runs it")
	}
	c, calls := startClock(t, filepath.Join(t.TempDir(), "localtime"))
	for _, jump := range []time.Duration{time.Hour, -2 * time.Hour} {
		fired := false
		c.Arm(c.Now().Add(24*time.Hour), func() { fired = true })
		ts := unix.NsecToTimeval(time.Now().Add(jump).UnixNano())
		if err := unix.Settimeofday(&ts); err != nil {
			t.Fatal(err)
		}
		waittest.ForSlow(t, calls, fmt.Sprintf("setting the clock by %v to wake the timer", jump))()
		if !fired {
			t.Errorf("setting the clock by %v did not run what was armed", jump)
		}
	}
}
