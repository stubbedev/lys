package clock_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/lib/wallclock/wallclocktest"

	"github.com/stubbedev/lys/shell/internal/clock/clocktest"
)

func unsetTZ(t *testing.T) {
	t.Setenv("TZ", "")
	if err := os.Unsetenv("TZ"); err != nil {
		t.Fatal(err)
	}
}

func TestTheWallClockFollowsAChangeOfTimeZone(t *testing.T) {
	unsetTZ(t)
	path := filepath.Join(t.TempDir(), "localtime")
	w := clocktest.Wall(t, path)
	changes, stop := w.Changes()
	defer stop()
	const offset = 5*3600 + 1800
	wallclocktest.SetZone(t, path, "XST", offset)
	waittest.For(t, changes, "the change of time zone to tick the wall clock's changes")
	if name, got := w.Now().Zone(); got != offset || name != "XST" {
		t.Errorf("the wall clock is in %s at offset %d after the change, want XST at %d", name, got, offset)
	}
	wallclocktest.SetZone(t, path, "YST", -3600)
	waittest.For(t, changes, "the second change of time zone to tick the wall clock's changes")
	if name, got := w.Now().Zone(); got != -3600 || name != "YST" {
		t.Errorf("the wall clock is in %s at offset %d after the second change, want YST at -3600", name, got)
	}
}

func TestTheWallClockKeepsTheZoneTZSets(t *testing.T) {
	t.Setenv("TZ", "UTC")
	path := filepath.Join(t.TempDir(), "localtime")
	w := clocktest.Wall(t, path)
	changes, stop := w.Changes()
	defer stop()
	name, before := w.Now().Zone()
	wallclocktest.SetZone(t, path, "XST", int32(before+5*3600+1800))
	fired := make(chan struct{})
	w.AfterFunc(50*time.Millisecond, func() { close(fired) })
	waittest.For(t, fired, "a countdown armed after the change of the zone file")
	select {
	case <-changes:
		t.Error("with TZ set a change of the zone file ticked the wall clock's changes")
	default:
	}
	if after, got := w.Now().Zone(); got != before || after != name {
		t.Errorf("with TZ set the wall clock moved from %s at %d to %s at %d", name, before, after, got)
	}
}
