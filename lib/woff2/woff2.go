// Package woff2 turns a WOFF2 font back into the TrueType or OpenType
// font it packs: its tables decompressed with Brotli, a transformed
// glyf, loca and hmtx table rebuilt, and the table directory and
// checksums written anew. A font collection is not unpacked.
package woff2

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"math/bits"
	"slices"

	"github.com/andybalholm/brotli"
)

// MaxBytes is the largest font Decode rebuilds.
const MaxBytes = 256 << 20

// The errors Decode returns unwrap to one of these: data that is no
// WOFF2 font or does not decode, and a WOFF2 file lys does not unpack.
var (
	ErrMalformed   = errors.New("woff2: malformed")
	ErrUnsupported = errors.New("woff2: unsupported")
)

// Signature is what a WOFF2 file starts with.
const Signature = "wOF2"

const (
	headerSize      = 48
	collectionFlav  = "ttcf"
	arbitraryTag    = 63
	headAdjustment  = 8
	checksumMagic   = 0xb1b0afba
	overlapSimple   = 0x40
	hmtxNoLSB       = 1 << 0
	hmtxNoMonoLSB   = 1 << 1
	optionOverlap   = 1 << 0
	sfntHeaderSize  = 12
	sfntRecordSize  = 16
	nullTransform   = 0
	glyfNullVersion = 3
)

// knownTags are the tags a table's flags name by index.
var knownTags = [...]string{
	"cmap", "head", "hhea", "hmtx", "maxp", "name", "OS/2", "post", "cvt ", "fpgm", "glyf", "loca", "prep", "CFF ", "VORG", "EBDT",
	"EBLC", "gasp", "hdmx", "kern", "LTSH", "PCLT", "VDMX", "vhea", "vmtx", "BASE", "GDEF", "GPOS", "GSUB", "EBSC", "JSTF", "MATH",
	"CBDT", "CBLC", "COLR", "CPAL", "SVG ", "sbix", "acnt", "avar", "bdat", "bloc", "bsln", "cvar", "fdsc", "feat", "fmtx", "fvar",
	"gvar", "hsty", "just", "lcar", "mort", "morx", "opbd", "prop", "trak", "Zapf", "Silf", "Glat", "Gloc", "Feat", "Sill",
}

// entry is one table of the directory: its tag, its length rebuilt,
// the length it is stored at in the decompressed stream, and whether
// it is stored transformed.
type entry struct {
	tag         string
	origLength  uint32
	storeLength uint32
	transformed bool
}

// Decode is the font data packs.
func Decode(data []byte) ([]byte, error) {
	if len(data) < headerSize || string(data[:4]) != Signature {
		return nil, fmt.Errorf("%w: no WOFF2 signature", ErrMalformed)
	}
	if string(data[4:8]) == collectionFlav {
		return nil, fmt.Errorf("%w: a font collection", ErrUnsupported)
	}
	flavor := data[4:8]
	numTables := int(binary.BigEndian.Uint16(data[12:]))
	compressed := binary.BigEndian.Uint32(data[20:])
	r := &reader{b: data[headerSize:]}
	entries := make([]entry, numTables)
	var total uint64
	for i := range entries {
		entries[i] = r.entry()
		total += uint64(entries[i].storeLength)
	}
	if r.err != nil {
		return nil, r.err
	}
	if total > MaxBytes {
		return nil, fmt.Errorf("%w: tables of %d bytes, past the %d rebuilt", ErrUnsupported, total, MaxBytes)
	}
	if uint64(compressed) > uint64(len(r.b)) {
		return nil, fmt.Errorf("%w: %d compressed bytes in a file of %d", ErrMalformed, compressed, len(data))
	}
	stream := make([]byte, total)
	if _, err := io.ReadFull(brotli.NewReader(bytes.NewReader(r.b[:compressed])), stream); err != nil {
		return nil, fmt.Errorf("%w: decompressing the tables: %w", ErrMalformed, err)
	}
	tables := make(map[string][]byte, numTables)
	for _, e := range entries {
		if _, dup := tables[e.tag]; dup {
			return nil, fmt.Errorf("%w: the table %q twice", ErrMalformed, e.tag)
		}
		tables[e.tag], stream = stream[:e.storeLength], stream[e.storeLength:]
	}
	if err := untransform(entries, tables); err != nil {
		return nil, err
	}
	return sfnt(flavor, tables)
}

