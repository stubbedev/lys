package backlight_test

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/logind"
	"github.com/stubbedev/lys/lib/logind/logindtest"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/udevmonitor/udevmonitortest"
	"github.com/stubbedev/lys/lib/waittest"

	"github.com/stubbedev/lys/desktop/service/backlight"
	"github.com/stubbedev/lys/desktop/service/backlight/backlighttest"
)

func newSysfs(t *testing.T, system *dbus.Conn, drift time.Duration) (*backlight.Sysfs, *udevmonitortest.Group) {
	t.Helper()
	udev := udevmonitortest.New(t)
	backlight.Inject(t, udev.Listen, drift)
	s := backlight.NewSysfs(system)
	t.Cleanup(func() { _ = s.Close() })
	return s, udev
}

func quietSysfs(t *testing.T) (*backlight.Sysfs, *udevmonitortest.Group) {
	t.Helper()
	return newSysfs(t, nil, time.Hour)
}

func subscribe(t *testing.T, s *backlight.Sysfs) <-chan struct{} {
	t.Helper()
	ticks, stop, err := s.Subscribe(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(stop)
	return ticks
}

const quietWindow = 50 * time.Millisecond

func ticksSoon(ticks <-chan struct{}) bool {
	select {
	case <-ticks:
		return true
	case <-time.After(quietWindow):
		return false
	}
}

func level(t *testing.T, name string) uint32 {
	t.Helper()
	d, err := backlight.ReadDevice(name)
	if err != nil {
		t.Fatal(err)
	}
	return d.Brightness
}

func TestSetWithoutASystemBusWritesSysfs(t *testing.T) {
	backlighttest.Fake(t, map[string]backlighttest.Device{"intel_backlight": {Type: "raw", Brightness: 3000, Max: 12000}})
	s, _ := quietSysfs(t)
	if err := s.Set(context.Background(), "intel_backlight", 50); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := level(t, "intel_backlight"); got != 6000 {
		t.Errorf("brightness = %d, want 6000 (50%% of 12000)", got)
	}
	if err := s.Set(context.Background(), "intel_backlight", 900); err != nil {
		t.Fatal(err)
	}
	if got := level(t, "intel_backlight"); got != 12000 {
		t.Errorf("clamped brightness = %d, want max", got)
	}
	if err := s.Set(context.Background(), "nothing", 50); err == nil {
		t.Error("Set on a device that is not there succeeded")
	}
}

func TestSetGoesThroughLogindWhenItAnswers(t *testing.T) {
	backlighttest.Fake(t, map[string]backlighttest.Device{"intel_backlight": {Type: "raw", Brightness: 3000, Max: 12000}})
	fake := logindtest.Start(t, logindtest.Config{Display: "2"})
	s, _ := newSysfs(t, fake.Bus.Conn(t), time.Hour)
	if err := s.Set(context.Background(), "intel_backlight", 50); err != nil {
		t.Fatal(err)
	}
	calls := fake.CallsTo("Session.SetBrightness")
	if len(calls) != 1 || !slices.Equal(calls[0].Args, []any{"backlight", "intel_backlight", uint32(6000)}) {
		t.Fatalf("SetBrightness calls %v, want one for 6000", calls)
	}
	if got := level(t, "intel_backlight"); got != 3000 {
		t.Errorf("a write logind took also went to sysfs: %d", got)
	}
}

func TestSetFallsBackToSysfsWhenLogindRefuses(t *testing.T) {
	backlighttest.Fake(t, map[string]backlighttest.Device{"intel_backlight": {Type: "raw", Brightness: 3000, Max: 12000}})
	fake := logindtest.Start(t, logindtest.Config{})
	s, _ := newSysfs(t, fake.Bus.Conn(t), time.Hour)
	if err := s.Set(context.Background(), "intel_backlight", 50); err != nil {
		t.Fatal(err)
	}
	if got := level(t, "intel_backlight"); got != 6000 {
		t.Errorf("brightness = %d, want the sysfs fallback's 6000", got)
	}
}

func TestSubscribeTicksOnWrites(t *testing.T) {
	backlighttest.Fake(t, map[string]backlighttest.Device{"intel_backlight": {Type: "raw", Brightness: 3000, Max: 12000}})
	s, _ := quietSysfs(t)
	ticks := subscribe(t, s)
	backlighttest.SetLevel(t, "intel_backlight", 9000)
	waittest.For(t, ticks, "a tick for a brightness write")
}

func TestBacklightHotplugWatchesTheNewDevice(t *testing.T) {
	backlighttest.Fake(t, nil)
	s, events := quietSysfs(t)
	ticks := subscribe(t, s)
	backlighttest.Add(t, "acpi_video0", backlighttest.Device{Type: "firmware", Brightness: 5, Max: 10})
	events.Send("ACTION=add", "SUBSYSTEM=backlight", "DEVPATH=/devices/virtual/backlight/acpi_video0")
	waittest.For(t, ticks, "a tick for the hotplugged device")
	time.Sleep(10 * time.Millisecond)
	backlighttest.SetLevel(t, "acpi_video0", 7)
	waittest.For(t, ticks, "a tick for a write to the hotplugged device")
}

func TestSubscribeStopsAndRefusesAfterClose(t *testing.T) {
	backlighttest.Fake(t, nil)
	s, _ := quietSysfs(t)
	ticks, stop, err := s.Subscribe(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	stop()
	for range ticks {
	}
	_ = s.Close()
	if _, _, err := s.Subscribe(context.Background()); !errors.Is(err, backlight.ErrClosed) {
		t.Errorf("Subscribe after Close: %v, want %v", err, backlight.ErrClosed)
	}
}

func TestCloseEndsEverySubscription(t *testing.T) {
	backlighttest.Fake(t, nil)
	s, _ := quietSysfs(t)
	ticks := subscribe(t, s)
	_ = s.Close()
	waittest.Until(t, func() bool {
		for {
			select {
			case _, ok := <-ticks:
				if !ok {
					return true
				}
			default:
				return false
			}
		}
	}, "Close to end the subscription")
}

func TestDriftWithoutADeviceWakesNobodyUntilOneAppears(t *testing.T) {
	backlighttest.Fake(t, nil)
	s, events := newSysfs(t, nil, time.Millisecond)
	ticks := subscribe(t, s)
	if ticksSoon(ticks) {
		t.Fatal("a machine without a backlight ticked")
	}
	backlighttest.Add(t, "acpi_video0", backlighttest.Device{Type: "firmware", Brightness: 5, Max: 10})
	events.Send("ACTION=add", "SUBSYSTEM=backlight", "DEVPATH=/devices/virtual/backlight/acpi_video0")
	waittest.For(t, ticks, "a tick from the hotplugged device")
	if ticksSoon(ticks) {
		t.Fatal("an unmoved level ticked")
	}
}

func TestDriftWithAStillLevelWakesNobody(t *testing.T) {
	backlighttest.Fake(t, map[string]backlighttest.Device{"acpi_video0": {Type: "firmware", Brightness: 5, Max: 10}})
	s, _ := newSysfs(t, nil, time.Millisecond)
	ticks := subscribe(t, s)
	if ticksSoon(ticks) {
		t.Fatal("an unmoved level ticked")
	}
}

func TestBacklightChangeUeventTicksAndOthersDoNot(t *testing.T) {
	backlighttest.Fake(t, nil)
	s, events := quietSysfs(t)
	ticks := subscribe(t, s)
	events.Send("ACTION=change", "SUBSYSTEM=drm", "HOTPLUG=1")
	if ticksSoon(ticks) {
		t.Fatal("a drm change ticked the backlight subscribers")
	}
	events.Send("ACTION=change", "SUBSYSTEM=backlight", "DEVPATH=/devices/virtual/backlight/x")
	waittest.For(t, ticks, "a tick for the level the kernel announced")
}

func TestLevelProbeSeesAChangeOnTheFileItHoldsOpen(t *testing.T) {
	dir := backlighttest.Fake(t, map[string]backlighttest.Device{"acpi_video0": {Type: "firmware", Brightness: 5, Max: 10}})
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	probe := backlight.OpenProbe(entries)
	defer probe.Close()
	if moved, err := probe.Moved(); moved || err != nil {
		t.Fatalf("unchanged level: moved %v, err %v", moved, err)
	}
	backlighttest.SetLevel(t, "acpi_video0", 7)
	if moved, err := probe.Moved(); !moved || err != nil {
		t.Fatalf("changed level: moved %v, err %v", moved, err)
	}
	if moved, err := probe.Moved(); moved || err != nil {
		t.Fatalf("settled level: moved %v, err %v", moved, err)
	}
	probe.CloseFirstFile()
	if _, err := probe.Moved(); err == nil {
		t.Fatal("a device that no longer reads was not reported")
	}
}

func TestLogindWriterResolvesTheDisplaySessionOnce(t *testing.T) {
	fake := logindtest.Start(t, logindtest.Config{Display: "2"})
	set := backlight.LogindWriter(fake.Bus.Conn(t))
	for _, value := range []uint32{300, 600} {
		if err := set(t.Context(), "intel_backlight", value); err != nil {
			t.Fatalf("set %d: %v", value, err)
		}
	}
	if got := fake.CallsTo("Manager.GetSession"); len(got) != 1 || !slices.Equal(got[0].Args, []any{"auto"}) {
		t.Errorf("GetSession calls %v, want one for auto", got)
	}
	calls := fake.CallsTo("Session.SetBrightness")
	if len(calls) != 2 {
		t.Fatalf("SetBrightness calls %v, want two", calls)
	}
	for i, value := range []uint32{300, 600} {
		want := []any{"backlight", "intel_backlight", value}
		if calls[i].Path != logind.SessionPath("2") || !slices.Equal(calls[i].Args, want) {
			t.Errorf("SetBrightness call %d: %v, want %v on %s", i, calls[i], want, logind.SessionPath("2"))
		}
	}
}

func TestLogindWriterWithoutASessionFails(t *testing.T) {
	fake := logindtest.Start(t, logindtest.Config{})
	set := backlight.LogindWriter(fake.Bus.Conn(t))
	if err := set(t.Context(), "intel_backlight", 300); err == nil {
		t.Fatal("set with no session succeeded")
	}
	if got := fake.CallsTo("Session.SetBrightness"); len(got) != 0 {
		t.Errorf("SetBrightness calls %v with no session", got)
	}
}
