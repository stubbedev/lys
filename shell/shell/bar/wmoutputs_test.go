package bar

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/ui/app"
)

func followOLED(p *wmOutputs) (<-chan []string, func()) {
	oled := make(chan []string, 8)
	stop := p.follow(func(names []string) { oled <- names })
	return oled, stop
}

func TestPrimaryFirstFollowsLysWMsPrimaryOutput(t *testing.T) {
	f := newFakeLysWM(t)
	f.Result("outputs", `[{"name":"DP-1"},{"name":"HDMI-A-1","primary":true}]`)
	dp, hdmi := &app.Output{Name: "DP-1"}, &app.Output{Name: "HDMI-A-1"}
	var p wmOutputs
	oled, stop := followOLED(&p)
	defer stop()
	waittest.For(t, oled, "the seeded outputs")
	outputs := p.first(func() []*app.Output { return []*app.Output{dp, hdmi} })
	if got := outputs(); !slices.Equal(got, []*app.Output{hdmi, dp}) {
		t.Fatalf("outputs %v, want lys-wm's primary HDMI-A-1 first", got)
	}
	f.Events <- `{"event":"output","change":"changed","output":{"name":"HDMI-A-1","primary":false}}`
	waittest.For(t, oled, "the output event")
	if got := outputs(); !slices.Equal(got, []*app.Output{dp, hdmi}) {
		t.Errorf("with no primary, outputs %v, want the order listed", got)
	}
}

func TestTheWallpaperLearnsLysWMsOLEDOutputs(t *testing.T) {
	f := newFakeLysWM(t)
	f.Result("outputs", `[{"name":"eDP-1","capabilities":{"oled":true}},{"name":"DP-1","capabilities":{"oled":false}}]`)
	var p wmOutputs
	oled, stop := followOLED(&p)
	defer stop()
	if got := waittest.For(t, oled, "the seeded outputs"); !slices.Equal(got, []string{"eDP-1"}) {
		t.Fatalf("seeded OLED outputs %q, want eDP-1", got)
	}
	f.Events <- `{"event":"output","change":"removed","output":{"name":"eDP-1","capabilities":{"oled":true}}}`
	if got := waittest.For(t, oled, "the output event"); len(got) != 0 {
		t.Errorf("OLED outputs after eDP-1 left %q, want none", got)
	}
}

func TestPrimaryFirstKeepsTheOrderWithoutLysWM(t *testing.T) {
	clearCompositorEnv(t)
	dp, hdmi := &app.Output{Name: "DP-1"}, &app.Output{Name: "HDMI-A-1"}
	var p wmOutputs
	oled, stop := followOLED(&p)
	stop()
	if len(oled) != 0 {
		t.Errorf("without lys-wm, OLED outputs %q were reported", <-oled)
	}
	if got := p.first(func() []*app.Output { return []*app.Output{dp, hdmi} })(); !slices.Equal(got, []*app.Output{dp, hdmi}) {
		t.Errorf("without lys-wm, outputs %v, want the order listed", got)
	}
}