// untransform rebuilds the transformed tables in place, glyf and loca
// before hmtx, whose left side bearings may come from the glyphs.
func untransform(entries []entry, tables map[string][]byte) error {
	transformed := map[string]entry{}
	for _, e := range entries {
		if e.transformed {
			transformed[e.tag] = e
		} else if e.storeLength != e.origLength {
			return fmt.Errorf("%w: the table %q stored at %d bytes of %d", ErrMalformed, e.tag, e.storeLength, e.origLength)
		}
	}
	var xMins []int16
	if _, ok := transformed["glyf"]; ok {
		l, ok := transformed["loca"]
		if !ok || l.storeLength != 0 {
			return fmt.Errorf("%w: a transformed glyf table without its loca", ErrMalformed)
		}
		glyf, loca, mins, err := rebuildGlyf(tables["glyf"])
		if err != nil {
			return err
		}
		if uint32(len(loca)) != l.origLength { //nolint:gosec // under MaxBytes
			return fmt.Errorf("%w: loca rebuilt at %d bytes, not %d", ErrMalformed, len(loca), l.origLength)
		}
		tables["glyf"], tables["loca"], xMins = glyf, loca, mins
	} else if _, ok := transformed["loca"]; ok {
		return fmt.Errorf("%w: a transformed loca table without its glyf", ErrMalformed)
	}
	if h, ok := transformed["hmtx"]; ok {
		hmtx, err := rebuildHmtx(tables, xMins)
		if err != nil {
			return err
		}
		if uint32(len(hmtx)) != h.origLength { //nolint:gosec // under MaxBytes
			return fmt.Errorf("%w: hmtx rebuilt at %d bytes, not %d", ErrMalformed, len(hmtx), h.origLength)
		}
		tables["hmtx"] = hmtx
	}
	for tag := range transformed {
		if tag != "glyf" && tag != "loca" && tag != "hmtx" {
			return fmt.Errorf("%w: the table %q transformed", ErrUnsupported, tag)
		}
	}
	return nil
}

// reader reads the file's big-endian fields, holding the first error
// it meets; every read after one gives zero values.
type reader struct {
	b   []byte
	err error
}

func (r *reader) fail(format string, args ...any) {
	if r.err == nil {
		r.err = fmt.Errorf("%w: "+format, append([]any{ErrMalformed}, args...)...)
	}
}

func (r *reader) bytes(n int) []byte {
	if r.err != nil {
		return nil
	}
	if n < 0 || n > len(r.b) {
		r.fail("%d bytes past the end", n)
		return nil
	}
	out := r.b[:n]
	r.b = r.b[n:]
	return out
}

func (r *reader) u8() uint8 {
	if b := r.bytes(1); b != nil {
		return b[0]
	}
	return 0
}

func (r *reader) u16() uint16 {
	if b := r.bytes(2); b != nil {
		return binary.BigEndian.Uint16(b)
	}
	return 0
}

func (r *reader) u32() uint32 {
	if b := r.bytes(4); b != nil {
		return binary.BigEndian.Uint32(b)
	}
	return 0
}

// base128 reads a UIntBase128: seven bits a byte, high bit set on all
// but the last, at most five bytes, no leading zeros, fitting 32 bits.
func (r *reader) base128() uint32 {
	var v uint32
	for i := range 5 {
		c := r.u8()
		if i == 0 && c == 0x80 {
			r.fail("a UIntBase128 with a leading zero")
			return 0
		}
		if v&0xfe000000 != 0 {
			r.fail("a UIntBase128 past 32 bits")
			return 0
		}
		v = v<<7 | uint32(c&0x7f)
		if c&0x80 == 0 {
			return v
		}
	}
	r.fail("a UIntBase128 longer than five bytes")
	return 0
}

// u255 reads a 255UInt16.
func (r *reader) u255() uint16 {
	const (
		wordCode       = 253
		oneMoreByte2   = 254
		oneMoreByte1   = 255
		lowestUCode    = 253
		oneMoreOffset2 = lowestUCode * 2
	)
	switch c := r.u8(); c {
	case wordCode:
		return r.u16()
	case oneMoreByte1:
		return uint16(r.u8()) + lowestUCode
	case oneMoreByte2:
		return uint16(r.u8()) + oneMoreOffset2
	default:
		return uint16(c)
	}
}

