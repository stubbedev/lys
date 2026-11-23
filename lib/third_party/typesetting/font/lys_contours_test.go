package font

import (
	"bytes"
	"slices"
	"testing"

	"github.com/stubbedev/lys/lib/testbuild"
	ot "github.com/stubbedev/lys/lib/third_party/typesetting/font/opentype"
)

func TestGlyphContoursAreThePointsTheOutlineIsBuiltFrom(t *testing.T) {
	face, err := ParseTTF(bytes.NewReader(lysQuicksand(t)))
	if err != nil {
		t.Fatalf("parse Quicksand: %v", err)
	}
	for _, wght := range []float32{300, 580, 700} {
		face.SetVariations([]Variation{{Tag: ot.MustNewTag("wght"), Value: wght}})
		contoured := 0
		for gid := range GID(face.nGlyphs) {
			contours, ok := face.AppendGlyphContours(nil, gid)
			if !ok {
				t.Fatalf("wght %v glyph %d: no contours from a glyf face", wght, gid)
			}
			points := make([]contourPoint, len(contours))
			for i, c := range contours {
				points[i] = contourPoint{SegmentPoint: SegmentPoint{X: c.X, Y: c.Y}, isOnCurve: c.OnCurve, isEndPoint: c.End}
			}
			outline, _ := face.GlyphDataOutline(gid)
			if !slices.Equal(buildSegments(points), outline.Segments) {
				t.Fatalf("wght %v glyph %d: the contours build another outline than GlyphDataOutline", wght, gid)
			}
			if len(contours) > 0 {
				contoured++
				if !contours[len(contours)-1].End {
					t.Fatalf("wght %v glyph %d: the last point does not end a contour", wght, gid)
				}
			}
		}
		if contoured == 0 {
			t.Fatalf("wght %v: no glyph has contours", wght)
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
