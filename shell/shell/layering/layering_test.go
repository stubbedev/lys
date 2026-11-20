package layering

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/app"
)

func TestForEachConfigLayer(t *testing.T) {
	for layer, want := range map[config.Layer]app.Layer{
		config.LayerBackground: app.LayerBackground,
		config.LayerBottom:     app.LayerBottom,
		config.LayerTop:        app.LayerTop,
		config.LayerOverlay:    app.LayerOverlay,
	} {
		if got := For(config.GeneralConfig{}, layer); got != want {
			t.Errorf("For(%q) = %d, want %d", layer, got, want)
		}
	}
	if got := For(config.GeneralConfig{}, "middle"); got != app.LayerTop {
		t.Errorf("an unknown layer = %d, want top", got)
	}
}

func TestTearingModeDemotesOnlyOverlay(t *testing.T) {
	tearing := config.GeneralConfig{TearingMode: true}
	if got := For(tearing, config.LayerOverlay); got != app.LayerTop {
		t.Errorf("overlay under tearing mode = %d, want top", got)
	}
	if got := For(tearing, config.LayerBottom); got != app.LayerBottom {
		t.Errorf("bottom under tearing mode = %d, want bottom", got)
	}
}

func TestOutputPicksTheMonitor(t *testing.T) {
	dp, hdmi := &app.Output{Name: "DP-1"}, &app.Output{Name: "HDMI-A-1"}
	outs := []*app.Output{dp, hdmi}
	if got := Output(outs, config.PopupMonitor{}); got != dp {
		t.Errorf("primary = %v, want the first output", got)
	}
	if got := Output(outs, config.OsdOnConnector("HDMI-A-1")); got != hdmi {
		t.Errorf("connector = %v, want HDMI-A-1", got)
	}
	if got := Output(outs, config.PopupOnConnector("DP-9")); got != dp {
		t.Errorf("missing connector = %v, want the primary fallback", got)
	}
	if got := Output(nil, config.OsdMonitor{}); got != nil {
		t.Errorf("no outputs = %v, want nil", got)
	}
}

func TestPrimaryFirstMovesOnlyThePrimaryToTheFront(t *testing.T) {
	dp, hdmi, edp := &app.Output{Name: "DP-1"}, &app.Output{Name: "HDMI-A-1"}, &app.Output{Name: "eDP-1"}
	outs := []*app.Output{dp, hdmi, edp}
	for primary, want := range map[string][]*app.Output{
		"eDP-1":    {edp, dp, hdmi},
		"HDMI-A-1": {hdmi, dp, edp},
		"DP-1":     {dp, hdmi, edp},
		"DP-9":     {dp, hdmi, edp},
		"":         {dp, hdmi, edp},
	} {
		if got := PrimaryFirst(outs, primary); !slices.Equal(got, want) {
			t.Errorf("PrimaryFirst(%q) = %v, want %v", primary, got, want)
		}
	}
	if !slices.Equal(outs, []*app.Output{dp, hdmi, edp}) {
		t.Errorf("PrimaryFirst reordered its input to %v", outs)
	}
	if got := Output(PrimaryFirst(outs, "HDMI-A-1"), config.OsdMonitor{}); got != hdmi {
		t.Errorf("\"primary\" with HDMI-A-1 primary = %v, want HDMI-A-1", got)
	}
}
