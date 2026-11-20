package bar

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/cavaview"
	"github.com/stubbedev/lys/lib/waittest"

	analyzer "github.com/stubbedev/lys/shell/cava"
)

func TestNewCavaBuildsEveryStyleAndStereo(t *testing.T) {
	for _, style := range []config.CavaStyle{config.CavaStyleBars, config.CavaStyleWave, config.CavaStylePeaks} {
		cfg := config.Defaults()
		cfg.Cava.Style = style
		cfg.Cava.Stereo = true
		cfg.Cava.Bars = 9
		module, err := createOnHeadlessLoop("cava", newTestContext(t, cfg))
		if err != nil {
			t.Fatalf("%s: %v", style, err)
		}
		m := module.(*cavaModule)
		if m.analyzer.plan.Channels() != 2 || len(m.paint.Values()) != 10 {
			t.Errorf("%s stereo: %d channels, %d values", style, m.analyzer.plan.Channels(), len(m.paint.Values()))
		}
		if got := len(m.analyzer.plan.Execute(nil, nil)); got != 10 {
			t.Errorf("a stereo frame has %d values, want 10", got)
		}
	}
}

func TestNewCavaRejectsAnUnsupportedInput(t *testing.T) {
	for input, ok := range map[config.CavaInput]bool{
		config.CavaInputPipeWire: true, config.CavaInputPulse: true,
		config.CavaInputFifo: true, config.CavaInputShmem: true,
		config.CavaInputAlsa: false, config.CavaInputJack: false, config.CavaInputOss: false,
		config.CavaInputSndio: false, config.CavaInputPortAudio: false, config.CavaInputWinscap: false,
	} {
		cfg := config.Defaults()
		cfg.Cava.Input = input
		_, err := createOnHeadlessLoop("cava", newTestContext(t, cfg))
		if (err == nil) != ok {
			t.Errorf("input %q: err %v", input, err)
		}
	}
}

// writeCavaConfig loads one config.yaml snippet.
func writeCavaConfig(t *testing.T, content string) *config.Config {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := config.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	return c
}

func TestLoadFileAppliesCavaModule(t *testing.T) {
	c := writeCavaConfig(t, `
modules:
  cava:
    bars: 12
    bar-width: 4
    color: "#a6e3a1"
    direction: mirror
    framerate: 30
    noise-reduction: 0.5
    monstercat: 1.5
    internal-padding: 3px
    source: alsa_output.monitor
`)
	cava := c.Cava
	if cava.Bars != 12 || cava.BarWidth != 4 {
		t.Errorf("bars/width = %d/%d, want 12/4", cava.Bars, cava.BarWidth)
	}
	if cava.Color.Kind != config.ColorCustom || cava.Color.Hex != "#a6e3a1" {
		t.Errorf("color = %+v", cava.Color)
	}
	if cava.Direction != config.CavaDirectionMirror || cava.Framerate != 30 {
		t.Errorf("direction/framerate = %q/%d", cava.Direction, cava.Framerate)
	}
	if cava.NoiseReduction != 0.5 || cava.Monstercat != 1.5 {
		t.Errorf("noise/monstercat = %v/%v", cava.NoiseReduction, cava.Monstercat)
	}
	if cava.InternalPadding.Unit != config.SizePixels || cava.InternalPadding.Value != 3 {
		t.Errorf("internal-padding = %+v, want 3px", cava.InternalPadding)
	}
	if cava.Source != "alsa_output.monitor" {
		t.Errorf("source = %q", cava.Source)
	}
}

func TestLoadFileCavaDefaults(t *testing.T) {
	c := writeCavaConfig(t, "")
	if c.Cava != config.DefaultsCava() {
		t.Errorf("defaults = %+v, want %+v", c.Cava, config.DefaultsCava())
	}
}

func TestLoadFileRejectsBadCava(t *testing.T) {
	for _, content := range []string{
		"modules:\n  cava:\n    style: spikes\n",
		"modules:\n  cava:\n    direction: sideways\n",
		"modules:\n  cava:\n    color: not-a-token\n",
	} {
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := config.LoadFile(path); err == nil {
			t.Errorf("%q: want a load error, got nil", strings.TrimSpace(content))
		}
	}
}

