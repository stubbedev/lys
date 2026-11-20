package settings

import (
	"context"
	"encoding/binary"
	"math"
	"os"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/pulse"
	"github.com/stubbedev/lys/desktop/shell/apptheme"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/pulse/native"
	"github.com/stubbedev/lys/lib/pulse/pulsetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	headsetSink   = "bluez_sink.00_11"
	headsetCardID = 7
)

func headsetCard(active string) native.CardInfo {
	return native.CardInfo{
		Index: headsetCardID, Name: "bluez_card.00_11", OwnerModule: native.InvalidIndex, Driver: "module-bluez5-device.c",
		Profiles: []native.CardProfile{
			{Name: "a2dp-sink", Description: "High Fidelity Playback (A2DP Sink)", Sinks: 1, Priority: 40, Available: true},
			{Name: "headset-head-unit", Description: "Headset Head Unit (HSP/HFP)", Sinks: 1, Sources: 1, Priority: 30, Available: true},
			{Name: "off", Description: "Off", Available: true},
		},
		ActiveProfile: active,
		Props:         native.PropList{"device.description": "Headset"},
	}
}

func firefox() native.StreamInfo {
	return native.StreamInfo{
		Index: 40, Name: "Playback", OwnerModule: native.InvalidIndex, Client: 12, Device: 1,
		SampleSpec: pulsetest.StereoSpec(), ChannelMap: pulsetest.StereoMap(), Volume: native.CVolume{native.VolumeNorm, native.VolumeNorm},
		Props:     native.PropList{"application.name": "Firefox", "media.title": "Song", "application.icon_name": "firefox"},
		HasVolume: true, VolumeWritable: true,
		Format: native.FormatInfo{Encoding: native.EncodingPCM, Props: native.PropList{}},
	}
}

func soundServer(t *testing.T) *pulsetest.Server {
	t.Helper()
	srv := pulsetest.New(t)
	srv.PutSink(pulsetest.Sink(1, "speakers", "Speakers", 2))
	headset := pulsetest.Sink(5, headsetSink, "Headset", 6)
	headset.Card, headset.Ports, headset.ActivePort = headsetCardID, nil, ""
	srv.PutSink(headset)
	srv.PutSource(pulsetest.Source(2, "speakers.monitor", "Monitor of Speakers", 1, "speakers"))
	srv.PutSource(pulsetest.Source(6, headsetSink+".monitor", "Monitor of Headset", 5, headsetSink))
	srv.PutSource(pulsetest.Source(10, "mic", "Microphone", native.InvalidIndex, ""))
	srv.PutCard(headsetCard("a2dp-sink"))
	srv.PutSinkInput(firefox())
	srv.SetDefaults(headsetSink, "mic")
	return srv
}

func useSoundServer(t *testing.T, srv *pulsetest.Server) {
	t.Helper()
	before := openPulse
	openPulse = func(ctx context.Context) (*pulse.Service, error) { return pulse.Connect(ctx, srv.Addr()) }
	t.Cleanup(func() { openPulse = before })
}

type soundRig struct {
	srv *pulsetest.Server
	p   *soundPanel
	l   *loop
}

func startSound(t *testing.T) *soundRig {
	t.Helper()
	return startSoundOn(t, soundServer(t))
}

func startSoundOn(t *testing.T, srv *pulsetest.Server) *soundRig {
	t.Helper()
	useSoundServer(t, srv)
	k := testKit(t, "")
	l := &loop{t: t, queue: make(chan func(), 64)}
	k.invoke = func(fn func()) { l.queue <- fn }
	p := soundLead(k).(*soundPanel)
	t.Cleanup(p.close)
	return &soundRig{srv: srv, p: p, l: l}
}

func calls(srv *pulsetest.Server, cmd uint32) int {
	n := 0
	for _, c := range srv.Calls() {
		if c == cmd {
			n++
		}
	}
	return n
}

