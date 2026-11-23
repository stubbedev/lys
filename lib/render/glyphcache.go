package render

// The glyph atlas: a process-wide cache of rasterized glyphs, keyed by
// (face, scale, glyph ID, subpixel bucket). Without it every Draw
// re-traces every outline through the rasterizer - a repainted label
// re-rasterizes its whole alphabet per frame - and every embedded
// bitmap strike re-scales through CatmullRom. Rasters are stored as
// 8bpp coverage masks and tinted at blit time, so one cached mask
// serves every text color; blits go through Canvas.blend like every
// other primitive, keeping the PushAlpha and premultiplied discipline
// by construction.

import (
	"image"
	"image/color"
	"math"

	"golang.org/x/image/draw"

	"github.com/stubbedev/lys/lib/third_party/typesetting/font"
	ot "github.com/stubbedev/lys/lib/third_party/typesetting/font/opentype"
	"github.com/stubbedev/lys/lib/third_party/vector"
)

// glyphKey pins one rasterized outline. scale is the device pixels per
// font unit the glyph rasterized at (the product of logical size and
// device scale); fx, fy are the quarter-pixel buckets of the fractional
// pen offset the mask was rendered with. A strike bitmap's pixels are a
// pure function of (face, glyph, scaled size) - the strike choice is
// itself a function of the face and glyph - so the bitmap cache keys on
// those three.
type glyphKey struct {
	face  *Typeface
	scale float64
	gid   font.GID
	fx    uint8
	fy    uint8
}

type bitmapKey struct {
	face *Typeface
	gid  font.GID
	w, h int
}

// GlyphMask is one cached 8bpp coverage mask, immutable once made: a
// GPU renderer keys its atlas by its ID. ox, oy locate the mask's origin
// relative to the glyph's integer pen position: the blit's target
// pixel is (penX + ox, penY + oy).
type GlyphMask struct {
	identity
	mask []uint8
	w, h int
	ox   int
	oy   int
}

// Pix is the mask's coverage, Size's width bytes per row.
func (g *GlyphMask) Pix() []uint8 { return g.mask }

// Size is the mask's size in device pixels.
func (g *GlyphMask) Size() (w, h int) { return g.w, g.h }

// GlyphCoverage is the coverage mask DrawGlyph tints with the text color
// for an outline glyph, and the device rect it covers with the pen at
// (penX, penY); false for a glyph drawn in color or from a bitmap, or a
// blank one.
func GlyphCoverage(face *Typeface, gid font.GID, scale, penX, penY float64, col Color) (*GlyphMask, Rect, bool) {
	if colorRasterOf(face, gid, scale, penX, penY, col) != nil {
		return nil, Rect{}, false
	}
	g, ok := cachedOutlineRaster(face, gid, scale, penX, penY)
	if !ok {
		return nil, Rect{}, false
	}
	return g, g.bounds(penX, penY), true
}

// Bounds is the device rect the mask covers with its pen at (penX, penY).
func (g *GlyphMask) Bounds(penX, penY float64) Rect { return g.bounds(penX, penY) }

// bounds is the device rect the raster covers with its pen at
// (penX, penY): the integer pen plus the raster's origin offset.
func (g *GlyphMask) bounds(penX, penY float64) Rect {
	return Rect{X: int(math.Floor(penX)) + g.ox, Y: int(math.Floor(penY)) + g.oy, W: g.w, H: g.h}
}

var (
	glyphs = newLRU[glyphKey, *GlyphMask](glyphCacheBudget)
	// bitmapScales caches embedded bitmap strikes - the color-emoji
	// PNGs and black-and-white masks - scaled to device size. The
	// scale is the expensive half (CatmullRom over a PNG-sized
	// source); the decode is already cached on the Typeface.
	bitmapScales = newLRU[bitmapKey, *image.RGBA](glyphCacheBudget)
)

// subpixelBucket quantizes the fractional part of a pen coordinate:
// four buckets in [0,1).
func subpixelBucket(frac float64) uint8 {
	b := int(frac * subpixelBuckets)
	if b >= subpixelBuckets {
		b = subpixelBuckets - 1
	}
	return uint8(b)
}

func penBucket(pen float64) uint8 { return subpixelBucket(pen - math.Floor(pen)) }

// bucketOffset is the fractional pen offset a bucket's mask is
// rasterized at: the bucket's lower edge. The mask is a pure function
// of its key, so a cached glyph looks the same whichever pen in the
// bucket happened to fill it first.
func bucketOffset(b uint8) float64 { return float64(b) / subpixelBuckets }

// cachedOutlineRaster returns the atlas entry for the outline glyph,
// rasterizing it into the cache on a miss. ok is false when the face
// has no outline segments for gid (a space, or a bitmap-only glyph);
// that negative is cached too, so blank glyphs stop re-parsing their
// empty outlines per draw.
func cachedOutlineRaster(t *Typeface, gid font.GID, scale, penX, penY float64) (*GlyphMask, bool) {
	key := glyphKey{
		face:  t,
		scale: scale,
		gid:   gid,
		fx:    penBucket(penX),
		fy:    penBucket(penY),
	}
	if g, ok := glyphs.get(key); ok {
		if g == nil {
			// Cached negative: the face has no outline segments for
			// this glyph (a space, or a bitmap-only emoji). The
			// caller falls through to the bitmap path.
			return nil, false
		}
		return g, true
	}
	outline, ok := fontOutline(t.face, gid)
	if !ok {
		glyphs.put(key, nil, 16)
		return nil, false
	}
	m := Affine{A: scale, C: scale * t.slant, D: -scale, E: bucketOffset(key.fx), F: bucketOffset(key.fy)}
	hinted, isHinted := t.hints.autofit(t.face).Hint(uint32(gid), outline, int64(math.Round(scale*t.upem*64)))
	g := rasterPath(devicePoint{m.E, m.F}, func(path pathBuilder) {
		outlinePath(outline, func(i int) devicePoint {
			p := outline.Points[i]
			y := m.F - float64(scale*float64(p.Y))
			if isHinted {
				y = m.F - float64(float64(hinted[i])/64)
			}
			x, _ := m.Apply(float64(p.X), (m.F-y)/scale)
			return devicePoint{x, y}
		}, path)
	})
	glyphs.put(key, g, len(g.mask))
	return g, true
}

