// Package devicepower powers down idle devices on battery: PCI runtime
// power management, USB autosuspend, Wi-Fi power save, the PCIe ASPM
// policy, SATA link power management and HD audio power save, and confirms
// NVMe APST. The policy is a pure function of what the machine has; reading
// and writing sysfs and nl80211 happen at the edges.
package devicepower

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/stubbedev/lys/lib/intconv"
)

type Source uint8

const (
	NoBattery Source = iota
	OnAC
	OnBattery
)

func (s Source) String() string {
	switch s {
	case OnAC:
		return "AC"
	case OnBattery:
		return "battery"
	default:
		return "no battery"
	}
}

type Mode uint8

const (
	ModeBattery Mode = iota
	ModeAlways
	ModeOff
)

var modeNames = [...]string{ModeBattery: "battery", ModeAlways: "always", ModeOff: "off"}

func (m Mode) String() string { return modeNames[m] }

func ParseMode(s string) (Mode, error) {
	i := slices.Index(modeNames[:], s)
	if i < 0 {
		return 0, fmt.Errorf("device power policy %q is none of %s", s, strings.Join(modeNames[:], ", "))
	}
	return intconv.Must[Mode](i), nil
}

type Bus uint8

const (
	BusPCI Bus = iota
	BusUSB
)

var busNames = [...]string{BusPCI: "pci", BusUSB: "usb"}

func (b Bus) String() string { return busNames[b] }

type DeviceID struct {
	Vendor, Product uint16
}

func (id DeviceID) String() string { return fmt.Sprintf("%04x:%04x", id.Vendor, id.Product) }

type Denied struct {
	Bus Bus
	ID  DeviceID
}

func (d Denied) String() string { return d.Bus.String() + ":" + d.ID.String() }

func ParseDenied(s string) (Denied, error) {
	bus, id, ok := strings.Cut(s, ":")
	b := slices.Index(busNames[:], bus)
	if !ok || b < 0 {
		return Denied{}, fmt.Errorf("denied device %q does not name its bus, pci or usb", s)
	}
	vendor, product, ok := strings.Cut(id, ":")
	if !ok {
		return Denied{}, fmt.Errorf("denied device %q is not bus:vendor:product", s)
	}
	v, err := strconv.ParseUint(vendor, 16, 16)
	if err != nil {
		return Denied{}, fmt.Errorf("denied device %q has vendor %q, not four hex digits: %w", s, vendor, err)
	}
	p, err := strconv.ParseUint(product, 16, 16)
	if err != nil {
		return Denied{}, fmt.Errorf("denied device %q has product %q, not four hex digits: %w", s, product, err)
	}
	return Denied{Bus: intconv.Must[Bus](b), ID: DeviceID{Vendor: uint16(v), Product: uint16(p)}}, nil
}

type Overrides struct {
	Mode Mode
	Deny []Denied
}

func (o Overrides) denies(bus Bus, id DeviceID) bool {
	return slices.Contains(o.Deny, Denied{Bus: bus, ID: id})
}

type Kind uint8

const (
	RuntimePM Kind = iota
	USBAutosuspend
	WiFiPowerSave
	ASPMPolicy
	SATALinkPower
	AudioPowerSave
	NVMeAPST
)

var kindNames = [...]string{
	RuntimePM:      "runtime PM",
	USBAutosuspend: "USB autosuspend",
	WiFiPowerSave:  "Wi-Fi power save",
	ASPMPolicy:     "PCIe ASPM",
	SATALinkPower:  "SATA link power",
	AudioPowerSave: "audio power save",
	NVMeAPST:       "NVMe APST",
}

func (k Kind) String() string { return kindNames[k] }

type Value string

