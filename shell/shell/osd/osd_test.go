package osd

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/service/pulse"
	"github.com/stubbedev/lys/desktop/shell/osdview"
	"github.com/stubbedev/lys/desktop/shell/reveal"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/internal/widgetipc"
	"github.com/stubbedev/lys/shell/service/brightness"
	"github.com/stubbedev/lys/shell/service/keytoggles"
)

// fakeWindow records what the OSD asks of its surface.
type fakeWindow struct {
	cfg     app.LayerConfig
	anchors []app.Anchor
	margins []app.Margins
	layers  []app.Layer
	closed  bool
}

func (w *fakeWindow) Close()                        { w.closed = true }
func (w *fakeWindow) SetAnchor(a app.Anchor) error  { w.anchors = append(w.anchors, a); return nil }
func (w *fakeWindow) SetMargin(m app.Margins) error { w.margins = append(w.margins, m); return nil }
func (w *fakeWindow) SetLayer(l app.Layer) error    { w.layers = append(w.layers, l); return nil }

// timer is one pending After.
type timer struct {
	d  time.Duration
	fn func()
}

// harness runs the OSD synchronously: Invoke is a call, After queues.
type harness struct {
	osd     *Osd
	outputs []*app.Output
	opened  []*fakeWindow
	timers  []timer
}

func newHarness(t *testing.T, cfg *config.Config) *harness {
	t.Helper()
	h := &harness{outputs: []*app.Output{{Name: "DP-1"}, {Name: "HDMI-A-1"}}}
	h.osd = New(Deps{
		Open: func(lc app.LayerConfig) (Window, error) {
			w := &fakeWindow{cfg: lc}
			h.opened = append(h.opened, w)
			return w, nil
		},
		Outputs: func() []*app.Output { return h.outputs },
		Invoke:  func(fn func()) { fn() },
		After:   func(d time.Duration, fn func()) { h.timers = append(h.timers, timer{d, fn}) },
		Font:    treetest.Font(t),
	}, cfg)
	return h
}

// fire runs the queued timer of duration d (the first one).
func (h *harness) fire(t *testing.T, d time.Duration) {
	t.Helper()
	i := slices.IndexFunc(h.timers, func(tm timer) bool { return tm.d == d })
	if i < 0 {
		t.Fatalf("no %v timer queued (have %v)", d, h.timers)
	}
	tm := h.timers[i]
	h.timers = slices.Delete(h.timers, i, i+1)
	tm.fn()
}

func quietConfig() *config.Config {
	cfg := config.Defaults()
	cfg.Animations.Enabled = false
	return cfg
}

// shownLabels are the texts of the mapped OSD's labels.
func shownLabels(h *harness) []string {
	return treetest.Labels(h.osd.face.view.Root())
}

func readyHarness(t *testing.T, cfg *config.Config) *harness {
	t.Helper()
	h := newHarness(t, cfg)
	h.fire(t, readyDelay)
	return h
}

// The start-up levels are not events: nothing maps inside the quiet
// period, and the first event after it does.
func TestQuietPeriodDropsEvents(t *testing.T) {
	h := newHarness(t, quietConfig())
	h.osd.Show(osdview.Slider("Speakers", osdview.VolumeIcon(40, false), 40, false))
	if len(h.opened) != 0 {
		t.Fatal("an event inside the quiet period mapped the OSD")
	}
	h.fire(t, readyDelay)
	h.osd.Show(osdview.Slider("Speakers", osdview.VolumeIcon(40, false), 40, false))
	if len(h.opened) != 1 {
		t.Fatalf("%d surfaces after the quiet period, want 1", len(h.opened))
	}
	lc := h.opened[0].cfg
	if lc.Namespace != "lys-shell-osd" || lc.Keyboard != app.KeyboardNone || lc.Output != h.outputs[0] {
		t.Errorf("surface %+v, want lys-shell-osd on the primary without keyboard", lc)
	}
	if lc.Width != 0 || lc.Height != 0 {
		t.Errorf("surface sized %dx%d, want content-sized", lc.Width, lc.Height)
	}
}