func openDeviceMenu(t *testing.T, r *soundRig, c *deviceControls) []widget.MenuItem {
	t.Helper()
	root, host := hosted(r.p)
	root.Arrange(render.Rect{W: 800, H: 3000})
	if !c.more.Visible() {
		t.Fatal("the device has no port or profile menu")
	}
	c.more.OnClick()
	if len(host.menus) != 1 {
		t.Fatalf("the … button opened %d menus", len(host.menus))
	}
	return host.menus[0]
}

func menuRow(t *testing.T, items []widget.MenuItem, label string) widget.MenuItem {
	t.Helper()
	for _, it := range items {
		if it.Label == label {
			return it
		}
	}
	t.Fatalf("no menu row %q", label)
	return widget.MenuItem{}
}

func checkedRows(items []widget.MenuItem) []string {
	var out []string
	for _, it := range items {
		if it.Kind == widget.ItemRadio && it.Checked {
			out = append(out, it.Label)
		}
	}
	return out
}

func TestChoosingAProfileIsOneSetCardProfileAndThePageFollowsTheServer(t *testing.T) {
	r := startSound(t)
	items := openDeviceMenu(t, r, r.p.output)
	tr := locale.T()
	if got := checkedRows(items); !slices.Equal(got, []string{"High Fidelity Playback (A2DP Sink)"}) {
		t.Fatalf("checked rows %q", got)
	}
	if slices.ContainsFunc(items, func(it widget.MenuItem) bool { return it.Label == tr.Get("settings-sound-port") }) {
		t.Fatal("a device without ports lists a port group")
	}
	menuRow(t, items, "Headset Head Unit (HSP/HFP)").Run()()
	if n := calls(r.srv, native.CmdSetCardProfile); n != 1 {
		t.Fatalf("%d SET_CARD_PROFILE for one choice", n)
	}
	if card, _ := r.srv.Card(headsetCardID); card.ActiveProfile != "headset-head-unit" {
		t.Fatalf("the server's card is on %q", card.ActiveProfile)
	}
	r.srv.PutCard(headsetCard("off"))
	r.l.until("the server's profile change shown", func() bool { return r.p.output.profile == "off" })
	if got := checkedRows(deviceMenu(r.p.output.ports, r.p.output.port, r.p.output.profiles, r.p.output.profile, nil, nil)); !slices.Equal(got, []string{"Off"}) {
		t.Fatalf("checked rows after the server's change %q", got)
	}
	if n := calls(r.srv, native.CmdSetCardProfile); n != 1 {
		t.Fatalf("following the server sent %d more SET_CARD_PROFILE", n-1)
	}
}

func TestADeviceWithOnePortAndNoCardHasNoMenu(t *testing.T) {
	srv := soundServer(t)
	plain := pulsetest.Sink(5, headsetSink, "Headset", 6)
	plain.Card, plain.Ports, plain.ActivePort = native.InvalidIndex, plain.Ports[:1], plain.Ports[0].Name
	srv.PutSink(plain)
	r := startSoundOn(t, srv)
	r.l.until("the headset shown", func() bool { return r.p.output.current.Name == headsetSink })
	if r.p.output.more.Visible() {
		t.Fatalf("a device with ports %v and profiles %v offers a menu", r.p.output.ports, r.p.output.profiles)
	}
	if items := deviceMenu([]deviceOption{{"a", "A"}}, "a", nil, "", nil, nil); len(items) != 0 {
		t.Fatalf("one port makes menu rows %+v", items)
	}
}

func TestAHeadsetSwitchedToHFPAppearsInTheInputList(t *testing.T) {
	r := startSound(t)
	if got := r.p.input.devices.Items(); !slices.Equal(got, []string{"Microphone"}) {
		t.Fatalf("inputs %q", got)
	}
	menuRow(t, openDeviceMenu(t, r, r.p.output), "Headset Head Unit (HSP/HFP)").Run()()
	source := pulsetest.Source(8, "bluez_source.00_11", "Headset Microphone", native.InvalidIndex, "")
	source.Card = headsetCardID
	r.srv.PutSource(source)
	r.l.until("the headset's microphone listed", func() bool {
		return slices.Contains(r.p.input.devices.Items(), "Headset Microphone")
	})
}

