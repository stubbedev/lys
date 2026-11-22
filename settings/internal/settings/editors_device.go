package settings

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/service/pulse"
	"github.com/stubbedev/lys/desktop/service/recorder/devicechoice"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
)

// deviceOptions are devices as the options of a live closed choice,
// writing the picked one's id.
func deviceOptions(devices []devicechoice.DeviceChoice) []choice {
	out := make([]choice, len(devices))
	for i, d := range devices {
		out[i] = choice{value: d.ID, label: d.Label}
	}
	return out
}

// newDeviceChoice is a live closed choice of devices.
func newDeviceChoice(k *kit, s slot, devices []devicechoice.DeviceChoice) *choiceControl {
	return newChoiceControl(k, s, closedChoice{options: deviceOptions(devices), live: true})
}

// cameraChoices lists the cameras (a seam for tests).
var cameraChoices = devicechoice.Cameras

// microphoneQueryTimeout bounds the audio daemon's source listing.
const microphoneQueryTimeout = 5 * time.Second

// microphoneChoices asks the audio daemon for its sources
// (dev.stubbe.lys.Audio1 ListSources), monitors dropped by their name; with
// no daemon the list is the default alone (a seam for tests).
var microphoneChoices = func() []devicechoice.DeviceChoice {
	sources, _ := audioDaemonSources()
	return devicechoice.MicrophoneChoices(sources)
}

func audioDaemonSources() ([]devicechoice.Source, bool) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, false
	}
	defer func() { _ = conn.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), microphoneQueryTimeout)
	defer cancel()
	entries, err := pulse.NewClient(conn).ListSources(ctx)
	if err != nil {
		log.Printf("settings: audio sources: %v", err)
		return nil, false
	}
	sources := make([]devicechoice.Source, len(entries))
	for i, e := range entries {
		sources[i] = devicechoice.Source{Name: e.Name, Description: e.Description, Monitor: strings.HasSuffix(e.Name, ".monitor")}
	}
	return sources, true
}

// webcamDevice is webcam_device_select.
var webcamDevice = withEditor(func(k *kit, s slot, _ config.FieldMeta) control {
	return newDeviceChoice(k, s, cameraChoices())
})

// microphoneDevice is microphone_device_select: the default alone at
// first, the daemon's sources once they arrive, the picked one's live level
// beside it.
var microphoneDevice = withEditor(func(k *kit, s slot, _ config.FieldMeta) control {
	return newLevelledDevice(k, s, microphoneTarget, func(s slot) control {
		c := newDeviceChoice(k, s, devicechoice.MicrophoneChoices(nil))
		async(k, microphoneChoices, func(devices []devicechoice.DeviceChoice) { c.setOptions(deviceOptions(devices)) })
		return c
	})
})
