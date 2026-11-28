package wakesource

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const thinkPadWakeup = "Device\tS-state\t  Status   Sysfs node\n" +
	"GLAN\t  S4\t*disabled  pci:0000:00:1f.6\n" +
	"XHC\t  S3\t*enabled   pci:0000:00:14.0\n" +
	"XDCI\t  S4\t*disabled\n" +
	"HDAS\t  S4\t*disabled  pci:0000:00:1f.3\n" +
	"RP01\t  S4\t*enabled   pci:0000:00:1c.0\n" +
	"PXSX\t  S4\t*disabled\n" +
	"LID\t  S4\t*enabled   platform:PNP0C0D:00\n" +
	"SLPB\t  S3\t*enabled   platform:PNP0C0E:00\n"

const frameworkWakeup = "Device\tS-state\t  Status   Sysfs node\n" +
	"PEG0\t  S4\t*enabled   pci:0000:00:06.0\n" +
	"\t\t*disabled  platform:PNP0A08:00\n" +
	"TXHC\t  S4\t*enabled   pci:0000:00:0d.0\n" +
	"AWAC\t  S4\t disabled\n"

func TestParseACPIReadsRealWakeupTables(t *testing.T) {
	got, err := ParseACPI(thinkPadWakeup)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 8 {
		t.Fatalf("%d devices, want 8: %+v", len(got), got)
	}
	if want := (ACPIDevice{Name: "XHC", State: "S3", Enabled: true, Nodes: []string{"pci:0000:00:14.0"}}); !reflect.DeepEqual(got[1], want) {
		t.Errorf("XHC reads %+v, want %+v", got[1], want)
	}
	if want := (ACPIDevice{Name: "XDCI", State: "S4"}); !reflect.DeepEqual(got[2], want) {
		t.Errorf("a device without a node reads %+v, want %+v", got[2], want)
	}
	if got[6].Name != "LID" || !got[6].Enabled || got[6].Nodes[0] != "platform:PNP0C0D:00" {
		t.Errorf("the lid reads %+v", got[6])
	}

	got, err = ParseACPI(frameworkWakeup)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"pci:0000:00:06.0", "platform:PNP0A08:00"}; !reflect.DeepEqual(got[0].Nodes, want) {
		t.Errorf("a device standing for two reads nodes %v, want %v", got[0].Nodes, want)
	}
	if got[2].Name != "AWAC" || got[2].Enabled {
		t.Errorf("a device without a valid wake flag reads %+v", got[2])
	}
}

func TestParseACPIRefusesWhatIsNoWakeupTable(t *testing.T) {
	for _, bad := range []string{
		"",
		"GLAN\t  S4\t*disabled  pci:0000:00:1f.6\n",
		"Device\tS-state\t  Status   Sysfs node\n\t\t*enabled   pci:0000:00:14.0\n",
		"Device\tS-state\t  Status   Sysfs node\nXHC\t  S3\t*maybe   pci:0000:00:14.0\n",
	} {
		if got, err := ParseACPI(bad); err == nil {
			t.Errorf("ParseACPI(%q) = %+v, want an error", bad, got)
		}
	}
}

var (
	mouse    = USBDevice{Name: "3-2", Product: "USB Receiver", Removable: RemovableYes, Wakeup: "enabled", Path: "/sys/bus/usb/devices/3-2/power/wakeup"}
	keyboard = USBDevice{Name: "3-8", Product: "Internal Keyboard", Removable: RemovableFixed, Wakeup: "enabled", Path: "/sys/bus/usb/devices/3-8/power/wakeup"}
	unplaced = USBDevice{Name: "3-9", Product: "Touchpad", Removable: RemovableUnknown, Wakeup: "enabled", Path: "/sys/bus/usb/devices/3-9/power/wakeup"}
	hub      = USBDevice{Name: "usb3", Hub: true, Removable: RemovableYes, Wakeup: "enabled", Path: "/sys/bus/usb/devices/usb3/power/wakeup"}
	quiet    = USBDevice{Name: "3-3", Product: "Flash Drive", Removable: RemovableYes, Wakeup: "disabled", Path: "/sys/bus/usb/devices/3-3/power/wakeup"}
)

func TestOnBatteryOnlyRemovableDevicesLoseWakeAndOnlyWhenOptedIn(t *testing.T) {
	all := []USBDevice{mouse, keyboard, unplaced, hub, quiet}
	if got := Plan(all, ModeBattery, true); !reflect.DeepEqual(got, []Change{{Device: mouse, Want: "disabled"}}) {
		t.Errorf("Plan on battery = %+v, want only the mouse disabled", got)
	}
	if got := Plan(all, ModeBattery, false); got != nil {
		t.Errorf("Plan on AC = %+v", got)
	}
	if got := Plan(all, ModeKeep, true); got != nil {
		t.Errorf("Plan without opting in = %+v", got)
	}
}

