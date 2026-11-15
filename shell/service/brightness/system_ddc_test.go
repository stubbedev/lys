package brightness

import (
	"context"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/service/backlight/backlighttest"
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
	backlighttest.Fake(t, nil)
	fake := newFakeI2C(t)
	monitor := &fakeMonitor{current: 40, max: 100, hold: make(chan struct{})}
	fake.plugOn(t, 2, monitor, "card1-DP-2")
	sys := newTestSystem(t, true)
	waitFor(t, "the initial DDC scan", func() bool { return len(ddcNames(t, sys)) == 1 })
	ticks, stop, err := sys.Subscribe(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer stop()

	if err := sys.Set(context.Background(), "ddci2c-2", 70); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ticks:
	case <-time.After(3 * time.Second):
		t.Fatal("no tick for a queued DDC write")
	}
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
	waitFor(t, "the queued write", func() bool {
		w := monitor.written()
		return len(w) > 0 && w[len(w)-1] == 80
	})
	waitFor(t, "the writer to settle", func() bool {
		sys.mu.Lock()
		defer sys.mu.Unlock()
		_, running := sys.requested["ddci2c-2"]
		return !running
	})
	if d, _ := ddcLevel(t, sys, "ddci2c-2"); d.Brightness != 80 {
		t.Errorf("after the write: %d, want 80", d.Brightness)
	}
}

func TestFailedDDCWriteFallsBackToTheMonitorsLevel(t *testing.T) {
	backlighttest.Fake(t, nil)
	fake := newFakeI2C(t)
	monitor := &fakeMonitor{current: 40, max: 100, failWrites: 1 << 20}
	fake.plug(t, 2, monitor)
	sys := newTestSystem(t, true)
	waitFor(t, "the initial DDC scan", func() bool { return len(ddcNames(t, sys)) == 1 })
	if err := sys.Set(context.Background(), "ddci2c-2", 90); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the failed write to settle", func() bool {
		d, _ := ddcLevel(t, sys, "ddci2c-2")
		return d.Brightness == 40
	})
	if d, _ := ddcLevel(t, sys, "ddci2c-2"); d.Connector != "" {
		t.Errorf("an unwired bus lights %q", d.Connector)
	}
}
