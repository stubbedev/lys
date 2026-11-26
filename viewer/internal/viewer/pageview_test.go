package viewer

import (
	"image"
	"math"
	"testing"

	"github.com/stubbedev/lys/desktop/dochelper"
)

var letter = dochelper.Size{W: 612, H: 792}

func TestAFittedPageIsDrawnWholeWithinItsBox(t *testing.T) {
	for _, box := range []image.Point{{800, 600}, {300, 900}, {1, 1}, {9000, 9000}} {
		tile, ok := fittedView(letter).tile(0, box)
		if !ok {
			t.Fatalf("no tile in %v", box)
		}
		full := letter.At(tile.Scale)
		if tile.Rect.Dx() > box.X || tile.Rect.Dy() > box.Y || tile.Rect.Dx() > dochelper.MaxSize || tile.Rect.Dy() > dochelper.MaxSize {
			t.Errorf("in %v the page draws %v", box, tile.Rect)
		}
		least := letter.At(minScale)
		roomy := box.X >= least.X && box.Y >= least.Y && box.X <= dochelper.MaxSize && box.Y <= dochelper.MaxSize
		if roomy && tile.Rect != (image.Rectangle{Max: full}) {
			t.Errorf("in %v the fitted page draws %v of %v", box, tile.Rect, full)
		}
	}
	if _, ok := fittedView(letter).tile(0, image.Point{}); ok {
		t.Error("an empty box has a tile")
	}
}

func TestZoomingKeepsThePointUnderTheAnchorAndStaysOnThePage(t *testing.T) {
	box := image.Pt(800, 600)
	v := fittedView(letter)
	at := image.Pt(500, 200)
	before, _ := v.tile(0, box)
	off := v.offset(before, box)
	px := float64(before.Rect.Min.X+at.X-off.X) / before.Scale
	zoomed := v.zoomedAt(4*before.Scale, at, box)
	after, _ := zoomed.tile(0, box)
	if after.Rect.Dx() != box.X || after.Rect.Dy() != box.Y {
		t.Fatalf("zoomed 4x the tile is %v, want the whole %v box", after.Rect, box)
	}
	if got := float64(after.Rect.Min.X+at.X) / after.Scale; math.Abs(got-px) > 1 {
		t.Errorf("the point under the anchor moved from %g to %g", px, got)
	}
	if far := zoomed.zoomedAt(1e9, at, box); far.scale != dochelper.MaxScale {
		t.Errorf("zooming past the most is %g, want %d", far.scale, dochelper.MaxScale)
	}
	if near := zoomed.zoomedAt(0, at, box); near.scale != minScale {
		t.Errorf("zooming out past the least is %g, want %g", near.scale, minScale)
	}
}

func TestScrollingStopsAtThePageEdgesAndSaysSo(t *testing.T) {
	box := image.Pt(800, 600)
	v := pageView{size: letter, scale: 2}
	if _, e := v.scrolled(0, -10, box); e != pastStart {
		t.Errorf("scrolling up from the top is %v, want pastStart", e)
	}
	down, e := v.scrolled(0, 100, box)
	if e != noEdge || down.y != 100 {
		t.Errorf("scrolling down 100 is %+v, %v", down, e)
	}
	bottom := v.atEnd(box)
	if _, e := bottom.scrolled(0, 1, box); e != pastEnd {
		t.Errorf("scrolling down from the bottom is %v, want pastEnd", e)
	}
	if _, e := fittedView(letter).scrolled(1, 0, box); e != pastEnd {
		t.Errorf("scrolling right on a page that fits is %v, want pastEnd", e)
	}
	shown := v.showing(dochelper.Box{X0: 500, Y0: 700, X1: 520, Y1: 710}, box)
	tile, _ := shown.tile(0, box)
	if !image.Rect(1000, 1400, 1040, 1420).In(tile.Rect) {
		t.Errorf("showing a box near the bottom right draws %v", tile.Rect)
	}
}
