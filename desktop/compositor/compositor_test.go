package compositor

import "testing"

func TestDetectTakesTheFirstAdvertisedSocketInOrder(t *testing.T) {
	for _, tc := range []struct {
		set  []string
		want Kind
	}{
		{nil, Unknown},
		{[]string{SwaySocketEnv}, Sway},
		{[]string{MangoSocketEnv}, Mango},
		{[]string{NiriSocketEnv}, Niri},
		{[]string{HyprlandSocketEnv}, Hyprland},
		{[]string{HyprlandSocketEnv, SwaySocketEnv}, Hyprland},
		{[]string{NiriSocketEnv, MangoSocketEnv, SwaySocketEnv}, Niri},
		{[]string{MangoSocketEnv, SwaySocketEnv}, Mango},
		{[]string{LysWMSocketEnv}, LysWM},
		{[]string{LysWMSocketEnv, HyprlandSocketEnv, NiriSocketEnv}, LysWM},
	} {
		env := map[string]string{}
		for _, k := range tc.set {
			env[k] = "/run/user/1000/socket"
		}
		if got := Detect(func(k string) string { return env[k] }); got != tc.want {
			t.Errorf("%v: %v, want %v", tc.set, got, tc.want)
		}
	}
}

func TestAnEmptyVariableAdvertisesNothing(t *testing.T) {
	env := map[string]string{LysWMSocketEnv: "", SwaySocketEnv: "/run/sway"}
	if got := Detect(func(k string) string { return env[k] }); got != Sway {
		t.Errorf("%v, want sway", got)
	}
}

func TestEveryKindHasItsName(t *testing.T) {
	for k, want := range map[Kind]string{Unknown: "unknown", LysWM: "lys-wm", Hyprland: "Hyprland", Niri: "niri", Mango: "mango", Sway: "sway"} {
		if got := k.String(); got != want {
			t.Errorf("%d: %q, want %q", k, got, want)
		}
	}
}
