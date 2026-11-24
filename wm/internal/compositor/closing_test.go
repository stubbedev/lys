package compositor

import (
	"image"
	"testing"
	"time"

	"github.com/stubbedev/lys/wm/internal/anim"
	swlr "github.com/stubbedev/lys/wm/internal/protocol/server/wlrlayershell"
)

func TestSlideOffLeavesThroughTheNearestEdge(t *testing.T) {
	area := image.Rect(0, 0, 1000, 600)
	cases := []struct{ r, want image.Rectangle }{
		{image.Rect(10, 100, 210, 500), image.Rect(-200, 100, 0, 500)},
		{image.Rect(800, 100, 990, 500), image.Rect(1000, 100, 1190, 500)},
		{image.Rect(300, 20, 700, 120), image.Rect(300, -100, 700, 0)},
		{image.Rect(300, 480, 700, 590), image.Rect(300, 600, 700, 710)},
	}
	for _, c := range cases {
		if got := slideOff(area, c.r); got != c.want {
			t.Errorf("%v slides to %v, want %v", c.r, got, c.want)
		}
		if slideOff(area, c.r).Overlaps(area) {
			t.Errorf("%v slides to a place still on the output", c.r)
		}
	}
}

func TestLayerSurfacesSlideFromTheEdgeTheyAreAnchoredTo(t *testing.T) {
	area := image.Rect(0, 0, 1000, 600)
	top, bottom := swlr.ZwlrLayerSurfaceV1AnchorTop, swlr.ZwlrLayerSurfaceV1AnchorBottom
	left, right := swlr.ZwlrLayerSurfaceV1AnchorLeft, swlr.ZwlrLayerSurfaceV1AnchorRight
	cases := []struct {
		r      image.Rectangle
		anchor swlr.ZwlrLayerSurfaceV1Anchor
		want   image.Rectangle
	}{
		{image.Rect(0, 0, 1000, 30), top | left | right, image.Rect(0, -30, 1000, 0)},
		{image.Rect(300, 550, 700, 600), bottom, image.Rect(300, 600, 700, 650)},
		{image.Rect(940, 10, 1000, 200), right | top, image.Rect(940, -190, 1000, 0)},
		{image.Rect(0, 0, 40, 600), left | top | bottom, image.Rect(-40, 0, 0, 600)},
		{image.Rect(400, 200, 600, 400), 0, image.Rect(400, -200, 600, 0)},
	}
	for _, c := range cases {
		if got := anchoredSlide(area, c.r, c.anchor); got != c.want {
			t.Errorf("%v anchored %v slides to %v, want %v", c.r, c.anchor, got, c.want)
		}
	}
}

func layerExitFrames(t *testing.T, st anim.Style) ([]image.Rectangle, []float64) {
	t.Helper()
	curve := anim.MustResolve(nil).Curve(anim.LayersOut)
	area, goal := image.Rect(0, 0, 1000, 600), image.Rect(400, 200, 650, 300)
	end, alpha := layerStyledAs(st, area, goal, 0)
	motion, fade := anim.NewTween(goal, anim.LerpRect), anim.NewTween(1.0, anim.Lerp)
	motion.Set(end, 0, curve)
	fade.Set(alpha, 0, curve)
	var rects []image.Rectangle
	var alphas []float64
	for now := time.Duration(0); now <= curve.Duration(); now += time.Millisecond {
		rects, alphas = append(rects, motion.At(now)), append(alphas, fade.At(now))
	}
	return rects, alphas
}

func TestALayerLeavingWithAPopinAbove100EnlargesAroundItsCentreWhileItFades(t *testing.T) {
	rects, alphas := layerExitFrames(t, anim.Style{Kind: anim.Popin, Percent: 108})
	for i := 1; i < len(rects); i++ {
		if rects[i].Dx() < rects[i-1].Dx() || rects[i].Dy() < rects[i-1].Dy() || alphas[i] > alphas[i-1] {
			t.Fatalf("frame %d: %v at %.3f after %v at %.3f", i, rects[i], alphas[i], rects[i-1], alphas[i-1])
		}
	}
	if last := rects[len(rects)-1]; last != image.Rect(390, 196, 660, 304) {
		t.Errorf("ends at %v, not 108%% around the centre", last)
	}
	if last := alphas[len(alphas)-1]; last != 0 {
		t.Errorf("ends at opacity %v", last)
	}
}

func TestALayerLeavingWithAPopinAtOrBelow100ShrinksWithoutFading(t *testing.T) {
	for _, percent := range []float64{100, 92} {
		rects, alphas := layerExitFrames(t, anim.Style{Kind: anim.Popin, Percent: percent})
		for i := 1; i < len(rects); i++ {
			if rects[i].Dx() > rects[i-1].Dx() || alphas[i] != 1 {
				t.Fatalf("popin %g%% frame %d: %v at %.3f after %v", percent, i, rects[i], alphas[i], rects[i-1])
			}
		}
	}
}

func TestALayerLeavingWithAFadeOnlyChangesItsOpacity(t *testing.T) {
	rects, alphas := layerExitFrames(t, anim.Style{Kind: anim.FadeStyle})
	for i, r := range rects {
		if r != image.Rect(400, 200, 650, 300) {
			t.Fatalf("frame %d moves to %v", i, r)
		}
	}
	if alphas[len(alphas)-1] != 0 {
		t.Errorf("ends at opacity %v", alphas[len(alphas)-1])
	}
}
