package settings

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/fakeclock"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/service/backlight"
	"github.com/stubbedev/lys/lib/dbustest"
	"github.com/stubbedev/lys/lib/render"
	dbus "github.com/stubbedev/lys/lib/third_party/godbus"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/shell/service/brightness"
	"github.com/stubbedev/lys/shell/service/nightlight"

	"github.com/stubbedev/lys/settings/internal/displays"
	"github.com/stubbedev/lys/settings/internal/locale"
)

type fakeBrightness struct {
	at      map[string]float64
	readErr error
	setErr  error
	sets    []string
	closed  bool
}

func (f *fakeBrightness) known(outputs []string) map[string]float64 {
	out := map[string]float64{}
	for _, o := range outputs {
		if l, ok := f.at[o]; ok {
			out[o] = l
		}
	}
	return out
}

func (f *fakeBrightness) levels(_ context.Context, outputs []string) (map[string]float64, error) {
	if f.readErr != nil {
		return nil, f.readErr
	}
	return f.known(outputs), nil
}

func (f *fakeBrightness) set(_ context.Context, output string, percent int32) (float64, error) {
	if f.setErr != nil {
		return 0, f.setErr
	}
	f.sets = append(f.sets, output)
	f.at[output] = float64(percent)
	return float64(percent), nil
}

func (f *fakeBrightness) close() error {
	f.closed = true
	return nil
}

func lightPanel(t *testing.T, f brightnessControl, err error) *displaysPanel {
	t.Helper()
	fakeHeads(t, []displays.Head{head("eDP-1", 0), head("HDMI-A-1", 1920)}, nil)
	k := testKit(t, "")
	k.brightness = func() (brightnessControl, error) { return f, err }
	p := displaysLead(k).(*displaysPanel)
	t.Cleanup(p.close)
	return p
}

func TestEachDisplayShowsAndSetsItsOwnBrightness(t *testing.T) {
	f := &fakeBrightness{at: map[string]float64{"eDP-1": 60}}
	p := lightPanel(t, f, nil)
	if note := rowText(p.lightRow, "description"); !p.lightRow.Visible() || !p.lightSlider.Visible() || p.lightSlider.Value() != 60 || note != "" {
		t.Fatalf("the built-in display shows row %v slider %v at %v, note %q", p.lightRow.Visible(), p.lightSlider.Visible(), p.lightSlider.Value(), note)
	}
	p.lightSlider.OnCommit(30)
	if !slices.Equal(f.sets, []string{"eDP-1"}) || f.at["eDP-1"] != 30 || p.lightSlider.Value() != 30 {
		t.Errorf("sliding to 30 set %v to %v, the slider shows %v", f.sets, f.at, p.lightSlider.Value())
	}
	p.selectOutput(1)
	if note := rowText(p.lightRow, "description"); p.lightSlider.Visible() || note != locale.T().Get("settings-displays-brightness-none") {
		t.Errorf("a display nothing drives shows slider %v, note %q", p.lightSlider.Visible(), note)
	}
	p.close()
	if !f.closed {
		t.Error("closing the page left lys-shell's brightness open")
	}
}

func TestABrightnessLysShellRefusesIsSaid(t *testing.T) {
	f := &fakeBrightness{at: map[string]float64{"eDP-1": 60}, setErr: errors.New("logind: access denied")}
	p := lightPanel(t, f, nil)
	p.lightSlider.OnCommit(30)
	if !p.status.Visible() || f.at["eDP-1"] != 60 {
		t.Errorf("a refused level shows status %v %q, level %v", p.status.Visible(), p.status.Text(), f.at["eDP-1"])
	}
}

func TestWithoutLysShellTheDisplaysSayTheirBrightnessCannotBeRead(t *testing.T) {
	p := lightPanel(t, nil, errors.New("no lys-shell"))
	want := locale.T().Get("settings-displays-brightness-unavailable", i18n.Str("reason", "no lys-shell"))
	if note := rowText(p.lightRow, "description"); !p.lightRow.Visible() || p.lightSlider.Visible() || note != want {
		t.Errorf("without lys-shell: row %v, slider %v, note %q", p.lightRow.Visible(), p.lightSlider.Visible(), note)
	}
}

type fakeNightlight struct {
	on         bool
	setErr     error
	previewErr error
	previews   []uint32
}

func (f *fakeNightlight) preview(_ context.Context, kelvin uint32) error {
	if f.previewErr != nil {
		return f.previewErr
	}
	f.previews = append(f.previews, kelvin)
	return nil
}

func (f *fakeNightlight) read(context.Context) (bool, error) { return f.on, nil }

