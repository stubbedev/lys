package config

import (
	"strings"
	"testing"
)

func TestTheShellsBuiltinBindingsParseAtLoad(t *testing.T) {
	for _, b := range ShellBuiltins() {
		a := ParseClickAction(b.String())
		if a.Kind != ClickShellBuiltin || a.Builtin != b || a.Command != "" || a.String() != b.String() {
			t.Errorf("%q = %+v", b, a)
		}
	}
	for _, s := range []string{"lys-shell idle toggle -i", "lys-shell notify dnd --until 10", "lys-shell recorder"} {
		if a := ParseClickAction(s); a.Kind != ClickShell || a.Command != s {
			t.Errorf("%q = %+v, want a shell command", s, a)
		}
	}
}

func TestTheDefaultTogglesAreBuiltins(t *testing.T) {
	for _, c := range []struct {
		name string
		got  ClickAction
		want ShellBuiltin
	}{
		{"idle-inhibit left-click", DefaultsIdleInhibit().LeftClick, ShellBuiltinIdleToggleIndefinite},
		{"idle-inhibit right-click", DefaultsIdleInhibit().RightClick, ShellBuiltinIdleToggle},
		{"notifications right-click", DefaultsNotification().RightClick, ShellBuiltinDndToggle},
		{"recorder left-click", DefaultsRecorder().LeftClick, ShellBuiltinRecorderToggle},
	} {
		if c.got.Kind != ClickShellBuiltin || c.got.Builtin != c.want {
			t.Errorf("%s = %+v, want the %s builtin", c.name, c.got, c.want)
		}
	}
}

// A binding the schema cannot mean is a load error naming the key, never
// a silent no-op or a shell command; the spellings it does mean load.
func TestAMalformedClickActionIsALoadError(t *testing.T) {
	for _, c := range []struct{ yaml, key, want string }{
		{"modules:\n  brightness:\n    scroll-up: brightness:+5x\n", "modules.brightness.scroll-up", `"brightness:+5x", expected brightness:toggle`},
		{"modules:\n  clock:\n    left-click: \"dropdown:\"\n", "modules.clock.left-click", `"dropdown:", expected dropdown:<name>`},
		{"modules:\n  workspaces:\n    scroll-up: focus:nxt\n", "modules.workspaces.scroll-up", "\"focus:nxt\", expected one of `focus:this`, `focus:next`, `focus:previous`, `focus:last`"},
		{"modules:\n  workspaces:\n    left-click: \"dropdown:\"\n", "modules.workspaces.left-click", `"dropdown:", expected dropdown:<name>`},
	} {
		_, err := LoadFile(writeFile(t, t.TempDir(), "config.yaml", c.yaml))
		if err == nil || !strings.Contains(err.Error(), c.key) || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%q loaded with %v, want an error at %s saying %s", c.yaml, err, c.key, c.want)
		}
	}
	cfg, err := LoadFile(writeFile(t, t.TempDir(), "config.yaml",
		"modules:\n  brightness:\n    scroll-up: brightness:-7\n    scroll-down: brightness:toggle\n  workspaces:\n    scroll-up: focus:last\n    left-click: dropdown:ws\n    right-click: focus-thing\n"))
	if err != nil {
		t.Fatal(err)
	}
	if b := cfg.Brightness; b.ScrollUp != (ClickAction{Kind: ClickBrightness, Brightness: -7}) || b.ScrollDown.Kind != ClickBrightnessToggle {
		t.Errorf("brightness bindings = %+v / %+v", b.ScrollUp, b.ScrollDown)
	}
	w := cfg.Workspaces
	if w.ScrollUp.Kind != WorkspaceClickFocusLast || w.LeftClick != (WorkspaceClickAction{Kind: WorkspaceClickDropdown, Arg: "ws"}) || w.RightClick != (WorkspaceClickAction{Kind: WorkspaceClickShell, Arg: "focus-thing"}) {
		t.Errorf("workspace bindings = %+v", w)
	}
}

