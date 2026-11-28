package devicepower

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

type fakeRadio struct {
	stations []Station
	fail     error
}

func (r *fakeRadio) Stations() ([]Station, error) { return r.stations, nil }

func (r *fakeRadio) SetPowerSave(s Station, on bool) error {
	if r.fail != nil {
		return r.fail
	}
	for i := range r.stations {
		if r.stations[i].Index == s.Index {
			r.stations[i].PowerSave = PowerSaveOff
			if on {
				r.stations[i].PowerSave = PowerSaveOn
			}
		}
	}
	return nil
}

type tree struct {
	t    *testing.T
	root string
}

func newTree(t *testing.T) tree {
	t.Helper()
	tr := tree{t: t, root: t.TempDir()}
	tr.pci("0000:00:14.3", "0x8086", "0xa840", "0x028000", "iwlwifi")
	tr.pci("0000:00:16.0", "0x8086", "0xa862", "0x078000", "mei_me")
	tr.usb("1-1", "046d", "c31c", "00", "03")
	tr.usb("1-2", "0781", "5581", "00", "08")
	tr.usb("1-3", "1234", "abcd", "00", "08")
	tr.usb("usb1", "1d6b", "0002", "09", "09")
	tr.file("module/pcie_aspm/parameters/policy", "[default] performance powersave powersupersave")
	tr.file("module/snd_hda_intel/parameters/power_save", "10")
	tr.file("class/scsi_host/host0/link_power_management_policy", "max_performance")
	tr.file("class/scsi_host/host1/proc_name", "usb-storage")
	tr.file("class/nvme/nvme0/power/pm_qos_latency_tolerance_us", "100000")
	return tr
}

func (tr tree) file(rel, content string) {
	tr.t.Helper()
	path := filepath.Join(tr.root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		tr.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content+"\n"), 0o600); err != nil {
		tr.t.Fatal(err)
	}
}

func (tr tree) read(rel string) string {
	tr.t.Helper()
	raw, err := os.ReadFile(filepath.Join(tr.root, rel))
	if err != nil {
		tr.t.Fatal(err)
	}
	return strings.TrimSpace(string(raw))
}

func (tr tree) pci(addr, vendor, device, class, driver string) {
	tr.t.Helper()
	dir := "bus/pci/devices/" + addr
	tr.file(dir+"/vendor", vendor)
	tr.file(dir+"/device", device)
	tr.file(dir+"/class", class)
	tr.file(dir+"/power/control", "on")
	if err := os.Symlink("../../../bus/pci/drivers/"+driver, filepath.Join(tr.root, dir, "driver")); err != nil {
		tr.t.Fatal(err)
	}
}

func (tr tree) usb(name, vendor, product, class string, interfaces ...string) {
	tr.t.Helper()
	dir := "bus/usb/devices/" + name
	tr.file(dir+"/idVendor", vendor)
	tr.file(dir+"/idProduct", product)
	tr.file(dir+"/bDeviceClass", class)
	tr.file(dir+"/power/control", "on")
	for i, c := range interfaces {
		tr.file(fmt.Sprintf("bus/usb/devices/%s:1.%d/bInterfaceClass", name, i), c)
	}
}

func (tr tree) kernelASPM() {
	tr.t.Helper()
	current := tr.read("module/pcie_aspm/parameters/policy")
	if strings.Contains(current, "[") {
		return
	}
	policies := []string{"default", "performance", "powersave", "powersupersave"}
	for i, p := range policies {
		if p == current {
			policies[i] = "[" + p + "]"
		}
	}
	tr.file("module/pcie_aspm/parameters/policy", strings.Join(policies, " "))
}

func (tr tree) enforce(radio Radio, source Source, rivals []Rival, ov Overrides) Applied {
	tr.t.Helper()
	applied, err := Enforce(Sysfs{Root: tr.root}, radio, source, rivals, ov)
	if err != nil {
		tr.t.Fatalf("enforcing on %s: %v", source, err)
	}
	tr.kernelASPM()
	return applied
}

type expect map[string]string