func TestDisabledOsdNeverShows(t *testing.T) {
	cfg := quietConfig()
	cfg.Osd.Enabled = false
	h := readyHarness(t, cfg)
	h.osd.Show(osdview.Slider("Speakers", "", 40, false))
	h.osd.ShowToast(widgetipc.ToastRequest{Label: new("hi")})
	if len(h.opened) != 0 {
		t.Error("a disabled OSD mapped")
	}
}

// The surface goes to osd.monitor, falling back to the primary when the
// connector is gone, and moves when the monitor changes.
func TestOsdFollowsTheConfiguredMonitor(t *testing.T) {
	cfg := quietConfig()
	cfg.Osd.Monitor = config.OsdOnConnector("HDMI-A-1")
	h := readyHarness(t, cfg)
	h.osd.Show(osdview.Slider("x", "", 1, false))
	if h.opened[0].cfg.Output != h.outputs[1] {
		t.Fatalf("opened on %s, want HDMI-A-1", h.opened[0].cfg.Output.Name)
	}
	next := quietConfig()
	next.Osd.Monitor = config.OsdOnConnector("DP-9")
	h.osd.SetConfig(next)
	if !h.opened[0].closed || len(h.opened) != 2 || h.opened[1].cfg.Output != h.outputs[0] {
		t.Errorf("a missing connector must move the open OSD to the primary")
	}
}

// A second event reuses the mapped surface and re-applies its anchor,
// margin, and layer; a config change
// re-anchors it.
func TestOneSurfaceReanchored(t *testing.T) {
	h := readyHarness(t, quietConfig())
	h.osd.Show(osdview.Slider("a", "", 1, false))
	h.osd.Show(osdview.Slider("b", "", 2, false))
	if len(h.opened) != 1 {
		t.Fatalf("%d surfaces, want one reused", len(h.opened))
	}
	next := quietConfig()
	next.Osd.Position = config.ScreenTopRight
	next.Styling.Scale = 2
	h.osd.SetConfig(next)
	w := h.opened[0]
	if got := w.anchors[len(w.anchors)-1]; got != app.AnchorTop|app.AnchorRight {
		t.Errorf("re-anchored to %v, want top|right", got)
	}
	if got := w.margins[len(w.margins)-1]; got != (app.Margins{Top: 300, Right: 300}) {
		t.Errorf("margins %+v, want 150px x scale 2 on the anchored edges", got)
	}
}

func TestTheGenieCollapsesTowardTheEdgeTheOsdSitsAt(t *testing.T) {
	for pos, want := range map[config.ScreenPosition]widget.Edge{
		config.ScreenTopLeft:      widget.EdgeTop,
		config.ScreenTopCenter:    widget.EdgeTop,
		config.ScreenTopRight:     widget.EdgeTop,
		config.ScreenCenterLeft:   widget.EdgeLeft,
		config.ScreenCenterRight:  widget.EdgeRight,
		config.ScreenBottomLeft:   widget.EdgeBottom,
		config.ScreenBottomCenter: widget.EdgeBottom,
		config.ScreenBottomRight:  widget.EdgeBottom,
	} {
		if got := genieEdge(pos); got != want {
			t.Errorf("%s collapses toward %v, want %v", pos, got, want)
		}
	}
}

func TestMarginsInsetOnlyTheAnchoredEdges(t *testing.T) {
	cfg := config.Defaults()
	cfg.Osd.Position = config.ScreenBottomLeft
	cfg.Osd.Margin = config.Size{Value: 10.7, Unit: config.SizePixels}
	if got := margins(cfg); got != (app.Margins{Bottom: 10, Left: 10}) {
		t.Errorf("margins %+v, want 10.7px truncated on bottom and left only", got)
	}
}