func TestScreenshotModesAndReadingsParse(t *testing.T) {
	for _, m := range ScreenshotModes {
		if got, err := ParseScreenshotMode(string(m)); err != nil || got != m {
			t.Errorf("ParseScreenshotMode(%q) = %q %v", m, got, err)
		}
	}
	for _, r := range ScreenReadings {
		if got, err := ParseScreenReading(string(r)); err != nil || got != r {
			t.Errorf("ParseScreenReading(%q) = %q %v", r, got, err)
		}
	}
	if _, err := ParseScreenshotMode("regoin"); err == nil || err.Error() != "unknown screenshot mode \"regoin\", expected one of `region`, `output`, `screen`, `window`" {
		t.Errorf("ParseScreenshotMode(regoin) = %v", err)
	}
	if _, err := ParseScreenshotMode(""); err == nil {
		t.Error("an empty screenshot mode parsed")
	}
	if _, err := ParseScreenReading("ocr"); err == nil || err.Error() != "unknown screen reading \"ocr\", expected `text` or `qr`" {
		t.Errorf("ParseScreenReading(ocr) = %v", err)
	}
}

func TestScreenshotBindingsParseAtLoad(t *testing.T) {
	for _, c := range []struct {
		line, canonical string
		want            ClickAction
	}{
		{"lys-shell screenshot", "lys-shell screenshot region", ClickAction{Kind: ClickScreenshot, Screenshot: ScreenshotBinding{Mode: ScreenshotRegion}}},
		{"lys-shell screenshot region", "", ClickAction{Kind: ClickScreenshot, Screenshot: ScreenshotBinding{Mode: ScreenshotRegion}}},
		{"lys-shell screenshot output", "", ClickAction{Kind: ClickScreenshot, Screenshot: ScreenshotBinding{Mode: ScreenshotOutput}}},
		{"lys-shell  screenshot output DP-1 ", "lys-shell screenshot output DP-1", ClickAction{Kind: ClickScreenshot, Screenshot: ScreenshotBinding{Mode: ScreenshotOutput, Output: "DP-1"}}},
		{"lys-shell screenshot screen", "", ClickAction{Kind: ClickScreenshot, Screenshot: ScreenshotBinding{Mode: ScreenshotScreen}}},
		{"lys-shell screenshot window", "", ClickAction{Kind: ClickScreenshot, Screenshot: ScreenshotBinding{Mode: ScreenshotWindow}}},
		{"lys-shell screenshot text", "", ClickAction{Kind: ClickScreenRead, Reading: ScreenReadText}},
		{"lys-shell screenshot qr", "", ClickAction{Kind: ClickScreenRead, Reading: ScreenReadQR}},
	} {
		a := ParseClickAction(c.line)
		if a != c.want {
			t.Errorf("%q = %+v, want %+v", c.line, a, c.want)
		}
		canonical := c.canonical
		if canonical == "" {
			canonical = c.line
		}
		if a.String() != canonical {
			t.Errorf("%q writes back as %q, want %q", c.line, a.String(), canonical)
		}
	}
	for _, line := range []string{"lys-shell screenshots region", "lys-shell recorder screenshot", "screenshot region"} {
		if a := ParseClickAction(line); a.Kind != ClickShell || a.Command != line {
			t.Errorf("%q = %+v, want a shell command", line, a)
		}
	}
	d := DefaultsScreenshot()
	if d.LeftClick.Screenshot.Mode != ScreenshotRegion || d.RightClick.Screenshot.Mode != ScreenshotOutput || d.MiddleClick.Screenshot.Mode != ScreenshotWindow {
		t.Errorf("screenshot defaults = %+v / %+v / %+v", d.LeftClick, d.RightClick, d.MiddleClick)
	}
}

// A `lys-shell screenshot` binding the shell cannot run is a load error
// naming the key, never a capture that fails at the click.
func TestAMalformedScreenshotBindingIsALoadError(t *testing.T) {
	for _, c := range []struct{ line, want string }{
		{"lys-shell screenshot regoin", "unknown screenshot mode \"regoin\""},
		{"lys-shell screenshot window 0x1", "window takes no further argument"},
		{"lys-shell screenshot region DP-1", "region takes no further argument"},
		{"lys-shell screenshot output DP-1 DP-2", "output takes no further argument"},
		{"lys-shell screenshot qr now", "qr takes no argument"},
	} {
		yaml := "modules:\n  screenshot:\n    left-click: " + c.line + "\n"
		_, err := LoadFile(writeFile(t, t.TempDir(), "config.yaml", yaml))
		if err == nil || !strings.Contains(err.Error(), "modules.screenshot.left-click") || !strings.Contains(err.Error(), c.want) ||
			!strings.Contains(err.Error(), screenshotBindingUsage) {
			t.Errorf("%q loaded with %v, want an error at modules.screenshot.left-click saying %s", c.line, err, c.want)
		}
	}
}
