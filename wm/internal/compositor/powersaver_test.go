package compositor

import (
	"errors"
	"image"
	"testing"

	"github.com/stubbedev/lys/wm/internal/backend"
	"github.com/stubbedev/lys/wm/internal/backend/headless"
	"github.com/stubbedev/lys/wm/internal/loop"
)

func TestLowRefreshModeTakesTheSlowestModeOfTheSizeAtOrAboveTheRate(t *testing.T) {
	big, small := image.Pt(2880, 1800), image.Pt(1920, 1200)
	mode := func(size image.Point, hz int32) backend.Mode { return backend.Mode{Size: size, RefreshMHz: hz * 1000} }
	for _, tc := range []struct {
		name   string
		modes  []backend.Mode
		within ModePolicy
		want   backend.Mode
	}{
		{"120 and 60 Hz", []backend.Mode{mode(big, 120), mode(big, 60)}, HighResMode{}, mode(big, 60)},
		{"nothing at or above the rate below the chosen", []backend.Mode{mode(big, 144), mode(big, 48)}, HighResMode{}, mode(big, 144)},
		{"the slowest above the rate", []backend.Mode{mode(big, 165), mode(big, 120), mode(big, 90), mode(big, 40)}, HighResMode{}, mode(big, 90)},
		{"59.94 Hz counts as 60", []backend.Mode{mode(big, 120), {Size: big, RefreshMHz: 59940}}, HighResMode{}, backend.Mode{Size: big, RefreshMHz: 59940}},
		{"never another size", []backend.Mode{mode(big, 120), mode(small, 60)}, HighResMode{}, mode(big, 120)},
		{"within an exact size", []backend.Mode{mode(big, 120), mode(small, 120), mode(small, 60)}, ExactMode{Size: small, RefreshMHz: 120000}, mode(small, 60)},
	} {
		got, err := lowRefreshMode{within: tc.within, hz: DefaultPowerSaverRate}.choose(tc.modes, tc.modes[0])
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got != tc.want {
			t.Errorf("%s: chose %+v, want %+v", tc.name, got, tc.want)
		}
	}
	if _, err := (lowRefreshMode{within: ExactMode{Size: small}, hz: DefaultPowerSaverRate}).choose([]backend.Mode{mode(big, 120)}, mode(big, 120)); err == nil {
		t.Error("a missing exact mode chose a mode under power saver, want the exact policy's error")
	}
}

// The power profile read before the first output lights picks the
// internal panel's first mode: under power saver it lights in its slow
// mode at once, so taking over a greeter's frame left in that mode needs
// no modeset; without power saver it lights in its fastest.
func TestThePowerSaverAtStartPicksTheFirstMode(t *testing.T) {
	big := image.Pt(2880, 1800)
	for _, tc := range []struct {
		saver bool
		want  int32
	}{{true, 60000}, {false, 120000}} {
		var b *headless.Backend
		r := setupWith(t, func(l *loop.Loop, _ *spyRenderer) backend.Backend {
			b = headless.New(l)
			if _, err := b.AddNamedOutput("eDP-1", backend.Mode{Size: big, RefreshMHz: 120000}, backend.Mode{Size: big, RefreshMHz: 60000}); err != nil {
				t.Fatal(err)
			}
			return b
		}, func(cfg *Config) { cfg.PowerSaverAtStart = tc.saver })
		var got backend.Mode
		r.on(func() { got = b.Outputs()[0].State().Mode })
		if got.Size != big || got.RefreshMHz != tc.want {
			t.Errorf("with power saver %v at start the panel lit in %+v, want %d mHz", tc.saver, got, tc.want)
		}
	}
}

// Changing the internal panel's mode is a full modeset that blanks the
// panel for a moment, and the xe laptop's 60 Hz mode is its 120 Hz one
// with a long vertical blank, which no seamless update reaches: every
// plug and unplug blanked the screen. Power saver now caps the frame
// rate at once and changes the lit panel's mode only when it next
// lights (power on after idle, lid open), where the modeset is unseen.
func TestPowerSaverChangesTheLitPanelsModeOnlyWhenItNextLights(t *testing.T) {
	big := image.Pt(2880, 1800)
	var b *headless.Backend
	r := setupWith(t, func(l *loop.Loop, _ *spyRenderer) backend.Backend {
		b = headless.New(l)
		if _, err := b.AddNamedOutput("eDP-1", backend.Mode{Size: big, RefreshMHz: 120000, Preferred: true}, backend.Mode{Size: big, RefreshMHz: 60000}); err != nil {
			t.Fatal(err)
		}
		return b
	}, func(*Config) {})
	mode := func() int32 {
		var hz int32
		r.on(func() { hz = b.Outputs()[0].State().Mode.RefreshMHz })
		return hz
	}
	cycle := func() {
		r.on(func() {
			o := r.comp.outputs[0]
			if err := errors.Join(r.comp.SetPower(o, false), r.comp.SetPower(o, true)); err != nil {
				t.Fatal(err)
			}
		})
	}
	var capped int
	r.on(func() {
		if err := r.comp.SetPowerSaver(true); err != nil {
			t.Fatal(err)
		}
		capped = b.Outputs()[0].FrameRateCap()
	})
	if got := mode(); got != 120000 {
		t.Fatalf("power saver switched the lit panel to %d mHz at once", got)
	}
	if capped != DefaultPowerSaverRate {
		t.Fatalf("power saver capped the lit panel at %d fps, want %d at once", capped, DefaultPowerSaverRate)
	}
	cycle()
	if got := mode(); got != 60000 {
		t.Fatalf("the panel lit again under power saver in %d mHz, want 60000", got)
	}
	r.on(func() {
		if err := r.comp.SetPowerSaver(false); err != nil {
			t.Fatal(err)
		}
	})
	if got := mode(); got != 60000 {
		t.Fatalf("leaving power saver switched the lit panel to %d mHz at once", got)
	}
	cycle()
	if got := mode(); got != 120000 {
		t.Fatalf("the panel lit again out of power saver in %d mHz, want 120000", got)
	}
}