func (f *fakeNightlight) changes(context.Context) (<-chan struct{}, error) {
	return make(chan struct{}), nil
}

func (f *fakeNightlight) set(_ context.Context, on bool) error {
	if f.setErr != nil {
		return f.setErr
	}
	f.on = on
	return nil
}

func (f *fakeNightlight) close() error { return nil }

func nightSwitch(t *testing.T, f *fakeNightlight) *shellSwitch[bool] {
	t.Helper()
	k := testKit(t, "")
	k.nightlight = func() (nightlightControl, error) { return f, nil }
	s := nightlightNow(k).(*shellSwitch[bool])
	t.Cleanup(s.close)
	return s
}

func TestTheDisplaysPageSwitchesTheNightLight(t *testing.T) {
	f := &fakeNightlight{}
	s := nightSwitch(t, f)
	if s.toggle.On() || s.Description() != locale.T().Get("settings-nightlight-is-off") {
		t.Fatalf("off reads on %v, %q", s.toggle.On(), s.Description())
	}
	s.toggle.SetOn(true)
	if !f.on || s.Description() != locale.T().Get("settings-nightlight-is-on") || s.problem.Visible() {
		t.Errorf("switching on left the shell's night light on %v, status %q, problem %q", f.on, s.Description(), s.problem.Text())
	}
}

func TestARefusedNightLightSwitchIsSaid(t *testing.T) {
	f := &fakeNightlight{setErr: errors.New("access denied")}
	s := nightSwitch(t, f)
	s.toggle.SetOn(true)
	if !s.problem.Visible() || s.toggle.On() {
		t.Errorf("a refusal shows %q (%v), switch on %v", s.problem.Text(), s.problem.Visible(), s.toggle.On())
	}
}

func TestTheNightLightAndBrightnessLiveOnDisplays(t *testing.T) {
	for _, path := range []string{"modules.nightlight.temperature", "modules.nightlight.schedule", "modules.brightness.min-brightness", "modules.brightness.enable-external"} {
		if got := sectionOf(t, displaysID, path); got == "" {
			t.Errorf("%s has no section on Displays", path)
		}
	}
	for page, paths := range map[string][]string{
		"nightlight": {"modules.nightlight.temperature", "modules.nightlight.schedule", "modules.nightlight.gamma"},
		"brightness": {"modules.brightness.min-brightness", "modules.brightness.enable-external"},
	} {
		for _, path := range paths {
			if slices.Contains(rowPaths(bothParts, page), path) {
				t.Errorf("%s is still on the %s page", path, page)
			}
		}
	}
	if got := sectionOf(t, displaysID, "modules.nightlight.temperature"); got != "settings-section-nightlight-filter" {
		t.Errorf("the temperature sits in %s, want the night light section with its switch", got)
	}
}

type levelSource struct{ devices []backlight.Device }

func (s *levelSource) Devices(context.Context) ([]backlight.Device, error) {
	return slices.Clone(s.devices), nil
}

func (s *levelSource) Subscribe(context.Context) (<-chan struct{}, func(), error) {
	return nil, func() {}, nil
}

func (s *levelSource) Set(_ context.Context, name string, percent float64) error {
	for i, d := range s.devices {
		if d.Name == name {
			s.devices[i].Brightness = backlight.RawLevel(percent, d.Max)
		}
	}
	return nil
}

func TestTheBrightnessClientReadsAndSetsLysShellsLevels(t *testing.T) {
	bus := dbustest.Start(t)
	src := &levelSource{devices: []backlight.Device{{Name: "intel_backlight", Type: backlight.TypeRaw, Brightness: 50, Max: 100, Connector: "eDP-1"}}}
	release, err := brightness.NewDaemon(src, func() (string, bool) { return "", false }, func() float64 { return 5 }).Export(bus.Conn(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	ctl, err := openBrightness(func() (*dbus.Conn, error) { return bus.Conn(t), nil })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ctl.close() })
	levels, err := ctl.levels(t.Context(), []string{"eDP-1", "HDMI-A-1"})
	if err != nil || len(levels) != 1 || levels["eDP-1"] != 50 {
		t.Fatalf("levels %v (%v), want eDP-1 at 50 alone", levels, err)
	}
	if level, err := ctl.set(t.Context(), "eDP-1", 2); err != nil || level != 5 {
		t.Errorf("setting 2%% gave %v (%v), want the 5%% floor", level, err)
	}
	if _, err := ctl.set(t.Context(), "HDMI-A-1", 50); err == nil {
		t.Error("setting a display nothing drives succeeded")
	}
}