// entry reads one table directory entry.
func (r *reader) entry() entry {
	flags := r.u8()
	var e entry
	if idx := flags & 0x3f; idx == arbitraryTag {
		e.tag = string(r.bytes(4))
	} else if int(idx) < len(knownTags) {
		e.tag = knownTags[idx]
	} else {
		r.fail("a table of tag index %d", idx)
	}
	version := flags >> 6
	e.origLength = r.base128()
	e.storeLength = e.origLength
	if e.tag == "glyf" || e.tag == "loca" {
		e.transformed = version != glyfNullVersion
	} else {
		e.transformed = version != nullTransform
	}
	if e.transformed {
		e.storeLength = r.base128()
	}
	return e
}

// glyfStreams are the seven streams a transformed glyf table holds.
type glyfStreams struct {
	nContour, nPoints, flags, glyphs, composite, bbox, instructions reader
	bboxBits, overlapBits                                           []byte
}

// rebuildGlyf rebuilds a transformed glyf table, its loca, and every
// glyph's xMin.
func rebuildGlyf(data []byte) (glyf, loca []byte, xMins []int16, err error) {
	r := &reader{b: data}
	r.u16()
	options := r.u16()
	numGlyphs := int(r.u16())
	indexFormat := r.u16()
	var sizes [7]int
	for i := range sizes {
		sizes[i] = int(r.u32())
	}
	var s glyfStreams
	for i, st := range []*reader{&s.nContour, &s.nPoints, &s.flags, &s.glyphs, &s.composite, &s.bbox, &s.instructions} {
		st.b = r.bytes(sizes[i])
	}
	bitmap := (numGlyphs + 31) >> 5 << 2
	s.bboxBits = s.bbox.bytes(bitmap)
	if options&optionOverlap != 0 {
		s.overlapBits = r.bytes((numGlyphs + 7) >> 3)
	}
	if r.err != nil || s.bbox.err != nil {
		return nil, nil, nil, errors.Join(r.err, s.bbox.err)
	}
	if indexFormat > 1 {
		return nil, nil, nil, fmt.Errorf("%w: loca index format %d", ErrMalformed, indexFormat)
	}
	glyf = make([]byte, 0, len(data)*2)
	offsets := make([]uint32, numGlyphs+1)
	xMins = make([]int16, numGlyphs)
	for i := range numGlyphs {
		var xMin int16
		if glyf, xMin, err = s.glyph(glyf, i); err != nil {
			return nil, nil, nil, fmt.Errorf("glyph %d: %w", i, err)
		}
		xMins[i] = xMin
		for len(glyf)%4 != 0 {
			glyf = append(glyf, 0)
		}
		if uint64(len(glyf)) > MaxBytes {
			return nil, nil, nil, fmt.Errorf("%w: glyphs past %d bytes", ErrUnsupported, MaxBytes)
		}
		offsets[i+1] = uint32(len(glyf)) //nolint:gosec // under MaxBytes
	}
	if indexFormat == 0 {
		if offsets[numGlyphs]/2 > math.MaxUint16 {
			return nil, nil, nil, fmt.Errorf("%w: %d glyph bytes in a short loca", ErrMalformed, offsets[numGlyphs])
		}
		loca = make([]byte, 0, 2*len(offsets))
		for _, o := range offsets {
			loca = binary.BigEndian.AppendUint16(loca, uint16(o/2)) //nolint:gosec // checked above
		}
	} else {
		loca = make([]byte, 0, 4*len(offsets))
		for _, o := range offsets {
			loca = binary.BigEndian.AppendUint32(loca, o)
		}
	}
	return glyf, loca, xMins, nil
}

func bit(bitmap []byte, i int) bool {
	return i>>3 < len(bitmap) && bitmap[i>>3]&(0x80>>(i&7)) != 0
}

// glyph appends glyph i to glyf, and gives its xMin.
func (s *glyfStreams) glyph(glyf []byte, i int) ([]byte, int16, error) {
	nContours := int16(s.nContour.u16()) //nolint:gosec // a signed field
	explicit := bit(s.bboxBits, i)
	var box [4]int16
	if explicit {
		for k := range box {
			box[k] = int16(s.bbox.u16()) //nolint:gosec // a signed field
		}
	}
	switch {
	case nContours == 0:
		if explicit {
			return nil, 0, fmt.Errorf("%w: an empty glyph with a bounding box", ErrMalformed)
		}
		return glyf, 0, s.err()
	case nContours < 0:
		if !explicit {
			return nil, 0, fmt.Errorf("%w: a composite glyph without its bounding box", ErrMalformed)
		}
		return s.compositeGlyph(glyf, box)
	}
	return s.simpleGlyph(glyf, int(nContours), explicit, box, bit(s.overlapBits, i))
}

