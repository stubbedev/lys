package autofit

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/lib/third_party/typesetting/font/opentype/tables"
)

func TestTheCoverageOfASubstitutionStopsAtTheFirstGlyphOfItsLastRange(t *testing.T) {
	var got []uint16
	eachCoverageGlyph(tables.Coverage2{Ranges: []tables.RangeRecord{
		{StartGlyphID: 3, EndGlyphID: 5},
		{StartGlyphID: 9, EndGlyphID: 11},
	}}, func(g uint16) { got = append(got, g) })
	if want := []uint16{3, 4, 5, 9}; !slices.Equal(got, want) {
		t.Errorf("ranges 3-5 and 9-11 iterate %v, want %v as af_coverage_iterator does", got, want)
	}
	got = got[:0]
	eachCoverageGlyph(tables.Coverage1{Glyphs: []uint16{7, 2, 8}}, func(g uint16) { got = append(got, g) })
	if want := []uint16{7, 2, 8}; !slices.Equal(got, want) {
		t.Errorf("a glyph list iterates %v, want all of %v", got, want)
	}
}

func TestEdgesOfATopToBottomScriptAreSortedDescending(t *testing.T) {
	for _, tc := range []struct {
		topToBottom bool
		want        []int16
	}{{false, []int16{-5, 10, 40}}, {true, []int16{40, 10, -5}}} {
		h := &glyphHints{}
		for _, pos := range []int16{10, 40, -5} {
			k := h.insertEdge(pos, dirUp, tc.topToBottom)
			h.edges[k] = edge{fpos: pos}
		}
		var got []int16
		for _, e := range h.edges {
			got = append(got, e.fpos)
		}
		if !slices.Equal(got, tc.want) {
			t.Errorf("top to bottom %v: edges at %v, want %v", tc.topToBottom, got, tc.want)
		}
	}
}
