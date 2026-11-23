package cff

import (
	"slices"
	"testing"

	ot "github.com/stubbedev/lys/lib/third_party/typesetting/font/opentype"
)

const (
	csHstem    = 1
	csRlineto  = 5
	csEndchar  = 14
	csHintmask = 19
	csCntrmask = 20
	csRmoveto  = 21
)

func csNumber(v int) byte { return byte(v + 139) }

func TestZeroLengthLinesAreDroppedUnlessAHintmaskPrecedesThem(t *testing.T) {
	charstring := []byte{
		csNumber(0), csNumber(10), csHstem,
		csNumber(10), csNumber(10), csRmoveto,
		csNumber(0), csNumber(0), csRlineto,
		csNumber(20), csNumber(0), csRlineto,
		csHintmask, 0x80,
		csNumber(0), csNumber(0), csRlineto,
		csNumber(0), csNumber(0), csRlineto,
		csCntrmask, 0x80,
		csNumber(0), csNumber(0), csRlineto,
		csNumber(0), csNumber(20), csRlineto,
		csEndchar,
	}
	f := &CFF{Charstrings: [][]byte{charstring}, localSubrs: [][][]byte{nil}}
	segments, _, err := f.LoadGlyph(0)
	if err != nil {
		t.Fatal(err)
	}
	pt := func(x, y float32) [3]ot.SegmentPoint { return [3]ot.SegmentPoint{{X: x, Y: y}} }
	want := []ot.Segment{
		{Op: ot.SegmentOpMoveTo, Args: pt(10, 10)},
		{Op: ot.SegmentOpLineTo, Args: pt(30, 10)},
		{Op: ot.SegmentOpLineTo, Args: pt(30, 10)},
		{Op: ot.SegmentOpLineTo, Args: pt(30, 30)},
		{Op: ot.SegmentOpLineTo, Args: pt(10, 10)},
	}
	if !slices.Equal(segments, want) {
		t.Fatalf("segments %v, want %v: a zero-length line stays only right after a hintmask, not after a cntrmask", segments, want)
	}
}