// err is the first error any stream met.
func (s *glyfStreams) err() error {
	return errors.Join(s.nContour.err, s.nPoints.err, s.flags.err, s.glyphs.err, s.composite.err, s.bbox.err, s.instructions.err)
}

func appendHeader(glyf []byte, nContours int16, box [4]int16) []byte {
	glyf = binary.BigEndian.AppendUint16(glyf, uint16(nContours)) //nolint:gosec // a signed field
	for _, v := range box {
		glyf = binary.BigEndian.AppendUint16(glyf, uint16(v)) //nolint:gosec // a signed field
	}
	return glyf
}

// compositeGlyph appends a composite glyph: its components as stored, then
// its instructions when it has them.
func (s *glyfStreams) compositeGlyph(glyf []byte, box [4]int16) ([]byte, int16, error) {
	const (
		argsAreWords    = 0x0001
		haveScale       = 0x0008
		moreComponents  = 0x0020
		haveXYScale     = 0x0040
		haveTwoByTwo    = 0x0080
		haveInstruction = 0x0100
	)
	glyf = appendHeader(glyf, -1, box)
	instructions := false
	for more := true; more; {
		start := s.composite.b
		flags := s.composite.u16()
		n := 2 + 2
		if flags&argsAreWords != 0 {
			n += 4
		} else {
			n += 2
		}
		switch {
		case flags&haveScale != 0:
			n += 2
		case flags&haveXYScale != 0:
			n += 4
		case flags&haveTwoByTwo != 0:
			n += 8
		}
		s.composite.b = start
		glyf = append(glyf, s.composite.bytes(n)...)
		more = flags&moreComponents != 0
		instructions = instructions || flags&haveInstruction != 0
		if s.composite.err != nil {
			return nil, 0, s.composite.err
		}
	}
	if instructions {
		glyf = s.appendInstructions(glyf)
	}
	return glyf, box[0], s.err()
}

func (s *glyfStreams) appendInstructions(glyf []byte) []byte {
	n := s.glyphs.u255()
	glyf = binary.BigEndian.AppendUint16(glyf, n)
	return append(glyf, s.instructions.bytes(int(n))...)
}

// simpleGlyph appends a simple glyph of nContours contours, its points
// decoded from the triplet encoding, its bounding box explicit or
// computed from its points.
func (s *glyfStreams) simpleGlyph(glyf []byte, nContours int, explicit bool, box [4]int16, overlap bool) ([]byte, int16, error) {
	endPts := make([]uint16, nContours)
	total := 0
	for k := range endPts {
		total += int(s.nPoints.u255())
		if total > math.MaxUint16+1 {
			return nil, 0, fmt.Errorf("%w: a glyph of more than %d points", ErrMalformed, math.MaxUint16+1)
		}
		endPts[k] = uint16(total - 1) //nolint:gosec // checked above
	}
	if err := s.err(); err != nil {
		return nil, 0, err
	}
	flags := s.flags.bytes(total)
	if flags == nil && total > 0 {
		return nil, 0, s.err()
	}
	xs, ys := make([]int32, total), make([]int32, total)
	var x, y int32
	for k, f := range flags {
		dx, dy := s.triplet(f & 0x7f)
		x, y = x+dx, y+dy
		xs[k], ys[k] = x, y
	}
	if err := s.err(); err != nil {
		return nil, 0, err
	}
	if !explicit && total > 0 {
		box = [4]int16{clamp16(slices.Min(xs)), clamp16(slices.Min(ys)), clamp16(slices.Max(xs)), clamp16(slices.Max(ys))}
	}
	glyf = appendHeader(glyf, int16(nContours), box) //nolint:gosec // nContours came from an int16
	for _, e := range endPts {
		glyf = binary.BigEndian.AppendUint16(glyf, e)
	}
	glyf = s.appendInstructions(glyf)
	glyf = appendPoints(glyf, flags, xs, ys, overlap)
	return glyf, box[0], s.err()
}

func clamp16(v int32) int16 {
	return int16(max(math.MinInt16, min(math.MaxInt16, v)))
}

