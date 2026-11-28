package brightness

import (
	"context"
	"testing"

	"github.com/stubbedev/lys/desktop/service/backlight/backlighttest"
	"github.com/stubbedev/lys/lib/waittest"
)

func ddcLevel(t *testing.T, sys *System, name string) (Device, bool) {
	t.Helper()
	devices, err := sys.Devices(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range devices {
		if d.Name == name {
			return d, true
		}
	}
	return Device{}, false
}

func TestQueuedDDCLevelShowsAndTicksBeforeTheWriteLands(t *testing.T) {
	class := backlighttest.Fake(t, nil)
	fake := newFakeI2C(t)
	monitor := &fakeMonitor{current: 40, max: 100, hold: make(chan struct{})}
	fake.plugOn(t, 2, monitor, "card1-DP-2")
	sys := newTestSystem(t, class, true)
	waittest.Until(t, func() bool { return len(ddcNames(t, sys)) == 1 }, "the initial DDC scan")
	ticks, stop, err := sys.Subscribe(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer stop()

	if err := sys.Set(context.Background(), "ddci2c-2", 70); err != nil {
		t.Fatal(err)
	}
	waittest.For(t, ticks, "a tick for a queued DDC write")
	if err := sys.Set(context.Background(), "ddci2c-2", 80); err != nil {
		t.Fatal(err)
	}
	if d, _ := ddcLevel(t, sys, "ddci2c-2"); d.Brightness != 80 || d.Connector != "DP-2" {
		t.Errorf("while the write is held: %+v, want the queued 80 on DP-2", d)
	}
	if len(monitor.written()) != 0 {
		t.Fatalf("the held write landed: %v", monitor.written())
	}

	close(monitor.hold)
	waittest.Until(t, func() bool {
		w := monitor.written()
		return len(w) > 0 && w[len(w)-1] == 80
	}, "the queued write")
	waittest.Until(t, func() bool {
		sys.mu.Lock()
		defer sys.mu.Unlock()
		_, running := sys.requested["ddci2c-2"]
		return !running
	}, "the writer to settle")
	if d, _ := ddcLevel(t, sys, "ddci2c-2"); d.Brightness != 80 {
		t.Errorf("after the write: %d, want 80", d.Brightness)
	}
}

func TestFailedDDCWriteFallsBackToTheMonitorsLevel(t *testing.T) {
	class := backlighttest.Fake(t, nil)
	fake := newFakeI2C(t)
	monitor := &fakeMonitor{current: 40, max: 100, failWrites: 1 << 20}
	fake.plug(t, 2, monitor)
	sys := newTestSystem(t, class, true)
	waittest.Until(t, func() bool { return len(ddcNames(t, sys)) == 1 }, "the initial DDC scan")
	if err := sys.Set(context.Background(), "ddci2c-2", 90); err != nil {
		t.Fatal(err)
	}
	waittest.Until(t, func() bool {
		d, _ := ddcLevel(t, sys, "ddci2c-2")
		return d.Brightness == 40
	}, "the failed write to settle")
	if d, _ := ddcLevel(t, sys, "ddci2c-2"); d.Connector != "" {
		t.Errorf("an unwired bus lights %q", d.Connector)
	}
}