// TestCavaInputPicksTheCapture pins the input dispatch: fifo and
// shmem read their source path, the rest record through pulse, which
// they need.
func TestCavaInputPicksTheCapture(t *testing.T) {
	ctx := newTestContext(t, config.Defaults())
	cfg := config.DefaultsCava()
	cfg.Input, cfg.Source = config.CavaInputFifo, "/tmp/cava.fifo"
	if in, err := cavaInput(ctx, cfg, 1, 4096); err != nil {
		t.Errorf("fifo: %v", err)
	} else if _, ok := in.(*analyzer.FifoInput); !ok {
		t.Errorf("fifo built %T", in)
	}
	cfg.Input = config.CavaInputShmem
	if in, err := cavaInput(ctx, cfg, 1, 4096); err != nil {
		t.Errorf("shmem: %v", err)
	} else if _, ok := in.(*analyzer.ShmemInput); !ok {
		t.Errorf("shmem built %T", in)
	}
	cfg.Input = config.CavaInputPipeWire
	ctx.Pulse = nil
	if _, err := cavaInput(ctx, cfg, 1, 4096); err == nil {
		t.Error("pipe-wire without an audio connection built a capture")
	}
}

type fakeCavaInput struct {
	stopped chan struct{}
	next    []float64
	onSound func()
}

func (f *fakeCavaInput) Start() error { return nil }
func (f *fakeCavaInput) Drained(dst []float64) []float64 {
	return append(dst[:0], f.next...)
}
func (f *fakeCavaInput) NotifySound(fn func()) { f.onSound = fn }
func (f *fakeCavaInput) Stop()                 { close(f.stopped) }

// TestCavaModuleEndsWithItsGeneration pins the lifetime: the frame
// ticker runs while the bar generation lives and, once it retires,
// the ticker and the capture stop on the loop.
func TestCavaModuleEndsWithItsGeneration(t *testing.T) {
	plan, err := analyzer.NewPlan(10, 44100, 1, 0.77, true, 50, 10000)
	if err != nil {
		t.Fatal(err)
	}
	in := &fakeCavaInput{stopped: make(chan struct{})}
	life, retire := context.WithCancel(context.Background())
	ticking := true
	a := newCavaAnalyzer(plan, in, 10, config.CavaStyleBars, nil, time.Second/60,
		func(time.Duration, func()) func() { return func() { ticking = false } })
	a.join(life, testSpawn(t), newCavaBarSlot(cavaview.NewSpectrum(config.DefaultsCava(), 10, 0, 1, false), func(fn func()) { fn() }))
	if !ticking {
		t.Fatal("the ticker stopped before the generation retired")
	}
	retire()
	waittest.For(t, in.stopped, "the capture to stop with its generation")
	if ticking {
		t.Error("the ticker outlived its generation")
	}
}

// cavaLoop drives a module's frame timer by hand: tick is the armed
// timer's frame, nil while none is armed.
type cavaLoop struct {
	tick    func()
	started int
}

func (l *cavaLoop) every(_ time.Duration, fn func()) func() {
	l.tick = fn
	l.started++
	return func() { l.tick = nil }
}

func newLoopedCava(t *testing.T) (*cavaModule, *fakeCavaInput, *cavaLoop) {
	t.Helper()
	plan, err := analyzer.NewPlan(10, 44100, 1, 0.77, true, 50, 10000)
	if err != nil {
		t.Fatal(err)
	}
	in := &fakeCavaInput{stopped: make(chan struct{})}
	loop := &cavaLoop{}
	a := newCavaAnalyzer(plan, in, 10, config.CavaStyleBars, nil, time.Second/60, loop.every)
	m := &cavaModule{paint: cavaview.NewSpectrum(config.DefaultsCava(), 10, 0, 1, false), analyzer: a}
	a.join(t.Context(), testSpawn(t), newCavaBarSlot(m.paint, func(fn func()) { fn() }))
	return m, in, loop
}

func loudSamples(n int) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = float64((i%32)*2000 - 32000)
	}
	return out
}

func TestCavaParksInSilenceAndWakesForSound(t *testing.T) {
	m, in, loop := newLoopedCava(t)
	loop.tick()
	if loop.tick != nil {
		t.Fatal("a silent first frame moved nothing, yet the frame timer still runs")
	}
	if in.onSound == nil {
		t.Fatal("parked without asking to hear about sound")
	}
	in.onSound()
	if loop.tick == nil || loop.started != 2 {
		t.Fatalf("sound did not restart the frame timer (started %d times)", loop.started)
	}
	in.next = loudSamples(m.analyzer.plan.InputSize())
	for range 5 {
		loop.tick()
		if loop.tick == nil {
			t.Fatal("parked while sound plays")
		}
	}
}