// rasterOutlineXform rasterizes an outline mapped from font units to
// pixels by m into an 8bpp coverage mask, the mask origin at the
// integer pen position: a color glyph's transformed clip. The bounds
// grow a pixel on every side, the anti-aliasing margin the per-pixel
// blit has always worked in.
func rasterOutlineXform(outline font.GlyphOutline, m Affine) *GlyphMask {
	place := func(p ot.SegmentPoint) devicePoint {
		x, y := m.Apply(float64(p.X), float64(p.Y))
		return devicePoint{x, y}
	}
	return rasterPath(place(ot.SegmentPoint{}), func(path pathBuilder) {
		for _, seg := range outline.Segments {
			args := seg.ArgsSlice()
			switch seg.Op {
			case ot.SegmentOpMoveTo:
				path.moveTo(place(args[0]))
			case ot.SegmentOpLineTo:
				path.lineTo(place(args[0]))
			case ot.SegmentOpQuadTo:
				path.quadTo(place(args[0]), place(args[1]))
			case ot.SegmentOpCubeTo:
				path.cubeTo(place(args[0]), place(args[1]), place(args[2]))
			}
		}
	})
}

type recordedPath struct {
	ops []ot.SegmentOp
	pts []devicePoint
}

func (r *recordedPath) moveTo(p devicePoint) {
	r.ops, r.pts = append(r.ops, ot.SegmentOpMoveTo), append(r.pts, p)
}

func (r *recordedPath) lineTo(p devicePoint) {
	r.ops, r.pts = append(r.ops, ot.SegmentOpLineTo), append(r.pts, p)
}

func (r *recordedPath) quadTo(c, p devicePoint) {
	r.ops, r.pts = append(r.ops, ot.SegmentOpQuadTo), append(r.pts, c, p)
}

func (r *recordedPath) cubeTo(c1, c2, p devicePoint) {
	r.ops, r.pts = append(r.ops, ot.SegmentOpCubeTo), append(r.pts, c1, c2, p)
}

func rasterPath(origin devicePoint, trace func(pathBuilder)) *GlyphMask {
	var rec recordedPath
	trace(&rec)
	minX, minY, maxX, maxY := origin.x, origin.y, origin.x, origin.y
	for _, p := range rec.pts {
		minX, minY = math.Min(minX, p.x), math.Min(minY, p.y)
		maxX, maxY = math.Max(maxX, p.x), math.Max(maxY, p.y)
	}
	ox := int(math.Floor(minX)) - 1
	oy := int(math.Floor(minY)) - 1
	w := int(math.Ceil(maxX)) + 1 - ox
	h := int(math.Ceil(maxY)) + 1 - oy
	if w <= 0 || h <= 0 {
		return &GlyphMask{}
	}
	rast := vector.NewRasterizer(w, h)
	pts := rec.pts
	next := func() (float32, float32) {
		p := pts[0]
		pts = pts[1:]
		return float32(p.x) - float32(ox), float32(p.y) - float32(oy)
	}
	for _, op := range rec.ops {
		switch op {
		case ot.SegmentOpMoveTo:
			rast.MoveTo(next())
		case ot.SegmentOpLineTo:
			rast.LineTo(next())
		case ot.SegmentOpQuadTo:
			cx, cy := next()
			ex, ey := next()
			rast.QuadTo(cx, cy, ex, ey)
		case ot.SegmentOpCubeTo:
			c1x, c1y := next()
			c2x, c2y := next()
			ex, ey := next()
			rast.CubeTo(c1x, c1y, c2x, c2y, ex, ey)
		}
	}
	mask := image.NewAlpha(image.Rect(0, 0, w, h))
	rast.Draw(mask, mask.Bounds(), image.NewUniform(color.Alpha{A: 255}), image.Point{})
	return &GlyphMask{mask: mask.Pix, w: w, h: h, ox: ox, oy: oy}
}

// cachedScaledBitmap returns gid's embedded bitmap strike scaled to
// the device size w×h, decoding and scaling on a miss; nil when the
// strike's data does not decode. The cache stores only the scaled
// result: the PNG-sized source is decoded for the scale and dropped.
func cachedScaledBitmap(t *Typeface, gid font.GID, w, h int) *image.RGBA {
	key := bitmapKey{face: t, gid: gid, w: w, h: h}
	if img, ok := bitmapScales.get(key); ok {
		return img
	}
	src := bitmapPixels(t.face, gid)
	if src == nil {
		return nil
	}
	scaled := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.CatmullRom.Scale(scaled, scaled.Bounds(), src, src.Bounds(), draw.Over, nil)
	bitmapScales.put(key, scaled, w*h*4)
	return scaled
}
