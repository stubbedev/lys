package ocr

import (
	"image"
	"math"
	"slices"
)

const (
	minSizeSlack = 2
	sizeSlack    = 0.15
)

type patch struct {
	box  image.Rectangle
	pix  []float32
	ink  image.Rectangle
	mass float32
}

func (l *labels) patch(ids []int32, x0, x1 int) (patch, bool) {
	var ink image.Rectangle
	for _, id := range ids {
		ink = ink.Union(l.comp(id).box)
	}
	box := ink.Inset(-1).Intersect(image.Rect(max(x0, 0), 0, min(x1, l.w), l.h))
	if box.Empty() {
		return patch{}, false
	}
	p := patch{box: box, pix: make([]float32, box.Dx()*box.Dy())}
	owned := func(x, y int) bool {
		return image.Pt(x, y).In(box) && slices.Contains(ids, l.id[y*l.w+x])
	}
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			k := y*l.w + x
			id, v := l.id[k], l.c[k]
			if v == 0 || (id != 0 && !slices.Contains(ids, id)) || (id == 0 && !beside(owned, x, y)) {
				continue
			}
			p.pix[(y-box.Min.Y)*box.Dx()+x-box.Min.X] = v
			p.mass += v
			if v >= inkThreshold {
				p.ink = p.ink.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return p, !p.ink.Empty()
}

func beside(owned func(x, y int) bool, x, y int) bool {
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			if owned(x+dx, y+dy) {
				return true
			}
		}
	}
	return false
}

type placement struct {
	g    *glyph
	err  float32
	norm float32
	penX float64
}

func sizeFits(a, b int) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return float64(d) <= math.Max(minSizeSlack, sizeSlack*float64(max(a, b)))
}

func matchGlyph(p *patch, g *glyph, base int, bound float32) (placement, bool) {
	best := placement{g: g, norm: bound}
	found := false
	for ph := range g.raster {
		r := &g.raster[ph]
		if !sizeFits(r.ink.Dx(), p.ink.Dx()) || !sizeFits(r.ink.Dy(), p.ink.Dy()) {
			continue
		}
		total := p.mass + r.mass
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				limit := (best.norm - g.prior) * total
				if limit <= 0 || abs32(p.mass-r.mass) >= limit {
					continue
				}
				ox, oy := p.ink.Min.X+dx-r.ink.Min.X, base+dy
				e, ok := difference(p, r, ox, oy, limit)
				if !ok {
					continue
				}
				best = placement{
					g:    g,
					err:  e + float32(g.prior*total),
					norm: e/total + g.prior,
					penX: float64(ox) + float64(ph)/float64(len(g.raster)),
				}
				found = true
			}
		}
	}
	return best, found
}

func difference(p *patch, r *glyphRaster, ox, oy int, limit float32) (float32, bool) {
	total := p.mass + r.mass
	placed := r.box.Add(image.Pt(ox, oy))
	in := p.box.Intersect(placed)
	pw, rw := p.box.Dx(), r.box.Dx()
	var shared float32
	for y := in.Min.Y; y < in.Max.Y; y++ {
		prow := p.pix[(y-p.box.Min.Y)*pw+in.Min.X-p.box.Min.X:][:in.Dx()]
		rrow := r.pix[(y-placed.Min.Y)*rw+in.Min.X-placed.Min.X:][:in.Dx()]
		for i, a := range prow {
			shared += min(a, rrow[i])
		}
	}
	e := total - float32(2*shared)
	return e, e < limit
}

func (gs *glyphSet) best(p *patch, base int) (placement, bool) {
	var best placement
	bound := float32(math.MaxFloat32)
	found := false
	for i := range gs.glyphs {
		if m, ok := matchGlyph(p, &gs.glyphs[i], base, bound); ok {
			best, bound, found = m, m.norm, true
		}
	}
	return best, found
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
