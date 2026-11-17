package bar

import (
	"errors"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/logtest"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/shell/service/nightlight"
)

type configWrite struct {
	path  string
	value any
}

type nightlightDropdown struct {
	view     *brightnessView
	card     *nightlightCard
	svc      *nightlightService
	writes   *[]configWrite
	saveFail *error
}

func newNightlightDropdown(t *testing.T, cfg *config.Config, env sunsetEnv) nightlightDropdown {
	t.Helper()
	ctx := newTestContext(t, cfg)
	ctx.Brightness = &fakeBrightnessSource{}
	writes := &[]configWrite{}
	saveFail := new(error)
	ctx.SetConfig = func(path string, value any, done func(error)) {
		*writes = append(*writes, configWrite{path, value})
		go done(*saveFail)
	}
	ctx.Nightlight = newNightlightService(ctx, env)
	v := onHeadlessLoop(func() *brightnessView { return brightnessDropdown(ctx).(*brightnessView) })
	t.Cleanup(func() { onHeadlessLoop(func() bool { v.dropdownClosed(); return true }) })
	if v.night == nil {
		t.Fatal("the brightness dropdown has no night light card with a night light running")
	}
	return nightlightDropdown{view: v, card: v.night, svc: ctx.Nightlight, writes: writes, saveFail: saveFail}
}

func (d nightlightDropdown) failure() string {
	if !d.card.failure.Visible() {
		return ""
	}
	return d.card.failure.Text()
}

func TestBrightnessDropdownTogglesTheNightLight(t *testing.T) {
	fake := &fakeSunset{clock: sunsetDay}
	d := newNightlightDropdown(t, sunsetConfig(false), fake.env(t))
	if d.card.toggle.On() || fake.running {
		t.Fatal("the night light starts on")
	}
	onHeadlessLoop(func() bool { d.card.toggle.SetOn(true); return true })
	if !fake.running || !d.svc.enabled || fake.applied.Temp() != 5000 {
		t.Fatalf("switching on: running %v enabled %v at %dK, want the configured 5000K", fake.running, d.svc.enabled, fake.applied.Temp())
	}
	if got, want := itemText(d.card.item, "description"), i18n.T("dropdown-brightness-nightlight-on", i18n.Str("kelvin", "5000")); got != want {
		t.Errorf("description %q, want %q", got, want)
	}
	onHeadlessLoop(func() bool { d.card.toggle.SetOn(false); return true })
	if fake.running || d.svc.enabled {
		t.Fatal("switching off left the filter on")
	}
	if fake.starts != 1 || fake.stops != 1 {
		t.Errorf("starts %d stops %d, want one of each", fake.starts, fake.stops)
	}
	if got := itemText(d.card.item, "description"); got != i18n.T("bar-nightlight-off") {
		t.Errorf("description %q, want off", got)
	}
}

func TestBrightnessDropdownSetsTheNightLightTemperature(t *testing.T) {
	fake := &fakeSunset{clock: sunsetDay}
	d := newNightlightDropdown(t, sunsetConfig(false), fake.env(t))
	if got := d.card.slider.Value(); got != 5000 {
		t.Fatalf("slider at %v, want the configured 5000K", got)
	}
	onHeadlessLoop(func() bool { d.card.slider.Knob.SetValue(3240); return true })
	if d.svc.cfg.Temperature != 3200 || d.svc.night.Temp() != 3200 {
		t.Errorf("service at %dK, want 3200K rounded to the step", d.svc.cfg.Temperature)
	}
	if fake.starts != 0 {
		t.Error("setting the strength while off turned the night light on")
	}
	want := []configWrite{{nightlightTemperaturePath, int64(3200)}}
	if len(*d.writes) != 1 || (*d.writes)[0] != want[0] {
		t.Errorf("config writes %v, want %v", *d.writes, want)
	}
	onHeadlessLoop(func() bool { d.card.toggle.SetOn(true); return true })
	if fake.applied.Temp() != 3200 {
		t.Fatalf("switched on at %dK, want the new 3200K", fake.applied.Temp())
	}
	onHeadlessLoop(func() bool { d.card.slider.Knob.SetValue(2500); return true })
	waitHeadless(t, "a new strength while on shown at once", func() bool { return fake.running && fake.applied.Temp() == 2500 })
	if d.failure() != "" {
		t.Errorf("an error %q after writes that landed", d.failure())
	}
}

