package bar

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/lib/wallclock/wallclocktest"

	"github.com/stubbedev/lys/shell/internal/clock/clocktest"
)

func zoneClockContext(t *testing.T) (ModuleContext, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "localtime")
	cfg := config.Defaults()
	cfg.Clock.Format = mustStrftimeFormat(t, "%H:%M %Z")
	ctx := newTestContext(t, cfg)
	ctx.Clock = clocktest.Wall(t, path)
	return ctx, path
}

func sampledClockLabels(t *testing.T, ctx ModuleContext) <-chan string {
	t.Helper()
	hub := testPollHub(t)
	life, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	labels := make(chan string, 16)
	if err := joinSampler(life, hub, clockSampler(ctx, time.Minute), func(text string) { labels <- text }); err != nil {
		t.Fatal(err)
	}
	return labels
}

func TestTheClockLabelFollowsAChangeOfTimeZone(t *testing.T) {
	t.Setenv("TZ", "")
	if err := os.Unsetenv("TZ"); err != nil {
		t.Fatal(err)
	}
	ctx, path := zoneClockContext(t)
	labels := sampledClockLabels(t, ctx)
	wallclocktest.SetZone(t, path, "XST", 5*3600+1800)
	for got := ""; !strings.HasSuffix(got, " XST"); {
		got = waittest.For(t, labels, "the clock label to re-render in the new zone")
	}
	if name, offset := ctx.Clock.Now().Zone(); name != "XST" || offset != 5*3600+1800 {
		t.Errorf("the clock is in %s at %d after the change, want XST at %d", name, offset, 5*3600+1800)
	}
}

func TestTheClockLabelKeepsTheZoneTZSets(t *testing.T) {
	t.Setenv("TZ", "UTC")
	ctx, path := zoneClockContext(t)
	labels := sampledClockLabels(t, ctx)
	wallclocktest.SetZone(t, path, "XST", 5*3600+1800)
	settled := make(chan struct{})
	ctx.Clock.AfterFunc(50*time.Millisecond, func() { close(settled) })
	waittest.For(t, settled, "a countdown armed after the change of the zone file")
	select {
	case got := <-labels:
		if strings.HasSuffix(got, " XST") {
			t.Errorf("with TZ set the clock label re-rendered in the zone file's zone: %q", got)
		}
	default:
	}
	if label, _ := clockSampler(ctx, time.Minute).sample(); strings.HasSuffix(label, " XST") {
		t.Errorf("with TZ set the clock label reads %q, the zone file's zone", label)
	}
}