func TestTheLevelMeterRecordsOnlyWhileThePageIsShown(t *testing.T) {
	srv := soundServer(t)
	useSoundServer(t, srv)
	k := testKit(t, "")
	l := &loop{t: t, queue: make(chan func(), 64)}
	k.invoke = func(fn func()) { l.queue <- fn }
	w := soundWindow(t, k)
	if len(srv.Records()) != 0 {
		t.Fatal("a capture before the Sound page was shown")
	}
	w.show(soundID)
	rec := srv.WaitRecords(t, 1)[0]
	if rec.Source != "mic" || rec.Props["media.name"] != meterSpec.Name {
		t.Fatalf("recording %q as %q", rec.Source, rec.Props["media.name"])
	}
	l.settle()
	w.show("fonts")
	srv.WaitRecords(t, 0)
	l.settle()
	w.show(soundID)
	srv.WaitRecords(t, 1)
}

func soundWindow(t *testing.T, k *kit) *window {
	t.Helper()
	w := newWindow(k, k.store.svc.Config(), func(_ time.Duration, fn func()) { fn() })
	w.stack.SetTransition(widget.StackNone, 0)
	t.Cleanup(func() {
		for _, p := range w.pages {
			p.close()
		}
	})
	return w
}

func pcm(amplitude int16, frames int) []byte {
	samples := make([]int16, frames)
	for i := range samples {
		samples[i] = amplitude
		if i%2 == 1 {
			samples[i] = -amplitude
		}
	}
	out := make([]byte, 2*frames)
	if _, err := binary.Encode(out, binary.LittleEndian, samples); err != nil {
		panic(err)
	}
	return out
}

func paintOnce(w widget.Widget) {
	w.Arrange(render.Rect{W: 200, H: 20})
	buf := make([]byte, render.Stride(200)*20)
	w.Paint(render.New(buf, render.Stride(200), 200, 20))
}

func TestTheLevelMeterRedrawsOncePerFrameWhateverTheSampleRate(t *testing.T) {
	r := startSound(t)
	rec := r.srv.WaitRecords(t, 1)[0]
	m := r.p.meter
	const chunks = 200
	for i := range chunks {
		rec.Push(pcm(int16(1000+100*i), 400))
	}
	r.l.until("the first redraw", func() bool { return m.redraws == 1 })
	r.l.drain()
	if m.redraws != 1 {
		t.Fatalf("%d chunks with no frame painted redrew %d times", chunks, m.redraws)
	}
	paintOnce(m)
	for i := range chunks {
		rec.Push(pcm(int16(20000+10*i), 400))
	}
	r.l.until("the redraw the painted frame lets through", func() bool { return m.redraws == 2 })
	r.l.drain()
	if m.redraws != 2 {
		t.Fatalf("%d chunks after one painted frame redrew %d times, want 2", chunks, m.redraws)
	}
	paintOnce(m)
	r.l.until("the loud level shown", func() bool { return m.Value() > 0.9 })
}

func TestATestTonePlaysOnTheChosenChannelOfTheDefaultOutput(t *testing.T) {
	r := startSound(t)
	treetest.Button(t, r.p.output.tests, locale.T().Get("settings-sound-test-channel", i18n.Str("channel", "Front Right"))).OnClick()
	plays := r.srv.Plays()
	if len(plays) != 1 || plays[0].Sink != headsetSink || plays[0].Volume != native.VolumeNorm {
		t.Fatalf("plays %+v", plays)
	}
	sample := r.srv.Samples()[plays[0].Name]
	samples := make([]int16, len(sample.Data)/2)
	if _, err := binary.Decode(sample.Data, binary.LittleEndian, samples); err != nil {
		t.Fatal(err)
	}
	var left, right int
	for i := 0; i+1 < len(samples); i += 2 {
		left = max(left, int(math.Abs(float64(samples[i]))))
		right = max(right, int(math.Abs(float64(samples[i+1]))))
	}
	if left != 0 || right == 0 || len(samples) != 2*toneFrames {
		t.Fatalf("left peak %d, right peak %d over %d samples", left, right, len(samples))
	}
}

