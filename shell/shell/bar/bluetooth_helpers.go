package bar

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/bluetooth"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

// btText resolves a key through the shell domain with its Fluent
// arguments as name/value pairs. An unknown key is a programming error
// and renders the loader's "No localization" marker.
func btText(key string, args ...any) string {
	var fluent []i18n.Arg
	for i := 0; i+1 < len(args); i += 2 {
		name := fmt.Sprint(args[i])
		switch v := args[i+1].(type) {
		case string:
			fluent = append(fluent, i18n.Str(name, v))
		case int:
			fluent = append(fluent, i18n.Int(name, int64(v)))
		case int64:
			fluent = append(fluent, i18n.Int(name, v))
		case uint8:
			fluent = append(fluent, i18n.Int(name, int64(v)))
		case uint16:
			fluent = append(fluent, i18n.Int(name, int64(v)))
		case uint32:
			fluent = append(fluent, i18n.Int(name, int64(v)))
		default:
			fluent = append(fluent, i18n.Str(name, fmt.Sprint(v)))
		}
	}
	return i18n.T(key, fluent...)
}

// batteryLevelIcons are a device battery's glyphs, emptiest first: the
// battery module's default ladder.
var batteryLevelIcons = config.DefaultsBattery().LevelIcons

// batteryLevelIcon is the glyph for a device's battery percentage, on
// the steps the battery module maps its icons onto.
func batteryLevelIcon(percent uint8) string {
	return batteryLevelIcons[levelIndexFloor(float64(percent), len(batteryLevelIcons))]
}

// deviceTypeKey is the Fluent key naming the device's
// kind.
func deviceTypeKey(hint *string, class *uint32) string {
	if hint != nil {
		switch *hint {
		case "audio-headphones":
			return "dropdown-bluetooth-type-headphones"
		case "audio-headset":
			return "dropdown-bluetooth-type-headset"
		case "audio-speakers":
			return "dropdown-bluetooth-type-loudspeaker"
		case "input-keyboard":
			return "dropdown-bluetooth-type-keyboard"
		case "input-mouse", "input-tablet":
			return "dropdown-bluetooth-type-mouse"
		case "input-gaming":
			return "dropdown-bluetooth-type-gamepad"
		case "video-display":
			return "dropdown-bluetooth-type-video-display"
		case "computer":
			return "dropdown-bluetooth-type-computer"
		case "phone":
			return "dropdown-bluetooth-type-phone"
		}
	}
	if class == nil {
		return "dropdown-bluetooth-type-unknown"
	}
	raw := *class
	minor := bluetooth.MinorClass(raw)
	switch bluetooth.MajorClass(raw) {
	case bluetooth.MajorComputer:
		return pick(minor, "computer", map[uint32]string{
			0x01: "desktop", 0x02: "server", 0x03: "laptop", 0x04: "handheld",
			0x05: "palm", 0x06: "wearable-computer", 0x07: "computer-tablet",
		})
	case bluetooth.MajorPhone:
		return pick(minor, "phone", map[uint32]string{
			0x01: "cellular", 0x02: "cordless", 0x03: "smartphone", 0x04: "modem",
		})
	case bluetooth.MajorNetwork:
		return "dropdown-bluetooth-type-network"
	case bluetooth.MajorAV:
		return pick(minor, "audio-video", map[uint32]string{
			0x01: "headset", 0x02: "handsfree", 0x04: "microphone", 0x05: "loudspeaker",
			0x06: "headphones", 0x07: "portable-audio", 0x08: "car-audio", 0x09: "set-top-box",
			0x0A: "hifi", 0x0B: "vcr", 0x0C: "video-camera", 0x0D: "camcorder",
			0x0E: "video-monitor", 0x0F: "video-display", 0x10: "video-conferencing", 0x12: "gaming",
		})
	case bluetooth.MajorPeripheral:
		return peripheralTypeKey(minor)
	case bluetooth.MajorImaging:
		switch {
		case minor&0x08 != 0:
			return "dropdown-bluetooth-type-printer"
		case minor&0x04 != 0:
			return "dropdown-bluetooth-type-scanner"
		case minor&0x02 != 0:
			return "dropdown-bluetooth-type-camera"
		case minor&0x01 != 0:
			return "dropdown-bluetooth-type-display"
		}
		return "dropdown-bluetooth-type-imaging"
	case bluetooth.MajorWearable:
		return pick(minor, "wearable", map[uint32]string{
			0x01: "wrist-watch", 0x02: "pager", 0x03: "jacket", 0x04: "helmet", 0x05: "glasses",
		})
	case bluetooth.MajorToy:
		return pick(minor, "toy", map[uint32]string{
			0x01: "robot", 0x02: "vehicle", 0x03: "doll", 0x04: "controller", 0x05: "game",
		})
	case bluetooth.MajorHealth:
		return "dropdown-bluetooth-type-health"
	}
	return "dropdown-bluetooth-type-unknown"
}

// pick maps a minor class through table, falling back to the major
// class's own key.
func pick(minor uint32, fallback string, table map[uint32]string) string {
	if name, ok := table[minor]; ok {
		return "dropdown-bluetooth-type-" + name
	}
	return "dropdown-bluetooth-type-" + fallback
}