// triplet reads the coordinate deltas of a point of flag f from the
// glyph stream.
func (s *glyfStreams) triplet(f byte) (dx, dy int32) {
	sign := func(flag byte, v int32) int32 {
		if flag&1 != 0 {
			return v
		}
		return -v
	}
	switch {
	case f < 10:
		b := s.glyphs.bytes(1)
		if b == nil {
			return 0, 0
		}
		return 0, sign(f, int32(f&14)<<7+int32(b[0]))
	case f < 20:
		b := s.glyphs.bytes(1)
		if b == nil {
			return 0, 0
		}
		return sign(f, int32((f-10)&14)<<7+int32(b[0])), 0
	case f < 84:
		b := s.glyphs.bytes(1)
		if b == nil {
			return 0, 0
		}
		b0 := int32(f - 20)
		return sign(f, 1+b0&0x30+int32(b[0]>>4)), sign(f>>1, 1+(b0&0x0c)<<2+int32(b[0]&0x0f))
	case f < 120:
		b := s.glyphs.bytes(2)
		if b == nil {
			return 0, 0
		}
		b0 := int32(f - 84)
		return sign(f, 1+(b0/12)<<8+int32(b[0])), sign(f>>1, 1+((b0%12)>>2)<<8+int32(b[1]))
	case f < 124:
		b := s.glyphs.bytes(3)
		if b == nil {
			return 0, 0
		}
		return sign(f, int32(b[0])<<4+int32(b[1]>>4)), sign(f>>1, int32(b[1]&0x0f)<<8+int32(b[2]))
	}
	b := s.glyphs.bytes(4)
	if b == nil {
		return 0, 0
	}
	return sign(f, int32(b[0])<<8+int32(b[1])), sign(f>>1, int32(b[2])<<8+int32(b[3]))
}

// appendPoints appends a simple glyph's flags, a run of the same one
// repeated, and its x and y deltas, each a byte where it fits, none
// where it is zero; the first point carries the overlap flag where the
// glyph has it.
func appendPoints(glyf, flags []byte, xs, ys []int32, overlap bool) []byte {
	const (
		onCurve = 0x01
		xShort  = 0x02
		yShort  = 0x04
		repeat  = 0x08
		xSame   = 0x10
		ySame   = 0x20
	)
	encode := func(d int32, short, same byte) (byte, int) {
		switch {
		case d == 0:
			return same, 0
		case d > -256 && d < 256:
			if d > 0 {
				return short | same, 1
			}
			return short, 1
		}
		return 0, 2
	}
	var xBytes, yBytes []byte
	var px, py int32
	out := make([]byte, len(flags))
	for k, f := range flags {
		if f&0x80 == 0 {
			out[k] |= onCurve
		}
		if k == 0 && overlap {
			out[k] |= overlapSimple
		}
		dx, dy := xs[k]-px, ys[k]-py
		px, py = xs[k], ys[k]
		xf, xn := encode(dx, xShort, xSame)
		yf, yn := encode(dy, yShort, ySame)
		out[k] |= xf | yf
		xBytes = appendDelta(xBytes, dx, xn)
		yBytes = appendDelta(yBytes, dy, yn)
	}
	for k := 0; k < len(out); {
		run := 1
		for k+run < len(out) && out[k+run] == out[k] && run < 256 {
			run++
		}
		if run > 1 {
			glyf = append(glyf, out[k]|repeat, byte(run-1))
		} else {
			glyf = append(glyf, out[k])
		}
		k += run
	}
	return append(append(glyf, xBytes...), yBytes...)
}

func appendDelta(b []byte, d int32, n int) []byte {
	switch n {
	case 1:
		return append(b, byte(abs(d))) //nolint:gosec // a short delta is under 256
	case 2:
		return binary.BigEndian.AppendUint16(b, uint16(int16(d))) //nolint:gosec // a font's coordinates are 16-bit
	}
	return b
}

func abs(v int32) int32 {
	if v < 0 {
		return -v
	}
	return v
}