func TestBrightnessDropdownRefusedTemperatureKeepsTheServiceValue(t *testing.T) {
	cfg := sunsetConfig(false)
	cfg.Nightlight.Gamma = 300
	fake := &fakeSunset{clock: sunsetDay}
	d := newNightlightDropdown(t, cfg, fake.env(t))
	onHeadlessLoop(func() bool { d.card.slider.Knob.SetValue(3000); return true })
	if d.svc.cfg.Temperature != 5000 {
		t.Errorf("service at %dK after a refused write, want 5000K", d.svc.cfg.Temperature)
	}
	if len(*d.writes) != 0 {
		t.Errorf("a refused strength reached the config: %v", *d.writes)
	}
	if !strings.Contains(d.failure(), "300%") {
		t.Errorf("error %q, want the gamma that refused it", d.failure())
	}
	if got := d.card.slider.Value(); got != 5000 {
		t.Errorf("slider at %v, want it back at the service's 5000K", got)
	}
}

func dragNightlightSlider(d nightlightDropdown, to float64) {
	onHeadlessLoop(func() bool {
		d.card.slider.Knob.SetPressed(true)
		d.card.slider.Knob.SetValue(to)
		d.card.slider.Knob.SetPressed(false)
		return true
	})
}

func TestBrightnessDropdownRefusedDragSnapsTheSliderBack(t *testing.T) {
	cfg := sunsetConfig(false)
	cfg.Nightlight.Gamma = 300
	fake := &fakeSunset{clock: sunsetDay}
	d := newNightlightDropdown(t, cfg, fake.env(t))
	dragNightlightSlider(d, 3000)
	if got := d.card.slider.Value(); got != 5000 {
		t.Errorf("slider at %v after a refused drag, want it back at the service's 5000K", got)
	}
	if !strings.Contains(d.failure(), "300%") {
		t.Errorf("error %q, want the gamma that refused it", d.failure())
	}
}

func TestBrightnessDropdownAcceptedDragKeepsTheSlider(t *testing.T) {
	fake := &fakeSunset{clock: sunsetDay}
	d := newNightlightDropdown(t, sunsetConfig(false), fake.env(t))
	dragNightlightSlider(d, 3200)
	if got := d.card.slider.Value(); got != 3200 {
		t.Errorf("slider at %v after an accepted drag, want 3200K", got)
	}
	if d.svc.cfg.Temperature != 3200 || d.failure() != "" {
		t.Errorf("service at %dK with error %q, want 3200K and none", d.svc.cfg.Temperature, d.failure())
	}
}

func TestBrightnessDropdownFailedSaveRestoresTheSavedTemperature(t *testing.T) {
	fake := &fakeSunset{clock: sunsetDay}
	d := newNightlightDropdown(t, sunsetConfig(false), fake.env(t))
	onHeadlessLoop(func() bool { d.card.toggle.SetOn(true); return true })
	full := errors.New("config: save: no space left on device")
	onHeadlessLoop(func() bool { *d.saveFail = full; return true })
	dragNightlightSlider(d, 3200)
	waitHeadless(t, "the failed save shown", func() bool { return d.failure() == full.Error() })
	if got := onHeadlessLoop(d.card.slider.Value); got != 5000 {
		t.Errorf("slider at %v after a failed save, want the saved 5000K", got)
	}
	if got := onHeadlessLoop(func() uint32 { return d.svc.cfg.Temperature }); got != 5000 {
		t.Errorf("service at %dK after a failed save, want the saved 5000K", got)
	}
	waitHeadless(t, "the display back at the saved strength", func() bool { return fake.applied.Temp() == 5000 })

	onHeadlessLoop(func() bool { *d.saveFail = nil; return true })
	dragNightlightSlider(d, 3200)
	if got := onHeadlessLoop(d.failure); got != "" {
		t.Errorf("a new attempt still shows %q", got)
	}
	waitHeadless(t, "the saved strength recorded", func() bool { return d.svc.savedTemp == 3200 })
	got, failure, applied := onHeadlessLoop(d.card.slider.Value), onHeadlessLoop(d.failure), onHeadlessLoop(func() int { return fake.applied.Temp() })
	if got != 3200 || failure != "" || applied != 3200 {
		t.Errorf("a saved write: slider %v error %q display %dK, want 3200K and no error", got, failure, applied)
	}
}

