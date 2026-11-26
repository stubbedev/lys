// Package ocr reads the text in a screen image. It recognises text
// drawn in fonts it is given: each candidate glyph is shaped and
// rasterised by lys-lib's render at the size and subpixel phase the
// screen shows it at and matched against the image's coverage, so it
// needs no trained model and runs no other program. Latin script is
// what it reads: ASCII, Latin-1 and common typographic punctuation, plus
// the ligatures and contextual forms the fonts draw for them.
package ocr

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"image"
	"math"
	"slices"

	"github.com/stubbedev/lys/lib/render"
)

var (
	// ErrNoText answers a reading that found no text.
	ErrNoText = errors.New("no text found")
	// ErrBlank answers a reading of a region with nothing drawn in it.
	ErrBlank = fmt.Errorf("%w: the region is blank", ErrNoText)
	// ErrNotText answers a reading of a region whose marks match none
	// of the fonts' glyphs.
	ErrNotText = fmt.Errorf("%w: nothing in the region reads as text", ErrNoText)
)

const (
	keepFit          = 0.04
	shortlistLen     = 3
	climbSteps       = 4
	switchMargin     = 0.05
	ratioPx          = 100
	rejectSample     = 0.5
	rejectLine       = 0.33
	minPx            = 6
	maxPx            = 160
	sizeStep         = 0.5
	shortLine        = 4
	baselineReach    = 0.35
	maxBaselineShift = 8
)

// Reader reads text drawn in its faces. It caches the glyphs it renders
// across readings and, like the faces, is not safe for concurrent use.
type Reader struct {
	faces  []*render.Typeface
	ratios map[*render.Typeface]faceRatios
	shapes map[*render.Typeface]faceGlyphs
	sets   map[setKey]*glyphSet
}

type setKey struct {
	face *render.Typeface
	px   float64
	full bool
}

type faceRatios struct{ x, cap, asc float64 }

// New is a Reader of text drawn in faces, which it uses as they are:
// a bold or other instance is a face of its own.
func New(faces ...*render.Typeface) (*Reader, error) {
	if len(faces) == 0 {
		return nil, errors.New("ocr: a reader needs at least one face")
	}
	return &Reader{
		faces:  slices.Clone(faces),
		ratios: map[*render.Typeface]faceRatios{},
		shapes: map[*render.Typeface]faceGlyphs{},
		sets:   map[setKey]*glyphSet{},
	}, nil
}

// Read is the text in img, line by line from the top, with the spaces
// its fonts' advances put between words and, in a monospaced face,
// before a line. A region with nothing drawn is ErrBlank, one with no
// line that reads as text ErrNotText; a stopped ctx ends the reading
// with its cause.
func (r *Reader) Read(ctx context.Context, img *image.RGBA) (string, error) {
	m, ok := newInkMap(img)
	if !ok {
		return "", ErrBlank
	}
	l := label(m)
	if len(l.comps) == 0 {
		return "", ErrBlank
	}
	var read []decoded
	var kept []line
	var prev *glyphSet
	for _, ln := range l.lines() {
		if err := ctx.Err(); err != nil {
			return "", context.Cause(ctx)
		}
		d, ok := r.readLine(&l, ln, prev)
		if !ok {
			continue
		}
		prev = d.gs
		read = append(read, d)
		kept = append(kept, ln)
	}
	if len(read) == 0 {
		return "", ErrNotText
	}
	r.harmonize(&l, kept, read)
	return layout(read), nil
}

func (r *Reader) readLine(l *labels, ln line, prev *glyphSet) (decoded, bool) {
	best := decoded{fit: math.Inf(1)}
	for _, cand := range baselines(ln) {
		if prev != nil {
			if d := decodeLine(l, cand, prev); d.fit <= keepFit && d.fit < best.fit {
				best = d
				continue
			}
		}
		for _, gs := range r.shortlist(l, cand) {
			if d := decodeLine(l, cand, gs); d.fit < best.fit {
				best = d
			}
		}
	}
	return best, best.fit <= rejectLine
}

func baselines(ln line) []line {
	if len(ln.clusters) >= shortLine {
		return []line{ln}
	}
	reach := min(maxBaselineShift, int(math.Ceil(baselineReach*float64(ln.box.Dy()))))
	out := make([]line, 0, 2*reach+1)
	for d := -reach; d <= reach; d++ {
		cand := ln
		cand.baseline += d
		out = append(out, cand)
	}
	return out
}