// A toast's duration overrides osd.duration; only the latest event's
// timer dismisses, and the dismissal unmaps after the exit.
func TestDismissFollowsTheLatestEvent(t *testing.T) {
	h := readyHarness(t, quietConfig())
	h.osd.ShowToast(widgetipc.ToastRequest{Label: new("saved"), DurationMS: new(uint32(900))})
	h.osd.Show(osdview.Slider("a", "", 1, false))
	h.fire(t, 900*time.Millisecond)
	if h.opened[0].closed {
		t.Fatal("a superseded event's timer dismissed the OSD")
	}
	h.fire(t, time.Duration(config.DefaultsOsd().Duration)*time.Millisecond)
	if !h.opened[0].closed {
		t.Error("the latest event's timer did not dismiss the OSD")
	}
	h.osd.Show(osdview.Slider("b", "", 1, false))
	if len(h.opened) != 2 {
		t.Error("an event after the dismissal did not map a fresh surface")
	}
}

// Presets supply the label and icon; request fields override them; an
// unknown preset still shows the request's fields.
func TestToastEvent(t *testing.T) {
	osd := config.DefaultsOsd()
	osd.Presets = []config.ToastPreset{{ID: "saved", Label: new("Saved"), Icon: new("ld-check-symbolic")}}
	ev := toastEvent(widgetipc.ToastRequest{Preset: new("saved")}, osd)
	if ev != osdview.Toast("Saved", "ld-check-symbolic") {
		t.Errorf("preset toast %+v", ev)
	}
	ev = toastEvent(widgetipc.ToastRequest{Preset: new("saved"), Label: new("Stored"), Percentage: new(30.0)}, osd)
	if ev != osdview.Toast("Stored", "ld-check-symbolic").WithPercent(30) {
		t.Errorf("overridden toast %+v", ev)
	}
	ev = toastEvent(widgetipc.ToastRequest{Preset: new("nope"), Label: new("x")}, osd)
	if ev != osdview.Toast("x", "") {
		t.Errorf("unknown preset toast %+v", ev)
	}
}

// fakeAudio scripts the default devices.
type fakeAudio struct {
	pulse.Source
	sink, source pulse.Device
}

func (f *fakeAudio) DefaultSink(context.Context) (pulse.OutputDevice, error) {
	return pulse.OutputDevice{Device: f.sink}, nil
}

func (f *fakeAudio) DefaultSource(context.Context) (pulse.InputDevice, error) {
	return pulse.InputDevice{Device: f.source}, nil
}

// A level flashes only when its rounded value or mute state moved, and
// carries the device's description.
func TestAudioEventsDedupe(t *testing.T) {
	src := &fakeAudio{
		sink:   pulse.Device{Description: "Speakers", Volume: pulse.VolumeFromPercentage(40, 2)},
		source: pulse.Device{Description: "Mic", Volume: pulse.VolumeFromPercentage(70, 1)},
	}
	var a audioEvents
	if evs := a.next(context.Background(), src); len(evs) != 2 {
		t.Fatalf("seed produced %d events, want both devices", len(evs))
	}
	src.sink.Volume = pulse.VolumeFromPercentage(40.2, 2)
	if evs := a.next(context.Background(), src); len(evs) != 0 {
		t.Errorf("a sub-percent move flashed %+v", evs)
	}
	src.source.Muted = true
	evs := a.next(context.Background(), src)
	want := osdview.Slider("Mic", "ld-mic-off-symbolic", src.source.Volume.AveragePercentage(), true)
	if len(evs) != 1 || evs[0] != want {
		t.Errorf("muting the mic flashed %+v, want one muted Mic slider", evs)
	}
}

// fakeLight scripts the backlights.
type fakeLight struct {
	brightness.Source
	devices []brightness.Device
}

func (f *fakeLight) Devices(context.Context) ([]brightness.Device, error) { return f.devices, nil }