func (tr tree) check(when string, want expect) {
	tr.t.Helper()
	for rel, value := range want {
		if got := tr.read(rel); got != value {
			tr.t.Errorf("%s: %s is %q, want %q", when, rel, got, value)
		}
	}
}

func TestBatteryAndACEdgesSetAndRestoreEveryAttribute(t *testing.T) {
	tr := newTree(t)
	radio := &fakeRadio{stations: []Station{{Name: "wlan0", Index: 3, PowerSave: PowerSaveOff}}}
	untouched := expect{
		"bus/pci/devices/0000:00:16.0/power/control": "on",
		"bus/usb/devices/1-1/power/control":          "on",
		"bus/usb/devices/usb1/power/control":         "on",
	}

	applied := tr.enforce(radio, OnBattery, nil, Overrides{})
	tr.check("on battery", expect{
		"bus/pci/devices/0000:00:14.3/power/control":         "auto",
		"bus/usb/devices/1-2/power/control":                  "auto",
		"bus/usb/devices/1-3/power/control":                  "auto",
		"module/pcie_aspm/parameters/policy":                 "default performance [powersave] powersupersave",
		"module/snd_hda_intel/parameters/power_save":         "1",
		"class/scsi_host/host0/link_power_management_policy": "med_power_with_dipm",
	})
	tr.check("on battery", untouched)
	if radio.stations[0].PowerSave != PowerSaveOn {
		t.Errorf("on battery: wlan0 power save %q, want on", radio.stations[0].PowerSave)
	}
	if len(applied.Written) != 7 {
		t.Errorf("on battery: wrote %d knobs, want 7: %+v", len(applied.Written), applied.Written)
	}

	tr.enforce(radio, OnAC, nil, Overrides{})
	tr.check("back on AC", expect{
		"bus/pci/devices/0000:00:14.3/power/control":         "on",
		"bus/usb/devices/1-2/power/control":                  "on",
		"bus/usb/devices/1-3/power/control":                  "on",
		"module/pcie_aspm/parameters/policy":                 "[default] performance powersave powersupersave",
		"module/snd_hda_intel/parameters/power_save":         "10",
		"class/scsi_host/host0/link_power_management_policy": "max_performance",
	})
	tr.check("back on AC", untouched)
	if radio.stations[0].PowerSave != PowerSaveOff {
		t.Errorf("back on AC: wlan0 power save %q, want off", radio.stations[0].PowerSave)
	}
}

func TestEnforcingTwiceWritesNothingTheSecondTime(t *testing.T) {
	tr := newTree(t)
	radio := &fakeRadio{stations: []Station{{Name: "wlan0", Index: 3, PowerSave: PowerSaveOff}}}
	tr.enforce(radio, OnBattery, nil, Overrides{})
	if again := tr.enforce(radio, OnBattery, nil, Overrides{}); len(again.Written) != 0 {
		t.Errorf("the second battery edge rewrote %+v", again.Written)
	}
}

func TestDenylistedDevicesKeepTheirSysfsValues(t *testing.T) {
	tr := newTree(t)
	ov := Overrides{Deny: []Denied{
		{Bus: BusPCI, ID: DeviceID{Vendor: 0x8086, Product: 0xa840}},
		{Bus: BusUSB, ID: DeviceID{Vendor: 0x0781, Product: 0x5581}},
	}}
	tr.enforce(nil, OnBattery, nil, ov)
	tr.check("denylisted on battery", expect{
		"bus/pci/devices/0000:00:14.3/power/control": "on",
		"bus/usb/devices/1-2/power/control":          "on",
		"bus/usb/devices/1-3/power/control":          "auto",
	})
}

func TestAnActiveTLPLeavesEveryAttributeAlone(t *testing.T) {
	tr := newTree(t)
	radio := &fakeRadio{stations: []Station{{Name: "wlan0", Index: 3, PowerSave: PowerSaveOff}}}
	applied := tr.enforce(radio, OnBattery, []Rival{TLP}, Overrides{})
	if len(applied.Written) != 0 {
		t.Errorf("wrote %+v with TLP active", applied.Written)
	}
	tr.check("TLP active", expect{
		"bus/pci/devices/0000:00:14.3/power/control":         "on",
		"bus/usb/devices/1-2/power/control":                  "on",
		"module/pcie_aspm/parameters/policy":                 "[default] performance powersave powersupersave",
		"module/snd_hda_intel/parameters/power_save":         "10",
		"class/scsi_host/host0/link_power_management_policy": "max_performance",
	})
	if radio.stations[0].PowerSave != PowerSaveOff {
		t.Error("TLP active: Wi-Fi power save changed")
	}
}

