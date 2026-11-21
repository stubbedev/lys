package wallpaper

import (
	"image/color"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/lib/render"
)

func TestOLEDDimNeedsTheOptionTheBatteryAndAnOLEDOutput(t *testing.T) {
	for name, c := range map[string]struct {
		dim  oledDim
		want bool
	}{
		"all three":        {oledDim{option: true, onBattery: true, oled: []string{"eDP-1"}}, true},
		"the option off":   {oledDim{onBattery: true, oled: []string{"eDP-1"}}, false},
		"on AC":            {oledDim{option: true, oled: []string{"eDP-1"}}, false},
		"not OLED":         {oledDim{option: true, onBattery: true, oled: []string{"DP-1"}}, false},
		"no OLED anywhere": {oledDim{option: true, onBattery: true}, false},
	} {
		if got := c.dim.dims("eDP-1"); got != c.want {
			t.Errorf("%s: dims eDP-1 %v, want %v", name, got, c.want)
		}
	}
}

func TestADimmedViewDarkensTheWallpaper(t *testing.T) {
	v := newView()
	v.show(imageOf(color.NRGBA{R: 255, G: 255, B: 255, A: 255}, 4, 4), TransitionNone)
	data, w := paintView(v, 4, 4)
	if got := px(data, w, 1, 1); got != render.RGB(0xff, 0xff, 0xff) {
		t.Fatalf("undimmed pixel = %v, want white", got)
	}
	v.setDim(true)
	data, w = paintView(v, 4, 4)
	dimmed := px(data, w, 1, 1)
	if dimmed.R() >= 0x80 || dimmed.R() == 0 || dimmed.G() != dimmed.R() || dimmed.B() != dimmed.R() || dimmed.A() != 0xff {
		t.Errorf("dimmed pixel = %v, want an opaque grey darker than half", dimmed)
	}
	v.setDim(false)
	data, w = paintView(v, 4, 4)
	if got := px(data, w, 1, 1); got != render.RGB(0xff, 0xff, 0xff) {
		t.Errorf("pixel after the dim lifts = %v, want white", got)
	}
}

func dimmedOutputs(h *harness) map[string]bool {
	out := map[string]bool{}
	for name, surf := range h.s.surfaces {
		out[name] = surf.view.dim
	}
	return out
}

func wantDimmed(t *testing.T, h *harness, why string, want map[string]bool) {
	t.Helper()
	got := dimmedOutputs(h)
	for name, dim := range want {
		if got[name] != dim {
			t.Errorf("%s: %s dimmed %v, want %v (all: %v)", why, name, got[name], dim, got)
		}
	}
}

func TestTheShellDimsOLEDOutputsOnBatteryWhileTheOptionIsOn(t *testing.T) {
	h := newHarness(t, config.AnimationNone, "eDP-1", "DP-1")
	h.plug("eDP-1", 64, 36)
	h.plug("DP-1", 64, 36)
	h.settle()
	h.s.SetOLEDOutputs([]string{"eDP-1"})
	h.s.SetOnBattery(true)
	wantDimmed(t, h, "the option off", map[string]bool{"eDP-1": false, "DP-1": false})

	cfg := config.Defaults()
	cfg.Wallpaper.Wallpaper = h.path("a.png")
	cfg.Wallpaper.OLEDDim = true
	h.s.SetConfig(cfg)
	wantDimmed(t, h, "the option on, on battery", map[string]bool{"eDP-1": true, "DP-1": false})

	h.s.SetOnBattery(false)
	wantDimmed(t, h, "on AC", map[string]bool{"eDP-1": false, "DP-1": false})

	h.s.SetOnBattery(true)
	h.s.SetOLEDOutputs(nil)
	wantDimmed(t, h, "lys-wm reports no OLED output", map[string]bool{"eDP-1": false, "DP-1": false})

	h.s.SetOLEDOutputs([]string{"eDP-1", "eDP-2"})
	h.s.svc.RegisterMonitor("eDP-2")
	h.plug("eDP-2", 64, 36)
	h.settle()
	wantDimmed(t, h, "an OLED output plugged in on battery", map[string]bool{"eDP-1": true, "eDP-2": true, "DP-1": false})

	cfg.Wallpaper.OLEDDim = false
	h.s.SetConfig(cfg)
	wantDimmed(t, h, "the option turned off", map[string]bool{"eDP-1": false, "eDP-2": false, "DP-1": false})
}
