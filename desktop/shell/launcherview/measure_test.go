package launcherview

import (
	"testing"

	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/desktop/config"
)

// Centre is the search panel's place, whatever the panel's size; the rest
// sit against their edges, offset from there.
func TestPlace(t *testing.T) {
	out := render.Rect{W: 1000, H: 1000}
	for _, c := range []struct {
		loc  config.LauncherLocation
		x, y int
		want render.Rect
	}{
		{config.LauncherCenter, 0, 0, render.Rect{X: 400, Y: 220, W: 200, H: 100}},
		{config.LauncherCenter, 10, 20, render.Rect{X: 400, Y: 220, W: 200, H: 100}},
		{config.LauncherNorth, 10, 20, render.Rect{X: 400, Y: 20, W: 200, H: 100}},
		{config.LauncherSouthEast, -10, -20, render.Rect{X: 790, Y: 880, W: 200, H: 100}},
		{config.LauncherWest, 10, 20, render.Rect{X: 10, Y: 450, W: 200, H: 100}},
	} {
		if got := Place(out, 200, 100, c.loc, c.x, c.y); got != c.want {
			t.Errorf("%s offset %d,%d: %v, want %v", c.loc, c.x, c.y, got, c.want)
		}
	}
	if a, b := Place(out, 200, 100, config.LauncherCenter, 0, 0), Place(out, 200, 500, config.LauncherCenter, 0, 0); a.Y != b.Y {
		t.Errorf("the field moved as the panel grew: %d, %d", a.Y, b.Y)
	}
}

func TestWidthInHoldsAPanelInsideAKnownOutput(t *testing.T) {
	if got := WidthIn(render.Rect{}, 720); got != 720 {
		t.Errorf("an output of no known size held 720 at %d", got)
	}
	if got := WidthIn(render.Rect{W: 600}, 720); got != 600-2*edgeGap {
		t.Errorf("a 600px output held 720 at %d, want %d", got, 600-2*edgeGap)
	}
	if got := WidthIn(render.Rect{W: 10}, 720); got != 1 {
		t.Errorf("an output narrower than its gaps held the panel at %d, want 1", got)
	}
}

func TestWidthPxFollowsTheConfigAndTheStylingScale(t *testing.T) {
	cfg := config.Defaults()
	cfg.Launcher.Width = config.Px(800)
	cfg.Styling.Scale = 2
	if got := WidthPx(cfg); got != 800 {
		t.Errorf("800px at styling scale 2 is %d pixels", got)
	}
	cfg.Launcher.Width = config.Scale(1.5)
	if got := WidthPx(cfg); got != 1.5*config.LauncherWidthBaseRem*16*2 {
		t.Errorf("a width of 1.5 at styling scale 2 is %d pixels", got)
	}
}
