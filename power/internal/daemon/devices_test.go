package daemon_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stubbedev/lys/power/internal/daemon"
	"github.com/stubbedev/lys/power/internal/devicepower"
	"github.com/stubbedev/lys/power/internal/supply"
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

func TestDevicePowerWithoutTheBusLeavesEveryKnobAlone(t *testing.T) {
	f := newFixture(t)
	path := f.audio(t)
	err := daemon.ApplyDevices(context.Background(), f.cfg, nil, f.log, devicepower.OnBattery)
	if !errors.Is(err, daemon.ErrNoBus) {
		t.Errorf("without the system bus: %v, want an error saying TLP cannot be ruled out", err)
	}
	unchanged(t, path)
}
