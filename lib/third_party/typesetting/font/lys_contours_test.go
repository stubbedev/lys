package font

import (
	"bytes"
	"math"
	"slices"
	"testing"

	"github.com/stubbedev/lys/lib/testbuild"
	ot "github.com/stubbedev/lys/lib/third_party/typesetting/font/opentype"
	"github.com/stubbedev/lys/lib/third_party/typesetting/font/opentype/tables"
)

func TestGlyphContoursAreThePointsTheOutlineIsBuiltFrom(t *testing.T) {
	face, err := ParseTTF(bytes.NewReader(lysQuicksand(t)))
	if err != nil {
		t.Fatalf("parse Quicksand: %v", err)
	}
	contoured := 0
	for gid := range GID(face.nGlyphs) {
		contours, ok := face.AppendGlyphContours(nil, gid)
		if !ok {
			t.Fatalf("glyph %d: no contours from a glyf face", gid)
		}
		points := make([]contourPoint, len(contours))
		for i, c := range contours {
			points[i] = contourPoint{SegmentPoint: SegmentPoint{X: c.X, Y: c.Y}, isOnCurve: c.OnCurve, isEndPoint: c.End}
		}
		outline, _ := face.GlyphDataOutline(gid)
		if !slices.Equal(buildSegments(points), outline.Segments) {
			t.Fatalf("glyph %d: the contours build another outline than GlyphDataOutline", gid)
		}
		if len(contours) > 0 {
			contoured++
			if !contours[len(contours)-1].End {
				t.Fatalf("glyph %d: the last point does not end a contour", gid)
			}
		}
	}
	if contoured == 0 {
		t.Fatal("no glyph has contours")
	}
}

func TestVariedGlyphContoursAreRoundedToFontUnitsAsFreeTypeLoadsThem(t *testing.T) {
	face, err := ParseTTF(bytes.NewReader(lysQuicksand(t)))
	if err != nil {
		t.Fatalf("parse Quicksand: %v", err)
	}
	for _, wght := range []float32{350, 580, 650} {
		face.SetVariations([]Variation{{Tag: ot.MustNewTag("wght"), Value: wght}})
		fractional := 0
		for gid := range GID(face.nGlyphs) {
			contours, _ := face.AppendGlyphContours(nil, gid)
			varied := face.getPointsForGlyph(gID(gid))
			varied = varied[:len(varied)-phantomCount]
			if len(contours) != len(varied) {
				t.Fatalf("wght %v glyph %d: %d contour points for %d outline points", wght, gid, len(contours), len(varied))
			}
			for i, c := range contours {
				v := varied[i]
				if c.X != float32(math.Round(float64(c.X))) || c.Y != float32(math.Round(float64(c.Y))) {
					t.Fatalf("wght %v glyph %d point %d at (%v, %v): not a whole font unit", wght, gid, i, c.X, c.Y)
				}
				if math.Abs(float64(c.X-v.X)) > 1 || math.Abs(float64(c.Y-v.Y)) > 1 || c.OnCurve != v.isOnCurve || c.End != v.isEndPoint {
					t.Fatalf("wght %v glyph %d point %d: %+v, more than a unit from the varied point %+v", wght, gid, i, c, v)
				}
				if v.X != float32(math.Round(float64(v.X))) || v.Y != float32(math.Round(float64(v.Y))) {
					fractional++
				}
			}
		}
		if fractional == 0 {
			t.Fatalf("premise: no point of Quicksand is fractional at wght %v", wght)
		}
	}
}

func TestGlyphContoursAppendToTheBufferGiven(t *testing.T) {
	face, err := ParseTTF(bytes.NewReader(lysQuicksand(t)))
	if err != nil {
		t.Fatalf("parse Quicksand: %v", err)
	}
	gid, _ := face.NominalGlyph('o')
	want, _ := face.AppendGlyphContours(nil, gid)
	prefix := []ContourPoint{{X: 1, Y: 2, OnCurve: true, End: true}}
	got, ok := face.AppendGlyphContours(prefix, gid)
	if !ok || !slices.Equal(got[:1], prefix) || !slices.Equal(got[1:], want) {
		t.Fatalf("appending o's contours to a one-point buffer gave %d points, want the point and o's %d", len(got), len(want))
	}
	if testbuild.Race {
		return
	}
	if allocs := testing.AllocsPerRun(10, func() { got, _ = face.AppendGlyphContours(got[:0], gid) }); allocs != 0 {
		t.Errorf("a simple glyph's contours into a buffer that fits them allocate %v times, want none", allocs)
	}
}

func TestGlyphContoursRejectAGlyphPastTheTable(t *testing.T) {
	face, err := ParseTTF(bytes.NewReader(lysQuicksand(t)))
	if err != nil {
		t.Fatalf("parse Quicksand: %v", err)
	}
	if _, ok := face.AppendGlyphContours(nil, GID(face.nGlyphs)); ok {
		t.Fatal("a glyph past the glyf table has contours")
	}
}

func TestComposeExactRoundsEachProductAsFTMulFix(t *testing.T) {
	part := tables.CompositeGlyphPart{Scale: [4]float32{0.5, 0, 0.5, 1}}
	comp := []contourPoint{{SegmentPoint: SegmentPoint{X: 1, Y: 1}}}
	composeExact(&part, comp, contourPoint{}, nil)
	if comp[0].X != 2 || comp[0].Y != 1 {
		t.Errorf("(1, 1) through x' = x/2 + y/2 lands at (%v, %v), want (2, 1): FT_MulFix rounds each half up", comp[0].X, comp[0].Y)
	}
}

func TestVectorLengthIsTheHypotenuse(t *testing.T) {
	for _, c := range []struct{ x, y int64 }{{0, 5}, {-7, 0}, {3 << 16, 4 << 16}, {-0x10000, 0x10000}, {12345, -67890}, {1, 1}} {
		got := vectorLength(c.x, c.y)
		want := math.Hypot(float64(c.x), float64(c.y))
		if c.x == 0 || c.y == 0 {
			if float64(got) != want {
				t.Errorf("length of (%d, %d) = %d, want exactly %v", c.x, c.y, got, want)
			}
			continue
		}
		if math.Abs(float64(got)-want) > 2 {
			t.Errorf("length of (%d, %d) = %d, want %v within 2", c.x, c.y, got, want)
		}
	}
}
