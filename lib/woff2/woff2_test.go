package woff2

import (
	"encoding/binary"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"

	lysinter "github.com/stubbedev/lys/lib/inter"
	"github.com/stubbedev/lys/lib/render"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// tablesOf is the tables of the TrueType font sfnt, by tag.
func tablesOf(t *testing.T, sfnt []byte) map[string][]byte {
	t.Helper()
	n := int(binary.BigEndian.Uint16(sfnt[4:]))
	out := make(map[string][]byte, n)
	for i := range n {
		rec := sfnt[sfntHeaderSize+sfntRecordSize*i:]
		off, length := binary.BigEndian.Uint32(rec[8:]), binary.BigEndian.Uint32(rec[12:])
		out[string(rec[:4])] = sfnt[off : off+length]
	}
	return out
}

// point is one point of a simple glyph, at its absolute position.
type point struct {
	x, y    int32
	onCurve bool
}

// glyph is what a glyph draws: its contours' ends and points, or a
// composite's components, its bounding box and instructions.
type glyph struct {
	contours     int16
	box          [4]int16
	endPts       []uint16
	points       []point
	components   []byte
	instructions []byte
}

// parseGlyph reads a TrueType glyph as the glyf table stores it.
func parseGlyph(t *testing.T, b []byte) glyph {
	t.Helper()
	if len(b) == 0 {
		return glyph{}
	}
	r := &reader{b: b}
	g := glyph{contours: int16(r.u16())}
	for k := range g.box {
		g.box[k] = int16(r.u16())
	}
	if g.contours < 0 {
		start := r.b
		instructions := false
		for more := true; more; {
			flags := r.u16()
			r.u16()
			n := 2
			if flags&1 != 0 {
				n = 4
			}
			switch {
			case flags&0x08 != 0:
				n += 2
			case flags&0x40 != 0:
				n += 4
			case flags&0x80 != 0:
				n += 8
			}
			r.bytes(n)
			more, instructions = flags&0x20 != 0, instructions || flags&0x100 != 0
		}
		g.components = start[:len(start)-len(r.b)]
		if instructions {
			g.instructions = r.bytes(int(r.u16()))
		}
		if r.err != nil {
			t.Fatal(r.err)
		}
		return g
	}
	g.endPts = make([]uint16, g.contours)
	for k := range g.endPts {
		g.endPts[k] = r.u16()
	}
	g.instructions = r.bytes(int(r.u16()))
	n := 0
	if g.contours > 0 {
		n = int(g.endPts[len(g.endPts)-1]) + 1
	}
	flags := make([]byte, 0, n)
	for len(flags) < n {
		f := r.u8()
		flags = append(flags, f)
		if f&0x08 != 0 {
			for range r.u8() {
				flags = append(flags, f)
			}
		}
	}
	g.points = make([]point, n)
	coord := func(short, same byte, set func(*point, int32)) {
		var v int32
		for k := range g.points {
			switch fl := flags[k]; {
			case fl&short != 0 && fl&same != 0:
				v += int32(r.u8())
			case fl&short != 0:
				v -= int32(r.u8())
			case fl&same == 0:
				v += int32(int16(r.u16()))
			}
			set(&g.points[k], v)
		}
	}
	coord(0x02, 0x10, func(p *point, v int32) { p.x = v })
	coord(0x04, 0x20, func(p *point, v int32) { p.y = v })
	for k, f := range flags {
		g.points[k].onCurve = f&1 != 0
	}
	if r.err != nil {
		t.Fatal(r.err)
	}
	return g
}

// glyphsOf is every glyph the font's glyf and loca tables hold.
func glyphsOf(t *testing.T, tables map[string][]byte) []glyph {
	t.Helper()
	short := binary.BigEndian.Uint16(tables["head"][50:]) == 0
	numGlyphs := int(binary.BigEndian.Uint16(tables["maxp"][4:]))
	at := func(i int) uint32 {
		if short {
			return 2 * uint32(binary.BigEndian.Uint16(tables["loca"][2*i:]))
		}
		return binary.BigEndian.Uint32(tables["loca"][4*i:])
	}
	out := make([]glyph, numGlyphs)
	for i := range out {
		out[i] = parseGlyph(t, tables["glyf"][at(i):at(i+1)])
	}
	return out
}

// equalGlyphs reports whether two glyphs draw the same.
func equalGlyphs(a, b glyph) bool {
	return a.contours == b.contours && a.box == b.box && slices.Equal(a.endPts, b.endPts) && slices.Equal(a.points, b.points) &&
		slices.Equal(a.components, b.components) && slices.Equal(a.instructions, b.instructions)
}

// A WOFF2 font decodes to the font it packs: every table it stores as
// is the same, head but for its checksum adjustment and flags, every
// glyph draws the same and has the same instructions, and the hmtx
// table is the same whether stored transformed or not; every checksum
// holds, and the font loads. So for a full font, a subset whose glyf
// and loca tables are not transformed, one whose hmtx table is too, and
// a monospaced one whose left side bearings past its one metric come
// from its glyphs.
func TestAFontDecodesToTheFontItPacks(t *testing.T) {
	for _, c := range []struct {
		woff2 string
		sfnt  []byte
	}{
		{"inter.woff2", lysinter.Regular},
		{"inter-null.woff2", fixture(t, "inter-subset.ttf")},
		{"inter-hmtx.woff2", fixture(t, "inter-subset.ttf")},
		{"mono.woff2", fixture(t, "mono.ttf")},
	} {
		got, err := Decode(fixture(t, c.woff2))
		if err != nil {
			t.Errorf("%s: %v", c.woff2, err)
			continue
		}
		gotTables, wantTables := tablesOf(t, got), tablesOf(t, c.sfnt)
		if !slices.Equal(slices.Sorted(maps.Keys(gotTables)), slices.Sorted(maps.Keys(wantTables))) {
			t.Errorf("%s holds the tables %v, want %v", c.woff2, slices.Sorted(maps.Keys(gotTables)), slices.Sorted(maps.Keys(wantTables)))
			continue
		}
		for tag, want := range wantTables {
			g := gotTables[tag]
			switch tag {
			case "glyf", "loca":
				continue
			case "head":
				g, want = slices.Concat(g[:8], g[12:16], g[18:]), slices.Concat(want[:8], want[12:16], want[18:])
			}
			if !slices.Equal(g, want) {
				t.Errorf("%s's %s table differs", c.woff2, tag)
			}
		}
		gotGlyphs, wantGlyphs := glyphsOf(t, gotTables), glyphsOf(t, wantTables)
		composites := 0
		for i := range wantGlyphs {
			if !equalGlyphs(gotGlyphs[i], wantGlyphs[i]) {
				t.Errorf("%s's glyph %d is %+v, want %+v", c.woff2, i, gotGlyphs[i], wantGlyphs[i])
			}
			if wantGlyphs[i].contours < 0 {
				composites++
			}
		}
		if composites == 0 {
			t.Errorf("%s has no composite glyph to rebuild", c.woff2)
		}
		n := int(binary.BigEndian.Uint16(got[4:]))
		for i := range n {
			rec := got[sfntHeaderSize+sfntRecordSize*i:]
			tag := string(rec[:4])
			table := gotTables[tag]
			if tag == "head" {
				table = slices.Concat(table[:8], make([]byte, 4), table[12:])
			}
			if checksum(table) != binary.BigEndian.Uint32(rec[4:]) {
				t.Errorf("%s's %s checksum does not hold", c.woff2, tag)
			}
		}
		if checksum(got) != checksumMagic {
			t.Errorf("%s's checksum adjustment does not hold", c.woff2)
		}
		if _, err := render.LoadFont(got); err != nil {
			t.Errorf("%s does not load: %v", c.woff2, err)
		}
	}
}

// Data that is no WOFF2 font, one cut short, one whose compressed
// tables stop short and one claiming more than it holds are
// malformed; a font collection and a font past MaxBytes are
// unsupported.
func TestABrokenFontSaysWhy(t *testing.T) {
	good := fixture(t, "inter-hmtx.woff2")
	damaged := slices.Clone(good)
	binary.BigEndian.PutUint32(damaged[20:], binary.BigEndian.Uint32(good[20:])/2)
	collection := slices.Clone(good)
	copy(collection[4:], collectionFlav)
	overlong := slices.Clone(good)
	binary.BigEndian.PutUint32(overlong[20:], uint32(len(good)))
	huge := slices.Concat(good[:headerSize], []byte{0x3f, 'b', 'i', 'g', ' ', 0x8f, 0xff, 0xff, 0xff, 0x7f})
	binary.BigEndian.PutUint16(huge[12:], 1)
	for name, c := range map[string]struct {
		data []byte
		want error
	}{
		"not a font":   {[]byte("wOFF and then some bytes past the forty-eight of a header"), ErrMalformed},
		"cut short":    {good[:len(good)/2], ErrMalformed},
		"damaged":      {damaged, ErrMalformed},
		"overlong":     {overlong, ErrMalformed},
		"a collection": {collection, ErrUnsupported},
		"huge":         {huge, ErrUnsupported},
	} {
		got, err := Decode(c.data)
		if !errors.Is(err, c.want) || got != nil {
			t.Errorf("%s decodes to %d bytes, %v; want %v", name, len(got), err, c.want)
		}
	}
}

// A UIntBase128 reads seven bits a byte, and refuses a leading zero,
// six bytes and a value past 32 bits; a 255UInt16 reads each of its
// four forms.
func TestTheVariableLengthNumbersRead(t *testing.T) {
	for _, c := range []struct {
		in   []byte
		want uint32
		ok   bool
	}{
		{[]byte{0x3f}, 63, true},
		{[]byte{0x81, 0x00}, 128, true},
		{[]byte{0x8f, 0xff, 0xff, 0xff, 0x7f}, 1<<32 - 1, true},
		{[]byte{0x80, 0x01}, 0, false},
		{[]byte{0x90, 0x80, 0x80, 0x80, 0x00}, 0, false},
		{[]byte{0x81, 0x81, 0x81, 0x81, 0x81, 0x01}, 0, false},
	} {
		r := &reader{b: c.in}
		if got := r.base128(); got != c.want || (r.err == nil) != c.ok {
			t.Errorf("UIntBase128 %x reads %d, %v", c.in, got, r.err)
		}
	}
	for _, c := range []struct {
		in   []byte
		want uint16
	}{{[]byte{252}, 252}, {[]byte{253, 0x12, 0x34}, 0x1234}, {[]byte{255, 7}, 260}, {[]byte{254, 7}, 513}} {
		r := &reader{b: c.in}
		if got := r.u255(); got != c.want || r.err != nil || len(r.b) != 0 {
			t.Errorf("255UInt16 %x reads %d, %v", c.in, got, r.err)
		}
	}
}
