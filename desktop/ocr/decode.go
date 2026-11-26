package ocr

import (
	"image"
	"math"
	"slices"
)

const (
	glyphPenalty     = 1.0
	skipWeight       = 2
	skipPenalty      = 2 * glyphPenalty
	splitAbove       = 0.06
	kernTolerance    = 0.08
	minKernTolerance = 1.5
	maxMerge         = 3
	maxSplit         = 3
	minPart          = 2
	cutDepth         = 0.6
	mergeSlack       = 2
	sampleCount      = 12
	sampleMinHeight  = 0.3
)

type decoded struct {
	gs     *glyphSet
	glyphs []placement
	fit    float64
}

type option struct {
	err     float32
	glyphs  []placement
	skipped int
}

type span struct{ x0, x1, depth int }

func decodeLine(l *labels, ln line, gs *glyphSet) decoded {
	n := len(ln.clusters)
	best := make([]option, n+1)
	for i := 1; i <= n; i++ {
		best[i].err = math.MaxFloat32
	}
	widest := 0
	for i := range gs.glyphs {
		widest = max(widest, gs.glyphs[i].raster[0].ink.Dx())
	}
	for i := range n {
		if best[i].err == math.MaxFloat32 {
			continue
		}
		skip := best[i]
		skip.err += float32(skipWeight*l.mass(ln.clusters[i].comps)) + skipPenalty
		skip.skipped++
		if skip.err < best[i+1].err {
			best[i+1] = skip
		}
		var ids []int32
		for k := 1; k <= maxMerge && i+k <= n; k++ {
			c := ln.clusters[i+k-1]
			if k > 1 && c.box.Max.X-ln.clusters[i].box.Min.X > widest+mergeSlack {
				break
			}
			ids = append(ids, c.comps...)
			var o option
			var ok bool
			switch {
			case k > 1:
				o, ok = single(l, gs, ids, math.MinInt, math.MaxInt, ln.baseline)
			case c.box.Dx() <= maxSplit*widest+mergeSlack:
				sp := splitter{l: l, gs: gs, ids: ids, base: ln.baseline, cuts: l.cuts(ids, c.box), memo: map[span]option{}}
				o, ok = sp.best(c.box.Min.X, c.box.Max.X, maxSplit)
			}
			if !ok {
				continue
			}
			if e := best[i].err + o.err; e < best[i+k].err {
				best[i+k] = option{err: e, glyphs: append(slices.Clone(best[i].glyphs), o.glyphs...), skipped: best[i].skipped}
			}
		}
	}
	d := decoded{gs: gs, glyphs: best[n].glyphs, fit: math.Inf(1)}
	if len(d.glyphs) == 0 {
		return d
	}
	sum := float64(best[n].skipped)
	for _, g := range d.glyphs {
		sum += math.Min(1, float64(g.norm))
	}
	d.fit = sum / float64(len(d.glyphs)+best[n].skipped)
	return d
}

func single(l *labels, gs *glyphSet, ids []int32, x0, x1, base int) (option, bool) {
	p, ok := l.patch(ids, x0, x1)
	if !ok {
		return option{}, false
	}
	m, ok := gs.best(&p, base)
	if !ok {
		return option{}, false
	}
	return option{err: m.err + glyphPenalty, glyphs: []placement{m}}, true
}

type splitter struct {
	l    *labels
	gs   *glyphSet
	ids  []int32
	base int
	cuts []int
	memo map[span]option
}

func (sp *splitter) best(x0, x1, depth int) (option, bool) {
	key := span{x0, x1, depth}
	if o, ok := sp.memo[key]; ok {
		return o, o.glyphs != nil
	}
	o, ok := sp.single(x0, x1)
	if depth > 1 && (!ok || o.glyphs[0].norm > splitAbove) {
		tol := math.Max(minKernTolerance, kernTolerance*sp.gs.px)
		for _, b := range sp.cuts {
			if b < x0+minPart || b > x1-minPart {
				continue
			}
			left, lok := sp.single(x0, b)
			if !lok {
				continue
			}
			right, rok := sp.best(b, x1, depth-1)
			if !rok {
				continue
			}
			lg, rg := left.glyphs[0], right.glyphs[0]
			if math.Abs(rg.penX-(lg.penX+lg.g.adv)) > tol {
				continue
			}
			if e := left.err + right.err; !ok || e < o.err {
				o, ok = option{err: e, glyphs: append(slices.Clone(left.glyphs), right.glyphs...)}, true
			}
		}
	}
	if !ok {
		o = option{}
	}
	sp.memo[key] = o
	return o, ok
}

func (sp *splitter) single(x0, x1 int) (option, bool) {
	key := span{x0, x1, 0}
	if o, ok := sp.memo[key]; ok {
		return o, o.glyphs != nil
	}
	o, ok := single(sp.l, sp.gs, sp.ids, x0, x1, sp.base)
	if !ok {
		o = option{}
	}
	sp.memo[key] = o
	return o, ok
}

func (l *labels) cuts(ids []int32, box image.Rectangle) []int {
	cols := make([]float32, box.Dx())
	for y := box.Min.Y; y < box.Max.Y; y++ {
		for x := box.Min.X; x < box.Max.X; x++ {
			if k := y*l.w + x; slices.Contains(ids, l.id[k]) {
				cols[x-box.Min.X] += l.c[k]
			}
		}
	}
	peak := slices.Max(cols)
	var out []int
	for i := 1; i < len(cols)-1; i++ {
		if cols[i] <= cols[i-1] && cols[i] <= cols[i+1] && cols[i] <= cutDepth*peak {
			out = append(out, box.Min.X+i, box.Min.X+i+1)
		}
	}
	return slices.Compact(out)
}

func samplePatches(l *labels, ln line) []patch {
	n := len(ln.clusters)
	step := max(1, n/sampleCount)
	var out []patch
	for i := 0; i < n && len(out) < sampleCount; i += step {
		c := ln.clusters[i]
		if float64(c.box.Dy()) < sampleMinHeight*float64(ln.box.Dy()) {
			continue
		}
		if p, ok := l.patch(c.comps, math.MinInt, math.MaxInt); ok {
			out = append(out, p)
		}
	}
	return out
}

func score(gs *glyphSet, samples []patch, base int) float64 {
	var sum float64
	for i := range samples {
		m, ok := gs.best(&samples[i], base)
		if !ok {
			sum++
			continue
		}
		sum += math.Min(1, float64(m.norm))
	}
	return sum / float64(len(samples))
}