func TestTheNightLightClientFollowsLysShell(t *testing.T) {
	bus := dbustest.Start(t)
	var daemon *nightlight.Daemon
	daemon = nightlight.NewDaemon(func(on bool) { go daemon.Show(on) }, func(int) {})
	release, err := daemon.Export(bus.Conn(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(release)
	ctl, err := openNightlight(func() (*dbus.Conn, error) { return bus.Conn(t), nil })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ctl.close() })
	if on, err := ctl.read(t.Context()); err != nil || on {
		t.Fatalf("at the start the night light reads %v (%v)", on, err)
	}
	ticks, err := ctl.changes(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if err := ctl.set(t.Context(), true); err != nil {
		t.Fatal(err)
	}
	waittest.For(t, ticks, "the switch settings made")
	if on, err := ctl.read(t.Context()); err != nil || !on {
		t.Errorf("after switching on the night light reads %v (%v)", on, err)
	}
}

func TestTheNightLightClientSaysWhenLysShellIsNotThere(t *testing.T) {
	bus := dbustest.Start(t)
	ctl, err := openNightlight(func() (*dbus.Conn, error) { return bus.Conn(t), nil })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ctl.close() })
	if _, err := ctl.read(t.Context()); err == nil {
		t.Error("the night light read without lys-shell on the bus")
	}
}

// The temperature slider's track is white as the filter tints it: red
// with no blue at the warmest end, white at the neutral end, and bluer
// at every step towards it.
func TestTheTemperatureTrackRunsFromWarmToNeutral(t *testing.T) {
	track := kelvinTrack()
	if len(track) != kelvinStops {
		t.Fatalf("%d stops, want %d", len(track), kelvinStops)
	}
	if warm := track[0]; warm.R() != 255 || warm.B() != 0 {
		t.Errorf("warmest stop %#x, want full red and no blue", uint32(warm))
	}
	if cool := track[len(track)-1]; cool != render.RGB(255, 255, 255) {
		t.Errorf("coolest stop %#x, want white", uint32(cool))
	}
	for i := 2; i < len(track); i++ {
		if track[i].B() <= track[i-1].B() {
			t.Errorf("stop %d (%#x) is no bluer than stop %d (%#x)", i, uint32(track[i]), i-1, uint32(track[i-1]))
		}
	}
}

func kelvinRow(t *testing.T, f *fakeNightlight) (*kelvinEditor, *fakeclock.Clock) {
	t.Helper()
	k := testKit(t, "modules:\n  nightlight:\n    temperature: 3200\n")
	clock := fakeclock.New()
	k.clock = clock
	k.nightlight = func() (nightlightControl, error) { return f, nil }
	c, ok := fieldControl(k, k.store, pageRow(t, displaysID, "modules.nightlight.temperature")).(*kelvinEditor)
	if !ok {
		t.Fatal("the temperature is not a kelvin slider")
	}
	return c, clock
}

// Preview shows the slider's temperature through lys-shell and says so
// until lys-shell puts the screen back.
func TestTheTemperaturePreviewShowsTheSlidersValueForAWhile(t *testing.T) {
	f := &fakeNightlight{}
	c, clock := kelvinRow(t, f)
	c.button.OnClick()
	if !slices.Equal(f.previews, []uint32{3200}) {
		t.Fatalf("previewed %v, want 3200", f.previews)
	}
	if c.button.Enabled() || c.label.Text() != locale.T().Get("settings-nightlight-previewing") {
		t.Errorf("while previewing the button is enabled %v and reads %q", c.button.Enabled(), c.label.Text())
	}
	clock.Advance(nightlight.PreviewLength - time.Millisecond)
	if c.button.Enabled() {
		t.Error("the button came back before the preview ended")
	}
	clock.Advance(time.Millisecond)
	if !c.button.Enabled() || c.label.Text() != locale.T().Get("settings-nightlight-preview") || c.problem.Visible() {
		t.Errorf("after the preview the button is enabled %v, reads %q, problem %q", c.button.Enabled(), c.label.Text(), c.problem.Text())
	}
}

func TestARefusedTemperaturePreviewIsSaid(t *testing.T) {
	f := &fakeNightlight{previewErr: errors.New("temperature refused")}
	c, _ := kelvinRow(t, f)
	c.button.OnClick()
	if !c.problem.Visible() || !strings.Contains(c.problem.Text(), "temperature refused") {
		t.Errorf("a refused preview shows %q (%v)", c.problem.Text(), c.problem.Visible())
	}
	if !c.button.Enabled() {
		t.Error("a refused preview left the button disabled")
	}
}