func TestPortsVolumeMuteAndAppsReachTheServer(t *testing.T) {
	r := startSound(t)
	r.p.output.devices.SetSelected(0)
	r.l.until("the speakers as the output", func() bool { return r.p.output.current.Name == "speakers" })
	items := openDeviceMenu(t, r, r.p.output)
	if got := checkedRows(items); !slices.Equal(got, []string{"Speaker"}) {
		t.Fatalf("checked ports %q", got)
	}
	menuRow(t, items, locale.T().Get("settings-sound-unplugged", i18n.Str("name", "Headphones"))).Run()()
	if d, _ := r.srv.Sink(1); d.ActivePort != "headphones" {
		t.Fatalf("the sink's port is %q", d.ActivePort)
	}
	r.p.output.mute.OnClick()
	if d, _ := r.srv.Sink(1); !d.Mute {
		t.Fatal("the sink was not muted")
	}
	if !r.p.output.mute.Active() || r.p.output.mute.glyph.Name() != speakerGlyphs.muted {
		t.Fatalf("the muted output shows %q, pressed %v", r.p.output.mute.glyph.Name(), r.p.output.mute.Active())
	}
	r.p.output.mute.OnClick()
	if d, _ := r.srv.Sink(1); d.Mute {
		t.Fatal("the sink stayed muted")
	}
	if r.p.output.mute.Active() || r.p.output.mute.glyph.Name() != speakerGlyphs.sounding {
		t.Fatalf("the unmuted output shows %q, pressed %v", r.p.output.mute.glyph.Name(), r.p.output.mute.Active())
	}
	r.p.output.balance.Knob.SetValue(-50)
	r.l.until("the balance written", func() bool {
		d, _ := r.srv.Sink(1)
		return d.Volume[0] == 0x8000 && d.Volume[1] == 0x4000
	})
	row := r.p.playing.rows[pulse.StreamKey{Index: 40, Type: pulse.StreamPlayback}]
	if row.title.Text() != "Firefox" || row.detail.Text() != "Song" || row.icon.Name() != "firefox" {
		t.Fatalf("app row %q, %q with icon %q", row.title.Text(), row.detail.Text(), row.icon.Name())
	}
	row.mute.OnClick()
	if st, _ := r.srv.SinkInput(40); !st.Mute {
		t.Fatal("the app was not muted")
	}
}

func TestBalanceKeepsTheLouderSideAndScalesTheOther(t *testing.T) {
	stereo := pulsetest.StereoMap()
	if got := withBalance([]float64{0.5, 0.5}, stereo, -0.5); !slices.Equal(got, []float64{0.5, 0.25}) {
		t.Fatalf("balance -0.5 = %v", got)
	}
	if b, ok := balanceOf([]float64{0.5, 0.25}, stereo); !ok || b != -0.5 {
		t.Fatalf("balance of 0.5/0.25 = %v, %v", b, ok)
	}
	if _, ok := balanceOf([]float64{1}, native.ChannelMap{native.ChannelMono}); ok {
		t.Fatal("a mono device has a balance")
	}
	if got := scaledTo([]float64{0.5, 0.25}, 1); !slices.Equal(got, []float64{1, 0.5}) {
		t.Fatalf("scaled = %v", got)
	}
}