func TestRestoreGivesWakeBackOnlyToWhatWasTaken(t *testing.T) {
	asleep := mouse
	asleep.Wakeup = "disabled"
	taken := map[string]bool{mouse.Path: true}
	if got := Restore([]USBDevice{asleep, quiet, keyboard}, taken); !reflect.DeepEqual(got, []Change{{Device: asleep, Want: "enabled"}}) {
		t.Errorf("Restore = %+v, want only the mouse enabled", got)
	}
	if got := Restore([]USBDevice{quiet}, taken); got != nil {
		t.Errorf("Restore of an unplugged mouse = %+v", got)
	}
}

func TestParseMode(t *testing.T) {
	for s, want := range map[string]Mode{"keep": ModeKeep, "battery": ModeBattery} {
		if got, err := ParseMode(s); err != nil || got != want {
			t.Errorf("ParseMode(%q) = %v, %v", s, got, err)
		}
	}
	if _, err := ParseMode("always"); err == nil || !strings.Contains(err.Error(), "keep, battery") {
		t.Errorf("ParseMode(always) = %v", err)
	}
}

func write(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, data := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestSysfsReadsTheWakeSourcesAndApplyWritesThem(t *testing.T) {
	sys, proc := t.TempDir(), t.TempDir()
	write(t, proc, map[string]string{"acpi/wakeup": thinkPadWakeup})
	write(t, sys, map[string]string{
		"bus/usb/devices/usb3/power/wakeup":  "enabled\n",
		"bus/usb/devices/usb3/bDeviceClass":  "09\n",
		"bus/usb/devices/3-2/power/wakeup":   "enabled\n",
		"bus/usb/devices/3-2/bDeviceClass":   "00\n",
		"bus/usb/devices/3-2/removable":      "removable\n",
		"bus/usb/devices/3-2/product":        "USB Receiver\n",
		"bus/usb/devices/3-2:1.0/bInterface": "03\n",
		"bus/usb/devices/3-4/power/wakeup":   "\n",
		"bus/usb/devices/3-4/bDeviceClass":   "00\n",
	})
	s := Sysfs{Root: sys, Proc: proc}
	acpi, err := s.ACPI()
	if err != nil || len(acpi) != 8 {
		t.Fatalf("ACPI = %d devices, %v", len(acpi), err)
	}
	usb, err := s.USB()
	if err != nil {
		t.Fatal(err)
	}
	devices := filepath.Join(sys, "bus/usb/devices")
	want := []USBDevice{
		{Name: "3-2", Product: "USB Receiver", Removable: RemovableYes, Wakeup: "enabled", Path: filepath.Join(devices, "3-2/power/wakeup")},
		{Name: "usb3", Hub: true, Removable: RemovableUnknown, Wakeup: "enabled", Path: filepath.Join(devices, "usb3/power/wakeup")},
	}
	if !reflect.DeepEqual(usb, want) {
		t.Fatalf("USB = %+v, want %+v", usb, want)
	}
	if err := Apply(Plan(usb, ModeBattery, true)); err != nil {
		t.Fatal(err)
	}
	if raw, err := os.ReadFile(want[0].Path); err != nil || string(raw) != "disabled" {
		t.Errorf("the receiver's wakeup reads %q, %v", raw, err)
	}
	if raw, err := os.ReadFile(want[1].Path); err != nil || string(raw) != "enabled\n" {
		t.Errorf("the hub's wakeup reads %q, %v", raw, err)
	}
	missing := Change{Device: USBDevice{Name: "9-9", Path: filepath.Join(sys, "gone/power/wakeup")}, Want: "disabled"}
	if err := Apply([]Change{missing}); err == nil || !strings.Contains(err.Error(), "9-9") {
		t.Errorf("a failed write = %v, want an error naming the device", err)
	}
}

func TestWithoutACPIOrUSBThereIsNothingToList(t *testing.T) {
	s := Sysfs{Root: t.TempDir(), Proc: t.TempDir()}
	if acpi, err := s.ACPI(); err != nil || acpi != nil {
		t.Errorf("ACPI = %v, %v", acpi, err)
	}
	if usb, err := s.USB(); err != nil || usb != nil {
		t.Errorf("USB = %v, %v", usb, err)
	}
}

func TestApplyToAnUnpluggedDeviceCreatesNothing(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "3-2/power")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	gone := Change{Device: USBDevice{Name: "3-2", Path: filepath.Join(dir, "wakeup")}, Want: "disabled"}
	if err := Apply([]Change{gone}); !errors.Is(err, fs.ErrNotExist) || !strings.Contains(err.Error(), "3-2") {
		t.Errorf("Apply to a missing attribute = %v, want ErrNotExist naming the device", err)
	}
	if _, err := os.Lstat(gone.Device.Path); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Apply created %s: %v", gone.Device.Path, err)
	}
}