func TestCavaFallsToRestBeforeParking(t *testing.T) {
	m, in, loop := newLoopedCava(t)
	in.next = loudSamples(m.analyzer.plan.InputSize())
	for range 30 {
		loop.tick()
	}
	if !slices.ContainsFunc(m.paint.Values(), func(v float64) bool { return v > cavaview.Still }) {
		t.Fatal("the bars never rose")
	}
	in.next = make([]float64, m.analyzer.plan.InputSize())
	frames := 0
	for loop.tick != nil {
		if frames++; frames > 2000 {
			t.Fatal("silence never settled")
		}
		loop.tick()
		if frames == 1 && loop.tick == nil {
			t.Fatal("parked on the first silent frame with the bars still up")
		}
	}
}

func TestCavaStaysStoppedWhenSoundFollowsItsGeneration(t *testing.T) {
	plan, err := analyzer.NewPlan(10, 44100, 1, 0.77, true, 50, 10000)
	if err != nil {
		t.Fatal(err)
	}
	in := &fakeCavaInput{stopped: make(chan struct{})}
	loop := &cavaLoop{}
	life, retire := context.WithCancel(context.Background())
	a := newCavaAnalyzer(plan, in, 10, config.CavaStyleBars, nil, time.Second/60, loop.every)
	a.join(life, testSpawn(t), newCavaBarSlot(cavaview.NewSpectrum(config.DefaultsCava(), 10, 0, 1, false), func(fn func()) { fn() }))
	loop.tick()
	retire()
	waittest.For(t, in.stopped, "the capture to stop with its generation")
	in.onSound()
	if loop.tick != nil {
		t.Error("sound restarted a retired module")
	}
}

func TestCavaOneCaptureAndAnalyzerForTwoBars(t *testing.T) {
	plan, err := analyzer.NewPlan(10, 44100, 1, 0.77, true, 50, 10000)
	if err != nil {
		t.Fatal(err)
	}
	hub := testPollHub(t)
	in := &fakeCavaInput{stopped: make(chan struct{})}
	loop := &cavaLoop{}
	builds := 0
	build := func() (*cavaAnalyzer, error) {
		builds++
		return newCavaAnalyzer(plan, in, 10, config.CavaStyleBars, hub, time.Second/60, loop.every), nil
	}
	var posted []func()
	post := func(fn func()) { posted = append(posted, fn) }
	bar1 := cavaview.NewSpectrum(config.DefaultsCava(), 10, 0, 1, false)
	bar2 := cavaview.NewSpectrum(config.DefaultsCava(), 10, 0, 1, false)
	first, err := hub.cavaAnalyzer("cava/x", build)
	if err != nil {
		t.Fatal(err)
	}
	second, err := hub.cavaAnalyzer("cava/x", build)
	if err != nil {
		t.Fatal(err)
	}
	if first != second || builds != 1 {
		t.Fatalf("%d captures for two bars, want one analyzer", builds)
	}
	first.join(t.Context(), testSpawn(t), newCavaBarSlot(bar1, post))
	second.join(t.Context(), testSpawn(t), newCavaBarSlot(bar2, post))
	if loop.started != 1 {
		t.Errorf("%d frame timers for two bars, want 1", loop.started)
	}
	in.next = loudSamples(plan.InputSize())
	for range 3 {
		loop.tick()
	}
	if len(posted) != 2 {
		t.Fatalf("%d calls waiting on the loop after three frames, want one per bar", len(posted))
	}
	if slices.ContainsFunc(bar1.Values(), func(v float64) bool { return v != 0 }) {
		t.Fatal("the analyzer touched a bar's widget off the loop")
	}
	for _, apply := range posted {
		apply()
	}
	if !slices.ContainsFunc(bar1.Values(), func(v float64) bool { return v > cavaview.Still }) || !slices.Equal(bar1.Values(), bar2.Values()) {
		t.Errorf("the bars show %v and %v, want the same frame", bar1.Values(), bar2.Values())
	}
}
