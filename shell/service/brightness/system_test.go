package brightness

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/service/backlight"
	"github.com/stubbedev/lys/desktop/service/backlight/backlighttest"
	"github.com/stubbedev/lys/desktop/uevent"
)

type fakeUevents struct {
	events chan uevent.Event
	closed chan struct{}
}

func (f *fakeUevents) Next() (uevent.Event, error) {
	select {
	case ev := <-f.events:
		return ev, nil
	case <-f.closed:
		return uevent.Event{}, errors.New("closed")
	}
}

func (f *fakeUevents) Close() error {
	close(f.closed)
	return nil
}

// newTestSystem builds a System on the fake trees with injected
// uevents; the caller has already faked BacklightDir (and DevDir for
// external monitors).
func newTestSystem(t *testing.T, external bool) *System {
	t.Helper()
	sys, _ := newTestSystemWithEvents(t, external)
	return sys
}

func newTestSystemWithEvents(t *testing.T, external bool) (*System, chan<- uevent.Event) {
	t.Helper()
	events := &fakeUevents{events: make(chan uevent.Event), closed: make(chan struct{})}
	orig := listenUevents
	listenUevents = func() (uevent.Reader, error) { return events, nil }
	t.Cleanup(func() { listenUevents = orig })
	sys := NewSystem(nil, external)
	sys.settle = 0
	t.Cleanup(func() { _ = sys.Close() })
	return sys, events.events
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func ddcNames(t *testing.T, sys *System) []string {
	t.Helper()
	devices, err := sys.Devices(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, d := range devices {
		if d.Type == TypeDDC {
			names = append(names, d.Name)
		}
	}
	return names
}

func TestOnlyADRMChangeWithHotplugIsAHotplug(t *testing.T) {
	if !isDRMHotplug(uevent.Event{Action: uevent.Change, Subsystem: "drm", Props: map[string]string{"HOTPLUG": "1"}}) {
		t.Error("a DRM change with HOTPLUG=1 is no hotplug")
	}
	for name, ev := range map[string]uevent.Event{
		"mode set":         {Action: uevent.Change, Subsystem: "drm", Props: map[string]string{}},
		"connector add":    {Action: uevent.Add, Subsystem: "drm", Props: map[string]string{"HOTPLUG": "1"}},
		"backlight change": {Action: uevent.Change, Subsystem: "backlight", Props: map[string]string{"HOTPLUG": "1"}},
	} {
		if isDRMHotplug(ev) {
			t.Errorf("a %s counted as a DRM hotplug", name)
		}
	}
}

func TestExternalMonitorsAppearAfterTheDetachedScan(t *testing.T) {
	backlighttest.Fake(t, nil)
	fake := newFakeI2C(t)
	fake.plug(t, 6, &fakeMonitor{current: 40, max: 100})
	sys := newTestSystem(t, true)
	waitFor(t, "the initial DDC scan", func() bool { return len(ddcNames(t, sys)) == 1 })
}

func TestExternalDisabledNeverScans(t *testing.T) {
	backlighttest.Fake(t, nil)
	fake := newFakeI2C(t)
	fake.plug(t, 6, &fakeMonitor{current: 40, max: 100})
	sys, events := newTestSystemWithEvents(t, false)
	events <- uevent.Event{Action: uevent.Change, Subsystem: "drm", Props: map[string]string{"HOTPLUG": "1"}}
	time.Sleep(50 * time.Millisecond)
	if names := ddcNames(t, sys); len(names) != 0 {
		t.Errorf("enable-external=false still found %v", names)
	}
}

func TestDRMHotplugRescansAndTicks(t *testing.T) {
	backlighttest.Fake(t, nil)
	fake := newFakeI2C(t)
	fake.plug(t, 1, &fakeMonitor{current: 20, max: 100})
	sys, events := newTestSystemWithEvents(t, true)
	ticks, stop, err := sys.Subscribe(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	waitFor(t, "the initial DDC scan", func() bool { return len(ddcNames(t, sys)) == 1 })
	// The detached initial scan ticks only when it finished after the
	// subscription; consume that tick if it came, never wait on one
	// that cannot (a fast scan under -race hung here).
	select {
	case <-ticks:
	case <-time.After(200 * time.Millisecond):
	}

	fake.plug(t, 8, &fakeMonitor{current: 70, max: 100})
	// A non-hotplug DRM change must not rescan.
	events <- uevent.Event{Action: uevent.Change, Subsystem: "drm", Props: map[string]string{}}
	time.Sleep(50 * time.Millisecond)
	if names := ddcNames(t, sys); len(names) != 1 {
		t.Fatalf("a mode-set change rescanned: %v", names)
	}

	events <- uevent.Event{Action: uevent.Change, Subsystem: "drm", Props: map[string]string{"HOTPLUG": "1"}}
	waitFor(t, "the hotplug rescan", func() bool { return len(ddcNames(t, sys)) == 2 })
	select {
	case <-ticks:
	case <-time.After(3 * time.Second):
		t.Fatal("no tick after a monitor appeared")
	}

	fake.unplug(t, 8)
	events <- uevent.Event{Action: uevent.Change, Subsystem: "drm", Props: map[string]string{"HOTPLUG": "1"}}
	waitFor(t, "the unplug rescan", func() bool { return len(ddcNames(t, sys)) == 1 })
}

func TestSetRoutesDDCNamesToTheMonitor(t *testing.T) {
	backlighttest.Fake(t, map[string]backlighttest.Device{"intel_backlight": {Type: "raw", Brightness: 3000, Max: 12000}})
	fake := newFakeI2C(t)
	monitor := &fakeMonitor{current: 10, max: 200}
	fake.plug(t, 2, monitor)
	sys := newTestSystem(t, true)
	waitFor(t, "the initial DDC scan", func() bool { return len(ddcNames(t, sys)) == 1 })

	if err := sys.Set(context.Background(), "ddci2c-2", 50); err != nil {
		t.Fatal(err)
	}
	waitFor(t, "the DDC write", func() bool {
		w := monitor.written()
		return len(w) > 0 && w[len(w)-1] == 100
	})
	devices, _ := sys.Devices(context.Background())
	for _, d := range devices {
		if d.Name == "ddci2c-2" && d.Brightness != 100 {
			t.Errorf("cached DDC level = %d, want 100", d.Brightness)
		}
	}
	// The internal panel still goes through sysfs, untouched by DDC.
	if device, _ := backlight.ReadDevice("intel_backlight"); device.Brightness != 3000 {
		t.Errorf("a DDC write moved the panel to %d", device.Brightness)
	}
	// An unknown name is neither: it errors instead of being dropped.
	if err := sys.Set(context.Background(), "ddci2c-9", 50); err == nil {
		t.Error("Set on an unknown device: want an error")
	}
}

func TestSubscribeStopsAndRefusesAfterClose(t *testing.T) {
	backlighttest.Fake(t, nil)
	sys := newTestSystem(t, false)
	ticks, stop, err := sys.Subscribe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	stop()
	for range ticks { // drain until the forwarder closes
	}
	_ = sys.Close()
	if _, _, err := sys.Subscribe(context.Background()); err == nil {
		t.Error("Subscribe after Close: want an error")
	}
}

func TestSetExternalOffClosesTheBusesAndStopsScanning(t *testing.T) {
	backlighttest.Fake(t, nil)
	fake := newFakeI2C(t)
	monitor := &fakeMonitor{current: 40, max: 100}
	fake.plug(t, 6, monitor)
	sys, events := newTestSystemWithEvents(t, true)
	waitFor(t, "the initial DDC scan", func() bool { return len(ddcNames(t, sys)) == 1 })
	ticks, stop, err := sys.Subscribe(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer stop()

	sys.SetExternal(false)
	if names := ddcNames(t, sys); len(names) != 0 {
		t.Errorf("enable-external turned off still lists %v", names)
	}
	monitor.mu.Lock()
	closes := monitor.closes
	monitor.mu.Unlock()
	if closes == 0 {
		t.Error("enable-external turned off left the bus open")
	}
	select {
	case <-ticks:
	case <-time.After(3 * time.Second):
		t.Error("no tick after the monitors went away")
	}

	fake.plug(t, 8, &fakeMonitor{current: 70, max: 100})
	events <- uevent.Event{Action: uevent.Change, Subsystem: "drm", Props: map[string]string{"HOTPLUG": "1"}}
	time.Sleep(50 * time.Millisecond)
	if names := ddcNames(t, sys); len(names) != 0 {
		t.Errorf("a hotplug with enable-external off scanned: %v", names)
	}
	if err := sys.Set(context.Background(), "ddci2c-6", 50); err == nil {
		t.Error("a write to a released monitor succeeded")
	}
	if got := monitor.written(); len(got) != 0 {
		t.Errorf("a released monitor was written: %v", got)
	}
}

func TestSetExternalOnScansAndARepeatDoesNot(t *testing.T) {
	backlighttest.Fake(t, nil)
	fake := newFakeI2C(t)
	fake.plug(t, 6, &fakeMonitor{current: 40, max: 100})
	sys, _ := newTestSystemWithEvents(t, false)
	if names := ddcNames(t, sys); len(names) != 0 {
		t.Fatalf("enable-external off found %v", names)
	}
	sys.SetExternal(true)
	waitFor(t, "the scan on enable", func() bool { return len(ddcNames(t, sys)) == 1 })
	manager := sys.ddc.Load()
	sys.SetExternal(true)
	if sys.ddc.Load() != manager {
		t.Error("enabling twice replaced the manager and its open buses")
	}
}
