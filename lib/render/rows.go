package render

import (
	"encoding/binary"
	"math"
)

// The whole-row compositing kernels the layer passes run on a raster
// canvas: each walks byte rows directly, four channels at a time in
// 16-bit lanes, with the arithmetic of the per-pixel blend it stands in
// for, so the pixels are the same.

// div255Lanes divides each 16-bit lane of l, holding v+128 for a v of at
// most 255·255, by 255, rounding v/255 half up.
func div255Lanes(l uint64) uint64 { return (l + l>>8&lanes) >> 8 & lanes }

// crossFadeWeight is the 0..255 weight a cross-fade at t gives the
// pixels it fades in.
func crossFadeWeight(t float64) uint64 { return uint64(math.Round(min(1, max(0, t)) * 255)) }

// mix is the cross-fade of a into b at weight w: each premultiplied
// channel a·(255−w)/255 + b·w/255, rounded half up.
func mix(a, b Color, w uint64) Color {
	return gather(div255Lanes(a.spread()*(255-w) + b.spread()*w + lanesHalf))
}

// pixelAt is the pixel at byte offset o of a row.
func pixelAt(row []byte, o int) Color { return Color(binary.LittleEndian.Uint32(row[o : o+4 : o+4])) }

// putPixel stores col at byte offset o of a row.
func putPixel(row []byte, o int, col Color) {
	binary.LittleEndian.PutUint32(row[o:o+4:o+4], uint32(col))
}

// opaquePair is the alpha bytes of two pixels read as one uint64.
const opaquePair = 0xff00_0000_ff00_0000

// pairAt is the two pixels at byte offset o of a row, as one uint64.
func pairAt(row []byte, o int) uint64 { return binary.LittleEndian.Uint64(row[o : o+8 : o+8]) }

// putPair stores two pixels, as one uint64, at byte offset o of a row.
func putPair(row []byte, o int, v uint64) { binary.LittleEndian.PutUint64(row[o:o+8:o+8], v) }

// overRow blends each nonzero pixel of src over the pixel of dst at the
// same offset, scaled by scale/255 first: blend without a brightness,
// for a row as long as src. Two opaque pixels at full opacity store as
// one word. It reports how many pixels it wrote.
func overRow(dst, src []byte, scale uint32) int {
	dst = dst[:len(src)]
	n := 0
	for o := 0; o < len(src); o += 4 {
		if o&4 == 0 && o+8 <= len(src) {
			switch p := pairAt(src, o); {
			case p == 0:
				o += 4
				continue
			case scale == 255 && p&opaquePair == opaquePair:
				putPair(dst, o, p)
				n += 2
				o += 4
				continue
			}
		}
		s := pixelAt(src, o)
		if s == 0 {
			continue
		}
		n++
		if scale != 255 {
			s = modulate(s, scale)
		}
		putPixel(dst, o, s.overClamped(pixelAt(dst, o)))
	}
	return n
}

// crossFadeRow fades the pixels of a row into those of b at weight w, in
// place, skipping the pixels the two share. It reports how many pixels
// it wrote.
func crossFadeRow(a, b []byte, w uint64) int {
	b = b[:len(a)]
	n := 0
	for o := 0; o < len(a); o += 4 {
		if o&4 == 0 && o+8 <= len(a) && pairAt(a, o) == pairAt(b, o) {
			o += 4
			continue
		}
		pa, pb := pixelAt(a, o), pixelAt(b, o)
		if pa == pb {
			continue
		}
		n++
		putPixel(a, o, mix(pa, pb, w))
	}
	return n
}

// crossFadeOverRow blends the cross-fade of a into b at weight w over
// dst, scaled by scale/255: crossFadeRow into a scratch row, then
// overRow of it, in one pass. Two opaque pixels the rows share store as
// one word at full opacity. It reports how many pixels it wrote.
func crossFadeOverRow(dst, a, b []byte, w uint64, scale uint32) int {
	a, b = a[:len(dst)], b[:len(dst)]
	n := 0
	for o := 0; o < len(dst); o += 4 {
		if o&4 == 0 && o+8 <= len(dst) && scale == 255 {
			if p := pairAt(a, o); p&opaquePair == opaquePair && p == pairAt(b, o) {
				putPair(dst, o, p)
				n += 2
				o += 4
				continue
			}
		}
		s := pixelAt(a, o)
		if pb := pixelAt(b, o); s != pb {
			s = mix(s, pb, w)
		}
		if s == 0 {
			continue
		}
		n++
		if scale != 255 {
			s = modulate(s, scale)
		}
		putPixel(dst, o, s.overClamped(pixelAt(dst, o)))
	}
	return n
}

// row is the byte row of c's pixels x0 <= x < x0+w on line y.
func (c *Canvas) row(x0, y, w int) []byte {
	o := y*c.stride + x0*4
	return c.data[o : o+w*4 : o+w*4]
}

// neutralBright reports whether c's brightness leaves colors as they
// are.
func (c *Canvas) neutralBright() bool { return c.bright == 0 || c.bright == 1 }
