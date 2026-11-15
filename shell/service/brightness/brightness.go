// Package brightness is the shell's brightness control: the kernel's
// backlights from desktop/service/backlight, external monitors over
// DDC/CI (ddc.go) re-scanned on DRM hotplug, the outputs each drives
// (output.go), and the shell's brightness daemon.
package brightness

import "github.com/stubbedev/lys/desktop/service/backlight"

// Device is one backlight or DDC/CI monitor snapshot.
type Device = backlight.Device

// Type is the kind of a Device.
type Type = backlight.Type

// Device types.
const (
	TypeRaw      = backlight.TypeRaw
	TypeFirmware = backlight.TypeFirmware
	TypePlatform = backlight.TypePlatform
	TypeDDC      = backlight.TypeDDC
)

// Source enumerates, watches and sets devices.
type Source = backlight.Source

// AveragePercentage is the mean level across devices: what the bar label
// and the OSD show, since a brightness action drives every device at
// once. False when there is none.
func AveragePercentage(devices []Device) (float64, bool) {
	return backlight.AveragePercentage(devices)
}
