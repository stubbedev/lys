package config

import "testing"

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
