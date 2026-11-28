package config

import (
	"testing"

	"github.com/stubbedev/lys/power/internal/devicepower"
)

func TestDevicePolicyDefaultsToBatteryWithNothingDenied(t *testing.T) {
	t.Setenv("LYS_POWER_DEVICE_POLICY", "")
	t.Setenv("LYS_POWER_DEVICE_DENY", "")
	cfg, err := Load(LoadPaths())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Devices.Mode != devicepower.ModeBattery || len(cfg.Devices.Deny) != 0 {
		t.Errorf("defaults %+v", cfg.Devices)
	}
}

func TestDevicePolicyReadsTheModeAndDenylist(t *testing.T) {
	t.Setenv("LYS_POWER_DEVICE_POLICY", "always")
	t.Setenv("LYS_POWER_DEVICE_DENY", "pci:8086:a840  usb:046d:c52b")
	cfg, err := Load(LoadPaths())
	if err != nil {
		t.Fatal(err)
	}
	want := []devicepower.Denied{
		{Bus: devicepower.BusPCI, ID: devicepower.DeviceID{Vendor: 0x8086, Product: 0xa840}},
		{Bus: devicepower.BusUSB, ID: devicepower.DeviceID{Vendor: 0x046d, Product: 0xc52b}},
	}
	if cfg.Devices.Mode != devicepower.ModeAlways || len(cfg.Devices.Deny) != 2 || cfg.Devices.Deny[0] != want[0] || cfg.Devices.Deny[1] != want[1] {
		t.Errorf("read %+v", cfg.Devices)
	}
}

func TestABadDevicePolicyFailsTheLoad(t *testing.T) {
	for env, value := range map[string]string{
		"LYS_POWER_DEVICE_POLICY": "sometimes",
		"LYS_POWER_DEVICE_DENY":   "8086:a840",
	} {
		t.Run(env, func(t *testing.T) {
			t.Setenv(env, value)
			if _, err := Load(LoadPaths()); err == nil {
				t.Errorf("%s=%s loaded", env, value)
			}
		})
	}
}
