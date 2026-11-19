package bar

import (
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/bluetooth"
)

func classOf(major, minor uint32) *uint32 {
	c := major<<8 | minor<<2
	return &c
}

func TestDeviceTypeKeyFromHintAndClass(t *testing.T) {
	for _, c := range []struct {
		hint  *string
		class *uint32
		want  string
	}{
		{nil, classOf(bluetooth.MajorPeripheral, 0x01), "joystick"},
		{nil, classOf(bluetooth.MajorPeripheral, 0x01<<4), "keyboard"},
		{nil, classOf(bluetooth.MajorAV, 0x0B), "vcr"},
		{nil, classOf(bluetooth.MajorAV, 0x10), "video-conferencing"},
		{nil, classOf(bluetooth.MajorAV, 0x02), "handsfree"},
		{nil, classOf(bluetooth.MajorAV, 0x09), "set-top-box"},
		{nil, classOf(bluetooth.MajorAV, 0x3F), "audio-video"},
		{nil, classOf(bluetooth.MajorComputer, 0), "computer"},
		{nil, classOf(bluetooth.MajorComputer, 0x03), "laptop"},
		{nil, classOf(bluetooth.MajorComputer, 0x07), "computer-tablet"},
		{nil, classOf(bluetooth.MajorPhone, 0x03), "smartphone"},
		{nil, classOf(bluetooth.MajorWearable, 0x01), "wrist-watch"},
		{nil, classOf(bluetooth.MajorWearable, 0x05), "glasses"},
		{nil, classOf(bluetooth.MajorToy, 0x01), "robot"},
		{nil, classOf(bluetooth.MajorNetwork, 0), "network"},
		{nil, classOf(bluetooth.MajorHealth, 0), "health"},
		{nil, classOf(bluetooth.MajorImaging, 0x04), "scanner"},
		{new("input-keyboard"), nil, "keyboard"},
		{nil, nil, "unknown"},
	} {
		if got := deviceTypeKey(c.hint, c.class); got != "dropdown-bluetooth-type-"+c.want {
			t.Errorf("deviceTypeKey(%v, %v) = %q, want %s", c.hint, c.class, got, c.want)
		}
		if got := btText(deviceTypeKey(c.hint, c.class)); got == deviceTypeKey(c.hint, c.class) || strings.HasPrefix(got, "No localization") {
			t.Errorf("type key %q has no text: %q", deviceTypeKey(c.hint, c.class), got)
		}
	}
}

// A device's battery glyph steps through Lucide's battery in quarters,
// as the battery module's default icons do.
func TestBatteryLevelIcon(t *testing.T) {
	for percent, want := range map[uint8]string{
		0: "ld-battery-symbolic", 24: "ld-battery-symbolic",
		25: "ld-battery-low-symbolic", 49: "ld-battery-low-symbolic",
		50: "ld-battery-medium-symbolic", 74: "ld-battery-medium-symbolic",
		75: "ld-battery-full-symbolic", 100: "ld-battery-full-symbolic", 255: "ld-battery-full-symbolic",
	} {
		if got := batteryLevelIcon(percent); got != want {
			t.Errorf("%d%% = %q, want %q", percent, got, want)
		}
	}
}

func TestServiceNameKey(t *testing.T) {
	for uuid, want := range map[string]string{
		"0000110b-0000-1000-8000-00805f9b34fb": "audio-sink",
		"0000110A-0000-1000-8000-00805F9B34FB": "audio-source",
		"0000111f-0000-1000-8000-00805f9b34fb": "handsfree",
		"00001116-0000-1000-8000-00805f9b34fb": "network-access",
		"00001133-0000-1000-8000-00805f9b34fb": "messaging",
		"00001800-0000-1000-8000-00805f9b34fb": "unknown",
		"6e400001-b5a3-f393-e0a9-e50e24dcca9e": "proprietary",
		"0000zzzz-0000-1000-8000-00805f9b34fb": "proprietary",
	} {
		if got := serviceNameKey(uuid); got != "dropdown-bluetooth-service-"+want {
			t.Errorf("%s = %q, want %s", uuid, got, want)
		}
	}
}