func TestBrightnessDropdownFailedSaveOfAnOlderStrengthKeepsTheNewer(t *testing.T) {
	fake := &fakeSunset{clock: sunsetDay}
	d := newNightlightDropdown(t, sunsetConfig(false), fake.env(t))
	full := errors.New("config: save: no space left on device")
	onHeadlessLoop(func() bool {
		*d.saveFail = full
		d.card.committed(3200)
		*d.saveFail = nil
		d.card.committed(2800)
		return true
	})
	waitHeadless(t, "both saves answered", func() bool { return d.svc.savedTemp == 2800 })
	if got := onHeadlessLoop(func() uint32 { return d.svc.cfg.Temperature }); got != 2800 {
		t.Errorf("service at %dK, want the newer 2800K kept", got)
	}
}

type refusingSunset struct {
	*fakeSunset
	err error
}

func (f *refusingSunset) Apply(nightlight.Setting) error { return f.err }

func TestBrightnessDropdownShowsAFailedNightLightWrite(t *testing.T) {
	fake := &fakeSunset{clock: sunsetDay}
	env := fake.env(t)
	refusal := errors.New("nightlight: another program owns the display colours")
	env.open = func(func()) (colourFilter, error) { return &refusingSunset{fake, refusal}, nil }
	d := newNightlightDropdown(t, sunsetConfig(false), env)
	onHeadlessLoop(func() bool { d.card.toggle.SetOn(true); return true })
	if d.card.toggle.On() {
		t.Error("the switch stays on although the display refused the filter")
	}
	if d.failure() != refusal.Error() {
		t.Errorf("error %q, want %q", d.failure(), refusal.Error())
	}
	onHeadlessLoop(func() bool { d.card.toggle.SetOn(true); return true })
	if d.card.toggle.On() || d.failure() != refusal.Error() {
		t.Error("a second attempt did not fail the same way")
	}
}

func TestBrightnessDropdownFollowsTheNightLightModule(t *testing.T) {
	fake := &fakeSunset{clock: sunsetDay}
	cfg := sunsetConfig(false)
	cfg.Nightlight.Format = "{{ status }} {{ config_temp }}"
	d := newNightlightDropdown(t, cfg, fake.env(t))
	m := onHeadlessLoop(func() *nightlightModule { return newNightlightModule(d.view.ctx, d.svc) })
	onHeadlessLoop(func() bool { m.RunAction(config.ParseClickAction(":toggle")); return true })
	if !d.card.toggle.On() {
		t.Error("the module's toggle did not show in the dropdown")
	}
	onHeadlessLoop(func() bool { d.card.toggle.SetOn(false); return true })
	if m.icon.Name() != cfg.Nightlight.IconOff || m.btn.Label().Text() != i18n.T("bar-nightlight-off")+" 5000" {
		t.Errorf("the dropdown's switch left the module at icon %q label %q", m.icon.Name(), m.btn.Label().Text())
	}
	if fake.starts != 1 {
		t.Errorf("starts = %d: the dropdown's own sync turned the filter on again", fake.starts)
	}

	onHeadlessLoop(func() bool { d.view.dropdownHidden(); return true })
	onHeadlessLoop(func() bool { m.RunAction(config.ParseClickAction(":toggle")); return true })
	if d.card.toggle.On() {
		t.Error("a hidden dropdown followed the night light")
	}
	onHeadlessLoop(func() bool { d.view.dropdownOpened(); return true })
	if !d.card.toggle.On() {
		t.Error("showing the dropdown again did not show the change it missed")
	}

	onHeadlessLoop(func() bool { d.svc.reconfigure(withTemperature(cfg, 2800).Nightlight); return true })
	if fake.applied.Temp() != 2800 || d.card.slider.Value() != 2800 {
		t.Errorf("a config change to 2800K shows %dK on the display and %v on the slider", fake.applied.Temp(), d.card.slider.Value())
	}
	if got := m.btn.Label().Text(); got != i18n.T("bar-nightlight-on")+" 2800" {
		t.Errorf("module label %q after the config change, want the new strength", got)
	}
}