func TestBrightnessEventsShowTheDevicesThatMoved(t *testing.T) {
	panel := func(pct uint32) brightness.Device { return brightness.Device{Name: "panel", Brightness: pct, Max: 100} }
	monitor := func(pct uint32) brightness.Device {
		return brightness.Device{Name: "monitor", Brightness: pct, Max: 100}
	}
	src := &fakeLight{devices: []brightness.Device{panel(20), monitor(60)}}
	var b brightnessEvents
	if ev, ok := b.next(context.Background(), src); ok {
		t.Fatalf("the seeding read flashed %+v", ev)
	}
	src.devices = []brightness.Device{panel(20), monitor(80)}
	ev, ok := b.next(context.Background(), src)
	if !ok || ev != osdview.Slider("Brightness", osdview.BrightnessIcon, 80, false) {
		t.Fatalf("one monitor moved: %+v %v, want its 80%%, not the 50%% average", ev, ok)
	}
	if ev, ok := b.next(context.Background(), src); ok {
		t.Errorf("unchanged levels flashed %+v", ev)
	}
	src.devices = []brightness.Device{panel(30), monitor(90)}
	if ev, ok := b.next(context.Background(), src); !ok || ev != osdview.Slider("Brightness", osdview.BrightnessIcon, 60, false) {
		t.Errorf("both moved: %+v %v, want their 60%% average", ev, ok)
	}
	src.devices = append(src.devices, brightness.Device{Name: "hotplugged", Brightness: 10, Max: 100})
	if ev, ok := b.next(context.Background(), src); ok {
		t.Errorf("a device appearing flashed %+v", ev)
	}
	src.devices = nil
	if ev, ok := b.next(context.Background(), src); ok {
		t.Errorf("no backlights flashed %+v", ev)
	}
}

// fakeLayouts scripts the compositor's keyboard layout.
type fakeLayouts struct {
	mu    sync.Mutex
	name  string
	err   error
	ticks chan struct{}
}

func (f *fakeLayouts) set(name string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.name, f.err = name, err
}

func (f *fakeLayouts) Layout() (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.name, f.err
}

func (f *fakeLayouts) Subscribe(context.Context) (<-chan struct{}, func(), error) {
	return f.ticks, nil, nil
}

// A layout switch flashes the new layout's name; the layout at start,
// a re-read of the same layout, an unknown one, and a failed read do
// not.
func TestLayoutEventsFlashSwitchesOnly(t *testing.T) {
	src := &fakeLayouts{name: "English (US)"}
	var l layoutEvents
	l.next(src)
	if _, ok := l.next(src); ok {
		t.Error("an unchanged layout flashed")
	}
	src.set("German", nil)
	ev, ok := l.next(src)
	if !ok || ev != osdview.Toast("German", LayoutIcon) {
		t.Errorf("switching to German flashed %+v %v, want its name and the keyboard glyph", ev, ok)
	}
	src.set("", nil)
	if _, ok := l.next(src); ok {
		t.Error("an unknown layout flashed")
	}
	src.set("", errors.New("compositor gone"))
	if _, ok := l.next(src); ok {
		t.Error("a failed read flashed")
	}
	src.set("German", nil)
	if _, ok := l.next(src); ok {
		t.Error("re-reading the last shown layout after a gap flashed")
	}
}

// Watch shows the layout a compositor switches to, and nothing for the
// layout it starts on.
func TestWatchShowsLayoutSwitches(t *testing.T) {
	h := readyHarness(t, quietConfig())
	calls := make(chan func(), 1)
	h.osd.d.Invoke = func(fn func()) { calls <- fn }
	src := &fakeLayouts{name: "English (US)", ticks: make(chan struct{})}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.osd.Watch(ctx, nil, nil, src, nil, nil)
	}()
	t.Cleanup(func() { cancel(); <-done })
	tick := func() {
		sent := make(chan struct{})
		go func() {
			src.ticks <- struct{}{}
			close(sent)
		}()
		waittest.For(t, sent, "Watch to follow the layout")
	}
	tick()
	src.set("German", nil)
	tick()
	waittest.For(t, calls, "switching the layout to show it")()
	if len(h.opened) != 1 || !slices.Contains(shownLabels(h), "German") {
		t.Errorf("the switch showed %v, want German", shownLabels(h))
	}
	select {
	case <-calls:
		t.Error("the layout the compositor started on was shown")
	default:
	}
}