func peripheralTypeKey(minor uint32) string {
	switch minor >> 4 {
	case 0x01:
		return "dropdown-bluetooth-type-keyboard"
	case 0x02:
		return "dropdown-bluetooth-type-mouse"
	case 0x03:
		return "dropdown-bluetooth-type-combo-keyboard"
	}
	return pick(minor&0x0F, "peripheral", map[uint32]string{
		0x01: "joystick", 0x02: "gamepad", 0x03: "remote", 0x04: "sensing",
		0x05: "tablet", 0x06: "card-reader",
	})
}

const btBaseUUIDSuffix = "-0000-1000-8000-00805f9b34fb"

// serviceNameKey maps a service UUID to its Fluent key via the SIG
// assigned 16-bit numbers.
func serviceNameKey(uuid string) string {
	lower := strings.ToLower(uuid)
	hexPart, ok := strings.CutPrefix(lower, "0000")
	if ok {
		hexPart, ok = strings.CutSuffix(hexPart, btBaseUUIDSuffix)
	}
	short, err := strconv.ParseUint(hexPart, 16, 16)
	if !ok || err != nil {
		return "dropdown-bluetooth-service-proprietary"
	}
	name := "unknown"
	switch s := uint16(short); {
	case s == 0x1101:
		name = "serial-port"
	case s == 0x1102:
		name = "lan-access"
	case s == 0x1103:
		name = "dialup-networking"
	case s == 0x1105:
		name = "object-push"
	case s == 0x1106:
		name = "file-transfer"
	case s == 0x1108 || s == 0x1112:
		name = "headset"
	case s == 0x110a:
		name = "audio-source"
	case s == 0x110b:
		name = "audio-sink"
	case s == 0x110c || s == 0x110e || s == 0x110f:
		name = "remote-control"
	case s == 0x110d:
		name = "audio-distribution"
	case s == 0x111e || s == 0x111f:
		name = "handsfree"
	case s >= 0x1115 && s <= 0x1117:
		name = "network-access"
	case s == 0x1124:
		name = "input-device"
	case s == 0x112d:
		name = "sim-access"
	case s == 0x112f || s == 0x1130:
		name = "phonebook"
	case s >= 0x1132 && s <= 0x1134:
		name = "messaging"
	}
	return "dropdown-bluetooth-service-" + name
}

// deviceDisplay is what the pairing card shows.
type deviceDisplay struct {
	name    string
	icon    string
	typeKey string
}

// unknownDisplay is the display for a request about a
// device the model has not seen.
var unknownDisplay = deviceDisplay{name: "-", icon: "ld-bluetooth-symbolic", typeKey: "dropdown-bluetooth-type-unknown"}

func resolveDeviceDisplay(d bluetooth.Device) deviceDisplay {
	name := d.Alias
	if name == "" {
		name = "-"
		if d.Name != nil {
			name = *d.Name
		}
	}
	return deviceDisplay{name: name, icon: bluetooth.DeviceIcon(d.Icon, d.Class), typeKey: deviceTypeKey(d.Icon, d.Class)}
}

// deviceCategory orders the rows: connected, paired, then the rest.
type deviceCategory uint8

const (
	categoryConnected deviceCategory = iota
	categoryPaired
	categoryAvailable
)

// deviceSnapshot is one row's data (DeviceSnapshot).
type deviceSnapshot struct {
	path      dbus.ObjectPath
	name      string
	icon      string
	typeKey   string
	battery   *uint8
	connected bool
	paired    bool
	category  deviceCategory
}

// categorizeDevice is nil for a device with nothing
// to call it, and for an unnamed unpaired one (an alias BlueZ derived
// from the address).
func categorizeDevice(d bluetooth.Device) *deviceSnapshot {
	category := categoryAvailable
	switch {
	case d.Connected:
		category = categoryConnected
	case d.Paired:
		category = categoryPaired
	}
	name := d.Alias
	switch {
	case name == "" && d.Name == nil:
		return nil
	case name == "":
		name = *d.Name
	case category == categoryAvailable && d.Name == nil:
		return nil
	}
	return &deviceSnapshot{
		path:      d.Path,
		name:      name,
		icon:      bluetooth.DeviceIcon(d.Icon, d.Class),
		typeKey:   deviceTypeKey(d.Icon, d.Class),
		battery:   d.BatteryPercentage,
		connected: d.Connected,
		paired:    d.Paired,
		category:  category,
	}
}

// splitDeviceLists: my devices (connected
// and paired) and the rest, each by category then name.
func splitDeviceLists(devices []bluetooth.Device) (mine, available []deviceSnapshot) {
	for _, d := range devices {
		snap := categorizeDevice(d)
		if snap == nil {
			continue
		}
		if snap.category == categoryAvailable {
			available = append(available, *snap)
		} else {
			mine = append(mine, *snap)
		}
	}
	order := func(a, b deviceSnapshot) int {
		return cmp.Or(cmp.Compare(a.category, b.category), strings.Compare(a.name, b.name))
	}
	slices.SortFunc(mine, order)
	slices.SortFunc(available, order)
	return mine, available
}
