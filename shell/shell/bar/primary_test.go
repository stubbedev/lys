package bar

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/ui/app"
)

func TestPrimaryFirstFollowsLysWMsPrimaryOutput(t *testing.T) {
	f := newFakeLysWM(t)
	f.Result("outputs", `[{"name":"DP-1"},{"name":"HDMI-A-1","primary":true}]`)
	dp, hdmi := &app.Output{Name: "DP-1"}, &app.Output{Name: "HDMI-A-1"}
	var p primaryOutput
	stop := p.follow()
	defer stop()
	outputs := p.first(func() []*app.Output { return []*app.Output{dp, hdmi} })
	if got := outputs(); !slices.Equal(got, []*app.Output{hdmi, dp}) {
		t.Fatalf("outputs %v, want lys-wm's primary HDMI-A-1 first", got)
	}
	f.Events <- `{"event":"output","change":"changed","output":{"name":"HDMI-A-1","primary":false}}`
	waittest.For(t, p.watcher.Load().Ticks(), "the output event")
	if got := outputs(); !slices.Equal(got, []*app.Output{dp, hdmi}) {
		t.Errorf("with no primary, outputs %v, want the order listed", got)
	}
}

func TestPrimaryFirstKeepsTheOrderWithoutLysWM(t *testing.T) {
	clearCompositorEnv(t)
	dp, hdmi := &app.Output{Name: "DP-1"}, &app.Output{Name: "HDMI-A-1"}
	var p primaryOutput
	p.follow()()
	if got := p.first(func() []*app.Output { return []*app.Output{dp, hdmi} })(); !slices.Equal(got, []*app.Output{dp, hdmi}) {
		t.Errorf("without lys-wm, outputs %v, want the order listed", got)
	}
}