func shownControls(root widget.Widget) []widget.Widget {
	hidden := map[widget.Widget]bool{}
	treetest.Walk(root, func(w widget.Widget) {
		if v, ok := w.(interface{ Visible() bool }); ok && !v.Visible() {
			treetest.Walk(w, func(x widget.Widget) { hidden[x] = true })
		}
	})
	var out []widget.Widget
	controlRoles := []widget.Role{widget.RoleComboBox, widget.RoleSwitch, widget.RoleButton, widget.RoleToggleButton, widget.RoleSlider}
	treetest.Walk(root, func(w widget.Widget) {
		r, ok := w.(interface{ Role() widget.Role })
		if !ok || !slices.Contains(controlRoles, r.Role()) {
			return
		}
		if !hidden[w] && widget.IsEnabled(w) {
			out = append(out, w)
		}
	})
	return out
}

func TestTabReachesEverySoundControl(t *testing.T) {
	srv := soundServer(t)
	useSoundServer(t, srv)
	k := testKit(t, "")
	l := &loop{t: t, queue: make(chan func(), 64)}
	k.invoke = func(fn func()) { l.queue <- fn }
	cfg := k.store.svc.Config()
	w := soundWindow(t, k)
	apptheme.New(cfg).Attach(w.root)
	w.show(soundID)
	panel := treetest.First[*soundPanel](t, w.page)
	l.until("the devices shown", func() bool { return len(panel.output.devices.Items()) == 2 })
	w.root.Arrange(render.Rect{W: 1400, H: 6000})
	router := &widget.Router{Root: w.root}
	if err := router.SetFocus(w.search); err != nil {
		t.Fatal(err)
	}
	visited := map[widget.Widget]bool{}
	for range maxTabStops {
		router.FocusNext()
		f := router.Focused()
		if f == widget.Widget(w.search) {
			break
		}
		visited[f] = true
	}
	controls := shownControls(panel)
	if len(controls) < 10 {
		t.Fatalf("only %d controls on the Sound page", len(controls))
	}
	for _, c := range controls {
		reached := false
		treetest.Walk(c, func(x widget.Widget) { reached = reached || visited[x] })
		if !reached {
			name, _ := c.(interface{ AccessibleName() string })
			t.Errorf("%T %q is out of Tab's reach", c, name.AccessibleName())
		}
	}
}

func TestAMutedServerShowsTheCrossedOutGlyphs(t *testing.T) {
	srv := soundServer(t)
	mic := pulsetest.Source(10, "mic", "Microphone", native.InvalidIndex, "")
	mic.Mute = true
	srv.PutSource(mic)
	r := startSoundOn(t, srv)
	r.l.until("the muted input shown", func() bool { return r.p.input.mute.Active() })
	if r.p.input.mute.glyph.Name() != micGlyphs.muted {
		t.Fatalf("the muted input shows %q", r.p.input.mute.glyph.Name())
	}
	if r.p.output.mute.Active() || r.p.output.mute.glyph.Name() != speakerGlyphs.sounding {
		t.Fatalf("the sounding output shows %q, pressed %v", r.p.output.mute.glyph.Name(), r.p.output.mute.Active())
	}
	if s, _ := srv.Source(10); !s.Mute {
		t.Fatal("showing the muted input unmuted it on the server")
	}
}

func TestAnAppWithoutAnIconShowsTheGenericAppIcon(t *testing.T) {
	if got := streamIcon(pulse.AudioStream{}); got != appFallbackIcon {
		t.Errorf("a stream without an icon shows %q", got)
	}
	if got := streamIcon(pulse.AudioStream{Media: pulse.MediaInfo{IconName: "mpv"}}); got != "mpv" {
		t.Errorf("a stream naming mpv's icon shows %q", got)
	}
}

func recorder(index, source uint32, name string) native.StreamInfo {
	st := firefox()
	st.Index, st.Device, st.Name = index, source, "Recording"
	st.Props = native.PropList{"application.name": name, "application.process.id": "1", "application.icon_name": "audio-recorder"}
	return st
}