func TestBtText(t *testing.T) {
	// The en-US wording comes from the shared Fluent bundle, not a Go
	// table: one placeholder key and one plain key.
	if got, want := btText("dropdown-bluetooth-battery", "percent", uint8(80)), i18n.T("dropdown-bluetooth-battery", i18n.Int("percent", 80)); got != want {
		t.Errorf("battery = %q", got)
	}
	if got := btText("dropdown-bluetooth-forget"); got != i18n.T("dropdown-bluetooth-forget") {
		t.Errorf("forget = %q, want the bundle's text", got)
	}
	if got, want := btText("dropdown-bluetooth-pairing-entering", "entered", 2, "total", 6), i18n.T("dropdown-bluetooth-pairing-entering", i18n.Int("entered", 2), i18n.Int("total", 6)); got != want {
		t.Errorf("entering = %q", got)
	}
	// An unknown key renders the loader's "No localization" marker.
	if got := btText("dropdown-bluetooth-nope"); got != `No localization for id: "dropdown-bluetooth-nope"` {
		t.Errorf("unknown key = %q", got)
	}
}

func TestCategorizeDevice(t *testing.T) {
	name := new("Name")
	cases := []struct {
		d    bluetooth.Device
		want *deviceCategory
		name string
	}{
		{bluetooth.Device{Alias: "A", Name: name, Connected: true}, new(categoryConnected), "A"},
		{bluetooth.Device{Alias: "A", Name: name, Paired: true}, new(categoryPaired), "A"},
		{bluetooth.Device{Alias: "A", Name: name}, new(categoryAvailable), "A"},
		// No alias falls back to the name.
		{bluetooth.Device{Name: name}, new(categoryAvailable), "Name"},
		// An unnamed unpaired device (alias derived from the address)
		// is hidden; a paired one is kept by its alias.
		{bluetooth.Device{Alias: "AA-BB"}, nil, ""},
		{bluetooth.Device{Alias: "AA-BB", Paired: true}, new(categoryPaired), "AA-BB"},
		{bluetooth.Device{}, nil, ""},
	}
	for i, c := range cases {
		got := categorizeDevice(c.d)
		switch {
		case c.want == nil && got != nil:
			t.Errorf("case %d: got %+v, want hidden", i, got)
		case c.want != nil && (got == nil || got.category != *c.want || got.name != c.name):
			t.Errorf("case %d: got %+v, want %v %q", i, got, *c.want, c.name)
		}
	}
}

func TestSplitDeviceListsSorts(t *testing.T) {
	name := new("n")
	mine, avail := splitDeviceLists([]bluetooth.Device{
		{Alias: "Zed", Name: name, Paired: true},
		{Alias: "Bee", Name: name},
		{Alias: "Yak", Name: name, Connected: true},
		{Alias: "Ant", Name: name, Paired: true},
		{Alias: "Ape", Name: name},
	})
	var got []string
	for _, s := range mine {
		got = append(got, s.name)
	}
	if len(got) != 3 || got[0] != "Yak" || got[1] != "Ant" || got[2] != "Zed" {
		t.Errorf("mine = %v, want connected first then paired by name", got)
	}
	if len(avail) != 2 || avail[0].name != "Ape" || avail[1].name != "Bee" {
		t.Errorf("available = %+v", avail)
	}
}

func TestResolveDeviceDisplay(t *testing.T) {
	if d := resolveDeviceDisplay(bluetooth.Device{}); d.name != "-" || d.icon != "ld-bluetooth-symbolic" {
		t.Errorf("empty device display = %+v", d)
	}
	if d := resolveDeviceDisplay(bluetooth.Device{Name: new("Pods")}); d.name != "Pods" {
		t.Errorf("name fallback = %+v", d)
	}
}