// rebuildHmtx rebuilds a transformed hmtx table from its advances and
// the left side bearings it keeps, the others the glyphs' xMins.
func rebuildHmtx(tables map[string][]byte, xMins []int16) ([]byte, error) {
	hhea, maxp := tables["hhea"], tables["maxp"]
	if len(hhea) < 36 || len(maxp) < 6 {
		return nil, fmt.Errorf("%w: a transformed hmtx table without hhea and maxp", ErrMalformed)
	}
	numHMetrics := int(binary.BigEndian.Uint16(hhea[34:]))
	numGlyphs := int(binary.BigEndian.Uint16(maxp[4:]))
	r := &reader{b: tables["hmtx"]}
	flags := r.u8()
	if flags&^(hmtxNoLSB|hmtxNoMonoLSB) != 0 || flags == 0 || numHMetrics < 1 || numHMetrics > numGlyphs {
		return nil, fmt.Errorf("%w: an hmtx transform of flags %#x for %d metrics of %d glyphs", ErrMalformed, flags, numHMetrics, numGlyphs)
	}
	if xMins == nil || len(xMins) != numGlyphs {
		return nil, fmt.Errorf("%w: an hmtx transform without the transformed glyf table of its %d glyphs", ErrMalformed, numGlyphs)
	}
	advances := make([]uint16, numHMetrics)
	for i := range advances {
		advances[i] = r.u16()
	}
	lsbs := slices.Clone(xMins)
	if flags&hmtxNoLSB == 0 {
		for i := range numHMetrics {
			lsbs[i] = int16(r.u16()) //nolint:gosec // a signed field
		}
	}
	if flags&hmtxNoMonoLSB == 0 {
		for i := numHMetrics; i < numGlyphs; i++ {
			lsbs[i] = int16(r.u16()) //nolint:gosec // a signed field
		}
	}
	if r.err != nil {
		return nil, r.err
	}
	out := make([]byte, 0, 4*numHMetrics+2*(numGlyphs-numHMetrics))
	for i, lsb := range lsbs {
		if i < numHMetrics {
			out = binary.BigEndian.AppendUint16(out, advances[i])
		}
		out = binary.BigEndian.AppendUint16(out, uint16(lsb)) //nolint:gosec // a signed field
	}
	return out, nil
}

// sfnt writes the font of flavor holding tables: its offset table, its
// table records sorted by tag, each table 4-byte aligned, every
// checksum and head's checksum adjustment set.
func sfnt(flavor []byte, tables map[string][]byte) ([]byte, error) {
	tags := make([]string, 0, len(tables))
	size := sfntHeaderSize + sfntRecordSize*len(tables)
	for tag, t := range tables {
		tags = append(tags, tag)
		size += (len(t) + 3) &^ 3
	}
	slices.Sort(tags)
	n := len(tags)
	if n == 0 || n > math.MaxUint16 {
		return nil, fmt.Errorf("%w: a font of %d tables", ErrMalformed, n)
	}
	out := make([]byte, sfntHeaderSize+sfntRecordSize*n, size)
	copy(out, flavor)
	entrySelector := bits.Len(uint(n)) - 1
	searchRange := (1 << entrySelector) * 16
	binary.BigEndian.PutUint16(out[4:], uint16(n))
	binary.BigEndian.PutUint16(out[6:], uint16(searchRange))       //nolint:gosec // at most n*16
	binary.BigEndian.PutUint16(out[8:], uint16(entrySelector))     //nolint:gosec // at most 15
	binary.BigEndian.PutUint16(out[10:], uint16(n*16-searchRange)) //nolint:gosec // at most n*16
	headAt := -1
	for i, tag := range tags {
		t := tables[tag]
		if tag == "head" {
			if len(t) < headAdjustment+4 {
				return nil, fmt.Errorf("%w: a head table of %d bytes", ErrMalformed, len(t))
			}
			t = slices.Clone(t)
			binary.BigEndian.PutUint32(t[headAdjustment:], 0)
			headAt = len(out)
		}
		rec := out[sfntHeaderSize+sfntRecordSize*i:]
		copy(rec, tag)
		binary.BigEndian.PutUint32(rec[4:], checksum(t))
		binary.BigEndian.PutUint32(rec[8:], uint32(len(out))) //nolint:gosec // under MaxBytes
		binary.BigEndian.PutUint32(rec[12:], uint32(len(t)))  //nolint:gosec // under MaxBytes
		out = append(out, t...)
		for len(out)%4 != 0 {
			out = append(out, 0)
		}
	}
	if headAt >= 0 {
		binary.BigEndian.PutUint32(out[headAt+headAdjustment:], checksumMagic-checksum(out))
	}
	return out, nil
}

// checksum is the sum of b's big-endian 32-bit words, zero padded.
func checksum(b []byte) uint32 {
	var sum uint32
	for len(b) >= 4 {
		sum += binary.BigEndian.Uint32(b)
		b = b[4:]
	}
	var tail [4]byte
	copy(tail[:], b)
	return sum + binary.BigEndian.Uint32(tail[:])
}