func TestAnAppIsMovedToAnotherOutputFromItsRow(t *testing.T) {
	r := startSound(t)
	row := r.p.playing.rows[pulse.StreamKey{Index: 40, Type: pulse.StreamPlayback}]
	if !row.route.Visible() || row.route.Selection() != "Speakers" {
		t.Fatalf("the route shows %q, visible %v", row.route.Selection(), row.route.Visible())
	}
	row.route.SetSelected(slices.Index(row.route.Items(), "Headset"))
	r.l.until("the stream moved on the server", func() bool {
		st, _ := r.srv.SinkInput(40)
		return st.Device == 5
	})
	if n := calls(r.srv, native.CmdMoveSinkInput); n != 1 {
		t.Fatalf("%d MOVE_SINK_INPUT for one choice", n)
	}
	r.l.until("the move shown", func() bool { return row.route.Selection() == "Headset" })
	if n := calls(r.srv, native.CmdMoveSinkInput); n != 1 {
		t.Fatalf("following the server sent %d more MOVE_SINK_INPUT", n-1)
	}
}

func TestOneOutputOffersNoRoute(t *testing.T) {
	srv := soundServer(t)
	srv.RemoveSink(5)
	srv.RemoveSource(6)
	srv.SetDefaults("speakers", "mic")
	r := startSoundOn(t, srv)
	r.l.until("one output listed", func() bool { return len(r.p.output.devices.Items()) == 1 })
	if row := r.p.playing.rows[pulse.StreamKey{Index: 40, Type: pulse.StreamPlayback}]; row.route.Visible() {
		t.Fatalf("a route among %q is offered", row.route.Items())
	}
}

func TestTheAppsRecordingTheMicrophoneAreListedAndMuted(t *testing.T) {
	srv := soundServer(t)
	srv.PutSourceOutput(recorder(60, 10, "Recorder"))
	srv.PutSourceOutput(recorder(61, 2, "Visualizer"))
	paused := recorder(62, 10, "Paused")
	paused.Corked = true
	srv.PutSourceOutput(paused)
	r := startSoundOn(t, srv)
	key := pulse.StreamKey{Index: 60, Type: pulse.StreamRecord}
	r.l.until("the recorder listed", func() bool { return r.p.recording.rows[key] != nil })
	if got := r.p.recording.keys; !slices.Equal(got, []pulse.StreamKey{key}) {
		t.Fatalf("recording streams %v, want the microphone's running one alone", got)
	}
	row := r.p.recording.rows[key]
	if row.title.Text() != "Recorder" || row.icon.Name() != "audio-recorder" || row.mute.glyph.Name() != micGlyphs.sounding || r.p.recording.empty.Visible() {
		t.Fatalf("row %q with icon %q and glyph %q, empty shown %v", row.title.Text(), row.icon.Name(), row.mute.glyph.Name(), r.p.recording.empty.Visible())
	}
	row.mute.OnClick()
	if st, _ := srv.SourceOutput(60); !st.Mute {
		t.Fatal("the recorder was not muted")
	}
	srv.RemoveSourceOutput(60)
	r.l.until("the recorder gone", func() bool { return r.p.recording.empty.Visible() && len(r.p.recording.keys) == 0 })
}

func TestTheSettingsOwnLevelMeterIsNotAnAppRecording(t *testing.T) {
	srv := soundServer(t)
	meter := recorder(63, 10, "lys-settings")
	meter.Props["application.process.id"] = strconv.Itoa(os.Getpid())
	srv.PutSourceOutput(meter)
	srv.PutSourceOutput(recorder(64, 10, "Recorder"))
	r := startSoundOn(t, srv)
	want := []pulse.StreamKey{{Index: 64, Type: pulse.StreamRecord}}
	r.l.until("the recorder listed", func() bool { return slices.Equal(r.p.recording.keys, want) })
}