// Watch shows each lock key its source reports, with the key's new
// state, and stops the source when it ends.
func TestWatchShowsLockKeysFromItsSource(t *testing.T) {
	h := readyHarness(t, quietConfig())
	calls := make(chan func(), 1)
	h.osd.d.Invoke = func(fn func()) { calls <- fn }
	reports := make(chan func(keytoggles.Event), 1)
	stopped := make(chan struct{})
	locks := func(fn func(keytoggles.Event)) (func(), error) {
		reports <- fn
		return func() { close(stopped) }, nil
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.osd.Watch(ctx, nil, nil, nil, nil, locks)
	}()
	report := waittest.For(t, reports, "Watch to start the lock-key source")
	expectNoSignal(t, "starting the lock-key source showed something", calls)
	report(keytoggles.Event{Key: keytoggles.NumLock, Active: true})
	waittest.For(t, calls, "a reported lock key to show")()
	if got := shownLabels(h); !slices.Contains(got, "\u2068Num Lock\u2069 On") {
		t.Errorf("the toggle reads %v, want Num Lock On", got)
	}
	cancel()
	<-done
	select {
	case <-stopped:
	default:
		t.Error("Watch ended without stopping the lock-key source")
	}
}

// A lock-key source that fails to start shows nothing and leaves
// Watch running until ctx ends.
func TestWatchSurvivesAFailedLockKeySource(t *testing.T) {
	h := readyHarness(t, quietConfig())
	h.osd.d.Invoke = func(func()) { t.Error("a failed lock-key source showed something") }
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.osd.Watch(ctx, nil, nil, nil, nil, func(func(keytoggles.Event)) (func(), error) {
			return nil, errors.New("no lys-wm")
		})
	}()
	expectNoSignal(t, "Watch ended before ctx", done)
	cancel()
	<-done
}

// lockKeySource counts its starts and stops for the tests of when the
// lock-key source runs.
type lockKeySource struct {
	started, stopped chan struct{}
}

func newLockKeySource() *lockKeySource {
	return &lockKeySource{started: make(chan struct{}, 8), stopped: make(chan struct{}, 8)}
}

func (s *lockKeySource) start(func(keytoggles.Event)) (func(), error) {
	s.started <- struct{}{}
	return func() { s.stopped <- struct{}{} }, nil
}

// quietWindow is how long what must not happen gets to happen anyway
// before a test concludes it does not.
const quietWindow = 100 * time.Millisecond

func expectNoSignal[T any](t *testing.T, what string, ch <-chan T) {
	t.Helper()
	select {
	case <-ch:
		t.Errorf("unexpected: %s", what)
	case <-time.After(quietWindow):
	}
}

func startWatch(t *testing.T, h *harness, locks LockKeys) (finish func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.osd.Watch(ctx, nil, nil, nil, nil, locks)
	}()
	return func() {
		cancel()
		<-done
	}
}

// A disabled OSD opens no input devices: the source never starts.
func TestWatchStartsNoLockKeySourceWhileDisabled(t *testing.T) {
	cfg := quietConfig()
	cfg.Osd.Enabled = false
	h := readyHarness(t, cfg)
	source := newLockKeySource()
	finish := startWatch(t, h, source.start)
	expectNoSignal(t, "a disabled OSD started the lock-key source", source.started)
	finish()
	expectNoSignal(t, "a source that never started was stopped", source.stopped)
}

// A reload turns the source on and off with the OSD, and one that
// leaves the setting alone does not touch it.
func TestLockKeySourceFollowsReloads(t *testing.T) {
	cfg := quietConfig()
	cfg.Osd.Enabled = false
	h := readyHarness(t, cfg)
	source := newLockKeySource()
	finish := startWatch(t, h, source.start)

	enabled := quietConfig()
	enabled.Osd.Enabled = true
	h.osd.SetConfig(enabled)
	waittest.For(t, source.started, "enabling the OSD to start the source")

	h.osd.SetConfig(enabled)
	expectNoSignal(t, "a reload with the OSD still on restarted the source", source.started)
	expectNoSignal(t, "a reload with the OSD still on stopped the source", source.stopped)

	h.osd.SetConfig(cfg)
	waittest.For(t, source.stopped, "disabling the OSD to stop the source")
	h.osd.SetConfig(cfg)
	expectNoSignal(t, "a reload with the OSD still off started the source", source.started)

	h.osd.SetConfig(enabled)
	waittest.For(t, source.started, "enabling the OSD again to start the source")
	finish()
	waittest.For(t, source.stopped, "the end of Watch to stop the source")
	expectNoSignal(t, "the source was stopped twice", source.stopped)
}

