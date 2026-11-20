package settings

import (
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

func soundDeviceRows(devices []pulse.Device, kind pulse.DeviceType, label func(pulse.Device) string) []widget.DropdownRow {
	rows := make([]widget.DropdownRow, len(devices))
	for i, d := range devices {
		icon := soundDeviceIcon(d, kind)
		rows[i] = widget.DropdownRow{Label: label(d), Leading: func() widget.Widget { return widget.NewThemeIcon(icon, appIconPx) }}
	}
	return rows
}
