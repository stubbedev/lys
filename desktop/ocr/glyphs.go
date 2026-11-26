package ocr

import (
	"image"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/third_party/typesetting/font"
)

const (
	phases   = 4
	shapePx  = 64
	penBase  = 1 << 11
	recordW  = 1 << 14
	recordH  = 1 << 12
	extended = 0.02
	typeset  = 0.03
)

type candidate struct {
	text  string
	prior float32
	core  bool
}

var repertoire = func() []candidate {
	var out []candidate
	for r := rune('!'); r <= '~'; r++ {
		core := r >= '0' && r <= '9' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z'
		out = append(out, candidate{string(r), 0, core})
	}
	for r := rune(0xC0); r <= 0xFF; r++ {
		out = append(out, candidate{string(r), extended, false})
	}
	for _, s := range []string{"–", "—", "‘", "’", "“", "”", "…", "•", "€", "°", "§", "©", "«", "»", "¿", "¡"} {
		out = append(out, candidate{s, typeset, false})
	}
	return out
}()

var sequences = []string{
	"fi", "fl", "ff", "ffi", "ffl", "ft", "fj", "tt", "Th",
	"--", "---", "...", "..", "::", ":=", "->", "<-", "=>", "==", "===", "!=", "!==",
	"<=", ">=", "//", "///", "/*", "*/", "++", "&&", "||", "<<", ">>", "|>", "<|",
	"~>", "<>", "</", "/>", "?.", "??", "##", "###", "www", "0x", "__", ";;", "=<<", ">>=",
}

type pen struct {
	gid   font.GID
	scale float64
	x, y  float64
}

type shapedGlyph struct {
	candidate
	pens []pen
	adv  float64
}

type faceGlyphs struct {
	glyphs []shapedGlyph
	space  float64
}

type penCapture struct {
	render.Player
	face *render.Typeface
	pens []pen
	own  bool
}

func (c *penCapture) Glyph(_ render.State, face *render.Typeface, gid font.GID, scale, x, y float64, _ render.Color, _ *render.GlyphMask) {
	c.own = c.own && face == c.face
	c.pens = append(c.pens, pen{gid: gid, scale: scale / shapePx, x: x / shapePx, y: (y - penBase) / shapePx})
}

func shapePens(face *render.Typeface, s string) ([]pen, float64, bool) {
	sh := face.Shape(s, shapePx)
	if sh.NotDefCount() > 0 {
		return nil, 0, false
	}
	cv := render.NewRecorder(recordW, recordH, 1, 1)
	list := cv.Recording()
	defer list.End()
	sh.Draw(cv, 0, penBase, render.RGB(255, 255, 255))
	c := penCapture{face: face, own: true}
	list.Play(&c)
	return c.pens, sh.Advance() / shapePx, c.own && len(c.pens) > 0
}

func shapeFace(face *render.Typeface) faceGlyphs {
	fg := faceGlyphs{space: face.Shape(" ", shapePx).Advance() / shapePx}
	single := map[rune]font.GID{}
	for _, c := range repertoire {
		pens, adv, ok := shapePens(face, c.text)
		if !ok || len(pens) != 1 {
			continue
		}
		single[[]rune(c.text)[0]] = pens[0].gid
		fg.glyphs = append(fg.glyphs, shapedGlyph{c, pens, adv})
	}
	for _, seq := range sequences {
		pens, adv, ok := shapePens(face, seq)
		if !ok || !contextual(seq, pens, single) {
			continue
		}
		fg.glyphs = append(fg.glyphs, shapedGlyph{candidate{text: seq}, pens, adv})
	}
	return fg
}

func contextual(seq string, pens []pen, single map[rune]font.GID) bool {
	rs := []rune(seq)
	if len(rs) != len(pens) {
		return true
	}
	for i, r := range rs {
		if g, ok := single[r]; !ok || g != pens[i].gid {
			return true
		}
	}
	return false
}

type glyphRaster struct {
	box  image.Rectangle
	pix  []float32
	ink  image.Rectangle
	mass float32
}

type glyph struct {
	text   string
	prior  float32
	adv    float64
	raster []glyphRaster
}

type glyphSet struct {
	face   *render.Typeface
	px     float64
	glyphs []glyph
	space  float64
	mono   bool
}

func newGlyphSet(face *render.Typeface, fg faceGlyphs, px float64, n int, coreOnly bool) *glyphSet {
	gs := &glyphSet{face: face, px: px, mono: face.IsMonospace(), space: fg.space * px}
	for _, sg := range fg.glyphs {
		if coreOnly && !sg.core {
			continue
		}
		if g, ok := makeGlyph(face, sg, px, n); ok {
			gs.glyphs = append(gs.glyphs, g)
		}
	}
	return gs
}

func makeGlyph(face *render.Typeface, sg shapedGlyph, px float64, n int) (glyph, bool) {
	g := glyph{text: sg.text, prior: sg.prior, adv: sg.adv * px, raster: make([]glyphRaster, n)}
	for ph := range n {
		r, ok := rasterize(face, sg.pens, px, float64(ph)/float64(n))
		if !ok {
			return glyph{}, false
		}
		g.raster[ph] = r
	}
	return g, true
}

func rasterize(face *render.Typeface, pens []pen, px, dx float64) (glyphRaster, bool) {
	type part struct {
		mask *render.GlyphMask
		at   image.Rectangle
	}
	parts := make([]part, 0, len(pens))
	var box image.Rectangle
	for _, p := range pens {
		mask, rect, ok := render.GlyphCoverage(face, p.gid, p.scale*px, float64(p.x*px)+dx, p.y*px, render.RGB(255, 255, 255))
		if !ok {
			continue
		}
		at := image.Rect(rect.X, rect.Y, rect.X+rect.W, rect.Y+rect.H)
		parts = append(parts, part{mask, at})
		box = box.Union(at)
	}
	if box.Empty() {
		return glyphRaster{}, false
	}
	r := glyphRaster{box: box, pix: make([]float32, box.Dx()*box.Dy())}
	for _, pt := range parts {
		w, _ := pt.mask.Size()
		for i, v := range pt.mask.Pix() {
			x, y := pt.at.Min.X+i%w, pt.at.Min.Y+i/w
			k := (y-box.Min.Y)*box.Dx() + x - box.Min.X
			r.pix[k] = min(1, r.pix[k]+float32(v)/255)
		}
	}
	for i, f := range r.pix {
		r.mass += f
		if f >= inkThreshold {
			x, y := box.Min.X+i%box.Dx(), box.Min.Y+i/box.Dx()
			r.ink = r.ink.Union(image.Rect(x, y, x+1, y+1))
		}
	}
	return r, !r.ink.Empty()
}