func TestTheInventoryReadsWhatTheTreeHolds(t *testing.T) {
	tr := newTree(t)
	inv, err := Sysfs{Root: tr.root}.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.PCI) != 2 || inv.PCI[0].Driver != "iwlwifi" || inv.PCI[0].Class != 0x028000 || inv.PCI[0].ID != (DeviceID{0x8086, 0xa840}) {
		t.Errorf("PCI %+v", inv.PCI)
	}
	if len(inv.USB) != 4 || inv.USB[0].Name != "1-1" || len(inv.USB[0].Interfaces) != 1 || inv.USB[0].Interfaces[0] != 0x03 {
		t.Errorf("USB %+v", inv.USB)
	}
	if len(inv.SATA) != 1 || inv.SATA[0].Name != "host0" {
		t.Errorf("SATA hosts %+v, want host0 alone", inv.SATA)
	}
	if len(inv.ASPM) != 1 || inv.ASPM[0].Value != ASPMDefault {
		t.Errorf("ASPM %+v", inv.ASPM)
	}
	if len(inv.NVMe) != 1 || inv.NVMe[0].APST != APSTEnabled {
		t.Errorf("NVMe %+v", inv.NVMe)
	}
}

func TestAMachineWithoutTheModulesHasNoSuchKnobs(t *testing.T) {
	inv, err := Sysfs{Root: t.TempDir()}.Read()
	if err != nil {
		t.Fatal(err)
	}
	if ds := Plan(inv, OnBattery, nil, Overrides{}); len(ds) != 0 {
		t.Errorf("an empty sysfs planned %+v", ds)
	}
}

func TestAnUnreadableAttributeNamesItsPath(t *testing.T) {
	tr := newTree(t)
	if err := os.Remove(filepath.Join(tr.root, "bus/pci/devices/0000:00:14.3/class")); err != nil {
		t.Fatal(err)
	}
	_, err := Sysfs{Root: tr.root}.Read()
	if err == nil || !strings.Contains(err.Error(), "0000:00:14.3/class") {
		t.Errorf("reading without a class file: %v, want an error naming it", err)
	}
}

func TestAWriteFailureIsReportedWithItsPath(t *testing.T) {
	tr := newTree(t)
	control := filepath.Join(tr.root, "bus/usb/devices/1-2/power/control")
	if err := os.Remove(control); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(control, 0o700); err != nil {
		t.Fatal(err)
	}
	ds := []Decision{
		{Kind: USBAutosuspend, Target: File(control), Current: ControlOn, Want: ControlAuto},
		{Kind: AudioPowerSave, Target: File(filepath.Join(tr.root, "module/snd_hda_intel/parameters/power_save")), Current: AudioIdleAC, Want: AudioIdleBattery},
	}
	applied, err := Apply(ds, nil)
	if err == nil || !strings.Contains(err.Error(), control) {
		t.Errorf("writing into a directory: %v, want an error naming %s", err, control)
	}
	if len(applied.Written) != 1 || applied.Written[0].Kind != AudioPowerSave {
		t.Errorf("one failure stopped the other writes: %+v", applied.Written)
	}
}

func TestAWiFiFailureNamesTheInterface(t *testing.T) {
	radio := &fakeRadio{stations: []Station{{Name: "wlan0", Index: 3}}, fail: syscall.EOPNOTSUPP}
	_, err := Apply([]Decision{{Kind: WiFiPowerSave, Target: radio.stations[0], Current: PowerSaveOff, Want: PowerSaveOn}}, radio)
	if !errors.Is(err, syscall.EOPNOTSUPP) || !strings.Contains(err.Error(), "wlan0") {
		t.Errorf("a refused power save: %v, want EOPNOTSUPP naming wlan0", err)
	}
}
