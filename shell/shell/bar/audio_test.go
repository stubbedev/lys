package bar

import (
	"context"
	"errors"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/feed"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/pulse"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/dbusx"
	"github.com/stubbedev/lys/lib/logtest"
	"github.com/stubbedev/lys/lib/pulse/native"
	"github.com/stubbedev/lys/lib/pulse/pulsetest"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/shell/service/privacy"
	"github.com/stubbedev/lys/shell/service/screenshare"
)

// fakePulseSource is a scripted pulse.Source: dev is both the default
// sink and the default source. The embedded nil Source leaves the
// dropdown's list and control methods unscripted; the audio tests that
// need them run against a pulsetest server instead.
type fakePulseSource struct {
	pulse.Source
	dev   pulse.Device
	err   error
	ticks chan struct{}
}

// pct is a stereo volume at a percentage.
func pct(p float64) pulse.Volume { return pulse.VolumeFromPercentage(p, 2) }

func (f *fakePulseSource) DefaultSink(context.Context) (pulse.OutputDevice, error) {
	return pulse.OutputDevice{Device: f.dev}, f.err
}

func (f *fakePulseSource) Subscribe(context.Context) (<-chan struct{}, func(), error) {
	return f.ticks, func() {}, nil
}

func (f *fakePulseSource) SetVolume(context.Context, float64) error { return nil }

func (f *fakePulseSource) SetMuted(context.Context, bool) error { return nil }

func (f *fakePulseSource) DefaultSource(context.Context) (pulse.InputDevice, error) {
	return pulse.InputDevice{Device: f.dev}, f.err
}

func (f *fakePulseSource) SetSourceMuted(context.Context, bool) error { return nil }

func (f *fakePulseSource) Capture(pulse.CaptureTarget, pulse.CaptureSpec, func([]byte)) (*pulse.Capture, error) {
	return nil, errors.New("fake: no capture")
}

// micServer is a private audio server with speakers, their monitor and
// a microphone, the speakers and the microphone the defaults.
func micServer(t *testing.T) (*pulsetest.Server, *pulse.Service) {
	t.Helper()
	srv := pulsetest.New(t)
	srv.PutSink(pulsetest.Sink(1, "speakers", "Speakers", 2))
	srv.PutSource(pulsetest.Source(2, "speakers.monitor", "Monitor of Speakers", 1, "speakers"))
	srv.PutSource(pulsetest.Source(3, "mic", "Built-in Microphone", native.InvalidIndex, ""))
	srv.SetDefaults("speakers", "mic")
	svc, err := pulse.Connect(context.Background(), srv.Addr())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	return srv, svc
}

func recording(index, source uint32, app string, pid int) native.StreamInfo {
	return native.StreamInfo{
		Index: index, Name: "capture", Device: source, OwnerModule: native.InvalidIndex, Client: native.InvalidIndex,
		SampleSpec: pulsetest.StereoSpec(), ChannelMap: pulsetest.StereoMap(),
		Volume: native.CVolume{native.VolumeNorm, native.VolumeNorm},
		Props:  native.PropList{"application.name": app, "application.process.id": strconv.Itoa(pid)},
		Format: native.FormatInfo{Encoding: native.EncodingPCM, Props: native.PropList{}},
	}
}

// fakeCameras is a camera list the test sets.
type fakeCameras struct {
	mu    sync.Mutex
	users []privacy.CameraUser
	tick  feed.Tick
}

func (f *fakeCameras) Users() []privacy.CameraUser {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.users)
}

func (f *fakeCameras) Subscribe() (<-chan struct{}, func()) { return f.tick.Subscribe() }

func (f *fakeCameras) set(users ...privacy.CameraUser) {
	f.mu.Lock()
	f.users = users
	f.mu.Unlock()
	feed.Notify(&f.tick)
}

// fakeShares is the portal's dev.stubbe.lys.ScreenShares1.
type fakeShares struct {
	conn    *dbus.Conn
	mu      sync.Mutex
	shares  []screenshare.Share
	stopped []dbus.ObjectPath
}

func (f *fakeShares) List() ([]screenshare.Share, *dbus.Error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.shares), nil
}

func (f *fakeShares) Stop(session dbus.ObjectPath) *dbus.Error {
	f.mu.Lock()
	i := slices.IndexFunc(f.shares, func(s screenshare.Share) bool { return s.Session == session })
	if i < 0 {
		f.mu.Unlock()
		return dbusx.InvalidArgs("no screen share")
	}
	f.shares = slices.Delete(f.shares, i, i+1)
	f.stopped = append(f.stopped, session)
	f.mu.Unlock()
	f.changed()
	return nil
}

