package config

import (
	"errors"
	"strings"
	"testing"
)

func TestReducedMotionTurnsEveryTransitionIntoAFade(t *testing.T) {
	a := DefaultsAnimations()
	slide, none := AnimationSlideUp, AnimationNone
	a.Transition = AnimationBounce
	a.Launcher.Enter = &slide
	a.Osd.Enter = &none
	if got := a.TransitionFor(AnimLauncher, false); got != AnimationSlideUp {
		t.Fatalf("launcher without reduced motion: %v", got)
	}
	a.ReducedMotion = true
	for _, s := range []AnimSurface{AnimNotifications, AnimDropdown, AnimLauncher, AnimPower} {
		for _, exiting := range []bool{false, true} {
			if got := a.TransitionFor(s, exiting); got != AnimationFade {
				t.Errorf("surface %d exiting %v: %v", s, exiting, got)
			}
			if a.DurationFor(s, exiting) == 0 {
				t.Errorf("surface %d exiting %v has no fade", s, exiting)
			}
		}
	}
	if got := a.TransitionFor(AnimOsd, false); got != AnimationNone {
		t.Errorf("a surface set to none: %v", got)
	}
	if a.PagesSlide() {
		t.Error("pages still slide")
	}
	if css := a.CSSOverrides(); !strings.Contains(css, "--cfg-anim-spin: 0s") || strings.Contains(css, "--duration-normal: 0ms") {
		t.Errorf("indicators should stop and colour fades stay: %s", css)
	}
	a.ReducedMotion, a.Enabled = true, false
	if got := a.TransitionFor(AnimLauncher, false); got != AnimationNone {
		t.Errorf("animations off: %v", got)
	}
}

func TestTheOwningCompositorDecidesAndTheKeySaysItHasNoEffect(t *testing.T) {
	var warnings []Diagnostic
	s := Load(t.TempDir(), func(d Diagnostic) { warnings = append(warnings, d) })
	var heard int
	s.Subscribe(func(_, next *Config) {
		if next.Animations.Reduced() {
			heard++
		}
	})
	s.SetSessionReducedMotion(true)
	s.SetSessionReducedMotion(true)
	if heard != 1 || !s.Config().Animations.Reduced() {
		t.Fatalf("heard %d, reduced %v", heard, s.Config().Animations.Reduced())
	}
	if len(warnings) != 0 {
		t.Fatalf("warned with the key unset: %v", warnings)
	}
	if err := s.SetByPath("animations.duration", ParseCLIValue("99")); err != nil {
		t.Fatal(err)
	}
	if !s.Config().Animations.Reduced() {
		t.Error("a config change forgot the compositor's say")
	}

	s.SetSessionReducedMotion(false)
	if err := s.SetByPath(reducedMotionKey, true); err != nil {
		t.Fatal(err)
	}
	if s.Config().Animations.Reduced() {
		t.Error("the key overruled the owning compositor")
	}
	if len(warnings) != 1 || warnings[0].Kind != DiagnosticWarning || warnings[0].Field("key") != reducedMotionKey || !strings.Contains(warnings[0].Hint, "animations.reduced_motion") {
		t.Fatalf("warnings %+v", warnings)
	}
	if err := s.SetByPath("animations.duration", ParseCLIValue("98")); err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 {
		t.Errorf("warned again with the key unchanged: %v", warnings)
	}
	if err := s.SetByPath(reducedMotionKey, false); err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 2 {
		t.Errorf("no warning for the key's new value: %v", warnings)
	}

	if err := s.SetByPath(reducedMotionKey, true); err != nil {
		t.Fatal(err)
	}
	warned := len(warnings)
	s.ClearSessionReducedMotion()
	if !s.Config().Animations.Reduced() {
		t.Error("the key does not count once the compositor has gone")
	}
	if len(warnings) != warned {
		t.Errorf("warned with no compositor: %v", warnings[warned:])
	}
	s.SetSessionReducedMotion(false)
	if len(warnings) != warned+1 {
		t.Errorf("no warning when a compositor takes the setting over a set key: %v", warnings)
	}
}

func TestMenusAreInstantUnlessAnimationsMenuSaysOtherwise(t *testing.T) {
	a := DefaultsAnimations()
	a.Duration = 900
	long := uint32(700)
	a.EnterDuration, a.ExitDuration = &long, &long
	for _, exiting := range []bool{false, true} {
		if got := a.DurationFor(AnimMenu, exiting); got != 0 {
			t.Errorf("menu exiting %v: %dms by default, want 0 whatever the global durations", exiting, got)
		}
		if got := a.DurationFor(AnimDropdown, exiting); got != 700 {
			t.Errorf("dropdown exiting %v: %dms, want the global 700", exiting, got)
		}
	}
	slide := AnimationSlideDown
	a.Menu = SurfaceAnimation{EnterDuration: &long, Enter: &slide}
	if got := a.DurationFor(AnimMenu, false); got != 700 {
		t.Errorf("a configured menu enter: %dms, want 700", got)
	}
	if got := a.TransitionFor(AnimMenu, false); got != AnimationSlideDown {
		t.Errorf("a configured menu enter: %v, want slide-down", got)
	}
	a.Enabled = false
	if got := a.DurationFor(AnimMenu, false); got != 0 {
		t.Errorf("animations off: a menu enter of %dms, want 0", got)
	}
}

func TestAnInvalidMenuAnimationFailsAndKeepsMenusInstant(t *testing.T) {
	for raw, wantErr := range map[string]string{
		"{enter-duration: -5}":   "invalid value: integer `-5`, expected u32",
		"{enter: wobble}":        "unknown variant `wobble`",
		"{enter-duration: soon}": `invalid type: string "soon", expected u32`,
	} {
		path := writeFile(t, t.TempDir(), "config.yaml", "animations:\n  menu: "+raw+"\n")
		cfg, err := LoadFile(path)
		var d Diagnostic
		if !errors.As(err, &d) || !strings.HasPrefix(d.Field("Field"), "animations.menu") || !strings.Contains(d.Field("Error"), wantErr) {
			t.Errorf("menu %s: error %v, want %q at animations.menu", raw, err, wantErr)
		}
		if got := cfg.Animations.DurationFor(AnimMenu, false); got != 0 {
			t.Errorf("menu %s: a rejected value left a %dms enter, want the instant default", raw, got)
		}
	}
}
