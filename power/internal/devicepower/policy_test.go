package devicepower

import (
	"slices"
	"testing"
)

func wifiCard() PCIDevice {
	return PCIDevice{
		Address: "0000:00:14.3",
		ID:      DeviceID{Vendor: 0x8086, Product: 0xa840},
		Class:   0x028000,
		Driver:  "iwlwifi",
		Control: Attr{Path: "/sys/bus/pci/devices/0000:00:14.3/power/control", Value: ControlOn},
	}
}

func usbDevice(name string, product uint16, interfaces ...uint8) USBDevice {
	return USBDevice{
		Name:       name,
		ID:         DeviceID{Vendor: 0x046d, Product: product},
		Interfaces: interfaces,
		Control:    Attr{Path: "/sys/bus/usb/devices/" + name + "/power/control", Value: ControlOn},
	}
}

func only(t *testing.T, ds []Decision) Decision {
	t.Helper()
	if len(ds) != 1 {
		t.Fatalf("got %d decisions, want 1: %+v", len(ds), ds)
	}
	return ds[0]
}

func TestTheTableGivesEveryKnobItsBatteryAndACValue(t *testing.T) {
	inv := Inventory{
		PCI:      []PCIDevice{wifiCard()},
		USB:      []USBDevice{usbDevice("1-3", 0x0001, 0x08)},
		Stations: []Station{{Name: "wlan0", Index: 3, PowerSave: PowerSaveOff}},
		ASPM:     []Attr{{Path: "aspm", Value: ASPMDefault}},
		SATA:     []SATAHost{{Name: "host0", Policy: Attr{Path: "lpm", Value: LinkMaxPerformance}}},
		Audio:    []Attr{{Path: "power_save", Value: AudioIdleAC}},
	}
	wantBattery := map[Kind]Value{
		RuntimePM: ControlAuto, USBAutosuspend: ControlAuto, WiFiPowerSave: PowerSaveOn,
		ASPMPolicy: ASPMPowersave, SATALinkPower: LinkMedPowerDIPM, AudioPowerSave: AudioIdleBattery,
	}
	wantAC := map[Kind]Value{
		RuntimePM: ControlOn, USBAutosuspend: ControlOn, WiFiPowerSave: PowerSaveOff,
		ASPMPolicy: ASPMDefault, SATALinkPower: LinkMaxPerformance, AudioPowerSave: AudioIdleAC,
	}
	for source, want := range map[Source]map[Kind]Value{OnBattery: wantBattery, OnAC: wantAC} {
		ds := Plan(inv, source, nil, Overrides{})
		if len(ds) != len(want) {
			t.Fatalf("on %s: %d decisions for %d knobs", source, len(ds), len(want))
		}
		for _, d := range ds {
			if d.Reason != Managed || d.Want != want[d.Kind] {
				t.Errorf("on %s: %s of %s wants %q (%s), want %q", source, d.Kind, d.Subject, d.Want, d.Reason, want[d.Kind])
			}
		}
	}
}

func TestAlwaysKeepsDevicesAsleepOnAC(t *testing.T) {
	inv := Inventory{
		PCI:   []PCIDevice{wifiCard()},
		USB:   []USBDevice{usbDevice("1-3", 0x0001, 0x08)},
		Audio: []Attr{{Path: "power_save", Value: AudioIdleBattery}},
	}
	for _, d := range Plan(inv, OnAC, nil, Overrides{Mode: ModeAlways}) {
		want := ControlAuto
		if d.Kind == AudioPowerSave {
			want = AudioIdleAC
		}
		if d.Want != want {
			t.Errorf("always on AC: %s wants %q, want %q", d.Kind, d.Want, want)
		}
	}
}

func TestInputAudioAndExcludedUSBDevicesAreNeverAutosuspended(t *testing.T) {
	for _, tc := range []struct {
		name   string
		device USBDevice
		want   Reason
	}{
		{"keyboard", usbDevice("1-1", 0xc31c, 0x03), InputDevice},
		{"receiver with a vendor interface", usbDevice("1-2", 0xc52b, 0xff, 0x03), InputDevice},
		{"headset with a HID interface", usbDevice("1-4", 0x0a8f, 0x01, 0x01, 0x03), InputDevice},
		{"sound card", usbDevice("1-5", 0x0a44, 0x01, 0x01), AudioDevice},
		{"bluetooth", usbDevice("1-6", 0x0001, 0xe0, 0xe0), ExcludedClass},
		{"modem", usbDevice("1-7", 0x0002, 0x02, 0x0a), ExcludedClass},
		{"printer", usbDevice("1-8", 0x0003, 0x07), ExcludedClass},
		{"unconfigured", usbDevice("1-9", 0x0004), Unconfigured},
		{"hub", USBDevice{Name: "usb1", Class: usbClassHub, Interfaces: []uint8{0x09}}, Hub},
	} {
		d := only(t, Plan(Inventory{USB: []USBDevice{tc.device}}, OnBattery, nil, Overrides{}))
		if d.Reason != tc.want || d.Pending() {
			t.Errorf("%s: reason %q pending %v, want %q and untouched", tc.name, d.Reason, d.Pending(), tc.want)
		}
	}
}