func (r *Reader) harmonize(l *labels, lns []line, ds []decoded) {
	count := map[*render.Typeface]int{}
	sizes := map[*render.Typeface][]float64{}
	for _, d := range ds {
		count[d.gs.face] += len(d.glyphs)
		if !slices.Contains(sizes[d.gs.face], d.gs.px) {
			sizes[d.gs.face] = append(sizes[d.gs.face], d.gs.px)
		}
	}
	dominant := r.faces[0]
	for _, face := range r.faces {
		if count[face] > count[dominant] {
			dominant = face
		}
	}
	for i, d := range ds {
		if d.gs.face == dominant {
			continue
		}
		alt := decoded{fit: math.Inf(1)}
		for _, px := range sizes[dominant] {
			if a := decodeLine(l, lns[i], r.set(dominant, px, true)); a.fit < alt.fit {
				alt = a
			}
		}
		if alt.fit <= d.fit+switchMargin {
			ds[i] = alt
		}
	}
}

type scored struct {
	gs  *glyphSet
	fit float64
}

func (r *Reader) shortlist(l *labels, ln line) []*glyphSet {
	samples := samplePatches(l, ln)
	if len(samples) == 0 {
		return nil
	}
	all := make([]scored, 0, len(r.faces))
	for _, face := range r.faces {
		if s, ok := r.bestSize(face, ln, samples); ok {
			all = append(all, s)
		}
	}
	slices.SortStableFunc(all, func(a, b scored) int { return cmp.Compare(a.fit, b.fit) })
	out := make([]*glyphSet, 0, shortlistLen)
	for _, s := range all[:min(shortlistLen, len(all))] {
		if s.fit <= rejectSample {
			out = append(out, r.set(s.gs.face, s.gs.px, true))
		}
	}
	return out
}

func (r *Reader) bestSize(face *render.Typeface, ln line, samples []patch) (scored, bool) {
	fit := func(px float64) scored {
		gs := r.set(face, px, false)
		return scored{gs, score(gs, samples, ln.baseline)}
	}
	best := scored{fit: math.Inf(1)}
	for _, px := range r.sizes(ln, face) {
		if s := fit(px); s.fit < best.fit {
			best = s
		}
	}
	if best.gs == nil {
		return best, false
	}
	for _, dir := range []float64{-sizeStep, sizeStep} {
		for range climbSteps {
			px := best.gs.px + dir
			if px < minPx || px > maxPx {
				break
			}
			s := fit(px)
			if s.fit >= best.fit {
				break
			}
			best = s
		}
	}
	return best, true
}

func (r *Reader) sizes(ln line, face *render.Typeface) []float64 {
	m := r.faceRatios(face)
	counts := map[int]int{}
	tallest := 0
	for _, c := range ln.clusters {
		if abs(c.box.Max.Y-ln.baseline) > 1 {
			continue
		}
		h := ln.baseline - c.box.Min.Y
		counts[h]++
		tallest = max(tallest, h)
	}
	mode, mc := 0, 0
	for h, c := range counts {
		if c > mc || (c == mc && h > mode) {
			mode, mc = h, c
		}
	}
	var out []float64
	add := func(h int, ratio float64) {
		if ratio <= 0 || h <= 0 {
			return
		}
		base := float64(math.Round(float64(h)/ratio/sizeStep) * sizeStep)
		for _, px := range []float64{base - sizeStep, base, base + sizeStep} {
			if px >= minPx && px <= maxPx && !slices.Contains(out, px) {
				out = append(out, px)
			}
		}
	}
	add(mode, m.x)
	add(mode, m.cap)
	add(tallest, m.cap)
	add(tallest, m.asc)
	return out
}

func (r *Reader) shaped(face *render.Typeface) faceGlyphs {
	if fg, ok := r.shapes[face]; ok {
		return fg
	}
	fg := shapeFace(face)
	r.shapes[face] = fg
	return fg
}

func (r *Reader) set(face *render.Typeface, px float64, full bool) *glyphSet {
	k := setKey{face, px, full}
	if gs, ok := r.sets[k]; ok {
		return gs
	}
	n := 1
	if full {
		n = phases
	}
	gs := newGlyphSet(face, r.shaped(face), px, n, !full)
	r.sets[k] = gs
	return gs
}

func (r *Reader) faceRatios(face *render.Typeface) faceRatios {
	if m, ok := r.ratios[face]; ok {
		return m
	}
	fg := r.shaped(face)
	height := func(s string) float64 {
		i := slices.IndexFunc(fg.glyphs, func(g shapedGlyph) bool { return g.text == s })
		if i < 0 {
			return 0
		}
		ras, ok := rasterize(face, fg.glyphs[i].pens, ratioPx, 0)
		if !ok {
			return 0
		}
		return float64(-ras.ink.Min.Y) / ratioPx
	}
	m := faceRatios{x: height("x"), cap: height("H"), asc: height("d")}
	r.ratios[face] = m
	return m
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
