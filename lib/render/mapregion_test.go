package render

import (
	"slices"
	"testing"
)

func regionArea(region []Rect) int {
	a := 0
	for _, r := range region {
		a += r.W * r.H
	}
	return a
}

func regionCovers(region []Rect, x, y int) bool {
	return slices.ContainsFunc(region, func(r Rect) bool { return r.Contains(x, y) })
}

// A map that keeps the axes aligned maps a region exactly: a zoom about
// its center halves each rect, and rects that shared an edge still do,
// so the image has no seam; a quarter turn is exact too.
func TestMapRegionKeepsAnAlignedImageExact(t *testing.T) {
	region := []Rect{{X: 0, Y: 0, W: 100, H: 30}, {X: 0, Y: 30, W: 100, H: 33}}
	got := Scale(0.5, 0.5).About(50, 31.5).MapRegion(region)
	want := []Rect{{X: 25, Y: 16, W: 50, H: 15}, {X: 25, Y: 31, W: 50, H: 16}}
	if !slices.Equal(got, want) {
		t.Errorf("a half zoom maps to %v, want %v", got, want)
	}
	for y := 16; y < 47; y++ {
		if !regionCovers(got, 50, y) {
			t.Errorf("the zoomed region has a seam at row %d", y)
		}
	}
	turned := Rotate(90).MapRegion([]Rect{{X: 0, Y: 0, W: 40, H: 10}})
	if !slices.Equal(turned, []Rect{{X: -10, Y: 0, W: 10, H: 40}}) {
		t.Errorf("a quarter turn maps to %v, want the 10x40 rect left of the origin", turned)
	}
	if got := Scale(0, 1).MapRegion(region); len(got) != 0 {
		t.Errorf("a map collapsing the width keeps %v, want nothing", got)
	}
	if got := Identity.MapRegion(nil); got != nil {
		t.Errorf("an empty region maps to %v, want none", got)
	}
}

// A rotated region is sampled in bands no more than maxRegionBands, its
// area close to the true image's, covering the image's middle and none
// of the corners a rotation leaves bare; rects that shared an edge
// leave no seam across it.
func TestMapRegionBandsARotatedImage(t *testing.T) {
	region := []Rect{{X: 0, Y: 0, W: 200, H: 50}, {X: 0, Y: 50, W: 200, H: 50}}
	m := Rotate(30).About(100, 50)
	got := m.MapRegion(region)
	if len(got) == 0 || len(got) > maxRegionBands {
		t.Fatalf("a rotated region maps to %d rects, want 1..%d", len(got), maxRegionBands)
	}
	if a, want := regionArea(got), 200*100; a < want*95/100 || a > want*105/100 {
		t.Errorf("the banded image covers %d px, want within 5%% of the true %d", a, want)
	}
	bounds := m.MapBounds(Rect{W: 200, H: 100})
	for _, r := range got {
		if r.Intersect(bounds) != r {
			t.Errorf("band %+v leaves the image's bounds %+v", r, bounds)
		}
	}
	if !regionCovers(got, 100, 50) {
		t.Error("the banded image misses its own center")
	}
	if regionCovers(got, bounds.X+1, bounds.Y+1) {
		t.Errorf("the banded image covers the bare corner at %d,%d", bounds.X+1, bounds.Y+1)
	}
	rows := map[int]int{}
	for _, r := range got {
		rows[r.Y]++
	}
	for y, n := range rows {
		if n != 1 {
			t.Errorf("row %d is split into %d spans, want the two rects' images merged into one", y, n)
		}
	}
}

// However tall the image, a rotated region stays within the band cap.
func TestMapRegionCapsItsBands(t *testing.T) {
	got := Rotate(45).MapRegion([]Rect{{W: 4000, H: 4000}})
	if len(got) > maxRegionBands {
		t.Errorf("a 4000px square rotated maps to %d rects, want at most %d", len(got), maxRegionBands)
	}
}
