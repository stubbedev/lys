package render

import (
	"bytes"
	"math"
	"testing"

	"golang.org/x/image/font/gofont/goregular"

	"github.com/stubbedev/lys/lib/render/internal/autofit"
	"github.com/stubbedev/lys/lib/third_party/typesetting/font"
)

const interRegular = "../../splash/internal/theme/assets/Inter-Regular.ttf"

func loadInter(t *testing.T) *Typeface {
	t.Helper()
	tf, err := LoadFont(readTestFile(t, interRegular))
	if err != nil {
		t.Fatal(err)
	}
	return tf
}

func runeGlyph(t *testing.T, tf *Typeface, r rune) font.GID {
	t.Helper()
	gid, ok := tf.face.NominalGlyph(r)
	if !ok {
		t.Fatalf("the face has no glyph for %q", r)
	}
	return gid
}

func loadTestOutline(tf *Typeface, gid font.GID) (autofit.Outline, bool) {
	var o autofit.Outline
	ok := autofitSource{tf.face}.LoadOutline(&o, uint32(gid))
	return o, ok
}

func contourYExtents(o autofit.Outline) [][2]int64 {
	var extents [][2]int64
	first := 0
	for _, last := range o.Ends {
		lo, hi := o.Points[first].Y, o.Points[first].Y
		for _, p := range o.Points[first : last+1] {
			lo, hi = min(lo, p.Y), max(hi, p.Y)
		}
		extents = append(extents, [2]int64{lo, hi})
		first = last + 1
	}
	return extents
}

func TestTheDotOfAnIStaysApartFromItsStem(t *testing.T) {
	tf := loadInter(t)
	gid := runeGlyph(t, tf, 'i')
	outline, _ := loadTestOutline(tf, gid)
	extents := contourYExtents(outline)
	if len(extents) != 2 {
		t.Fatalf("premise: Inter's i has %d contours, want a stem and a dot", len(extents))
	}
	stem, dot := extents[0], extents[1]
	if dot[0] < stem[0] {
		stem, dot = dot, stem
	}
	for px := 9; px <= 24; px++ {
		scale := float64(px) / tf.upem
		glyphs.dropAll()
		g, _, ok := GlyphCoverage(tf, gid, scale, 0, 0, RGB(255, 255, 255))
		if !ok {
			t.Fatalf("%dpx: no coverage for i", px)
		}
		column, densest := 0, 0
		for x := range g.w {
			sum := 0
			for y := range g.h {
				sum += int(g.mask[y*g.w+x])
			}
			if sum > densest {
				column, densest = x, sum
			}
		}
		top := int(math.Floor(-float64(dot[1])*scale)) - g.oy
		bottom := int(math.Ceil(-float64(stem[0])*scale)) - g.oy
		rows := make([]int, 0, g.h)
		peak := 0
		for y := max(top, 0); y < min(bottom, g.h); y++ {
			a := int(g.mask[y*g.w+column])
			rows = append(rows, a)
			peak = max(peak, a)
		}
		const dotInk, gap, stemInk = 1, 2, 3
		state := 0
		for _, a := range rows {
			switch {
			case state == 0 && 2*a > peak:
				state = dotInk
			case state == dotInk && 4*a < peak:
				state = gap
			case state == gap && 2*a > peak:
				state = stemInk
			}
		}
		if state != stemInk {
			t.Errorf("at %dpx the dot of the i runs into its stem: no row between them is under a quarter of the stem's coverage", px)
		}
	}
}

func TestHintingPutsTheXHeightOnAWholeRow(t *testing.T) {
	tf := loadInter(t)
	gid := runeGlyph(t, tf, 'x')
	outline, _ := loadTestOutline(tf, gid)
	hf := tf.hints.autofit(tf.face)
	offGrid := 0
	for px := int64(9); px <= 32; px++ {
		ys, ok := hf.Hint(nil, uint32(gid), outline, px*64)
		if !ok {
			t.Fatalf("%dpx: x is not hinted", px)
		}
		top := ys[0]
		for _, y := range ys {
			top = max(top, y)
		}
		if top%64 != 0 {
			t.Errorf("at %dpx the top of x sits at %d/64 px, off a whole row", px, top)
		}
		unhinted := 0.0
		for _, p := range outline.Points {
			unhinted = max(unhinted, float64(p.Y)*float64(px)/tf.upem)
		}
		if unhinted != math.Round(unhinted) {
			offGrid++
		}
	}
	if offGrid == 0 {
		t.Fatal("premise: the unhinted x-height is a whole pixel at every size")
	}
}

func TestAGlyphNoScriptCoversDrawsUnhinted(t *testing.T) {
	tf, err := LoadFont(goregular.TTF)
	if err != nil {
		t.Fatal(err)
	}
	gid := runeGlyph(t, tf, 'ℓ')
	outline, _ := loadTestOutline(tf, gid)
	if _, hinted := tf.hints.autofit(tf.face).Hint(nil, uint32(gid), outline, 13*64); hinted {
		t.Fatal("ℓ, which no script covers, is hinted; FreeType gives it its CJK fallback, which the port does not have")
	}
	scale := 13 / tf.upem
	glyphs.dropAll()
	got, _, _ := GlyphCoverage(tf, gid, scale, 0, 0, RGB(255, 255, 255))
	want := new(glyphScratch).raster(devicePoint{}, func(path pathBuilder) {
		outlinePath(outline, func(i int) devicePoint {
			p := outline.Points[i]
			return devicePoint{scale * float64(p.X), -scale * float64(p.Y)}
		}, path)
	})
	if got.ox != want.ox || got.oy != want.oy || !bytes.Equal(got.mask, want.mask) {
		t.Error("ℓ is not drawn as its outline scaled linearly")
	}
}
