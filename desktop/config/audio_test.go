package config

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func diagnosticsTitled(err error, title string) []Diagnostic {
	var out []Diagnostic
	for _, e := range unwrapJoined(err) {
		if d, ok := errors.AsType[Diagnostic](e); ok && d.Title == title {
			out = append(out, d)
		}
	}
	return out
}

func TestAudioSectionLoads(t *testing.T) {
	cfg, err := LoadFile(writeConfig(t, `modules:
  audio:
    speaker-icons: [a, b]
    speaker-icon-muted: s-off
    mic-icon: m-on
    mic-icon-muted: m-off
    mic-hide-when-idle: true
    capture-camera-icon: cam
    capture-screen-icon: scr
    capture-color: status-error
    format: "{{ percent }} {{ mic_percent }}"
    scroll-up: lys-shell audio input-volume +2
    dropdown-app-icons: native
`))
	if err != nil {
		t.Fatalf("a valid audio section: %v", err)
	}
	a := cfg.Audio
	if !slices.Equal(a.SpeakerIcons, []string{"a", "b"}) || a.SpeakerIconMuted != "s-off" || a.MicIcon != "m-on" ||
		a.MicIconMuted != "m-off" || !a.MicHideWhenIdle || a.CaptureCameraIcon != "cam" || a.CaptureScreenIcon != "scr" ||
		a.CaptureColor != mustColor("status-error") || a.Format != "{{ percent }} {{ mic_percent }}" ||
		a.DropdownAppIcons != AppIconSourceNative {
		t.Errorf("audio = %+v", a)
	}
	if a.ScrollUp.Kind != ClickShell || a.ScrollUp.Command != "lys-shell audio input-volume +2" {
		t.Errorf("scroll-up = %+v, want the input-volume command", a.ScrollUp)
	}
	if a.ScrollDown != DefaultsAudio().ScrollDown {
		t.Errorf("an unset scroll-down = %+v, want the default", a.ScrollDown)
	}
}

func TestAudioDefaultsTargetTheSpeaker(t *testing.T) {
	a := DefaultsAudio()
	for name, tc := range map[string]struct {
		got  ClickAction
		want string
	}{
		"left-click":   {a.LeftClick, "dropdown:audio"},
		"middle-click": {a.MiddleClick, "lys-shell audio output-mute"},
		"scroll-up":    {a.ScrollUp, "lys-shell audio output-volume +5"},
		"scroll-down":  {a.ScrollDown, "lys-shell audio output-volume -5"},
		"right-click":  {a.RightClick, ""},
	} {
		if tc.got.String() != tc.want {
			t.Errorf("%s = %q, want %q", name, tc.got.String(), tc.want)
		}
	}
	if !slices.Contains(DefaultBarLayout().Right, BarItem{Module: ModuleAudio}) {
		t.Errorf("the default layout's right section %+v has no audio module", DefaultBarLayout().Right)
	}
}

func TestInvalidAudioValuesAreErrors(t *testing.T) {
	for _, tc := range []struct{ body, field string }{
		{"capture-color: not-a-color", "modules.audio.capture-color"},
		{"dropdown-app-icons: theme", "modules.audio.dropdown-app-icons"},
		{"mic-hide-when-idle: sometimes", "modules.audio.mic-hide-when-idle"},
		{"speaker-icons: ld-volume-symbolic", "modules.audio.speaker-icons"},
	} {
		cfg, err := LoadFile(writeConfig(t, "modules:\n  audio:\n    "+tc.body+"\n    mic-icon: kept\n"))
		diags := diagnosticsTitled(err, "invalid config value")
		if len(diags) != 1 || diags[0].Field("Field") != tc.field {
			t.Errorf("%q: diagnostics %+v from %v, want one for %s", tc.body, diags, err, tc.field)
			continue
		}
		if cfg.Audio.MicIcon != "kept" {
			t.Errorf("%q: the section's valid keys did not apply: mic-icon %q", tc.body, cfg.Audio.MicIcon)
		}
	}
}

func TestRemovedModulesInALayoutNameAudio(t *testing.T) {
	for _, name := range []string{"volume", "microphone", "privacy", "screen-share"} {
		want := "bar module `" + name + "` was removed; use `audio`"
		for shape, left := range map[string]string{
			"plain":   name,
			"classed": "{module: " + name + ", class: x}",
			"grouped": "{name: g, modules: [clock, " + name + "]}",
		} {
			cfg, err := LoadFile(writeConfig(t, "bar:\n  layout:\n    - left: [clock, "+left+"]\n"))
			diags := diagnosticsTitled(err, "invalid config value")
			if len(diags) != 1 || diags[0].Field("Error") != want {
				t.Errorf("%s %s: diagnostics %+v from %v, want %q", shape, name, diags, err, want)
			}
			if len(cfg.Bar.Layout) != 1 || !slices.Equal(cfg.Bar.Layout[0].Right, DefaultBarLayout().Right) {
				t.Errorf("%s %s: a refused layout = %+v, want the default", shape, name, cfg.Bar.Layout)
			}
		}
	}
	var module BarModule
	err := module.UnmarshalConfig("volume")
	if removed, ok := errors.AsType[*RemovedModuleError](err); !ok || removed.Module != "volume" || removed.Replacement != ModuleAudio {
		t.Errorf("volume = %v, want a RemovedModuleError naming audio", err)
	}
}

func TestOtherLayoutErrorsAreNotRemovedModules(t *testing.T) {
	for _, left := range []string{"nope", "{module: clock}", "{name: g, modules: [nope]}"} {
		_, err := LoadFile(writeConfig(t, "bar:\n  layout:\n    - left: ["+left+"]\n"))
		if err == nil || strings.Contains(err.Error(), "was removed") || !strings.Contains(err.Error(), "untagged enum BarItem") {
			t.Errorf("%s: %v, want the untagged-enum error", left, err)
		}
	}
	if _, err := LoadFile(writeConfig(t, "bar:\n  layout:\n    - left: [audio, {module: audio, class: x}]\n")); err != nil {
		t.Errorf("audio in a layout: %v", err)
	}
}

func TestRemovedModuleSectionsNameAudio(t *testing.T) {
	cfg, err := LoadFile(writeConfig(t, `modules:
  volume:
    format: "{{ percent }}"
  microphone: {}
  privacy: {}
  screen-share: {}
  audio:
    mic-icon: kept
`))
	diags := diagnosticsTitled(err, "removed config key")
	var fields []string
	for _, d := range diags {
		fields = append(fields, d.Field("Field"))
		name := strings.TrimPrefix(d.Field("Field"), "modules.")
		if d.Kind != DiagnosticError || d.Hint != "modules.audio" || d.Field("Reason") != "bar module `"+name+"` was removed; use `audio`" {
			t.Errorf("diagnostic %+v", d)
		}
	}
	if want := []string{"modules.microphone", "modules.privacy", "modules.screen-share", "modules.volume"}; !slices.Equal(fields, want) {
		t.Errorf("removed sections %v from %v, want %v", fields, err, want)
	}
	if cfg.Audio.MicIcon != "kept" {
		t.Errorf("the audio section did not apply beside the removed ones: %q", cfg.Audio.MicIcon)
	}
	if _, err := LoadFile(writeConfig(t, "modules:\n  audio:\n    mic-icon: kept\n")); err != nil {
		t.Errorf("a modules section without removed modules = %v", err)
	}
}
