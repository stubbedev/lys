package batteryalert

import (
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/powersupply"
)

type kindDisplay struct {
	icon, label string
}

var genericDisplay = kindDisplay{"ld-battery-full-symbolic", "battery-device-generic"}

var kindDisplays = map[powersupply.DeviceKind]kindDisplay{
	powersupply.KindMouse:       {"ld-mouse-symbolic", "battery-device-mouse"},
	powersupply.KindTouchpad:    {"ld-mouse-symbolic", "battery-device-touchpad"},
	powersupply.KindKeyboard:    {"ld-keyboard-symbolic", "battery-device-keyboard"},
	powersupply.KindGamingInput: {"ld-gamepad-2-symbolic", "battery-device-controller"},
	powersupply.KindPen:         {"ld-tablet-symbolic", "battery-device-pen"},
}

func displayOf(k powersupply.DeviceKind) kindDisplay {
	if d, ok := kindDisplays[k]; ok {
		return d
	}
	return genericDisplay
}

// PeripheralIcon is the themed icon for the peripheral's kind.
func PeripheralIcon(p powersupply.Peripheral) string { return displayOf(p.Kind).icon }

// PeripheralName is the model the kernel reports, or the kind's name
// when it reports none.
func PeripheralName(p powersupply.Peripheral) string {
	if p.Model != "" {
		return p.Model
	}
	return i18n.T(displayOf(p.Kind).label)
}
