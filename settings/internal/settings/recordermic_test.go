package settings

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/service/pulse"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/pulse/native"
	"github.com/stubbedev/lys/lib/pulse/pulsetest"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const recorderDevicePath = "modules.recorder.microphone-device"

func recorderMic(t *testing.T, yaml string) (*kit, *micMeter) {
	t.Helper()
	k := testKit(t, yaml)
	m, ok := fieldControl(k, k.store, pageRow(t, "recorder", recorderDevicePath)).(*micMeter)
	if !ok {
		t.Fatal("the recorder's microphone row is no microphone with a level meter")
	}
	t.Cleanup(m.close)
	return k, m
}

func setAndRefresh(t *testing.T, k *kit, m *micMeter, path string, v any) {
	t.Helper()
	if err := k.store.set(path, v); err != nil {
		t.Fatalf("set %s: %v", path, err)
	}
	m.refresh()
}

func TestTheRecordersMicrophoneLevelFollowsTheMicrophoneSwitchAndDevice(t *testing.T) {
	srv := soundServer(t)
	srv.PutSource(pulsetest.Source(11, "usb-mic", "USB Microphone", native.InvalidIndex, ""))
	useSoundServer(t, srv)
	k, m := recorderMic(t, "")
	if m.meter.Visible() || len(srv.Records()) != 0 {
		t.Fatal("with the microphone off the page records it")
	}
	setAndRefresh(t, k, m, recorderMicrophonePath, true)
	if rec := srv.WaitRecords(t, 1)[0]; rec.Source != "mic" || !m.meter.Visible() {
		t.Fatalf("with the microphone on the meter records %q, shown %v", rec.Source, m.meter.Visible())
	}
	setAndRefresh(t, k, m, recorderDevicePath, "usb-mic")
	if rec := srv.WaitRecords(t, 1)[0]; rec.Source != "usb-mic" {
		t.Fatalf("the chosen microphone is not the one measured: %q", rec.Source)
	}
	m.refresh()
	srv.WaitRecords(t, 1)
	setAndRefresh(t, k, m, recorderMicrophonePath, false)
	srv.WaitRecords(t, 0)
	if m.meter.Visible() {
		t.Error("the meter stays after the microphone was turned off")
	}
	setAndRefresh(t, k, m, recorderMicrophonePath, true)
	srv.WaitRecords(t, 1)
	m.close()
	srv.WaitRecords(t, 0)
}

func TestWithoutASoundServerTheRecordersMicrophoneSaysSoOnce(t *testing.T) {
	before := openPulse
	opened := 0
	openPulse = func(context.Context) (*pulse.Service, error) {
		opened++
		return nil, errors.New("no sound server")
	}
	t.Cleanup(func() { openPulse = before })
	k, m := recorderMic(t, "modules:\n  recorder:\n    microphone: true\n")
	want := locale.T().Get("settings-sound-unreachable") + ": no sound server"
	if !slices.Contains(treetest.Labels(m), want) || !m.problem.Visible() {
		t.Fatalf("an unreachable sound server reads %q", treetest.Labels(m))
	}
	setAndRefresh(t, k, m, recorderDevicePath, "usb-mic")
	if opened != 1 {
		t.Errorf("the sound server was tried %d times, want once", opened)
	}
}

func TestNoMicrophoneIsTheDefaultInputAndANameHoldingNULIsRefused(t *testing.T) {
	if _, err := micCaptureTarget("bad\x00name"); err == nil {
		t.Error("a device name holding NUL became a capture target")
	}
	if target, err := micCaptureTarget(""); err != nil || target != pulse.CaptureDefaultInput() {
		t.Errorf("no device reads %v, %v, want the default input", target, err)
	}
}
