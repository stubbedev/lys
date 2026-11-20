package webcamframe

import (
	"image"
	"testing"
)

func TestTheMarginIsFivePercentOfTheShorterSide(t *testing.T) {
	if m := Margin(1920, 1080); m != 54 {
		t.Errorf("margin = %d, want 5%% of 1080", m)
	}
}

func TestTheFrameIsPlacedInTheTravelInsideTheMargin(t *testing.T) {
	for name, c := range map[string]struct {
		p    Placement
		w, h int32
		want image.Rectangle
	}{
		"bottom right": {Placement{X: 100, Y: 100, Size: 20}, 1920, 1080, image.Rect(1482, 810, 1482+384, 810+216)},
		"top left":     {Placement{X: 0, Y: 0, Size: 20}, 1920, 1080, image.Rect(54, 54, 54+384, 54+216)},
		"clamped":      {Placement{X: 250, Y: 50, Size: 20}, 1920, 1080, image.Rect(1482, 432, 1482+384, 432+216)},
		"tiny":         {Placement{Size: 0}, 1920, 1080, image.Rect(54, 54, 54+80, 54+60)},
		"no travel":    {Placement{X: 100, Size: 100}, 100, 100, image.Rect(5, 5, 105, 65)},
	} {
		if got := Rect(c.p, c.w, c.h); got != c.want {
			t.Errorf("%s: %v, want %v", name, got, c.want)
		}
	}
}

func TestAFramesRectReadsBackAsItsPlacement(t *testing.T) {
	for _, p := range []Placement{{0, 0, 20}, {100, 100, 20}, {37, 81, 33}, {50, 50, 100}, {100, 0, 5}} {
		r := Rect(p, 1920, 1080)
		got := At(int32(r.Min.X), int32(r.Min.Y), int32(r.Dx()), 1920, 1080)
		if p.Size == 100 {
			p.X, p.Y = 0, 0
		}
		if got != p {
			t.Errorf("%+v drawn at %v reads back as %+v", p, r, got)
		}
	}
}

func TestAFrameDraggedPastTheMarginStopsAtTheEdgeOfItsTravel(t *testing.T) {
	if got := At(-500, 5000, 384, 1920, 1080); got != (Placement{X: 0, Y: 100, Size: 20}) {
		t.Errorf("a frame beyond the screen reads %+v, want it flush bottom left", got)
	}
	if got := At(0, 0, 10_000, 1920, 1080); got.Size != 100 {
		t.Errorf("a frame wider than the screen reads size %d, want 100", got.Size)
	}
	if got := At(0, 0, 0, 1920, 1080); got.Size != 1 {
		t.Errorf("a frame with no width reads size %d, want the smallest", got.Size)
	}
}
