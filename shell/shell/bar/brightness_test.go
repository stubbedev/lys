package bar

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/feed"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/barview"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/lib/waittest"

	"github.com/stubbedev/lys/shell/service/brightness"
)

func TestBrightnessLabel(t *testing.T) {
	if got := brightnessLabel("{{ percent }}%", 42.6); got != "43%" {
		t.Errorf("= %q, want 43%%", got)
	}
	if got := brightnessLabel("{{ percent }}", 0); got != "0" {
		t.Errorf("= %q, want 0", got)
	}
}

func TestBrightnessIconBands(t *testing.T) {
	levels := []string{"low", "mid", "high"}
	for _, tc := range []struct {
		percent float64
		want    string
	}{{0, "low"}, {33, "low"}, {34, "mid"}, {66, "mid"}, {67, "high"}, {100, "high"}, {150, "high"}, {-5, "low"}} {
		if got := brightnessIcon(levels, tc.percent); got != tc.want {
			t.Errorf("%v%% = %q, want %q", tc.percent, got, tc.want)
		}
	}
	if got := brightnessIcon(nil, 50); got != "" {
		t.Errorf("no icons = %q", got)
	}
}

// fakeBrightnessSource is a scripted brightness.Source, safe across
// the follow goroutine and the test.
type fakeBrightnessSource struct {
	mu      sync.Mutex
	devices []brightness.Device
	ticks   chan struct{}
	sets    []brightnessSet
	// deadlines is each Set's time left when it began, zero for none.
	deadlines []time.Duration
	// gate, when set, holds each Set until it receives: a slow display.
	gate chan struct{}
}

// brightnessSet is one recorded Set call.
type brightnessSet struct {
	name    string
	percent float64
}

func (f *fakeBrightnessSource) Devices(context.Context) ([]brightness.Device, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.devices), nil
}

func (f *fakeBrightnessSource) setDevices(devices []brightness.Device) {
	f.mu.Lock()
	f.devices = devices
	f.mu.Unlock()
}

func (f *fakeBrightnessSource) Subscribe(context.Context) (<-chan struct{}, func(), error) {
	return f.ticks, func() {}, nil
}

func (f *fakeBrightnessSource) Set(ctx context.Context, name string, percent float64) error {
	f.mu.Lock()
	f.sets = append(f.sets, brightnessSet{name, percent})
	var left time.Duration
	if d, ok := ctx.Deadline(); ok {
		left = time.Until(d)
	}
	f.deadlines = append(f.deadlines, left)
	gate := f.gate
	f.mu.Unlock()
	if gate != nil {
		<-gate
	}
	return nil
}

func (f *fakeBrightnessSource) recordedDeadlines() []time.Duration {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.deadlines)
}

func (f *fakeBrightnessSource) recorded() []brightnessSet {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.sets)
}

func TestBrightnessModuleShowsAverageAndRestyles(t *testing.T) {
	cfg := config.Defaults()
	cv, err := config.ParseColorValue("status-warning")
	if err != nil {
		t.Fatal(err)
	}
	above := config.ThresholdEntry{Above: ptrF(80), LabelColor: &cv}
	cfg.Brightness.Thresholds = []config.ThresholdEntry{above}

	source := &fakeBrightnessSource{
		devices: []brightness.Device{
			{Brightness: 4000, Max: 10000},
			{Brightness: 6000, Max: 10000},
		},
		ticks: make(chan struct{}, 2),
	}
	style := computeStyle(cfg, styling.Default())
	m := &brightnessModule{
		ctx:    ModuleContext{Config: cfg, Font: testFont(t), Style: &style, gen: testGen(t)},
		source: source,
	}
	m.btn = newBarButton(m.ctx, m.icon, "")
	if err := readAndApply(m.source.Devices, m.apply); err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if got := m.btn.Label().Text(); got != "50%" {
		t.Errorf("label = %q, want 50%%", got)
	}
	if strings.Contains(m.btn.InlineStyle(), "--bar-btn-label-color: "+cv.ToCSS()) {
		t.Errorf("the threshold colored the button at 50%%: %q", m.btn.InlineStyle())
	}

	source.setDevices([]brightness.Device{{Brightness: 9000, Max: 10000}})
	if err := readAndApply(m.source.Devices, m.apply); err != nil {
		t.Fatal(err)
	}
	if got := m.btn.Label().Text(); got != "90%" {
		t.Errorf("label = %q, want 90%%", got)
	}
	if !strings.Contains(m.btn.InlineStyle(), "--bar-btn-label-color: "+cv.ToCSS()) {
		t.Errorf("at 90%% the button vars = %q, want the status-warning threshold", m.btn.InlineStyle())
	}
}

