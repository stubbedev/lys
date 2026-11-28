package config

import (
	"strings"
	"testing"

	"github.com/stubbedev/lys/wm/internal/compositor"
	"github.com/stubbedev/lys/wm/internal/render"
	"github.com/stubbedev/lys/wm/internal/wm"
)

func TestOutputsTakeATransformAMirrorAndReservedEdges(t *testing.T) {
	if _, err := Parse("t.yaml", []byte("outputs:\n  - match: DP-1\n  - match: 'desc:DP-1'\n  - match: HDMI-A-1\n    mirror: DP-1\n")); err != nil {
		t.Errorf("a name and a description that read alike, and a mirror of another output: %v", err)
	}
	cfg, err := Parse("t.yaml", []byte("outputs:\n  - match: DP-1\n    transform: flipped-270\n    reserved: [10, 0, 20, 5]\n  - match: HDMI-A-1\n    mirror: 'desc:Dell U2720Q'\n  - match: eDP-1\n    transform: normal\n"))
	if err != nil {
		t.Fatal(err)
	}
	if r := cfg.Outputs.Named[0]; r.Transform == nil || *r.Transform != render.Flipped270 || r.Reserved != (compositor.Insets{Top: 10, Bottom: 20, Left: 5}) || r.Mirror != nil {
		t.Errorf("the first rule %+v", r)
	}
	if r := cfg.Outputs.Named[1]; r.Mirror == nil || *r.Mirror != (wm.OutputMatch{Description: "Dell U2720Q"}) || r.Transform != nil {
		t.Errorf("the mirror rule %+v, want no transform so the panel's applies", r)
	}
	if r := cfg.Outputs.Named[2]; r.Transform == nil || *r.Transform != render.Normal {
		t.Errorf("the rule that sets normal %+v, want it to override the panel's orientation", r)
	}
	for text, problem := range map[string]string{
		"outputs:\n  - match: DP-1\n    transform: 45\n":                                  `"45" is not one of normal, 90`,
		"outputs:\n  - match: DP-1\n    reserved: [1, 2, 3]\n":                            "needs four numbers",
		"outputs:\n  - match: DP-1\n    reserved: [1, 2, 3, -4]\n":                        "none below 0",
		"outputs:\n  - match: DP-1\n    mirror: '*'\n":                                    "needs a connector name",
		"outputs:\n  - match: DP-1\n    mirror: DP-1\n":                                   `outputs[0].mirror: "DP-1" is the output this rule configures`,
		"outputs:\n  - match: 'desc:Dell'\n    mirror: 'desc:Dell'\n":                     `"desc:Dell" is the output this rule configures`,
		"outputs:\n  - match: DP-1\n  - match: HDMI-A-1\n  - match: DP-1\n    scale: 2\n": `outputs[2].match: "DP-1" is matched by an earlier rule too`,
	} {
		if _, err := Parse("t.yaml", []byte(text)); err == nil || !strings.Contains(err.Error(), problem) {
			t.Errorf("%q: %v, want %q", text, err, problem)
		}
	}
}
