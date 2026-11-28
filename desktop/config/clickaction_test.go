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
