package layers

import (
	"regexp"
	"testing"
)

func TestEachSurfacesPatternMatchesItsOwnNamespacesAlone(t *testing.T) {
	namespaces := map[Kind][]string{}
	for _, k := range All {
		switch s := k.(type) {
		case Surface:
			namespaces[k] = []string{string(s)}
		case OutputSurface:
			namespaces[k] = []string{s.On("DP-1"), s.On("eDP-1")}
		default:
			t.Fatalf("%T is no surface kind", k)
		}
	}
	seen := map[string]bool{}
	for _, k := range All {
		if seen[k.Name()] {
			t.Errorf("%s is listed twice", k.Name())
		}
		seen[k.Name()] = true
		re := regexp.MustCompile(k.Pattern())
		for other, names := range namespaces {
			for _, name := range names {
				if got, want := re.MatchString(name), other == k; got != want {
					t.Errorf("%s's pattern %q matches %s: %t, want %t", k.Name(), k.Pattern(), name, got, want)
				}
			}
		}
	}
	if got := Bar.On("HDMI-A-1"); got != "lys-shell-bar-HDMI-A-1" {
		t.Errorf("the bar on HDMI-A-1 is %q", got)
	}
}
