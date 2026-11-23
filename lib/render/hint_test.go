package render

import (
	"bytes"
	"math"
	"slices"
	"testing"
)

const (
	hintTestPx       = 14.666666666666666
	hintTestW        = 64
	hintTestH        = 64
	hintTestPenX     = 8
	hintTestBaseline = 40
	fullRow          = 230
)

var hintTestScales = []float64{1, 1.5}

type glyphPaint [hintTestH][hintTestW]uint8

func paintCoverage(t *testing.T, g *GlyphMask) glyphPaint {
	t.Helper()
	cv, data := newTestCanvas(hintTestW, hintTestH)
	at := g.bounds(hintTestPenX, hintTestBaseline)
	blitMask(cv, cv.clip, g, at.X, at.Y, RGB(255, 255, 255))
	var c glyphPaint
	for y := range hintTestH {
		for x := range hintTestW {
			c[y][x] = pxAt(data, Stride(hintTestW), x, y).A()
		}
	}
	return c
}

func hintedCoverage(t *testing.T, tf *Typeface, r rune, scale float64) glyphPaint {
	t.Helper()
	gid, ok := tf.face.NominalGlyph(r)
	if !ok {
		t.Fatalf("no glyph for %q", r)
	}
	glyphs.dropAll()
	g, ok := cachedOutlineRaster(tf, gid, scale, hintTestPenX, hintTestBaseline)
	if !ok {
		t.Fatalf("no outline for %q", r)
	}
	return paintCoverage(t, g)
}

func unhintedCoverage(t *testing.T, tf *Typeface, r rune, scale float64) glyphPaint {
	t.Helper()
	gid, _ := tf.face.NominalGlyph(r)
	outline, ok := tf.face.GlyphDataOutline(gid)
	if !ok {
		t.Fatalf("no outline for %q", r)
	}
	return paintCoverage(t, rasterOutlineXform(outline, Affine{A: scale, D: -scale}))
}

func (c *glyphPaint) inkedRows() (top, bottom int) {
	top, bottom = -1, -1
	for y := range hintTestH {
		if c[y] != [hintTestW]uint8{} {
			if top < 0 {
				top = y
			}
			bottom = y
		}
	}
	return top, bottom
}

func (c *glyphPaint) inkedColumns() (left, right int) {
	left, right = hintTestW, -1
	for y := range hintTestH {
		for x := range hintTestW {
			if c[y][x] != 0 {
				left, right = min(left, x), max(right, x)
			}
		}
	}
	return left, right
}

func (c *glyphPaint) centerColumn() int {
	left, right := c.inkedColumns()
	return (left + right) / 2
}

func (c *glyphPaint) columnRuns(x int) [][2]int {
	var runs [][2]int
	for y := range hintTestH {
		if c[y][x] == 0 {
			continue
		}
		if n := len(runs); n > 0 && runs[n-1][1] == y-1 {
			runs[n-1][1] = y
			continue
		}
		runs = append(runs, [2]int{y, y})
	}
	return runs
}

func (c *glyphPaint) solidBetween(x int, run [2]int) bool {
	for y := run[0]; y <= run[1]; y++ {
		if c[y][x] < fullRow {
			return false
		}
	}
	return true
}

func deviceScaleOf(tf *Typeface, ds float64) float64 { return hintTestPx * ds / tf.upem }

func TestHintedCrossbarFillsWholeRows(t *testing.T) {
	tf := loadCantarell(t)
	for _, ds := range hintTestScales {
		c := hintedCoverage(t, tf, 'H', deviceScaleOf(tf, ds))
		x := c.centerColumn()
		runs := c.columnRuns(x)
		if len(runs) != 1 {
			t.Fatalf("scale %v: H centre column has ink runs %v, want the crossbar alone", ds, runs)
		}
		if !c.solidBetween(x, runs[0]) {
			t.Errorf("scale %v: H crossbar rows %v have coverage %v, want every row >= %d", ds, runs[0], column(&c, x, runs[0]), fullRow)
		}
		if above, below := c[runs[0][0]-1][x], c[runs[0][1]+1][x]; above != 0 || below != 0 {
			t.Errorf("scale %v: rows next to the crossbar have coverage %d and %d, want 0", ds, above, below)
		}
	}
}

func TestUnhintedCrossbarStraddlesRows(t *testing.T) {
	tf := loadCantarell(t)
	c := unhintedCoverage(t, tf, 'H', deviceScaleOf(tf, 1))
	x := c.centerColumn()
	runs := c.columnRuns(x)
	if len(runs) != 1 || c.solidBetween(x, runs[0]) {
		t.Fatalf("unhinted H crossbar %v with coverage %v is solid, want it spread over partial rows", runs, column(&c, x, runs[0]))
	}
}

func column(c *glyphPaint, x int, run [2]int) []uint8 {
	col := make([]uint8, 0, run[1]-run[0]+1)
	for y := run[0]; y <= run[1]; y++ {
		col = append(col, c[y][x])
	}
	return col
}

func TestHintedStemEndsFillWholeRows(t *testing.T) {
	tf := loadCantarell(t)
	for _, ds := range hintTestScales {
		for _, r := range "Hl" {
			c := hintedCoverage(t, tf, r, deviceScaleOf(tf, ds))
			top, bottom := c.inkedRows()
			if bottom != hintTestBaseline-1 {
				t.Errorf("scale %v: %q ends on row %d, want the row above the baseline %d", ds, r, bottom, hintTestBaseline-1)
			}
			if c[top] != c[top+1] || c[bottom] != c[bottom-1] {
				t.Errorf("scale %v: %q stem ends %v and %v differ from the stem rows %v and %v", ds, r, c[top], c[bottom], c[top+1], c[bottom-1])
			}
		}
	}
}