func TestBrightnessNoDevicesShowsDashes(t *testing.T) {
	cfg := config.Defaults()
	source := &fakeBrightnessSource{ticks: make(chan struct{}, 1)}
	style := computeStyle(cfg, styling.Default())
	m := &brightnessModule{
		ctx:    ModuleContext{Config: cfg, Font: testFont(t), Style: &style, gen: testGen(t)},
		source: source,
	}
	m.btn = newBarButton(m.ctx, m.icon, "")
	if err := readAndApply(m.source.Devices, m.apply); err != nil {
		t.Fatal(err)
	}
	if got := m.btn.Label().Text(); got != "--%" {
		t.Errorf("label = %q, want --%%", got)
	}
}

func TestNewBrightnessRequiresSource(t *testing.T) {
	cfg := config.Defaults()
	ctx := newTestContext(t, cfg)
	ctx.Brightness = nil
	if _, err := createOnHeadlessLoop("brightness", ctx); err == nil {
		t.Fatal("nil source: want an error, got a module")
	}
}

func TestLoadFileAppliesBrightness(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	content := "modules:\n  brightness:\n    format: \"L {{ percent }}\"\n    min-brightness: 5\n    enable-external: false\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := config.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}
	if c.Brightness.Format != "L {{ percent }}" || c.Brightness.MinBrightness != 5 || c.Brightness.EnableExternal {
		t.Errorf("config = %+v", c.Brightness)
	}
}

func TestLoadFileRejectsBadBrightness(t *testing.T) {
	for _, content := range []string{
		"modules:\n  brightness:\n    min-brightness: -1\n",
	} {
		path := filepath.Join(t.TempDir(), "config.yaml")
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := config.LoadFile(path); err == nil {
			t.Errorf("%q: want a load error, got nil", content)
		}
	}
}

// The module's icon is the level icon for the mean brightness; without
// a device it is the first one.
func TestBrightnessModuleIconFollowsTheLevel(t *testing.T) {
	cfg := config.Defaults()
	cfg.Brightness.LevelIcons = []string{"low", "high"}
	ctx := newTestContext(t, cfg)
	source := &fakeBrightnessSource{devices: []brightness.Device{{Brightness: 9000, Max: 10000}}}
	ctx.Brightness = source
	module, err := createOnHeadlessLoop("brightness", ctx)
	if err != nil {
		t.Fatal(err)
	}
	m := module.(*brightnessModule)
	if m.icon == nil || m.icon.Name() != "high" {
		t.Fatalf("icon at 90%% = %v, want high", m.icon)
	}
	source.setDevices(nil)
	if err := readAndApply(m.source.Devices, m.apply); err != nil {
		t.Fatal(err)
	}
	if m.icon.Name() != "low" || m.btn.Label().Text() != "--%" {
		t.Errorf("no device: icon %q label %q", m.icon.Name(), m.btn.Label().Text())
	}
}

func TestBrightnessStepActionHoldsTheConfiguredFloor(t *testing.T) {
	cfg := config.Defaults()
	if cfg.Brightness.MinBrightness != 5 {
		t.Fatalf("default min-brightness = %d, want 5", cfg.Brightness.MinBrightness)
	}
	ctx := newTestContext(t, cfg)
	source := &fakeBrightnessSource{devices: []brightness.Device{
		{Name: "panel", Brightness: 80, Max: 1000},
		{Name: "monitor", Brightness: 50, Max: 100},
	}}
	ctx.Brightness = source
	runClickAction(ctx, config.ParseClickAction("brightness:-10"))
	waitForSets := func(n int) []brightnessSet {
		t.Helper()
		waittest.Until(t, func() bool { return len(source.recorded()) >= n }, fmt.Sprintf("%d brightness writes", n))
		return source.recorded()
	}
	want := []brightnessSet{{"panel", 5}, {"monitor", 40}}
	if got := waitForSets(2); !slices.Equal(got, want) {
		t.Errorf("-10 wrote %v, want %v", got, want)
	}
	runClickAction(ctx, config.ParseClickAction("brightness:5"))
	if got := waitForSets(4)[2:]; !slices.Equal(got, []brightnessSet{{"panel", 13}, {"monitor", 55}}) {
		t.Errorf("+5 wrote %v, want steps from each rounded level", got)
	}
}