// A source that fails is left alone until the OSD is enabled again.
func TestFailedLockKeySourceRetriesOnTheNextEnable(t *testing.T) {
	h := readyHarness(t, quietConfig())
	attempts := make(chan struct{}, 8)
	finish := startWatch(t, h, func(func(keytoggles.Event)) (func(), error) {
		attempts <- struct{}{}
		return nil, errors.New("no lys-wm")
	})
	defer finish()
	waittest.For(t, attempts, "the first attempt")
	on := quietConfig()
	h.osd.SetConfig(on)
	expectNoSignal(t, "a reload with the OSD still on retried", attempts)
	off := quietConfig()
	off.Osd.Enabled = false
	h.osd.SetConfig(off)
	expectNoSignal(t, "disabling the OSD retried", attempts)
	h.osd.SetConfig(on)
	waittest.For(t, attempts, "enabling the OSD to retry")
}

// A show after a dismissal maps a new surface over the tree the last one
// showed: nothing is built again.
func TestAShowAfterADismissalReusesTheTree(t *testing.T) {
	h := readyHarness(t, quietConfig())
	dismiss := time.Duration(config.DefaultsOsd().Duration) * time.Millisecond
	h.osd.Show(osdview.Slider("a", "", 1, false))
	h.fire(t, dismiss)
	h.osd.Show(osdview.Slider("b", "", 2, false))
	if len(h.opened) != 2 {
		t.Fatalf("%d surfaces, want a new one after the dismissal", len(h.opened))
	}
	if h.opened[1].cfg.Root != h.opened[0].cfg.Root {
		t.Error("the second surface built its tree again")
	}
	if got := shownLabels(h); !slices.Contains(got, "b") {
		t.Errorf("the kept tree shows %v, want the new event", got)
	}
	if !h.osd.face.view.Root().Revealed() {
		t.Error("the kept tree must reveal again")
	}
	h.fire(t, dismiss)
	allocs := testing.AllocsPerRun(20, func() {
		h.osd.Show(osdview.Slider("c", "", 3, false))
		h.fire(t, dismiss)
	})
	t.Logf("a show after a dismissal: %v allocations", allocs)
	if allocs > 16 {
		t.Errorf("a show after a dismissal allocates %v, want the tree kept", allocs)
	}
}

// A new styling scale is a new tree: the labels and the width are built
// at the scale.
func TestAScaleChangeBuildsTheTreeAgain(t *testing.T) {
	h := readyHarness(t, quietConfig())
	dismiss := time.Duration(config.DefaultsOsd().Duration) * time.Millisecond
	h.osd.Show(osdview.Slider("a", "", 1, false))
	h.fire(t, dismiss)
	next := quietConfig()
	next.Styling.Scale = 2
	h.osd.SetConfig(next)
	h.osd.Show(osdview.Slider("b", "", 2, false))
	if h.opened[1].cfg.Root == h.opened[0].cfg.Root {
		t.Error("a tree built at scale 1 showed at scale 2")
	}
}

// An OSD fading in shows from the shared start opacity, readable from
// its first frame; a slide still enters from hidden, and the exit still
// fades all the way out.
func TestOsdFadesInFromAReadableOpacity(t *testing.T) {
	cfg := config.Defaults()
	h := readyHarness(t, cfg)
	h.osd.Show(osdview.Slider("Speakers", "", 40, false))
	rev := h.osd.face.view.Root()
	if got := rev.Progress(); got != reveal.FadeFrom {
		t.Errorf("the OSD fade starts at %v, want %v", got, reveal.FadeFrom)
	}
	rev.Finish()
	h.fire(t, time.Duration(cfg.Osd.Duration)*time.Millisecond)
	rev.Finish()
	if got := rev.Progress(); got != 0 {
		t.Errorf("the exit lands at %v, want hidden", got)
	}

	slide := config.Defaults()
	enter := config.AnimationSlideDown
	slide.Animations.Osd.Enter = &enter
	h = readyHarness(t, slide)
	h.osd.Show(osdview.Slider("Speakers", "", 40, false))
	if got := h.osd.face.view.Root().Progress(); got != 0 {
		t.Errorf("a sliding OSD starts at %v, want hidden", got)
	}
}