func (f *fakeShares) set(shares ...screenshare.Share) {
	f.mu.Lock()
	f.shares = shares
	f.mu.Unlock()
	f.changed()
}

func (f *fakeShares) stoppedSessions() []dbus.ObjectPath {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.stopped)
}

func (f *fakeShares) changed() { _ = screenshare.Changed.Emit(f.conn, screenshare.Path, struct{}{}) }

// servePortalShares serves the fake on a private bus and points the bar
// at it.
func servePortalShares(t *testing.T) (*fakeShares, shareSource) {
	t.Helper()
	bus := dbustest.Start(t)
	f := &fakeShares{conn: bus.Conn(t)}
	release, err := dbusx.Serve(f.conn, dbusx.Service{
		Name: screenshare.BusName, Path: screenshare.Path, Interface: screenshare.Interface,
		Methods: f, Signals: []dbusx.SignalDecl{screenshare.Changed},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	shares, err := screenshare.Follow(bus.Conn(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(shares.Close)
	return f, shares
}

var chromeShare = screenshare.Share{
	Session: "/org/freedesktop/portal/desktop/session/1_9/s", App: "org.chromium.Chromium", Kind: screenshare.KindScreenCast,
	Targets: []screenshare.Target{{Kind: screenshare.TargetOutput, Name: "DP-1"}},
}

// captureFixture is a context with a private audio server, a camera
// list and the portal's shares behind it.
func captureFixture(t *testing.T) (ModuleContext, *pulsetest.Server, *fakeCameras, *fakeShares) {
	t.Helper()
	srv, svc := micServer(t)
	f, shares := servePortalShares(t)
	cams := &fakeCameras{}
	ctx := newTestContext(t, config.Defaults())
	ctx.Pulse, ctx.Cameras, ctx.ScreenShares = svc, cams, shares
	return ctx, srv, cams, f
}

func device(volume float64, muted bool) *pulse.Device {
	return &pulse.Device{Volume: pct(volume), Muted: muted}
}

func TestTheAudioFaceShowsTheSpeakerAndTheMicrophone(t *testing.T) {
	cfg := config.DefaultsAudio()
	f := audioFaceOf(cfg, audioRead{speaker: device(42, false), mic: device(80, false)})
	if !f.visible || !f.speakerShown || f.speakerIcon != cfg.SpeakerIcons[1] || f.label != "42%" {
		t.Errorf("live speaker: %+v", f)
	}
	if !f.micShown || f.micIcon != cfg.MicIcon || f.micCapturing {
		t.Errorf("live microphone: %+v", f)
	}
	if f.description != "42%" {
		t.Errorf("description %q, want the level alone", f.description)
	}

	f = audioFaceOf(cfg, audioRead{speaker: device(42, true), mic: device(80, true)})
	if f.speakerIcon != cfg.SpeakerIconMuted || f.label != "42%" || f.micIcon != cfg.MicIconMuted || !f.micShown {
		t.Errorf("both muted: %+v", f)
	}
	want := i18n.T("bar-audio-speaker-muted", i18n.Str("level", "42%")) + ", " + i18n.T("bar-audio-mic-muted")
	if f.description != want {
		t.Errorf("muted description %q, want %q", f.description, want)
	}

	cfg.Format = "{{ percent }}/{{ mic_percent }}"
	if f := audioFaceOf(cfg, audioRead{speaker: device(42, false), mic: device(80, false)}); f.label != "42/80" {
		t.Errorf("label %q, want both levels", f.label)
	}
	cfg.IconShow = false
	if f := audioFaceOf(cfg, audioRead{speaker: device(42, false)}); f.speakerShown || f.micShown || !f.visible {
		t.Errorf("icon-show off and no microphone: %+v", f)
	}
}

func TestTheAudioFaceWithoutDevices(t *testing.T) {
	cfg := config.DefaultsAudio()
	f := audioFaceOf(cfg, audioRead{mic: device(80, false)})
	if !f.visible || f.speakerShown || f.label != "" || f.hasLevel || !f.micShown {
		t.Errorf("microphone alone: %+v", f)
	}
	if f := audioFaceOf(cfg, audioRead{}); f.visible || f.micShown || f.speakerShown {
		t.Errorf("no devices and nothing capturing: %+v, want it hidden", f)
	}
	cfg.MicHideWhenIdle = true
	if f := audioFaceOf(cfg, audioRead{speaker: device(42, false), mic: device(80, false)}); f.micShown {
		t.Errorf("mic-hide-when-idle with nothing recording shows the microphone: %+v", f)
	}
}

func TestTheCaptureIndicatorLightsForEachCapture(t *testing.T) {
	cfg := config.DefaultsAudio()
	cfg.MicHideWhenIdle = true
	base := audioRead{speaker: device(42, false), mic: device(80, false)}
	for _, tc := range []struct {
		name                string
		capture             captureState
		mic, camera, screen bool
		detail              string
	}{
		{"idle", captureState{}, false, false, false, ""},
		{"microphone", captureState{mics: []privacy.MicUser{{Name: "Firefox"}}}, true, false, false, "bar-audio-capture-mic"},
		{"camera", captureState{cameras: []privacy.CameraUser{{PID: 7, Name: "zoom"}}}, false, true, false, "bar-audio-capture-camera"},
		{"screen", captureState{shares: []screenshare.Share{chromeShare}}, false, false, true, "bar-audio-capture-screen"},
	} {
		r := base
		r.capture = tc.capture
		f := audioFaceOf(cfg, r)
		if f.micCapturing != tc.mic || f.micShown != tc.mic || f.cameraShown != tc.camera || f.screenShown != tc.screen {
			t.Errorf("%s: %+v", tc.name, f)
		}
		hasDetail := tc.detail != "" && strings.Contains(f.description, i18n.T(tc.detail))
		if (tc.detail != "") != hasDetail || (tc.detail == "" && f.description != "42%") {
			t.Errorf("%s: description %q", tc.name, f.description)
		}
	}
	if f := audioFaceOf(cfg, audioRead{capture: captureState{cameras: []privacy.CameraUser{{PID: 7}}}}); !f.visible || !f.cameraShown {
		t.Errorf("a camera in use without audio devices: %+v, want the indicator shown", f)
	}
}

// newAudioOnHeadlessLoop mounts the module with its chrome.
func newAudioOnHeadlessLoop(t *testing.T, ctx ModuleContext) (*audioModule, *widget.Box) {
	t.Helper()
	mod, err := createOnHeadlessLoop("audio", ctx)
	if err != nil {
		t.Fatal(err)
	}
	m := mod.(*audioModule)
	chrome := widget.NewBox(widget.Row, 0, 0)
	onHeadlessLoop(func() bool { m.setChrome(chrome); return true })
	return m, chrome
}

func lit(icon *widget.Icon) bool { return icon.Visible() && icon.HasClass("capturing") }

func TestTheAudioModuleFollowsTheServerAndWhatCaptures(t *testing.T) {
	ctx, srv, cams, shares := captureFixture(t)
	cfg := ctx.Config.Audio
	m, chrome := newAudioOnHeadlessLoop(t, ctx)
	waitHeadless(t, "the speaker and the microphone", func() bool {
		return chrome.Visible() && m.btn.Label().Text() == "50%" && m.speaker.Visible() && m.speaker.Name() == cfg.SpeakerIcons[1] &&
			m.mic.Visible() && m.mic.Name() == cfg.MicIcon
	})
	if onHeadlessLoop(func() bool { return lit(m.mic) || m.camera.Visible() || m.screen.Visible() }) {
		t.Fatal("the capture indicator lights with nothing capturing")
	}

	srv.PutSourceOutput(recording(41, 3, "lys-shell", os.Getpid()))
	srv.PutSourceOutput(recording(40, 2, "OBS", 4242))
	srv.PutSourceOutput(recording(42, 3, "Firefox", 4343))
	waitHeadless(t, "the microphone lit", func() bool { return lit(m.mic) && strings.Contains(m.mic.InlineStyle(), cfg.CaptureColor.ToCSS()) })
	srv.RemoveSourceOutput(42)
	waitHeadless(t, "the microphone out again", func() bool { return !lit(m.mic) && m.mic.InlineStyle() == "" })
	if onHeadlessLoop(func() bool { return lit(m.mic) }) {
		t.Error("a monitor recording or lys-shell's own recorder lights the microphone")
	}

	cams.set(privacy.CameraUser{PID: 777, Name: "zoom", Device: "/dev/video0"})
	waitHeadless(t, "the camera lit", func() bool { return m.camera.Visible() && !m.screen.Visible() && !lit(m.mic) })
	cams.set()
	shares.set(chromeShare)
	waitHeadless(t, "the screen lit", func() bool { return m.screen.Visible() && !m.camera.Visible() })
	shares.set()
	waitHeadless(t, "the indicator off", func() bool { return !m.screen.Visible() && !m.camera.Visible() && !lit(m.mic) })

	if err := applyAudioBuiltin(context.Background(), ctx.Pulse, audioBuiltin{target: audioSpeaker}); err != nil {
		t.Fatal(err)
	}
	waitHeadless(t, "the speaker muted", func() bool { return m.speaker.Name() == cfg.SpeakerIconMuted && m.btn.Label().Text() == "50%" })
	if err := applyAudioBuiltin(context.Background(), ctx.Pulse, audioBuiltin{target: audioMic}); err != nil {
		t.Fatal(err)
	}
	waitHeadless(t, "the microphone muted", func() bool { return m.mic.Name() == cfg.MicIconMuted })

	srv.RemoveSink(1)
	srv.RemoveSource(3)
	waitHeadless(t, "the module hidden without devices", func() bool { return !chrome.Visible() })
}

func TestTheAudioModuleNeedsItsSourceAndHidesOnAFailedRead(t *testing.T) {
	ctx := newTestContext(t, config.Defaults())
	if _, err := createOnHeadlessLoop("audio", ctx); err == nil || !strings.Contains(err.Error(), "no audio source") {
		t.Fatalf("no audio source: %v, want an error", err)
	}
	buf := logtest.Capture(t)
	ctx.Pulse = &fakePulseSource{err: errors.New("broken pipe"), ticks: make(chan struct{})}
	_, chrome := newAudioOnHeadlessLoop(t, ctx)
	if onHeadlessLoop(chrome.Visible) {
		t.Error("shown after a failed read")
	}
	if got := onHeadlessLoop(buf.String); !strings.Contains(got, "audio: first read: ") || !strings.Contains(got, "broken pipe") {
		t.Errorf("log = %q, want the failure with its module", got)
	}
}

func TestAudioThresholdsReadTheSpeaker(t *testing.T) {
	cfg := config.Defaults()
	warn, err := config.ParseColorValue("status-warning")
	if err != nil {
		t.Fatal(err)
	}
	cfg.Audio.Thresholds = []config.ThresholdEntry{{Above: ptrF(60), LabelColor: &warn}}
	ctx := newTestContext(t, cfg)
	m := newAudioModule(ctx, &fakePulseSource{})
	m.apply(audioRead{speaker: device(65, false), mic: device(10, false)})
	if got := m.btn.thresholds; got.LabelColor == nil || *got.LabelColor != warn {
		t.Errorf("speaker at 65%%: thresholds %+v, want the warning", got)
	}
	m.apply(audioRead{speaker: device(30, false), mic: device(90, false)})
	if !m.btn.thresholds.IsEmpty() {
		t.Errorf("speaker at 30%%, microphone at 90%%: thresholds %+v, want none", m.btn.thresholds)
	}
}

// volumeOf is a device's first channel.
func volumeOf(d native.DeviceInfo) native.Volume { return d.Volume[0] }

func TestAudioActionsTargetTheirDevice(t *testing.T) {
	srv, svc := micServer(t)
	ctx := newTestContext(t, config.Defaults())
	ctx.Pulse = svc
	sink := func() native.DeviceInfo { d, _ := srv.Sink(1); return d }
	mic := func() native.DeviceInfo { d, _ := srv.Source(3); return d }
	await := func(what string, cond func() bool) {
		t.Helper()
		waitHeadless(t, what, cond)
	}
	clicks := ctx.Config.Audio.Clicks()

	speakerBefore, micBefore := volumeOf(sink()), volumeOf(mic())
	runClickAction(ctx, clicks.ScrollUp)
	await("the speaker raised", func() bool { return volumeOf(sink()) > speakerBefore })
	runClickAction(ctx, clicks.ScrollDown)
	runClickAction(ctx, clicks.ScrollDown)
	await("the speaker lowered", func() bool { return volumeOf(sink()) < speakerBefore })
	runClickAction(ctx, clicks.MiddleClick)
	await("the speaker muted", func() bool { return sink().Mute })
	if volumeOf(mic()) != micBefore || mic().Mute {
		t.Errorf("the speaker's actions moved the microphone: %+v", mic())
	}

	speakerNow := volumeOf(sink())
	runShellBuiltin(ctx, "lys-shell audio input-volume -10")
	await("the microphone lowered", func() bool { return volumeOf(mic()) < micBefore })
	runShellBuiltin(ctx, "lys-shell audio input-mute")
	await("the microphone muted", func() bool { return mic().Mute })
	runShellBuiltin(ctx, "lys-shell audio output-volume 100")
	await("the speaker at 100", func() bool { return volumeOf(sink()) == native.VolumeNorm })
	if !sink().Mute || speakerNow == native.VolumeNorm {
		t.Errorf("the microphone's actions moved the speaker: %+v", sink())
	}
}

func TestAudioBuiltinsParse(t *testing.T) {
	for verb, want := range map[string]audioBuiltin{
		"audio output-mute":      {target: audioSpeaker},
		"audio input-mute":       {target: audioMic},
		"audio output-volume +5": {target: audioSpeaker},
		"audio input-volume 30":  {target: audioMic},
	} {
		got, ok, err := parseAudioBuiltin(verb)
		if !ok || err != nil || got.target != want.target || (got.level == nil) != strings.HasSuffix(verb, "mute") {
			t.Errorf("%q = %+v %v %v", verb, got, ok, err)
		}
	}
	for _, verb := range []string{"", "audio", "audio output-volume", "audio sinks", "audio status", "audio output-mute now", "media play-pause", "screenshot region"} {
		if _, ok, _ := parseAudioBuiltin(verb); ok {
			t.Errorf("%q read as an in-process audio builtin", verb)
		}
	}
	if _, ok, err := parseAudioBuiltin("audio output-volume loud"); !ok || err == nil || !strings.Contains(err.Error(), `parse volume level "loud"`) {
		t.Errorf("a bad level: %v %v, want its parse error", ok, err)
	}
	buf := logtest.Capture(t)
	ctx := newTestContext(t, config.Defaults())
	runShellBuiltin(ctx, "lys-shell audio input-volume +x")
	if got := buf.String(); !strings.Contains(got, `click: lys-shell audio input-volume +x: parse volume delta "+x"`) || strings.Contains(got, "shelling out") {
		t.Errorf("log = %q, want the parse error and no subprocess", got)
	}
}

func TestTheAudioDropdownListsWhatCapturesAndStopsAShare(t *testing.T) {
	ctx, srv, cams, shares := captureFixture(t)
	v := onHeadlessLoop(func() *audioView { return audioDropdown(ctx).(*audioView) })
	defer onHeadlessLoop(func() bool { v.dropdownClosed(); return true })
	if onHeadlessLoop(v.capture.Visible) {
		t.Fatal("the capture section shows with nothing capturing")
	}
	var signalled []int
	onHeadlessLoop(func() bool {
		v.capture.signal = func(pid int, sig syscall.Signal) error {
			if sig == syscall.SIGTERM {
				signalled = append(signalled, pid)
			}
			return nil
		}
		return true
	})

	srv.PutSourceOutput(recording(40, 2, "OBS", 4242))
	srv.PutSourceOutput(recording(42, 3, "Firefox", 4343))
	cams.set(privacy.CameraUser{PID: 777, Name: "zoom", Device: "/dev/video0"})
	shares.set(chromeShare)
	labels := func() []string { return treetest.Labels(v.capture) }
	waitHeadless(t, "every capture listed", func() bool {
		l := labels()
		return v.capture.Visible() && slices.Contains(l, "Firefox") && slices.Contains(l, "zoom") && slices.Contains(l, "org.chromium.Chromium")
	})
	if l := onHeadlessLoop(labels); slices.Contains(l, "OBS") || !slices.ContainsFunc(l, func(s string) bool { return strings.Contains(s, "DP-1") }) {
		t.Errorf("labels %q, want no monitor recording and the shared screen", l)
	}
	actions := onHeadlessLoop(func() []widget.Widget { return treetest.WithClass(v.capture, "audio-capture-action") })
	if len(actions) != 3 {
		t.Fatalf("%d actions, want mute, end and stop", len(actions))
	}
	onHeadlessLoop(func() bool { actions[0].(*widget.Button).OnClick(); return true })
	waitHeadless(t, "Firefox's stream muted", func() bool { st, _ := srv.SourceOutput(42); return st.Mute })
	if st, _ := srv.SourceOutput(40); st.Mute {
		t.Error("muting Firefox muted the monitor recording too")
	}
	if src, _ := srv.Source(3); src.Mute {
		t.Error("muting Firefox muted the microphone")
	}
	onHeadlessLoop(func() bool { actions[1].(*widget.Button).ClickAt(widget.Point{}); return true })
	confirm := onHeadlessLoop(func() []widget.Widget { return treetest.WithClass(v.capture, "audio-capture-confirm") })
	if len(confirm) != 1 {
		t.Fatalf("%d confirm buttons after End, want the camera row's", len(confirm))
	}
	onHeadlessLoop(func() bool { confirm[0].(*widget.Button).ClickAt(widget.Point{}); return true })
	if !slices.Equal(signalled, []int{777}) {
		t.Errorf("ended %v, want the camera's process", signalled)
	}
	actions = onHeadlessLoop(func() []widget.Widget { return treetest.WithClass(v.capture, "audio-capture-action") })
	onHeadlessLoop(func() bool { actions[2].(*widget.Button).OnClick(); return true })
	waitHeadless(t, "the share stopped and its row gone", func() bool {
		return !slices.Contains(labels(), "org.chromium.Chromium")
	})
	if got := shares.stoppedSessions(); !slices.Equal(got, []dbus.ObjectPath{chromeShare.Session}) {
		t.Errorf("stopped %v, want the share's session", got)
	}

	srv.RemoveSourceOutput(42)
	cams.set()
	waitHeadless(t, "the capture section hidden", func() bool { return !v.capture.Visible() })
}

func cameraDropdown(t *testing.T, fail error) (*audioView, *fakeCameras, *[]syscall.Signal) {
	t.Helper()
	ctx, _, cams, _ := captureFixture(t)
	v := onHeadlessLoop(func() *audioView { return audioDropdown(ctx).(*audioView) })
	t.Cleanup(func() { onHeadlessLoop(func() bool { v.dropdownClosed(); return true }) })
	var signals []syscall.Signal
	onHeadlessLoop(func() bool {
		v.capture.signal = func(pid int, sig syscall.Signal) error {
			if pid != 777 {
				t.Errorf("signalled pid %d, want the camera's 777", pid)
			}
			signals = append(signals, sig)
			return fail
		}
		return true
	})
	cams.set(privacy.CameraUser{PID: 777, Name: "zoom", Device: "/dev/video0"})
	waitHeadless(t, "the camera listed", func() bool { return slices.Contains(treetest.Labels(v.capture), "zoom") })
	return v, cams, &signals
}

func clickOnly(t *testing.T, root widget.Widget, class string) {
	t.Helper()
	found := onHeadlessLoop(func() []widget.Widget { return treetest.WithClass(root, class) })
	if len(found) != 1 {
		t.Fatalf("%d widgets of class %s, want one", len(found), class)
	}
	onHeadlessLoop(func() bool { found[0].(*widget.Button).ClickAt(widget.Point{}); return true })
}

func countClass(root widget.Widget, class string) int {
	return onHeadlessLoop(func() int { return len(treetest.WithClass(root, class)) })
}

func TestEndingACameraAsksInTheRowFirst(t *testing.T) {
	v, cams, signals := cameraDropdown(t, nil)
	clickOnly(t, v.capture, "audio-capture-action")
	if len(*signals) != 0 {
		t.Fatalf("End alone sent %v, want nothing before the row confirms", *signals)
	}
	ask := i18n.T("dropdown-privacy-end-confirm", i18n.Str("app", "zoom"))
	if l := onHeadlessLoop(func() []string { return treetest.Labels(v.capture) }); !slices.Contains(l, ask) {
		t.Errorf("labels %q, want the row asking %q", l, ask)
	}

	cams.set(privacy.CameraUser{PID: 777, Name: "zoom", Device: "/dev/video0"}, privacy.CameraUser{PID: 900, Name: "cheese", Device: "/dev/video1"})
	waitHeadless(t, "the second camera listed", func() bool { return slices.Contains(treetest.Labels(v.capture), "cheese") })
	if n := countClass(v.capture, "audio-capture-confirm"); n != 1 {
		t.Fatalf("%d confirm buttons after a rebuild, want the asking row to keep asking", n)
	}

	clickOnly(t, v.capture, "audio-capture-cancel")
	if len(*signals) != 0 {
		t.Errorf("Cancel sent %v, want nothing", *signals)
	}
	if n := countClass(v.capture, "audio-capture-confirm"); n != 0 {
		t.Errorf("%d confirm buttons after Cancel, want the row back to End", n)
	}

	onHeadlessLoop(func() bool {
		treetest.WithClass(v.capture, "audio-capture-action")[0].(*widget.Button).ClickAt(widget.Point{})
		return true
	})
	clickOnly(t, v.capture, "audio-capture-confirm")
	if !slices.Equal(*signals, []syscall.Signal{syscall.SIGTERM}) {
		t.Errorf("confirming sent %v, want one SIGTERM", *signals)
	}
	if n := countClass(v.capture, "audio-capture-confirm") + countClass(v.capture, "audio-capture-failed"); n != 0 {
		t.Errorf("the row still asks or fails after a sent signal (%d)", n)
	}
}

func TestAPromptEndsWithItsCamera(t *testing.T) {
	v, cams, signals := cameraDropdown(t, nil)
	clickOnly(t, v.capture, "audio-capture-action")
	cams.set()
	waitHeadless(t, "the camera gone", func() bool { return !v.capture.Visible() })
	cams.set(privacy.CameraUser{PID: 777, Name: "zoom", Device: "/dev/video0"})
	waitHeadless(t, "the camera back", func() bool { return slices.Contains(treetest.Labels(v.capture), "zoom") })
	if n := countClass(v.capture, "audio-capture-confirm"); n != 0 {
		t.Errorf("%d confirm buttons on a camera that came back, want the old prompt gone", n)
	}
	if len(*signals) != 0 {
		t.Errorf("sent %v, want nothing", *signals)
	}
}

func TestAFailedEndShowsInTheDropdown(t *testing.T) {
	v, _, signals := cameraDropdown(t, syscall.EPERM)
	clickOnly(t, v.capture, "audio-capture-action")
	clickOnly(t, v.capture, "audio-capture-confirm")
	if len(*signals) != 1 {
		t.Fatalf("sent %v, want one signal", *signals)
	}
	want := i18n.T("dropdown-privacy-end-failed", i18n.Str("app", "zoom"), i18n.Str("error", syscall.EPERM.Error()))
	if l := onHeadlessLoop(func() []string { return treetest.Labels(v.capture) }); !slices.Contains(l, want) {
		t.Errorf("labels %q, want the failure %q", l, want)
	}
	if n := countClass(v.capture, "audio-capture-failed"); n != 1 {
		t.Errorf("%d failed rows, want the camera's", n)
	}
	clickOnly(t, v.capture, "audio-capture-action")
	if n := countClass(v.capture, "audio-capture-confirm"); n != 1 {
		t.Errorf("%d confirm buttons after End on a failed row, want it to ask again", n)
	}
	if len(*signals) != 1 {
		t.Errorf("End on a failed row sent %v, want no new signal before it confirms", *signals)
	}
}

func TestOnlyEndAndStopAreDestructive(t *testing.T) {
	ctx, srv, cams, shares := captureFixture(t)
	v := onHeadlessLoop(func() *audioView { return audioDropdown(ctx).(*audioView) })
	defer onHeadlessLoop(func() bool { v.dropdownClosed(); return true })
	muted := recording(42, 3, "Firefox", 4343)
	muted.Mute = true
	srv.PutSourceOutput(muted)
	cams.set(privacy.CameraUser{PID: 777, Name: "zoom", Device: "/dev/video0"})
	shares.set(chromeShare)
	waitHeadless(t, "every capture listed", func() bool { return len(treetest.WithClass(v.capture, "audio-capture-action")) == 3 })
	destructive := onHeadlessLoop(func() []bool {
		var out []bool
		for _, b := range treetest.WithClass(v.capture, "audio-capture-action") {
			out = append(out, widget.HasClass(b, css.DestructiveAction))
		}
		return out
	})
	if !slices.Equal(destructive, []bool{false, true, true}) {
		t.Errorf("destructive (Unmute, End, Stop) = %v, want only End and Stop", destructive)
	}
}

type stopGate struct {
	calls     atomic.Int32
	answers   chan error
	deadlines chan time.Duration
}

func newStopGate() *stopGate {
	return &stopGate{answers: make(chan error), deadlines: make(chan time.Duration, 8)}
}

func (g *stopGate) wait(ctx context.Context) error {
	g.calls.Add(1)
	if dl, ok := ctx.Deadline(); ok {
		g.deadlines <- time.Until(dl)
	} else {
		g.deadlines <- -1
	}
	select {
	case err := <-g.answers:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

type gatedShares struct {
	shareSource
	gate *stopGate
}

func (g gatedShares) Stop(ctx context.Context, session dbus.ObjectPath) error {
	if err := g.gate.wait(ctx); err != nil {
		return err
	}
	return g.shareSource.Stop(ctx, session)
}

type toasts struct {
	mu     sync.Mutex
	labels []string
}

func (r *toasts) record(label, _ string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.labels = append(r.labels, label)
}

func (r *toasts) list() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.labels)
}

func checkStopButton(t *testing.T, root widget.Widget, rebuild func(), class string, gate *stopGate, shown *toasts, failed, rowLabel string) {
	t.Helper()
	button := func() *widget.Button {
		found := treetest.WithClass(root, class)
		if len(found) != 1 {
			t.Fatalf("%d widgets of class %s, want one", len(found), class)
		}
		return found[0].(*widget.Button)
	}
	clickOnly(t, root, class)
	waitHeadless(t, "the stop started", func() bool { return gate.calls.Load() == 1 })
	if d := <-gate.deadlines; d <= 0 || d > offLoopTimeout {
		t.Errorf("the stop runs with %v left, want a deadline within %v", d, offLoopTimeout)
	}
	if onHeadlessLoop(func() bool { return widget.IsEnabled(button()) }) {
		t.Error("the button is enabled while its stop runs")
	}
	onHeadlessLoop(func() bool { rebuild(); return true })
	if onHeadlessLoop(func() bool { return widget.IsEnabled(button()) }) {
		t.Error("the rebuilt button is enabled while its stop runs")
	}
	onHeadlessLoop(func() bool { button().ClickAt(widget.Point{}); return true })
	if n := gate.calls.Load(); n != 1 {
		t.Errorf("%d stops after a second click while pending, want one", n)
	}
	gate.answers <- errors.New("no reply")
	waitHeadless(t, "the failure toasted", func() bool { return len(shown.list()) == 1 })
	if got := shown.list(); got[0] != failed {
		t.Errorf("toasted %q, want %q", got[0], failed)
	}
	waitHeadless(t, "the button enabled again", func() bool { return widget.IsEnabled(button()) })
	if n := gate.calls.Load(); n != 1 {
		t.Fatalf("%d stops once the first failed, want the click while pending to have started none", n)
	}

	clickOnly(t, root, class)
	waitHeadless(t, "the second stop started", func() bool { return gate.calls.Load() == 2 })
	<-gate.deadlines
	gate.answers <- nil
	waitHeadless(t, "the stopped row gone", func() bool { return !slices.Contains(treetest.Labels(root), rowLabel) })
	if got := shown.list(); len(got) != 1 {
		t.Errorf("toasts %q, want none for the success", got)
	}
}

func TestAScreenShareStopIsPendingToastsAndRetries(t *testing.T) {
	ctx, _, _, shares := captureFixture(t)
	gate := newStopGate()
	shown := &toasts{}
	ctx.ScreenShares = gatedShares{shareSource: ctx.ScreenShares, gate: gate}
	ctx.Toast = shown.record
	v := onHeadlessLoop(func() *audioView { return audioDropdown(ctx).(*audioView) })
	defer onHeadlessLoop(func() bool { v.dropdownClosed(); return true })
	shares.set(chromeShare)
	waitHeadless(t, "the share listed", func() bool { return slices.Contains(treetest.Labels(v.capture), "org.chromium.Chromium") })
	failed := i18n.T("dropdown-screen-share-stop-failed", i18n.Str("app", "org.chromium.Chromium"), i18n.Str("error", "no reply"))
	checkStopButton(t, v.capture, v.capture.render, "audio-capture-action", gate, shown, failed, "org.chromium.Chromium")
	if got := shares.stoppedSessions(); !slices.Equal(got, []dbus.ObjectPath{chromeShare.Session}) {
		t.Errorf("stopped %v, want the share once", got)
	}
}

func TestShareDescriptions(t *testing.T) {
	other := screenshare.Share{
		Session: "/s/2", Kind: screenshare.KindScreenCast,
		Targets: []screenshare.Target{{Kind: screenshare.TargetWindow, Name: "lys-wm-3", Title: "Notes"}},
	}
	if got := shareTargets(other); got != "Window \u201c\u2068Notes\u2069\u201d" {
		t.Errorf("window target %q, want its title", got)
	}
	if got := shareApp(other); got != "An app" {
		t.Errorf("an app without an id reads %q", got)
	}
	if got := shareTargets(chromeShare); !strings.Contains(got, "DP-1") {
		t.Errorf("screen target %q, want its connector", got)
	}
}
