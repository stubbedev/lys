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
	if z.top == e.inkAbove {
		return false
	}
	return e.y >= math.Min(z.ref, z.over)-fuzz && e.y <= math.Max(z.ref, z.over)+fuzz
}

func (z blueZone) row(e hintEdge, scale, fy float64) float64 {
	ref := math.Round(fy - float64(scale*z.ref))
	if math.Abs(e.y-z.over) >= math.Abs(e.y-z.ref) {
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
	y        float64
	xlo, xhi float64
	inkAbove bool
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
				y:        float64(first.Y),
				xlo:      xlo,
				xhi:      xhi,
				inkAbove: (last.X > first.X) == counterClockwise,
			})
		}
		i = j
	}
	return edges
}

type yAnchor struct {
	y, row float64
}

type verticalHint []yAnchor

func hintVertical(edges []hintEdge, zones []blueZone, scale, fy, upem float64) verticalHint {
	device := func(e hintEdge) float64 { return fy - float64(scale*e.y) }
	h := make(verticalHint, 0, len(edges))
	zoned := make([]bool, len(edges))
	rows := make([]float64, len(edges))
	fuzz := float64(upem * zoneFuzzPerEm)
	for i, e := range edges {
		best := -1
		for k, z := range zones {
			if z.contains(e, fuzz) && (best < 0 || math.Abs(e.y-z.ref) < math.Abs(e.y-zones[best].ref)) {
				best = k
			}
		}
		if best >= 0 {
			rows[i], zoned[i] = zones[best].row(e, scale, fy), true
			h = h.with(e.y, rows[i])
		}
	}
	stemmed := make([]bool, len(edges))
	for _, s := range pairStems(edges, float64(upem*stemMaxWidthPerEm)) {
		b, t := edges[s.bottom], edges[s.top]
		width := math.Max(1, math.Round(float64(scale*(t.y-b.y))))
		switch {
		case zoned[s.bottom] && zoned[s.top]:
		case zoned[s.bottom]:
			h = h.with(t.y, rows[s.bottom]-width)
		case zoned[s.top]:
			h = h.with(b.y, rows[s.top]+width)
		default:
			top := math.Round(float64((device(b)+device(t))/2) - float64(width/2))
			h = h.with(t.y, top).with(b.y, top+width)
		}
		stemmed[s.bottom], stemmed[s.top] = true, true
	}
	for i, e := range edges {
		if !zoned[i] && !stemmed[i] {
			h = h.with(e.y, math.Round(device(e)))
		}
	}
	return h
}

func (h verticalHint) with(y, row float64) verticalHint {
	k, found := slices.BinarySearchFunc(h, y, compareAnchorY)
	if found || (k > 0 && h[k-1].row < row) || (k < len(h) && h[k].row > row) {
		return h
	}
	return slices.Insert(h, k, yAnchor{y: y, row: row})
}

func compareAnchorY(a yAnchor, y float64) int { return cmp.Compare(a.y, y) }

func (h verticalHint) device(y, scale, fy float64) float64 {
	if len(h) == 0 {
		return fy - float64(scale*y)
	}
	k, found := slices.BinarySearchFunc(h, y, compareAnchorY)
	switch {
	case found:
		return h[k].row
	case k == 0:
		return h[0].row + float64(scale*(h[0].y-y))
	case k == len(h):
		return h[k-1].row - float64(scale*(y-h[k-1].y))
	}
	lo, hi := h[k-1], h[k]
	return lo.row + float64((hi.row-lo.row)*(y-lo.y)/(hi.y-lo.y))
}

type stem struct {
	bottom, top int
}

func pairStems(edges []hintEdge, maxWidth float64) []stem {
	nearest := func(i int) int {
		e, best := edges[i], -1
		for j, o := range edges {
			if o.inkAbove == e.inkAbove || math.Min(e.xhi, o.xhi) <= math.Max(e.xlo, o.xlo) {
				continue
			}
			gap := o.y - e.y
			if !e.inkAbove {
				gap = -gap
			}
			if gap <= 0 || gap > maxWidth {
				continue
			}
			if best < 0 || gap < math.Abs(edges[best].y-e.y) {
				best = j
			}
		}
		return best
	}
	var stems []stem
	for i, e := range edges {
		if !e.inkAbove {
			continue
		}
		if j := nearest(i); j >= 0 && nearest(j) == i {
			stems = append(stems, stem{bottom: i, top: j})
		}
	}
	return stems
}