// paintCounter is a leaf that counts its paints.
type paintCounter struct {
	widget.Base
	paints int
}

func (p *paintCounter) Measure(widget.Axis, int) widget.Sizes { return widget.FixedSizes(8) }
func (p *paintCounter) Paint(*render.Canvas)                  { p.paints++ }
func (p *paintCounter) HitTest(pt widget.Point) widget.Widget { return p.HitLeaf(p, pt) }

// fadeHarness shows an OSD with a counting leaf in its card over a
// compositor whose opacity takes the fade of a surface's root only, as
// lys-ui's application does; fades records the opacities it was asked
// for.
func fadeHarness(t *testing.T, alpha bool) (h *harness, leaf *paintCounter, fades *[]float64, frame func()) {
	t.Helper()
	fades = new([]float64)
	h = readyHarness(t, config.Defaults())
	widget.SetSurfaceFade(func(w widget.Widget, a float64) bool {
		if !alpha || len(h.opened) == 0 || w != h.opened[len(h.opened)-1].cfg.Root {
			return false
		}
		*fades = append(*fades, a)
		return true
	})
	t.Cleanup(func() { widget.SetSurfaceFade(nil) })
	h.osd.Show(osdview.Slider("Speakers", "", 40, false))
	leaf = &paintCounter{}
	card, ok := treetest.WithClass(h.osd.face.view.Root(), "osd")[0].(*widget.Box)
	if !ok {
		t.Fatal("the OSD's card is no box")
	}
	card.Append(leaf, false)
	root := h.opened[0].cfg.Root
	const w, ht = 400, 200
	frame = func() {
		widget.SizeWithin(root, widget.Size{W: w, H: ht})
		root.Arrange(render.Rect{W: w, H: ht})
		if _, any := widget.CollectDamage(root); any {
			root.Paint(render.New(make([]byte, render.Stride(w)*ht), render.Stride(w), w, ht))
		}
	}
	frame()
	if leaf.paints == 0 {
		t.Fatal("the OSD's first frame painted no card")
	}
	leaf.paints = 0
	return h, leaf, fades, frame
}

// The OSD's revealer is its surface's root, so its fade runs as the
// surface's opacity: the card paints once and the fade's frames repaint
// none of it.
func TestOsdFadesAsSurfaceOpacity(t *testing.T) {
	h, leaf, fades, frame := fadeHarness(t, true)
	h.osd.face.view.Root().Finish()
	frame()
	h.fire(t, time.Duration(config.DefaultsOsd().Duration)*time.Millisecond)
	h.osd.face.view.Root().Finish()
	frame()
	if leaf.paints != 0 {
		t.Errorf("the OSD's fade painted the card %d times, want 0", leaf.paints)
	}
	if want := []float64{reveal.FadeFrom, 1, 0}; !slices.Equal(*fades, want) {
		t.Errorf("surface opacities %v, want %v", *fades, want)
	}
	if !h.opened[0].closed {
		t.Error("the faded-out OSD stayed mapped")
	}
}

// Where the compositor cannot fade the surface the OSD still fades, by
// painting the card through each frame's alpha.
func TestOsdFadesByPaintingWithoutSurfaceOpacity(t *testing.T) {
	h, leaf, fades, frame := fadeHarness(t, false)
	h.osd.face.view.Root().Finish()
	frame()
	if leaf.paints == 0 {
		t.Error("a fade without surface opacity never repainted the card")
	}
	if len(*fades) != 0 {
		t.Errorf("surface opacities %v without a compositor to take them", *fades)
	}
}
