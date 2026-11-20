package settings

import (
	"context"
	"fmt"
	"log"

	"github.com/stubbedev/lys/desktop/service/pulse"
	"github.com/stubbedev/lys/shell/cava"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

// deviceLevel is the live level of an audio device: the Sound page's meter,
// fed by a capture of the device at the meter's rate. The connection is made
// off the loop, and the capture lives until the page drops it.
type deviceLevel struct {
	*widget.Box
	meter   *levelMeter
	problem *widget.Label
	svc     *pulse.Service
	capture *pulse.Capture
	// want is the device to record, nil while there is none; held is the
	// one capture records.
	want   *pulse.CaptureTarget
	held   pulse.CaptureTarget
	closed bool
}

func newDeviceLevel(k *kit) *deviceLevel {
	t := locale.T()
	d := &deviceLevel{Box: gapBox(widget.Column), meter: newLevelMeter(k)}
	d.meter.SetAccessibleName(t.Get("settings-device-level"))
	d.meter.SetTooltip(t.Get("settings-device-level"))
	d.problem = wrapped(k.label("", "setting-problem", css.Caption, css.Error))
	d.problem.SetVisible(false)
	d.Append(d.meter, false)
	d.Append(d.problem, false)
	async(k, func() soundConnection {
		ctx, cancel := context.WithTimeout(context.Background(), soundTimeout)
		defer cancel()
		svc, err := openPulse(ctx)
		return soundConnection{svc, err}
	}, func(c soundConnection) {
		if c.err != nil {
			d.fail(fmt.Errorf("%s: %w", t.Get("settings-sound-unreachable"), c.err))
			return
		}
		if d.closed {
			_ = c.svc.Close()
			return
		}
		d.svc = c.svc
		d.record()
	})
	return d
}

// follow records want, or nothing for a device name refused by err.
func (d *deviceLevel) follow(want pulse.CaptureTarget, err error) {
	if err != nil {
		d.want = nil
		d.stop()
		d.fail(err)
		return
	}
	d.want = &want
	d.record()
}

func (d *deviceLevel) record() {
	if d.svc == nil || d.closed || d.want == nil || (d.capture != nil && d.held == *d.want) {
		return
	}
	d.stop()
	capture, err := d.svc.Capture(*d.want, meterSpec, d.meter.feed)
	if err != nil {
		d.fail(fmt.Errorf("settings: record %v for its level: %w", *d.want, err))
		return
	}
	d.capture, d.held = capture, *d.want
	d.problem.SetVisible(false)
}

func (d *deviceLevel) stop() {
	if d.capture != nil {
		d.capture.Close()
		d.capture = nil
	}
	d.meter.SetValue(0)
}

func (d *deviceLevel) fail(err error) {
	log.Printf("settings: device level: %v", err)
	d.problem.SetText(err.Error())
	d.problem.SetVisible(true)
}

func (d *deviceLevel) close() {
	d.closed = true
	d.stop()
	if d.svc != nil {
		_ = d.svc.Close()
		d.svc = nil
	}
}

// levelledDevice is a device picker over s with the live level of the device
// it names below it: target is what a stored name records.
type levelledDevice struct {
	*widget.Box
	slot   slot
	pick   control
	level  *deviceLevel
	target func(name string) (pulse.CaptureTarget, error)
}

func newLevelledDevice(k *kit, s slot, target func(name string) (pulse.CaptureTarget, error), pick func(s slot) control) *levelledDevice {
	c := &levelledDevice{Box: gapBox(widget.Column), slot: s, target: target, level: newDeviceLevel(k)}
	c.pick = pick(slot{
		get: s.get,
		set: func(v any) error {
			err := s.set(v)
			c.follow()
			return err
		},
		unset: func() {
			s.unset()
			c.follow()
		},
	})
	c.Append(c.pick, false)
	c.Append(c.level, false)
	c.follow()
	return c
}

func (c *levelledDevice) follow() { c.level.follow(c.target(c.slot.text())) }

func (c *levelledDevice) refresh() {
	c.pick.refresh()
	c.follow()
}

func (c *levelledDevice) close() { c.level.close() }

// microphoneTarget is what the recorder records for its microphone device:
// the default input while it names none.
func microphoneTarget(name string) (pulse.CaptureTarget, error) {
	if name == "" {
		return pulse.CaptureDefaultInput(), nil
	}
	return pulse.CaptureNamed(name)
}

// audioSourceEditor is cava's source picker with its live level below it.
func audioSourceEditor(k *kit, s slot, spec choiceSpec) control {
	return newLevelledDevice(k, s, cava.Target, func(s slot) control { return newChoiceEditor(k, s, spec) })
}
