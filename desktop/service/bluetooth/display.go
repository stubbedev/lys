package bluetooth

import "fmt"

// The Bluetooth class-of-device major classes.
const (
	MajorComputer   = 0x01
	MajorPhone      = 0x02
	MajorNetwork    = 0x03
	MajorAV         = 0x04
	MajorPeripheral = 0x05
	MajorImaging    = 0x06
	MajorWearable   = 0x07
	MajorToy        = 0x08
	MajorHealth     = 0x09
)

// MajorClass is the major device class of a raw class of device.
func MajorClass(raw uint32) uint32 { return (raw >> 8) & 0x1F }

// MinorClass is the minor device class of a raw class of device.
func MinorClass(raw uint32) uint32 { return (raw >> 2) & 0x3F }

// DeviceIcon picks a device's icon from BlueZ's icon hint, else the
// class of device.
func DeviceIcon(hint *string, class *uint32) string {
	if hint != nil {
		switch *hint {
		case "audio-headphones", "audio-headset":
			return "ld-headphones-symbolic"
		case "audio-speakers":
			return "ld-speaker-symbolic"
		case "input-keyboard":
			return "ld-keyboard-symbolic"
		case "input-mouse", "input-tablet":
			return "ld-mouse-symbolic"
		case "input-gaming":
			return "ld-gamepad-2-symbolic"
		case "video-display", "computer":
			return "ld-monitor-symbolic"
		case "phone":
			return "ld-smartphone-symbolic"
		}
	}
	if class == nil {
		return "ld-bluetooth-symbolic"
	}
	raw := *class
	minor := MinorClass(raw)
	switch MajorClass(raw) {
	case MajorComputer:
		switch minor {
		case 0x03:
			return "ld-laptop-symbolic"
		case 0x07:
			return "ld-tablet-symbolic"
		}
		return "ld-monitor-symbolic"
	case MajorPhone:
		return "ld-smartphone-symbolic"
	case MajorAV:
		return avIcon(minor)
	case MajorPeripheral:
		return peripheralIcon(minor)
	case MajorImaging:
		switch {
		case minor&0x08 != 0:
			return "ld-printer-symbolic"
		case minor&0x02 != 0:
			return "ld-camera-symbolic"
		case minor&0x01 != 0:
			return "ld-monitor-symbolic"
		}
		return "ld-printer-symbolic"
	case MajorWearable:
		return "ld-watch-symbolic"
	}
	return "ld-bluetooth-symbolic"
}

func avIcon(minor uint32) string {
	switch minor {
	case 0x04:
		return "ld-mic-symbolic"
	case 0x05, 0x07, 0x08, 0x0A:
		return "ld-speaker-symbolic"
	case 0x09, 0x0B:
		return "ld-tv-symbolic"
	case 0x0C, 0x0D:
		return "ld-camera-symbolic"
	case 0x0E, 0x0F, 0x10:
		return "ld-monitor-symbolic"
	case 0x12:
		return "ld-gamepad-2-symbolic"
	}
	return "ld-headphones-symbolic"
}

func peripheralIcon(minor uint32) string {
	switch minor >> 4 {
	case 0x01, 0x03:
		return "ld-keyboard-symbolic"
	case 0x02:
		return "ld-mouse-symbolic"
	}
	switch minor & 0x0F {
	case 0x01, 0x02:
		return "ld-gamepad-2-symbolic"
	case 0x05:
		return "ld-mouse-symbolic"
	}
	return "ld-bluetooth-symbolic"
}

// FormatPasskey zero-pads a passkey to six digits.
func FormatPasskey(passkey uint32) string { return fmt.Sprintf("%06d", passkey) }
