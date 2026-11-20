package daemon_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stubbedev/lys/power/internal/daemon"
	"github.com/stubbedev/lys/power/internal/devicepower"
	"github.com/stubbedev/lys/power/internal/supply"
	"github.com/stubbedev/lys/power/internal/wakesource"
)

func (f *fixture) deviceSource(t *testing.T) devicepower.Source {
	t.Helper()
	source, err := daemon.DeviceSource(supply.Supply{Dir: f.cfg.SupplyDir})
	if err != nil {
		t.Fatal(err)
	}
	return source
}

func TestDevicePowerFollowsTheBattery(t *testing.T) {
	f := newFixture(t)
	if got := f.deviceSource(t); got != devicepower.OnAC {
		t.Errorf("plugged in: %s, want AC", got)
	}
	f.unplug(t)
	if got := f.deviceSource(t); got != devicepower.OnBattery {
		t.Errorf("unplugged: %s, want battery", got)
	}
	if err := os.RemoveAll(filepath.Join(f.root, "ps", "BAT0")); err != nil {
		t.Fatal(err)
	}
	if got := f.deviceSource(t); got != devicepower.NoBattery {
		t.Errorf("without a battery: %s, want no battery", got)
	}
}

func (f *fixture) audio(t *testing.T) string {
	t.Helper()
	path := filepath.Join(f.cfg.SysDir, "module/snd_hda_intel/parameters/power_save")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("10"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func unchanged(t *testing.T, path string) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "10" {
		t.Errorf("%s became %q", path, raw)
	}
}

func TestDevicePowerNeedsNothingWithoutABatteryOrWhenOff(t *testing.T) {
	f := newFixture(t)
	path := f.audio(t)
	if err := daemon.ApplyDevices(context.Background(), f.cfg, nil, f.log, devicepower.NoBattery); err != nil {
		t.Errorf("without a battery: %v", err)
	}
	f.cfg.Devices.Mode = devicepower.ModeOff
	if err := daemon.ApplyDevices(context.Background(), f.cfg, nil, f.log, devicepower.OnBattery); err != nil {
		t.Errorf("turned off: %v", err)
	}
	unchanged(t, path)
}

func (f *fixture) usbDevice(t *testing.T, name, removable string) string {
	t.Helper()
	dev := filepath.Join(f.cfg.SysDir, "bus/usb/devices", name)
	for file, content := range map[string]string{"power/wakeup": "enabled\n", "bDeviceClass": "00\n", "removable": removable + "\n"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(dev, file)), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dev, file), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(dev, "power/wakeup")
}

func wakeup(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(raw))
}

func TestUSBWakeIsTakenOnBatteryAndGivenBackOnACOnlyWhenOptedIn(t *testing.T) {
	f := newFixture(t)
	mouse := f.usbDevice(t, "3-2", "removable")
	keyboard := f.usbDevice(t, "3-8", "fixed")
	if changes, err := daemon.NewUSBWake(f.cfg).Follow(devicepower.OnBattery); err != nil || changes != nil || wakeup(t, mouse) != "enabled" {
		t.Fatalf("without opting in: %v, %v; the mouse wakes %s", changes, err, wakeup(t, mouse))
	}
	f.cfg.USBWake = wakesource.ModeBattery
	w := daemon.NewUSBWake(f.cfg)
	if _, err := w.Follow(devicepower.OnBattery); err != nil {
		t.Fatal(err)
	}
	if wakeup(t, mouse) != "disabled" || wakeup(t, keyboard) != "enabled" {
		t.Errorf("on battery the mouse wakes %s and the built-in keyboard %s", wakeup(t, mouse), wakeup(t, keyboard))
	}
	if _, err := w.Follow(devicepower.OnAC); err != nil {
		t.Fatal(err)
	}
	if wakeup(t, mouse) != "enabled" {
		t.Errorf("on AC the mouse wakes %s, want enabled again", wakeup(t, mouse))
	}
	quiet := f.usbDevice(t, "3-3", "removable")
	if err := os.WriteFile(quiet, []byte("disabled"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Follow(devicepower.OnAC); err != nil || wakeup(t, quiet) != "disabled" {
		t.Errorf("on AC a device lys-power never touched wakes %s, %v", wakeup(t, quiet), err)
	}
}

func TestDevicePowerWithoutTheBusLeavesEveryKnobAlone(t *testing.T) {
	f := newFixture(t)
	path := f.audio(t)
	err := daemon.ApplyDevices(context.Background(), f.cfg, nil, f.log, devicepower.OnBattery)
	if !errors.Is(err, daemon.ErrNoBus) {
		t.Errorf("without the system bus: %v, want an error saying TLP cannot be ruled out", err)
	}
	unchanged(t, path)
}