func TestRuntimePMOnlyForAllowlistedDrivers(t *testing.T) {
	unbound := wifiCard()
	unbound.Driver = ""
	mei := wifiCard()
	mei.Class, mei.Driver = 0x078000, "mei_me"
	wrongClass := wifiCard()
	wrongClass.Class = 0x020000
	for _, tc := range []struct {
		name   string
		device PCIDevice
		want   Reason
	}{
		{"wifi", wifiCard(), Managed},
		{"unbound", unbound, Unbound},
		{"mei", mei, NotAllowlisted},
		{"allowlisted driver in another class", wrongClass, NotAllowlisted},
	} {
		d := only(t, Plan(Inventory{PCI: []PCIDevice{tc.device}}, OnBattery, nil, Overrides{}))
		if d.Reason != tc.want {
			t.Errorf("%s: reason %q, want %q", tc.name, d.Reason, tc.want)
		}
	}
}

func TestDenylistedDevicesAreUntouched(t *testing.T) {
	inv := Inventory{PCI: []PCIDevice{wifiCard()}, USB: []USBDevice{usbDevice("1-3", 0x0001, 0x08)}}
	ov := Overrides{Deny: []Denied{
		{Bus: BusPCI, ID: DeviceID{Vendor: 0x8086, Product: 0xa840}},
		{Bus: BusUSB, ID: DeviceID{Vendor: 0x046d, Product: 0x0001}},
	}}
	for _, d := range Plan(inv, OnBattery, nil, ov) {
		if d.Reason != Denylisted || d.Pending() {
			t.Errorf("%s: reason %q pending %v, want denylisted and untouched", d.Subject, d.Reason, d.Pending())
		}
	}
}

func TestADenyEntryNamesItsBus(t *testing.T) {
	ov := Overrides{Deny: []Denied{{Bus: BusUSB, ID: DeviceID{Vendor: 0x8086, Product: 0xa840}}}}
	if d := only(t, Plan(Inventory{PCI: []PCIDevice{wifiCard()}}, OnBattery, nil, ov)); d.Reason != Managed {
		t.Errorf("a USB deny entry stopped a PCI device with the same ID: %q", d.Reason)
	}
}

func TestGlobalConditionsStopEveryWrite(t *testing.T) {
	inv := Inventory{
		PCI:   []PCIDevice{wifiCard()},
		ASPM:  []Attr{{Path: "aspm", Value: ASPMDefault}},
		Audio: []Attr{{Path: "power_save", Value: AudioIdleAC}},
	}
	for _, tc := range []struct {
		name   string
		source Source
		rivals []Rival
		ov     Overrides
		want   Reason
	}{
		{"off", OnBattery, nil, Overrides{Mode: ModeOff}, Disabled},
		{"TLP", OnBattery, []Rival{TLP}, Overrides{}, RivalActive},
		{"no battery", NoBattery, nil, Overrides{}, WithoutBattery},
	} {
		for _, d := range Plan(inv, tc.source, tc.rivals, tc.ov) {
			if d.Reason != tc.want || d.Pending() {
				t.Errorf("%s: %s reason %q pending %v, want %q", tc.name, d.Kind, d.Reason, d.Pending(), tc.want)
			}
		}
	}
}

func TestNVMeAPSTIsReportedNeverWritten(t *testing.T) {
	inv := Inventory{NVMe: []NVMeController{{"nvme0", APSTEnabled}, {"nvme1", APSTDisabled}, {"nvme2", APSTUnsupported}}}
	got := Plan(inv, OnBattery, nil, Overrides{})
	want := []Reason{APSTOn, APSTOff, APSTAbsent}
	for i, d := range got {
		if d.Reason != want[i] || d.Pending() {
			t.Errorf("%s: reason %q pending %v, want %q", d.Subject, d.Reason, d.Pending(), want[i])
		}
	}
}

func TestAKnobAlreadyInPlaceIsNotPending(t *testing.T) {
	card := wifiCard()
	card.Control.Value = ControlAuto
	if d := only(t, Plan(Inventory{PCI: []PCIDevice{card}}, OnBattery, nil, Overrides{})); d.Pending() {
		t.Errorf("a device already at auto is pending: %+v", d)
	}
}

func TestParseDenied(t *testing.T) {
	got, err := ParseDenied("usb:046d:C52B")
	if err != nil || got != (Denied{Bus: BusUSB, ID: DeviceID{Vendor: 0x046d, Product: 0xc52b}}) {
		t.Errorf("usb:046d:C52B parsed to %v, %v", got, err)
	}
	for _, bad := range []string{"046d:c52b", "sdio:046d:c52b", "pci:8086", "pci:xyz:0001", "pci:8086:10000"} {
		if _, err := ParseDenied(bad); err == nil {
			t.Errorf("%q parsed", bad)
		}
	}
}

func TestParseMode(t *testing.T) {
	for _, m := range []Mode{ModeBattery, ModeAlways, ModeOff} {
		if got, err := ParseMode(m.String()); err != nil || got != m {
			t.Errorf("%s parsed to %v, %v", m, got, err)
		}
	}
	if _, err := ParseMode("sometimes"); err == nil {
		t.Error("an unknown mode parsed")
	}
}

func TestRivalsAmongActiveUnits(t *testing.T) {
	for _, tc := range []struct {
		active []string
		want   []Rival
	}{
		{nil, nil},
		{[]string{"tlp.service"}, []Rival{TLP}},
		{[]string{"auto-cpufreq.service", "tuned.service"}, []Rival{AutoCpufreq, Tuned}},
		{[]string{"tuned.service", "tuned-ppd.service"}, nil},
	} {
		if got := RivalsAmong(tc.active); !slices.Equal(got, tc.want) {
			t.Errorf("active %v: rivals %v, want %v", tc.active, got, tc.want)
		}
	}
}
