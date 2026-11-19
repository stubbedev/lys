package modes

import (
	"testing"

	"github.com/stubbedev/lys/desktop/compositor"
)

func TestDetectWindowsFollowsTheBarsCompositorOrder(t *testing.T) {
	all := []string{compositor.LysWMSocketEnv, compositor.HyprlandSocketEnv, compositor.NiriSocketEnv, compositor.MangoSocketEnv, compositor.SwaySocketEnv}
	for _, tc := range []struct {
		set  []string
		want WindowBackend
	}{
		{nil, nil},
		{[]string{compositor.SwaySocketEnv}, swayBackend{}},
		{[]string{compositor.MangoSocketEnv, compositor.HyprlandSocketEnv}, hyprlandBackend{}},
		{[]string{compositor.MangoSocketEnv, compositor.SwaySocketEnv}, mangoBackend{}},
		{[]string{compositor.NiriSocketEnv, compositor.MangoSocketEnv}, niriBackend{}},
		{[]string{compositor.LysWMSocketEnv, compositor.HyprlandSocketEnv}, lysWMBackend{}},
	} {
		for _, k := range all {
			t.Setenv(k, "")
		}
		for _, k := range tc.set {
			t.Setenv(k, "/run/test-"+k)
		}
		if got := DetectWindows(); got != tc.want {
			t.Errorf("%v: %T, want %T", tc.set, got, tc.want)
		}
	}
}