func TestUnhintedStemTopIsPartial(t *testing.T) {
	tf := loadCantarell(t)
	c := unhintedCoverage(t, tf, 'H', deviceScaleOf(tf, 1))
	top, _ := c.inkedRows()
	if c[top] == c[top+1] {
		t.Fatal("unhinted H cap row is full, want the cap height between rows")
	}
}

func TestHintedXHeightBarsFillWholeRows(t *testing.T) {
	tf := loadCantarell(t)
	for _, ds := range hintTestScales {
		z := hintedCoverage(t, tf, 'z', deviceScaleOf(tf, ds))
		top, bottom := z.inkedRows()
		x := z.centerColumn()
		if z[top][x] < fullRow || z[bottom][x] < fullRow {
			t.Errorf("scale %v: z bars have coverage %d (top) and %d (bottom), want >= %d", ds, z[top][x], z[bottom][x], fullRow)
		}
		if bottom != hintTestBaseline-1 {
			t.Errorf("scale %v: z ends on row %d, want %d", ds, bottom, hintTestBaseline-1)
		}
		o := hintedCoverage(t, tf, 'o', deviceScaleOf(tf, ds))
		otop, obottom := o.inkedRows()
		if otop != top || obottom != bottom {
			t.Errorf("scale %v: o spans rows %d-%d, z %d-%d: an overshoot under a pixel must not add a row", ds, otop, obottom, top, bottom)
		}
	}
	unhinted := unhintedCoverage(t, tf, 'z', deviceScaleOf(tf, 1))
	top, _ := unhinted.inkedRows()
	if x := unhinted.centerColumn(); unhinted[top][x] >= fullRow {
		t.Errorf("unhinted z top row has coverage %d, want the x-height between rows", unhinted[top][x])
	}
}

func TestHintedOvershootOfTwoPixelsAddsTwoRows(t *testing.T) {
	tf := loadCantarell(t)
	var zone blueZone
	for _, z := range tf.blueZones() {
		if z.top && z.over > z.ref && (zone.ref == 0 || z.ref < zone.ref) {
			zone = z
		}
	}
	if zone.over <= zone.ref {
		t.Fatal("Cantarell has no x-height overshoot")
	}
	scale := 2 / (zone.over - zone.ref)
	zgid, _ := tf.face.NominalGlyph('z')
	ogid, _ := tf.face.NominalGlyph('o')
	glyphs.dropAll()
	z, _ := cachedOutlineRaster(tf, zgid, scale, 0, 0)
	o, _ := cachedOutlineRaster(tf, ogid, scale, 0, 0)
	if rows := inkTop(z) - inkTop(o); rows != 2 {
		t.Errorf("o tops out %d rows above z, want the two-pixel overshoot kept as 2", rows)
	}
}

func inkTop(g *GlyphMask) int {
	for y := range g.h {
		if slices.ContainsFunc(g.mask[y*g.w:(y+1)*g.w], func(v uint8) bool { return v != 0 }) {
			return g.oy + y
		}
	}
	return g.oy + g.h
}

func TestHintingLeavesXUntouched(t *testing.T) {
	tf := loadCantarell(t)
	for _, ds := range hintTestScales {
		for _, r := range "Hl" {
			scale := deviceScaleOf(tf, ds)
			hinted := hintedCoverage(t, tf, r, scale)
			unhinted := unhintedCoverage(t, tf, r, scale)
			hl, hr := hinted.inkedColumns()
			ul, ur := unhinted.inkedColumns()
			if hl != ul || hr != ur {
				t.Errorf("scale %v: %q columns %d-%d, unhinted %d-%d", ds, r, hl, hr, ul, ur)
			}
			row := hintTestBaseline - 2
			if hinted[row] != unhinted[row] {
				t.Errorf("scale %v: %q stem row %v differs from the unhinted %v", ds, r, hinted[row], unhinted[row])
			}
		}
	}
}

func TestHintingKeepsSubpixelPositions(t *testing.T) {
	tf := loadCantarell(t)
	gid, _ := tf.face.NominalGlyph('l')
	scale := deviceScaleOf(tf, 1)
	glyphs.dropAll()
	whole, _ := cachedOutlineRaster(tf, gid, scale, 0, 0)
	half, _ := cachedOutlineRaster(tf, gid, scale, 0.5, 0)
	if whole.h != half.h || bytes.Equal(whole.mask, half.mask) {
		t.Fatalf("a half-pixel pen moved the glyph vertically or not at all: %dx%d vs %dx%d", whole.w, whole.h, half.w, half.h)
	}
	row := func(g *GlyphMask, y int) []uint8 { return g.mask[y*g.w : (y+1)*g.w] }
	sum := func(p []uint8) int {
		n := 0
		for _, v := range p {
			n += int(v)
		}
		return n
	}
	mid := whole.h / 2
	if a, b := sum(row(whole, mid)), sum(row(half, mid)); math.Abs(float64(a-b)) > 2 {
		t.Errorf("stem coverage %d at a whole pen, %d at a half pen: x placement must keep the stem's area", a, b)
	}
}

func TestHintingIsDeterministic(t *testing.T) {
	first := loadCantarell(t)
	second := loadCantarell(t)
	for _, ds := range hintTestScales {
		for _, r := range "Hamburgefonstiv" {
			a := hintedCoverage(t, first, r, deviceScaleOf(first, ds))
			b := hintedCoverage(t, first, r, deviceScaleOf(first, ds))
			c := hintedCoverage(t, second, r, deviceScaleOf(second, ds))
			if a != b || a != c {
				t.Errorf("scale %v: %q painted differently on a repeat", ds, r)
			}
		}
	}
}
