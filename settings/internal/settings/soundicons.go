package settings

import (
	"cmp"
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/service/pulse"
	"github.com/stubbedev/lys/ui/widget"
)

var formFactorIcons = map[string]string{
	"headphone":  "ld-headphones-symbolic",
	"headset":    "ld-headset-symbolic",
	"hands-free": "ld-headset-symbolic",
	"speaker":    "ld-speaker-symbolic",
	"tv":         "ld-tv-symbolic",
	"webcam":     "ld-webcam-symbolic",
	"microphone": "ld-mic-symbolic",
}

func soundDeviceIcon(d pulse.Device, kind pulse.DeviceType) string {
	if icon, ok := formFactorIcons[d.Properties["device.form_factor"]]; ok {
		return icon
	}
	switch {
	case d.Properties["device.bus"] == "bluetooth":
		return "ld-bluetooth-symbolic"
	case strings.Contains(strings.ToLower(d.ActivePort), "hdmi"), strings.Contains(strings.ToLower(d.ActivePort), "displayport"):
		return "ld-monitor-symbolic"
	case d.Properties["device.bus"] == "usb":
		return "ld-usb-symbolic"
	case kind == pulse.DeviceInput:
		return "ld-mic-symbolic"
	}
	return "ld-speaker-symbolic"
}

func soundDeviceName(d pulse.Device) string { return cmp.Or(d.Description, d.Name) }

func showSoundDevices(d *widget.Dropdown, devices []pulse.Device, kind pulse.DeviceType, current pulse.DeviceKey) []pulse.DeviceKey {
	keys := make([]pulse.DeviceKey, len(devices))
	names := make([]string, len(devices))
	for i, dev := range devices {
		keys[i], names[i] = dev.Key, soundDeviceName(dev)
	}
	selected := slices.Index(keys, current)
	if slices.Equal(d.Items(), names) && d.Selected() == selected {
		return keys
	}
	rows := make([]widget.DropdownRow, len(devices))
	for i, dev := range devices {
		icon := soundDeviceIcon(dev, kind)
		rows[i] = widget.DropdownRow{Label: names[i], Leading: func() widget.Widget { return widget.NewThemeIcon(icon, appIconPx) }}
	}
	d.SetRows(rows, selected)
	return keys
}
