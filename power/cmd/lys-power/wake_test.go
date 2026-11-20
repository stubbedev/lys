package main

import (
	"errors"
	"testing"

	"github.com/stubbedev/lys/power/internal/wakesource"
)

func TestStatusListsWhatMayWakeTheMachine(t *testing.T) {
	acpi := []wakesource.ACPIDevice{
		{Name: "XHC", State: "S3", Enabled: true, Nodes: []string{"pci:0000:00:14.0"}},
		{Name: "GLAN", State: "S4", Nodes: []string{"pci:0000:00:1f.6"}},
		{Name: "LID", State: "S4", Enabled: true, Nodes: []string{"platform:PNP0C0D:00"}},
	}
	usb := []wakesource.USBDevice{
		{Name: "3-2", Product: "USB Receiver", Removable: wakesource.RemovableYes, Wakeup: "enabled"},
		{Name: "3-3", Product: "Flash Drive", Removable: wakesource.RemovableYes, Wakeup: "disabled"},
		{Name: "usb3", Hub: true, Wakeup: "enabled"},
	}
	want := "wake:        USB wake policy keep (LYS_POWER_USB_WAKE=battery keeps removable USB devices from waking the machine on battery)\n" +
		"  acpi XHC      S3  pci:0000:00:14.0\n" +
		"  acpi LID      S4  platform:PNP0C0D:00\n" +
		"  usb  3-2      removable USB Receiver\n" +
		"  usb  usb3     hub       \n"
	if got := string(appendWake(nil, wakesource.ModeKeep, acpi, usb, nil)); got != want {
		t.Errorf("status prints\n%q\nwant\n%q", got, want)
	}
	want = "wake:        USB wake policy battery\n             reading /proc/acpi/wakeup: permission denied\n"
	if got := string(appendWake(nil, wakesource.ModeBattery, nil, nil, errors.New("reading /proc/acpi/wakeup: permission denied"))); got != want {
		t.Errorf("status prints %q, want %q", got, want)
	}
}
