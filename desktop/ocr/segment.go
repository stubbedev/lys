package ocr

import (
	"image"
	"slices"
)

const (
	thinBand      = 0.45
	thinBandReach = 0.6
	clusterJoin   = 0.5
	massJoin      = 1.2
)

type component struct {
	box  image.Rectangle
	id   int32
	mass float32
}

type labels struct {
	inkMap
	id    []int32
	comps []component
}

func label(m inkMap) labels {
	l := labels{inkMap: m, id: make([]int32, m.w*m.h)}
	var stack []int
	for start, v := range m.c {
		if v < inkThreshold || l.id[start] != 0 {
			continue
		}
		id := int32(len(l.comps) + 1) //nolint:gosec // one component per inked pixel at most, and a region's pixels fit an int32
		c := component{id: id, box: image.Rect(start%m.w, start/m.w, start%m.w+1, start/m.w+1)}
		l.id[start] = id
		stack = append(stack[:0], start)
		for len(stack) > 0 {
			p := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			x, y := p%m.w, p/m.w
			c.box = c.box.Union(image.Rect(x, y, x+1, y+1))
			c.mass += m.c[p]
			for ny := max(y-1, 0); ny <= min(y+1, m.h-1); ny++ {
				for nx := max(x-1, 0); nx <= min(x+1, m.w-1); nx++ {
					q := ny*m.w + nx
					if l.id[q] == 0 && m.c[q] >= inkThreshold {
						l.id[q] = id
						stack = append(stack, q)
					}
				}
			}
		}
		l.comps = append(l.comps, c)
	}
	return l
}

func (l *labels) comp(id int32) component { return l.comps[id-1] }

func (l *labels) mass(ids []int32) float32 {
	var m float32
	for _, id := range ids {
		m += l.comp(id).mass
	}
	return m
}

type cluster struct {
	box   image.Rectangle
	comps []int32
}

type line struct {
	box      image.Rectangle
	clusters []cluster
	baseline int
}

type band struct {
	y0, y1 int
	comps  []int32
}

func (l *labels) lines() []line {
	order := make([]int32, len(l.comps))
	for i := range order {
		order[i] = int32(i + 1)
	}
	slices.SortStableFunc(order, func(a, b int32) int { return l.comp(a).box.Min.Y - l.comp(b).box.Min.Y })
	var bands []band
	for _, id := range order {
		c := l.comp(id)
		if n := len(bands); n > 0 && c.box.Min.Y < bands[n-1].y1 {
			bands[n-1].y1 = max(bands[n-1].y1, c.box.Max.Y)
			bands[n-1].comps = append(bands[n-1].comps, id)
			continue
		}
		bands = append(bands, band{y0: c.box.Min.Y, y1: c.box.Max.Y, comps: []int32{id}})
	}
	bands = mergeThinBands(bands)
	out := make([]line, 0, len(bands))
	for _, b := range bands {
		out = append(out, l.makeLine(b.comps))
	}
	return out
}

func mergeThinBands(bands []band) []band {
	if len(bands) < 2 {
		return bands
	}
	heights := make([]int, len(bands))
	for i, b := range bands {
		heights[i] = b.y1 - b.y0
	}
	slices.Sort(heights)
	median := float64(heights[len(heights)/2])
	for i := 0; i < len(bands); {
		b := bands[i]
		if float64(b.y1-b.y0) >= thinBand*median {
			i++
			continue
		}
		target, gap := -1, thinBandReach*median
		if i > 0 && float64(b.y0-bands[i-1].y1) <= gap {
			target, gap = i-1, float64(b.y0-bands[i-1].y1)
		}
		if i+1 < len(bands) && float64(bands[i+1].y0-b.y1) <= gap {
			target = i + 1
		}
		if target < 0 {
			i++
			continue
		}
		t := &bands[target]
		t.y0, t.y1 = min(t.y0, b.y0), max(t.y1, b.y1)
		t.comps = append(t.comps, b.comps...)
		bands = slices.Delete(bands, i, i+1)
	}
	return bands
}

func (l *labels) makeLine(ids []int32) line {
	slices.SortStableFunc(ids, func(a, b int32) int { return l.comp(a).box.Min.X - l.comp(b).box.Min.X })
	var ln line
	for _, id := range ids {
		c := l.comp(id)
		ln.box = ln.box.Union(c.box)
		if n := len(ln.clusters); n > 0 && slices.ContainsFunc(ln.clusters[n-1].comps, func(o int32) bool { return l.joins(l.comp(o), c) }) {
			last := &ln.clusters[n-1]
			last.box = last.box.Union(c.box)
			last.comps = append(last.comps, id)
			continue
		}
		ln.clusters = append(ln.clusters, cluster{box: c.box, comps: []int32{id}})
	}
	ln.baseline = baseline(ln.clusters)
	return ln
}

func (l *labels) joins(a, b component) bool {
	upper, lower := a, b
	if b.box.Max.Y < a.box.Max.Y {
		upper, lower = b, a
	}
	if upper.box.Dx() > lower.box.Dx()+1 || upper.mass > lower.mass*massJoin {
		return false
	}
	overlap := min(a.box.Max.X, b.box.Max.X) - max(a.box.Min.X, b.box.Min.X)
	if float64(overlap) < clusterJoin*float64(upper.box.Dx()) {
		return false
	}
	top, ok := l.topWithin(lower, upper.box.Min.X, upper.box.Max.X)
	return ok && top >= upper.box.Max.Y
}

func (l *labels) topWithin(c component, x0, x1 int) (int, bool) {
	x0, x1 = max(x0, c.box.Min.X), min(x1, c.box.Max.X)
	for y := c.box.Min.Y; y < c.box.Max.Y; y++ {
		for x := x0; x < x1; x++ {
			if l.id[y*l.w+x] == c.id {
				return y, true
			}
		}
	}
	return 0, false
}

func baseline(cs []cluster) int {
	weight := map[int]int{}
	for _, c := range cs {
		weight[c.box.Max.Y] += c.box.Dx()
	}
	best, bw := 0, -1
	for y, w := range weight {
		if w > bw || (w == bw && y > best) {
			best, bw = y, w
		}
	}
	return best
}
