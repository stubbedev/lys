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

	if err := c.pick.(*deviceSelect).slot.set("speakers.monitor"); err != nil {
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
	if !c.level.problem.Visible() || !strings.Contains(c.level.problem.Text(), "invalid capture device name") {
		t.Fatalf("a source no capture can name says %v %q", c.level.problem.Visible(), c.level.problem.Text())
	}
	c.refresh()
	if rec := srv.WaitRecords(t, 1)[0]; rec.Source != "mic" || c.level.problem.Visible() {
		t.Fatalf("the stored source records %q again, problem shown %v", rec.Source, c.level.problem.Visible())
	}
}

func TestADeviceLevelWithoutASoundServerSaysSo(t *testing.T) {
	before := openPulse
	openPulse = func(context.Context) (*pulse.Service, error) { return nil, errors.New("no server in the test") }
	t.Cleanup(func() { openPulse = before })
	k := testKit(t, "")
	c := fieldControl(k, k.store, field("modules.recorder.microphone-device", microphoneDevice)).(*levelledDevice)
	t.Cleanup(c.close)
	if !c.level.problem.Visible() || !strings.Contains(c.level.problem.Text(), locale.T().Get("settings-sound-unreachable")) || !strings.Contains(c.level.problem.Text(), "no server in the test") {
		t.Fatalf("the problem reads %v %q", c.level.problem.Visible(), c.level.problem.Text())
	}
	if c.level.capture != nil {
		t.Fatal("a capture without a server")
	}
}
