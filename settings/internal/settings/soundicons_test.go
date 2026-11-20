package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/service/pulse"
)

func TestASoundDeviceShowsWhatItIs(t *testing.T) {
	dev := func(props map[string]string, port string) pulse.Device {
		return pulse.Device{Properties: props, ActivePort: port}
	}
	for _, c := range []struct {
		d    pulse.Device
		kind pulse.DeviceType
		want string
	}{
		{dev(map[string]string{"device.form_factor": "headset", "device.bus": "bluetooth"}, ""), pulse.DeviceOutput, "ld-headset-symbolic"},
		{dev(map[string]string{"device.bus": "bluetooth"}, ""), pulse.DeviceOutput, "ld-bluetooth-symbolic"},
		{dev(map[string]string{"device.bus": "pci"}, "hdmi-output-0"), pulse.DeviceOutput, "ld-monitor-symbolic"},
		{dev(map[string]string{"device.bus": "usb"}, "analog-output"), pulse.DeviceOutput, "ld-usb-symbolic"},
		{dev(map[string]string{"device.bus": "pci"}, "analog-input-mic"), pulse.DeviceInput, "ld-mic-symbolic"},
		{dev(nil, ""), pulse.DeviceOutput, "ld-speaker-symbolic"},
	} {
		if got := soundDeviceIcon(c.d, c.kind); got != c.want {
			t.Errorf("%+v reads %s, want %s", c.d.Properties, got, c.want)
		}
	}
}
