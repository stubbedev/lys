package main

import (
	"context"
	"math"
	"sync"
	"testing"

	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/dbusx"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/power/autobrightness"

	"github.com/stubbedev/lys/shell/service/brightness"
)

type fakeBacklights struct {
	mu      sync.Mutex
	devices []brightness.Device
}

func (f *fakeBacklights) Devices(context.Context) ([]brightness.Device, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]brightness.Device(nil), f.devices...), nil
}

func (f *fakeBacklights) Subscribe(context.Context) (<-chan struct{}, func(), error) {
	return nil, func() {}, nil
}

func (f *fakeBacklights) Set(_ context.Context, name string, percent float64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, d := range f.devices {
		if d.Name == name {
			f.devices[i].Brightness = uint32(math.Round(percent / 100 * float64(d.Max)))
		}
	}
	return nil
}

func serveBrightness(t *testing.T, src brightness.Source, focused string) {
	t.Helper()
	release, err := brightness.NewDaemon(src, func() (string, bool) { return focused, focused != "" }, func() float64 { return 5 }).Export(dbustest.SessionConn(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
}

func TestBrightnessCommandsAgainstTheDaemon(t *testing.T) {
	dbustest.Session(t)
	runSteps(t, []step{{[]string{"brightness", "+10"}, "Error: Brightness service not running. Start lys-shell shell first.\n"}})
	src := &fakeBacklights{devices: []brightness.Device{
		{Name: "intel_backlight", Type: brightness.TypeRaw, Brightness: 500, Max: 1000, Connector: "eDP-1"},
		{Name: "ddci2c-6", Type: brightness.TypeDDC, Brightness: 40, Max: 100, Connector: "DP-2"},
	}}
	serveBrightness(t, src, "eDP-1")
	runSteps(t, []step{
		{[]string{"brightness", "+10"}, "eDP-1: 60%\n"},
		{[]string{"brightness", "-10", "--output", "DP-2"}, "DP-2: 30%\n"},
		{[]string{"brightness", "-o", "DP-2", "-100"}, "DP-2: 5%\n"},
		{[]string{"brightness", "0"}, "eDP-1: 5%\n"},
		{[]string{"brightness", "100", "-o", "eDP-1"}, "eDP-1: 100%\n"},
		{[]string{"brightness", "50", "--output", "HDMI-A-1"}, "Error: Failed to set brightness: no backlight or DDC/CI monitor drives output HDMI-A-1 (outputs with one: DP-2, eDP-1)\n"},
	})
	devices, _ := src.Devices(t.Context())
	if devices[0].Brightness != 1000 || devices[1].Brightness != 5 {
		t.Errorf("levels after the run = %+v", devices)
	}
}

func TestBrightnessCommandNeedsAFocusedOutputOrAName(t *testing.T) {
	dbustest.Session(t)
	src := &fakeBacklights{devices: []brightness.Device{{Name: "ddci2c-6", Type: brightness.TypeDDC, Brightness: 40, Max: 100, Connector: "DP-2"}}}
	serveBrightness(t, src, "")
	runSteps(t, []step{
		{[]string{"brightness", "+5"}, "Error: Failed to set brightness: the compositor reports no focused output; name one with --output\n"},
		{[]string{"brightness", "+5", "-o", "DP-2"}, "DP-2: 45%\n"},
	})
}

type fakeAutoBrightness struct {
	resumes dbustest.Var[int]
	refuse  *dbus.Error
}

func (f *fakeAutoBrightness) Resume() *dbus.Error {
	if f.refuse != nil {
		return f.refuse
	}
	f.resumes.Update(func(n int) int { return n + 1 })
	return nil
}

func serveAutoBrightness(t *testing.T, f *fakeAutoBrightness) func() {
	t.Helper()
	release, err := dbusx.Serve(dbustest.SessionConn(t), dbusx.Service{
		Name:       autobrightness.ServiceName,
		Path:       autobrightness.ServicePath,
		Interface:  autobrightness.Interface,
		Methods:    f,
		Properties: dbusx.Getters{autobrightness.Property: func() any { return autobrightness.Paused.String() }},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	return release
}

func TestBrightnessAutoHandsTheBacklightBack(t *testing.T) {
	dbustest.Session(t)
	notRunning := "Error: automatic brightness is not running: nothing owns dev.stubbe.lys.AutoBrightness1 on the session bus\n"
	runSteps(t, []step{{[]string{"brightness", "auto"}, notRunning}})
	f := &fakeAutoBrightness{}
	release := serveAutoBrightness(t, f)
	runSteps(t, []step{
		{[]string{"brightness", "auto"}, ""},
		{[]string{"brightness", "auto", "--output", "DP-2"}, "Error: automatic brightness takes no output; drop --output DP-2\n"},
	})
	if got := f.resumes.Load(); got != 1 {
		t.Errorf("Resume reached automatic brightness %d times, want once", got)
	}
	release()
	runSteps(t, []step{{[]string{"brightness", "auto"}, notRunning}})
}

func TestBrightnessAutoReportsARefusal(t *testing.T) {
	dbustest.Session(t)
	serveAutoBrightness(t, &fakeAutoBrightness{refuse: dbusx.Failed("no ambient light sensor")})
	runSteps(t, []step{{[]string{"brightness", "auto"}, "Error: Failed to resume automatic brightness: hand the backlight back to dev.stubbe.lys.AutoBrightness1: no ambient light sensor\n"}})
}

func TestBrightnessAutoLeavesTheShellsDaemonAlone(t *testing.T) {
	dbustest.Session(t)
	src := &fakeBacklights{devices: []brightness.Device{{Name: "intel_backlight", Type: brightness.TypeRaw, Brightness: 500, Max: 1000, Connector: "eDP-1"}}}
	serveBrightness(t, src, "eDP-1")
	f := &fakeAutoBrightness{}
	serveAutoBrightness(t, f)
	runSteps(t, []step{
		{[]string{"brightness", "auto"}, ""},
		{[]string{"brightness", "+10"}, "eDP-1: 60%\n"},
	})
	if got := f.resumes.Load(); got != 1 {
		t.Errorf("Resume reached automatic brightness %d times, want once", got)
	}
}
