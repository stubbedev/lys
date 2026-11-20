package settings

import (
	"context"
	"fmt"
	"log"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/service/pulse"
	"github.com/stubbedev/lys/desktop/service/recorder/devicechoice"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const recorderMicrophonePath = "modules.recorder.microphone"

func micCaptureTarget(device string) (pulse.CaptureTarget, error) {
	if device == "" {
		return pulse.CaptureDefaultInput(), nil
	}
	return pulse.CaptureNamed(device)
}

type micMeter struct {
	*widget.Box
	k           *kit
	device      *deviceSelect
	meter       *levelMeter
	problem     *widget.Label
	svc         *pulse.Service
	capture     *pulse.Capture
	recording   string
	tried       bool
	connecting  bool
	unreachable bool
	closed      bool
}

var microphoneDevice = withEditor(func(k *kit, s slot, _ config.FieldMeta) control {
	m := &micMeter{Box: gapBox(widget.Column), k: k}
	m.device = newDeviceSelect(k, s, devicechoice.MicrophoneChoices(nil))
	m.meter = newLevelMeter(k)
	m.meter.SetAccessibleName(locale.T().Get("settings-sound-level"))
	m.problem = wrapped(k.label("", css.Error, css.Caption))
	m.problem.SetVisible(false)
	m.Append(m.device, false)
	m.Append(m.meter, false)
	m.Append(m.problem, false)
	async(k, microphoneChoices, m.device.setChoices)
	m.follow()
	return m
})

func (m *micMeter) refresh() {
	m.device.refresh()
	m.follow()
}

func (m *micMeter) wanted() bool {
	on, _ := m.k.store.value(recorderMicrophonePath).(bool)
	return on && !m.closed
}

func (m *micMeter) follow() {
	m.meter.SetVisible(m.wanted())
	switch {
	case !m.wanted():
		m.stopCapture()
	case m.svc == nil:
		m.connect()
	case !m.tried || m.recording != m.device.slot.text():
		m.startCapture()
	}
}

func (m *micMeter) connect() {
	if m.connecting || m.unreachable {
		return
	}
	m.connecting = true
	async(m.k, func() soundConnection {
		ctx, cancel := context.WithTimeout(context.Background(), soundTimeout)
		defer cancel()
		svc, err := openPulse(ctx)
		return soundConnection{svc, err}
	}, func(c soundConnection) {
		m.connecting = false
		if c.err != nil {
			m.unreachable = true
			m.fail(fmt.Errorf("%s: %w", locale.T().Get("settings-sound-unreachable"), c.err))
			return
		}
		if m.closed {
			_ = c.svc.Close()
			return
		}
		m.svc = c.svc
		m.follow()
	})
}

func (m *micMeter) startCapture() {
	m.stopCapture()
	device := m.device.slot.text()
	m.tried, m.recording = true, device
	target, err := micCaptureTarget(device)
	if err != nil {
		m.fail(err)
		return
	}
	capture, err := m.svc.Capture(target, meterSpec, m.meter.feed)
	if err != nil {
		m.fail(err)
		return
	}
	m.problem.SetVisible(false)
	m.capture = capture
}

func (m *micMeter) stopCapture() {
	if m.capture != nil {
		m.capture.Close()
		m.capture = nil
	}
	m.tried = false
	m.meter.SetValue(0)
}

func (m *micMeter) fail(err error) {
	log.Printf("settings: recorder microphone level: %v", err)
	m.problem.SetText(err.Error())
	m.problem.SetVisible(true)
}

func (m *micMeter) close() {
	m.closed = true
	m.stopCapture()
	if m.svc != nil {
		_ = m.svc.Close()
		m.svc = nil
	}
}