// fakeAutoBrightness is a scripted automatic brightness, safe across
// the follow goroutine and the test.
type fakeAutoBrightness struct {
	mu      sync.Mutex
	control brightness.Control
	resumes int
	changes feed.Feed[struct{}]
}

func (f *fakeAutoBrightness) Control() brightness.Control {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.control
}

func (f *fakeAutoBrightness) Subscribe() (<-chan struct{}, func()) { return f.changes.Subscribe() }

func (f *fakeAutoBrightness) Resume(context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resumes++
	return nil
}

func (f *fakeAutoBrightness) set(control brightness.Control) {
	f.mu.Lock()
	f.control = control
	f.mu.Unlock()
	f.changes.Publish(struct{}{})
}

func (f *fakeAutoBrightness) resumed() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.resumes
}

// The module shows who sets the backlight: the level icon and the
// plain level without automatic brightness, its icon while it follows
// the room's light, and the level icon again, described as paused,
// while a manual change holds it.
func TestBrightnessModuleShowsWhoSetsTheBacklight(t *testing.T) {
	cfg := config.Defaults()
	cfg.Brightness.LevelIcons = []string{"low", "high"}
	ctx := newTestContext(t, cfg)
	ctx.Brightness = &fakeBrightnessSource{devices: []brightness.Device{{Brightness: 9000, Max: 10000}}}
	auto := &fakeAutoBrightness{}
	ctx.AutoBrightness = auto
	module, err := createOnHeadlessLoop("brightness", ctx)
	if err != nil {
		t.Fatal(err)
	}
	m := module.(*brightnessModule)
	following := i18n.T("bar-brightness-following", i18n.Str("level", "90%"))
	paused := i18n.T("bar-brightness-paused", i18n.Str("level", "90%"))
	shows := func(icon string, described, notDescribed []string) func() bool {
		return func() bool {
			tooltip := m.btn.TooltipText()
			if m.icon.Name() != icon || m.btn.Label().Text() != "90%" {
				return false
			}
			for _, d := range described {
				if !strings.Contains(tooltip, d) {
					return false
				}
			}
			for _, d := range notDescribed {
				if strings.Contains(tooltip, d) {
					return false
				}
			}
			return true
		}
	}
	manual := shows("high", []string{"90%"}, []string{following, paused})
	if !onHeadlessLoop(manual) {
		t.Fatalf("manual: icon %q tooltip %q, want the level icon and the plain level", m.icon.Name(), m.btn.TooltipText())
	}
	auto.set(brightness.ControlFollowing)
	waitHeadless(t, "the automatic icon and description", shows(brightnessAutoIcon, []string{following}, []string{paused}))
	auto.set(brightness.ControlPaused)
	waitHeadless(t, "the level icon described as paused", shows("high", []string{paused}, []string{following}))
	auto.set(brightness.ControlManual)
	waitHeadless(t, "the manual look again", manual)
}

// Without automatic brightness on the bus the module looks as it
// always did: the level icon, the level alone in its tooltip.
func TestBrightnessModuleWithoutAutomaticBrightness(t *testing.T) {
	cfg := config.Defaults()
	cfg.Brightness.LevelIcons = []string{"low", "high"}
	ctx := newTestContext(t, cfg)
	ctx.Brightness = &fakeBrightnessSource{devices: []brightness.Device{{Brightness: 2000, Max: 10000}}}
	module, err := createOnHeadlessLoop("brightness", ctx)
	if err != nil {
		t.Fatal(err)
	}
	m := module.(*brightnessModule)
	if m.icon.Name() != "low" || m.control != brightness.ControlManual {
		t.Errorf("icon %q control %v, want the level icon under manual control", m.icon.Name(), m.control)
	}
	if got, want := m.btn.TooltipText(), barview.ModuleDescription(m.btn.Title(), "", "20%"); got != want {
		t.Errorf("tooltip %q, want %q", got, want)
	}
}

func TestBrightnessDescription(t *testing.T) {
	if got := brightnessDescription(brightness.ControlManual, "40%"); got != "" {
		t.Errorf("manual = %q, want the label to speak for itself", got)
	}
	if got, want := brightnessDescription(brightness.ControlFollowing, "40%"), i18n.T("bar-brightness-following", i18n.Str("level", "40%")); got != want {
		t.Errorf("following = %q, want %q", got, want)
	}
	if got, want := brightnessDescription(brightness.ControlPaused, "40%"), i18n.T("bar-brightness-paused", i18n.Str("level", "40%")); got != want {
		t.Errorf("paused = %q, want %q", got, want)
	}
}
