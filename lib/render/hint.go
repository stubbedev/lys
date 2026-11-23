package render

import (
	"cmp"
	"math"
	"slices"

	"github.com/stubbedev/lys/lib/third_party/typesetting/font"
	ot "github.com/stubbedev/lys/lib/third_party/typesetting/font/opentype"
)

const (
	edgeFlatnessPerEm  = 1.0 / 256
	edgeMinLengthPerEm = 1.0 / 64
	stemMaxWidthPerEm  = 0.3
	zoneFuzzPerEm      = 1.0 / 64
)

type blueZone struct {
	ref, over float64
	top       bool
}

var blueZoneRunes = [...]struct {
	flat, round rune
	top         bool
}{
	{flat: 'H', round: 'o', top: false},
	{flat: 'x', round: 'o', top: true},
	{flat: 'H', round: 'O', top: true},
	{flat: 'd', top: true},
	{flat: 'p', top: false},
}

func (t *Typeface) blueZones() []blueZone {
	if !t.bluesRead {
		t.blues, t.bluesRead = measureBlueZones(t.face), true
	}
	return t.blues
}

func measureBlueZones(f *font.Face) []blueZone {
	zones := make([]blueZone, 0, len(blueZoneRunes))
	for _, z := range blueZoneRunes {
		lo, hi, ok := runeExtent(f, z.flat)
		if !ok {
			continue
		}
		zone := blueZone{ref: lo, over: lo, top: z.top}
		if z.top {
			zone.ref, zone.over = hi, hi
		}
		if rlo, rhi, ok := runeExtent(f, z.round); ok {
			if z.top && rhi > hi {
				zone.over = rhi
			}
			if !z.top && rlo < lo {
				zone.over = rlo
			}
		}
		zones = append(zones, zone)
	}
	return zones
}

func runeExtent(f *font.Face, r rune) (lo, hi float64, ok bool) {
	if r == 0 {
		return 0, 0, false
	}
	gid, ok := f.NominalGlyph(r)
	if !ok {
		return 0, 0, false
	}
	outline, ok := f.GlyphDataOutline(gid)
	if !ok || len(outline.Segments) == 0 {
		return 0, 0, false
	}
	lo, hi = math.Inf(1), math.Inf(-1)
	for i := range outline.Segments {
		for _, p := range outline.Segments[i].ArgsSlice() {
			lo, hi = math.Min(lo, float64(p.Y)), math.Max(hi, float64(p.Y))
		}
	}
	return lo, hi, true
}

func (z blueZone) contains(e hintEdge, fuzz float64) bool {
	if z.top == e.inkAfter {
		return false
	}
	return e.at >= math.Min(z.ref, z.over)-fuzz && e.at <= math.Max(z.ref, z.over)+fuzz
}

func (z blueZone) row(e hintEdge, scale, fy float64) float64 {
	ref := math.Round(fy - float64(scale*z.ref))
	if math.Abs(e.at-z.over) >= math.Abs(e.at-z.ref) {
		return ref
	}
	overshoot := float64(scale * math.Abs(z.over-z.ref))
	if overshoot < 1 {
		return ref
	}
	if z.top {
		return ref - math.Round(overshoot)
	}
	return ref + math.Round(overshoot)
}

type hintEdge struct {
	at       float64
	lo, hi   float64
	inkAfter bool
}

func outlineEdges(outline font.GlyphOutline, upem float64) []hintEdge {
	points := make([]ot.SegmentPoint, 0, 3*len(outline.Segments))
	var starts []int
	for i := range outline.Segments {
		seg := &outline.Segments[i]
		if seg.Op == ot.SegmentOpMoveTo {
			starts = append(starts, len(points))
		}
		points = append(points, seg.ArgsSlice()...)
	}
	contours := make([][]ot.SegmentPoint, 0, len(starts))
	area := 0.0
	for i, s := range starts {
		end := len(points)
		if i+1 < len(starts) {
			end = starts[i+1]
		}
		c := points[s:end]
		if len(c) > 1 && c[0] == c[len(c)-1] {
			c = c[:len(c)-1]
		}
		contours = append(contours, c)
		area += contourArea(c)
	}
	flat, minLength := float64(upem*edgeFlatnessPerEm), float64(upem*edgeMinLengthPerEm)
	edges := make([]hintEdge, 0, len(outline.Segments)/2)
	for _, c := range contours {
		edges = appendContourEdges(edges, c, flat, minLength, area > 0)
	}
	return edges
}

func contourArea(c []ot.SegmentPoint) float64 {
	sum := 0.0
	for i, p := range c {
		q := c[(i+1)%len(c)]
		sum += float64(float64(p.X)*float64(q.Y)) - float64(float64(q.X)*float64(p.Y))
	}
	return sum
}

