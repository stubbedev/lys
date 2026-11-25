package render

import (
	"math"
	"slices"
)

// maxRegionBands bounds the bands MapRegion samples a rotated image in,
// so the region it hands a compositor stays a few dozen rects whatever
// the surface's size.
const maxRegionBands = 64

// minRegionBand is the shortest band MapRegion samples, in pixels.
const minRegionBand = 2

// MapRegion is region's image under m as whole-pixel rects: exact for a
// map that keeps the axes aligned (scales, translations, quarter
// turns), its edges rounded half up so rects sharing an edge still
// share it; for one that rotates or skews, the image sampled in at
// most maxRegionBands horizontal bands, each the image's extent across
// the band's middle row, as a coarse rasterization of it would be.
func (m Affine) MapRegion(region []Rect) []Rect {
	if len(region) == 0 {
		return nil
	}
	if (negligible(m.B) && negligible(m.C)) || (negligible(m.A) && negligible(m.D)) {
		return m.mapAligned(region)
	}
	return m.mapBanded(region)
}

func roundEdge(v float64) int { return int(math.Floor(v + 0.5)) }

// negligible reports a map coefficient that is zero but for rounding,
// as a quarter turn's cosine is.
func negligible(v float64) bool { return math.Abs(v) < 1e-9 }

// corners is r's four corners in order around it.
func corners(r Rect) [4][2]float64 {
	x0, y0, x1, y1 := float64(r.X), float64(r.Y), float64(r.X+r.W), float64(r.Y+r.H)
	return [4][2]float64{{x0, y0}, {x1, y0}, {x1, y1}, {x0, y1}}
}

func (m Affine) mapAligned(region []Rect) []Rect {
	out := make([]Rect, 0, len(region))
	for _, r := range region {
		if r.Empty() {
			continue
		}
		x0, y0 := math.Inf(1), math.Inf(1)
		x1, y1 := math.Inf(-1), math.Inf(-1)
		for _, c := range corners(r) {
			x, y := m.Apply(c[0], c[1])
			x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x), max(y1, y)
		}
		l, t, rr, b := roundEdge(x0), roundEdge(y0), roundEdge(x1), roundEdge(y1)
		if rr > l && b > t {
			out = append(out, Rect{X: l, Y: t, W: rr - l, H: b - t})
		}
	}
	return out
}

// span is one stretch of a band row, from x0 to x1.
type span struct{ x0, x1 float64 }

func (m Affine) mapBanded(region []Rect) []Rect {
	quads := make([][4][2]float64, 0, len(region))
	top, bottom := math.Inf(1), math.Inf(-1)
	for _, r := range region {
		if r.Empty() {
			continue
		}
		var q [4][2]float64
		for i, c := range corners(r) {
			q[i][0], q[i][1] = m.Apply(c[0], c[1])
			top, bottom = min(top, q[i][1]), max(bottom, q[i][1])
		}
		quads = append(quads, q)
	}
	if len(quads) == 0 || !(bottom > top) {
		return nil
	}
	extent := bottom - top
	bands := min(maxRegionBands, max(1, int(math.Ceil(extent/minRegionBand))))
	step := extent / float64(bands)
	spans := make([]span, 0, len(quads))
	out := make([]Rect, 0, bands)
	for i := range bands {
		y0, y1 := roundEdge(top+float64(float64(i)*step)), roundEdge(top+float64(float64(i+1)*step))
		if y1 <= y0 {
			continue
		}
		mid := float64(y0+y1) / 2
		spans = spans[:0]
		for _, q := range quads {
			if s, ok := crossing(q, mid); ok {
				spans = append(spans, s)
			}
		}
		slices.SortFunc(spans, func(a, b span) int {
			switch {
			case a.x0 < b.x0:
				return -1
			case a.x0 > b.x0:
				return 1
			}
			return 0
		})
		for j := 0; j < len(spans); {
			cur := spans[j]
			for j++; j < len(spans) && roundEdge(spans[j].x0) <= roundEdge(cur.x1); j++ {
				cur.x1 = max(cur.x1, spans[j].x1)
			}
			if l, r := roundEdge(cur.x0), roundEdge(cur.x1); r > l {
				out = append(out, Rect{X: l, Y: y0, W: r - l, H: y1 - y0})
			}
		}
	}
	return out
}

// crossing is where the row y crosses the convex quad q, false when it
// misses it.
func crossing(q [4][2]float64, y float64) (span, bool) {
	s := span{x0: math.Inf(1), x1: math.Inf(-1)}
	for i := range q {
		a, b := q[i], q[(i+1)%len(q)]
		if a[1] == b[1] || y < min(a[1], b[1]) || y > max(a[1], b[1]) {
			continue
		}
		x := a[0] + float64((y-a[1])*(b[0]-a[0]))/(b[1]-a[1])
		s.x0, s.x1 = min(s.x0, x), max(s.x1, x)
	}
	return s, s.x1 > s.x0
}
