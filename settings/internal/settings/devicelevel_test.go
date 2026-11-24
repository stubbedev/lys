package settings

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/service/pulse"
	"github.com/stubbedev/lys/lib/pulse/pulsetest"
	"github.com/stubbedev/lys/shell/cava"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func levelledRow(t *testing.T, srv *pulsetest.Server, config, path string, opts ...rowOpt) (*levelledDevice, *kit, *loop) {
	t.Helper()
	useSoundServer(t, srv)
	k := testKit(t, config)
	l := &loop{t: t, queue: make(chan func(), 64)}
	k.invoke = func(fn func()) { l.queue <- fn }
	c := fieldControl(k, k.store, field(path, opts...)).(*levelledDevice)
	t.Cleanup(c.close)
	return c, k, l
}

func TestTheRecorderMicrophoneShowsTheLevelOfTheDeviceItRecords(t *testing.T) {
	srv := soundServer(t)
	const path = "modules.recorder.microphone-device"
	c, k, l := levelledRow(t, srv, "", path, microphoneDevice)
	rec := srv.WaitRecords(t, 1)[0]
	if rec.Source != "mic" || rec.Props["media.name"] != meterSpec.Name {
		t.Fatalf("the default microphone records %q as %q", rec.Source, rec.Props["media.name"])
	}
	rec.Push(pcm(20000, 400))
	l.until("the microphone's level shown", func() bool { return c.level.meter.Value() > 0.9 })

	if err := c.pick.(*choiceControl).slot.set("speakers.monitor"); err != nil {
		t.Fatal(err)
	}
	if v := k.store.value(path); v != "speakers.monitor" {
		t.Fatalf("picking stored %v", v)
	}
	moved := srv.WaitRecords(t, 1)[0]
	if moved.Source != "speakers.monitor" {
		t.Fatalf("after the pick the level records %q", moved.Source)
	}
	if c.level.meter.Value() != 0 {
		t.Fatalf("the old device's level %v stays on the new one", c.level.meter.Value())
	}
	c.refresh()
	if got := srv.Records(); len(got) != 1 || got[0] != moved {
		t.Fatalf("a refresh with the same device reopened the capture: %v", got)
	}
	c.close()
	srv.WaitRecords(t, 0)
}

func TestCavasSourceShowsTheLevelOfWhatCavaRecords(t *testing.T) {
	srv := soundServer(t)
	c, k, _ := levelledRow(t, srv, "modules:\n  cava:\n    source: auto\n", "modules.cava.source")
	if rec := srv.WaitRecords(t, 1)[0]; rec.Source != headsetSink+".monitor" {
		t.Fatalf("auto records %q, want the default output's monitor", rec.Source)
	}
	if err := k.store.set("modules.cava.source", "mic"); err != nil {
		t.Fatal(err)
	}
	c.refresh()
	if rec := srv.WaitRecords(t, 1)[0]; rec.Source != "mic" {
		t.Fatalf("a named source records %q", rec.Source)
	}
	c.level.follow(cava.Target("bad\x00name"))
	srv.WaitRecords(t, 0)
	if st := c.level.state; st.kind != stateError || st.Text() != locale.T().Get("settings-device-level-failed") || !strings.Contains(st.detail.Text(), "invalid capture device name") {
		t.Fatalf("a source no capture can name shows %v %q over %q", st.kind, st.Text(), st.detail.Text())
	}
	c.refresh()
	if rec := srv.WaitRecords(t, 1)[0]; rec.Source != "mic" || c.level.state.Visible() {
		t.Fatalf("the stored source records %q again, its state shown %v", rec.Source, c.level.state.Visible())
	}
}

func TestADeviceLevelWithoutASoundServerSaysSo(t *testing.T) {
	before := openPulse
	openPulse = func(context.Context) (*pulse.Service, error) { return nil, errors.New("no server in the test") }
	t.Cleanup(func() { openPulse = before })
	k := testKit(t, "")
	c := fieldControl(k, k.store, field("modules.recorder.microphone-device", microphoneDevice)).(*levelledDevice)
	t.Cleanup(c.close)
	if st := c.level.state; st.kind != stateUnavailable || st.Text() != locale.T().Get("settings-sound-unreachable") || st.detail.Text() != "no server in the test" || c.level.meter.Visible() {
		t.Fatalf("without a server the level shows %v %q over %q, the meter shown %v", st.kind, st.Text(), st.detail.Text(), c.level.meter.Visible())
	}
	if c.level.capture != nil {
		t.Fatal("a capture without a server")
	}
}