const (
	ControlOn          Value = "on"
	ControlAuto        Value = "auto"
	PowerSaveOn        Value = "on"
	PowerSaveOff       Value = "off"
	ASPMDefault        Value = "default"
	ASPMPowersave      Value = "powersave"
	LinkMaxPerformance Value = "max_performance"
	LinkMedPowerDIPM   Value = "med_power_with_dipm"
	AudioIdleAC        Value = "10"
	AudioIdleBattery   Value = "1"
)

type tunable struct {
	ac, battery Value
	sleepsOnAC  bool
}

var tunables = [...]tunable{
	RuntimePM:      {ac: ControlOn, battery: ControlAuto, sleepsOnAC: true},
	USBAutosuspend: {ac: ControlOn, battery: ControlAuto, sleepsOnAC: true},
	WiFiPowerSave:  {ac: PowerSaveOff, battery: PowerSaveOn},
	ASPMPolicy:     {ac: ASPMDefault, battery: ASPMPowersave},
	SATALinkPower:  {ac: LinkMaxPerformance, battery: LinkMedPowerDIPM},
	AudioPowerSave: {ac: AudioIdleAC, battery: AudioIdleBattery},
}

func want(kind Kind, source Source, mode Mode) Value {
	t := tunables[kind]
	if source == OnBattery || (mode == ModeAlways && t.sleepsOnAC) {
		return t.battery
	}
	return t.ac
}

type pciDriver struct {
	class  uint32
	driver string
}

var runtimePMDrivers = [...]pciDriver{
	{0x0280, "iwlwifi"},
	{0x0280, "ath11k_pci"},
	{0x0280, "ath12k_pci"},
	{0x0280, "mt7921e"},
	{0x0280, "mt7925e"},
	{0x0d11, "btintel_pcie"},
	{0x0108, "nvme"},
	{0x0c05, "i801_smbus"},
}

const (
	usbClassAudio    = 0x01
	usbClassComm     = 0x02
	usbClassHID      = 0x03
	usbClassPrinter  = 0x07
	usbClassHub      = 0x09
	usbClassCDCData  = 0x0a
	usbClassWireless = 0xe0
)

type Attr struct {
	Path  string
	Value Value
}

type PCIDevice struct {
	Address string
	ID      DeviceID
	Class   uint32
	Driver  string
	Control Attr
}

type USBDevice struct {
	Name       string
	ID         DeviceID
	Class      uint8
	Interfaces []uint8
	Control    Attr
}

type Station struct {
	Name      string
	Index     int
	PowerSave Value
}

type SATAHost struct {
	Name   string
	Policy Attr
}

type APST uint8

const (
	APSTEnabled APST = iota
	APSTDisabled
	APSTUnsupported
)

type NVMeController struct {
	Name string
	APST APST
}

type Inventory struct {
	PCI      []PCIDevice
	USB      []USBDevice
	Stations []Station
	ASPM     []Attr
	SATA     []SATAHost
	Audio    []Attr
	NVMe     []NVMeController
}

type Rival uint8

const (
	TLP Rival = iota
	AutoCpufreq
	Tuned
)

var rivalNames = [...]string{TLP: "TLP", AutoCpufreq: "auto-cpufreq", Tuned: "tuned"}

func (r Rival) String() string { return rivalNames[r] }

type Reason uint8

const (
	Managed Reason = iota
	Disabled
	RivalActive
	WithoutBattery
	Unbound
	NotAllowlisted
	Denylisted
	InputDevice
	AudioDevice
	ExcludedClass
	Hub
	Unconfigured
	APSTOn
	APSTOff
	APSTAbsent
)

var reasonNames = [...]string{
	Managed:        "managed",
	Disabled:       "device power policy is off",
	RivalActive:    "another power manager is active",
	WithoutBattery: "no battery",
	Unbound:        "no driver bound",
	NotAllowlisted: "driver not on the runtime PM allowlist",
	Denylisted:     "denylisted",
	InputDevice:    "input device, never autosuspended",
	AudioDevice:    "audio device, never autosuspended",
	ExcludedClass:  "printer, Bluetooth, modem or serial device, never autosuspended",
	Hub:            "hub, autosuspended by the kernel",
	Unconfigured:   "no active configuration",
	APSTOn:         "enabled",
	APSTOff:        "disabled (nvme_core.default_ps_max_latency_us=0)",
	APSTAbsent:     "not supported by the controller",
}