func withTemperature(cfg *config.Config, kelvin uint32) *config.Config {
	next := *cfg
	next.Nightlight.Temperature = kelvin
	return &next
}

func TestBrightnessDropdownWithoutANightLight(t *testing.T) {
	ctx := newTestContext(t, config.Defaults())
	ctx.Brightness = &fakeBrightnessSource{}
	v := onHeadlessLoop(func() *brightnessView { return brightnessDropdown(ctx).(*brightnessView) })
	defer onHeadlessLoop(func() bool { v.dropdownClosed(); return true })
	if v.night != nil || len(treetest.WithClass(v, "brightness-nightlight")) != 0 {
		t.Error("a night light card without a night light")
	}
}

func TestBrightnessDropdownSaysWhyTheNightLightIsUnavailable(t *testing.T) {
	env := (&fakeSunset{clock: sunsetDay}).env(t)
	env.open = func(func()) (colourFilter, error) { return nil, nightlight.ErrUnsupported }
	logtest.Capture(t)
	d := newNightlightDropdown(t, sunsetConfig(false), env)
	if d.card.toggle.Enabled() || d.card.slider.Enabled() {
		t.Error("the night light's controls stay usable without a colour protocol")
	}
	want := i18n.T("dropdown-brightness-nightlight-unavailable", i18n.Str("reason", nightlight.ErrUnsupported.Error()))
	if got := itemText(d.card.item, "description"); got != want {
		t.Errorf("description %q, want %q", got, want)
	}
	if !widget.HasClass(d.card.card, "brightness-nightlight") {
		t.Error("the card lost its class")
	}
}

func TestNightlightModuleNeedsTheShellsNightLight(t *testing.T) {
	ctx := newTestContext(t, config.Defaults())
	if _, err := newNightlight(ctx); err == nil {
		t.Error("a nightlight module without the shell's night light")
	}
	ctx.Nightlight = &nightlightService{ctx: ctx, unavailable: errors.New("nightlight: no state directory")}
	if _, err := newNightlight(ctx); err == nil || !strings.Contains(err.Error(), "no state directory") {
		t.Errorf("err = %v, want why the night light is unavailable", err)
	}
}

func TestAnOlderFailedSaveOfTheSameStrengthDoesNotUndoANewerOne(t *testing.T) {
	fake := &fakeSunset{clock: sunsetDay}
	d := newNightlightDropdown(t, sunsetConfig(false), fake.env(t))
	onHeadlessLoop(func() bool { d.card.toggle.SetOn(true); return true })
	full := errors.New("config: save: no space left on device")
	onHeadlessLoop(func() bool {
		*d.saveFail = full
		d.card.committed(3200)
		*d.saveFail = nil
		d.card.committed(3200)
		return true
	})
	waitHeadless(t, "the newer save recorded", func() bool { return d.svc.savedTemp == 3200 })
	got, failure, applied := onHeadlessLoop(d.card.slider.Value), onHeadlessLoop(d.failure), onHeadlessLoop(func() int { return fake.applied.Temp() })
	if got != 3200 || failure != "" || applied != 3200 {
		t.Errorf("after an older failed and a newer saved write of 3200K: slider %v error %q display %dK, want 3200K and no error", got, failure, applied)
	}
}
