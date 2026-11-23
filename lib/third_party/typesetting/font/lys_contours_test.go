package font

import (
	"bytes"
	"slices"
	"testing"

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
			contours, ok := face.GlyphContours(gid)
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

func TestGlyphContoursRejectAGlyphPastTheTable(t *testing.T) {
	face, err := ParseTTF(bytes.NewReader(lysQuicksand(t)))
	if err != nil {
		t.Fatalf("parse Quicksand: %v", err)
	}
	if _, ok := face.GlyphContours(GID(face.nGlyphs)); ok {
		t.Fatal("a glyph past the glyf table has contours")
	}
}