func appendContourEdges(edges []hintEdge, c []ot.SegmentPoint, flat, minLength float64, counterClockwise bool) []hintEdge {
	n := len(c)
	at := func(i int) ot.SegmentPoint { return c[i%n] }
	start := -1
	for i := range n {
		if math.Abs(float64(c[i].Y)-float64(at(i+n-1).Y)) > flat {
			start = i
			break
		}
	}
	if start < 0 {
		return edges
	}
	for i := 0; i < n; {
		first := at(start + i)
		xlo, xhi := float64(first.X), float64(first.X)
		j := i + 1
		for ; j < n; j++ {
			p := at(start + j)
			if math.Abs(float64(p.Y)-float64(first.Y)) > flat {
				break
			}
			xlo, xhi = math.Min(xlo, float64(p.X)), math.Max(xhi, float64(p.X))
		}
		last := at(start + j - 1)
		if j-i >= 2 && xhi-xlo >= minLength && last.X != first.X {
			edges = append(edges, hintEdge{
				at:       float64(first.Y),
				lo:       xlo,
				hi:       xhi,
				inkAfter: (last.X > first.X) == counterClockwise,
			})
		}
		i = j
	}
	return edges
}

type gridAnchor struct {
	at, row float64
}

type axisHint struct {
	origin, scale float64
	anchors       []gridAnchor
}

func hintVertical(edges []hintEdge, zones []blueZone, scale, fy, upem float64) axisHint {
	device := func(e hintEdge) float64 { return fy - float64(scale*e.at) }
	h := axisHint{origin: fy, scale: -scale, anchors: make([]gridAnchor, 0, len(edges))}
	zoned := make([]bool, len(edges))
	rows := make([]float64, len(edges))
	fuzz := float64(upem * zoneFuzzPerEm)
	for i, e := range edges {
		best := -1
		for k, z := range zones {
			if z.contains(e, fuzz) && (best < 0 || math.Abs(e.at-z.ref) < math.Abs(e.at-zones[best].ref)) {
				best = k
			}
		}
		if best >= 0 {
			rows[i], zoned[i] = zones[best].row(e, scale, fy), true
			h = h.with(e.at, rows[i])
		}
	}
	stemmed := make([]bool, len(edges))
	for _, s := range pairStems(edges, float64(upem*stemMaxWidthPerEm)) {
		b, t := edges[s.lo], edges[s.hi]
		width := math.Max(1, math.Round(float64(scale*(t.at-b.at))))
		switch {
		case zoned[s.lo] && zoned[s.hi]:
		case zoned[s.lo]:
			h = h.with(t.at, rows[s.lo]-width)
		case zoned[s.hi]:
			h = h.with(b.at, rows[s.hi]+width)
		default:
			top := math.Round(float64((device(b)+device(t))/2) - float64(width/2))
			h = h.with(t.at, top).with(b.at, top+width)
		}
		stemmed[s.lo], stemmed[s.hi] = true, true
	}
	for i, e := range edges {
		if !zoned[i] && !stemmed[i] {
			h = h.with(e.at, math.Round(device(e)))
		}
	}
	return h
}

func (h axisHint) with(at, row float64) axisHint {
	k, found := slices.BinarySearchFunc(h.anchors, at, compareAnchor)
	if found || (k > 0 && !h.ordered(h.anchors[k-1].row, row)) || (k < len(h.anchors) && !h.ordered(row, h.anchors[k].row)) {
		return h
	}
	h.anchors = slices.Insert(h.anchors, k, gridAnchor{at: at, row: row})
	return h
}

func (h axisHint) ordered(a, b float64) bool {
	if h.scale < 0 {
		return a >= b
	}
	return a <= b
}

func compareAnchor(a gridAnchor, at float64) int { return cmp.Compare(a.at, at) }

func (h axisHint) device(at float64) float64 {
	n := len(h.anchors)
	if n == 0 {
		return h.origin + float64(h.scale*at)
	}
	k, found := slices.BinarySearchFunc(h.anchors, at, compareAnchor)
	switch {
	case found:
		return h.anchors[k].row
	case k == 0:
		return h.anchors[0].row + float64(h.scale*(at-h.anchors[0].at))
	case k == n:
		return h.anchors[n-1].row + float64(h.scale*(at-h.anchors[n-1].at))
	}
	lo, hi := h.anchors[k-1], h.anchors[k]
	return lo.row + float64((hi.row-lo.row)*(at-lo.at)/(hi.at-lo.at))
}

type stem struct {
	lo, hi int
}

func pairStems(edges []hintEdge, maxWidth float64) []stem {
	nearest := func(i int) int {
		e, best := edges[i], -1
		for j, o := range edges {
			if o.inkAfter == e.inkAfter || math.Min(e.hi, o.hi) <= math.Max(e.lo, o.lo) {
				continue
			}
			gap := o.at - e.at
			if !e.inkAfter {
				gap = -gap
			}
			if gap <= 0 || gap > maxWidth {
				continue
			}
			if best < 0 || gap < math.Abs(edges[best].at-e.at) {
				best = j
			}
		}
		return best
	}
	var stems []stem
	for i, e := range edges {
		if !e.inkAfter {
			continue
		}
		if j := nearest(i); j >= 0 && nearest(j) == i {
			stems = append(stems, stem{lo: i, hi: j})
		}
	}
	return stems
}