func (r Reason) String() string { return reasonNames[r] }

type Target interface {
	fmt.Stringer
	target()
}

type File string

func (f File) String() string { return string(f) }
func (File) target()          {}

func (s Station) String() string { return s.Name }
func (Station) target()          {}

type Decision struct {
	Kind    Kind
	Subject string
	Target  Target
	Current Value
	Want    Value
	Reason  Reason
}

func (d Decision) Pending() bool { return d.Reason == Managed && d.Current != d.Want }

func Plan(inv Inventory, source Source, rivals []Rival, ov Overrides) []Decision {
	var out []Decision
	add := func(kind Kind, subject string, target Target, current Value, reason Reason) {
		d := Decision{Kind: kind, Subject: subject, Target: target, Current: current, Reason: reason}
		if reason == Managed {
			switch {
			case ov.Mode == ModeOff:
				d.Reason = Disabled
			case len(rivals) > 0:
				d.Reason = RivalActive
			case source == NoBattery:
				d.Reason = WithoutBattery
			default:
				d.Want = want(kind, source, ov.Mode)
			}
		}
		out = append(out, d)
	}
	for _, d := range inv.PCI {
		add(RuntimePM, d.Address+" "+d.Driver, File(d.Control.Path), d.Control.Value, pciReason(d, ov))
	}
	for _, d := range inv.USB {
		add(USBAutosuspend, d.Name+" "+d.ID.String(), File(d.Control.Path), d.Control.Value, usbReason(d, ov))
	}
	for _, s := range inv.Stations {
		add(WiFiPowerSave, s.Name, s, s.PowerSave, Managed)
	}
	for _, a := range inv.ASPM {
		add(ASPMPolicy, "pcie_aspm", File(a.Path), a.Value, Managed)
	}
	for _, h := range inv.SATA {
		add(SATALinkPower, h.Name, File(h.Policy.Path), h.Policy.Value, Managed)
	}
	for _, a := range inv.Audio {
		add(AudioPowerSave, "snd_hda_intel", File(a.Path), a.Value, Managed)
	}
	for _, c := range inv.NVMe {
		out = append(out, Decision{Kind: NVMeAPST, Subject: c.Name, Reason: apstReasons[c.APST]})
	}
	return out
}

var apstReasons = [...]Reason{APSTEnabled: APSTOn, APSTDisabled: APSTOff, APSTUnsupported: APSTAbsent}

func pciReason(d PCIDevice, ov Overrides) Reason {
	switch {
	case ov.denies(BusPCI, d.ID):
		return Denylisted
	case d.Driver == "":
		return Unbound
	case !slices.Contains(runtimePMDrivers[:], pciDriver{class: d.Class >> 8, driver: d.Driver}):
		return NotAllowlisted
	default:
		return Managed
	}
}

func usbReason(d USBDevice, ov Overrides) Reason {
	classes := append([]uint8{d.Class}, d.Interfaces...)
	switch {
	case ov.denies(BusUSB, d.ID):
		return Denylisted
	case slices.Contains(classes, usbClassHID):
		return InputDevice
	case slices.Contains(classes, usbClassAudio):
		return AudioDevice
	case d.Class == usbClassHub:
		return Hub
	case slices.ContainsFunc(classes, excludedUSBClass):
		return ExcludedClass
	case len(d.Interfaces) == 0:
		return Unconfigured
	default:
		return Managed
	}
}

func excludedUSBClass(c uint8) bool {
	switch c {
	case usbClassPrinter, usbClassWireless, usbClassComm, usbClassCDCData:
		return true
	default:
		return false
	}
}
