package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/stubbedev/lys/power/internal/daemon"
	"github.com/stubbedev/lys/power/internal/devicepower"
)

func TestStatusListsEachKnobWithItsValueAndWhy(t *testing.T) {
	inv := devicepower.Inventory{
		PCI: []devicepower.PCIDevice{{
			Address: "0000:00:14.3", Class: 0x028000, Driver: "iwlwifi",
			Control: devicepower.Attr{Path: "/sys/bus/pci/devices/0000:00:14.3/power/control", Value: devicepower.ControlOn},
		}},
		USB: []devicepower.USBDevice{{
			Name: "1-1", ID: devicepower.DeviceID{Vendor: 0x046d, Product: 0xc31c}, Interfaces: []uint8{0x03},
			Control: devicepower.Attr{Path: "/sys/bus/usb/devices/1-1/power/control", Value: devicepower.ControlOn},
		}},
		NVMe: []devicepower.NVMeController{{Name: "nvme0", APST: devicepower.APSTEnabled}},
	}
	state := daemon.DeviceState{Source: devicepower.OnBattery, Decisions: devicepower.Plan(inv, devicepower.OnBattery, nil, devicepower.Overrides{})}
	out := string(appendDevices(nil, devicepower.ModeBattery, state, nil))
	for _, want := range []string{
		"policy battery, power source battery",
		"0000:00:14.3 iwlwifi",
		"on -> auto",
		"managed",
		"1-1 046d:c31c",
		"input device, never autosuspended",
		"NVMe APST",
		"enabled",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("status lacks %q:\n%s", want, out)
		}
	}
}

func TestStatusNamesTheRivalAndTheFailure(t *testing.T) {
	out := string(appendDevices(nil, devicepower.ModeBattery, daemon.DeviceState{Source: devicepower.OnAC, Rivals: []devicepower.Rival{devicepower.TLP}}, nil))
	if !strings.Contains(out, "[TLP] active, device power is left to it") {
		t.Errorf("status with TLP:\n%s", out)
	}
	out = string(appendDevices(nil, devicepower.ModeBattery, daemon.DeviceState{}, errors.New("no system bus")))
	if !strings.Contains(out, "no system bus") {
		t.Errorf("status with an error:\n%s", out)
	}
}
