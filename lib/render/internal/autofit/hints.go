package autofit

// Tag says whether an outline point lies on the curve or is a quadratic
// or cubic control point.
type Tag uint8

// The point tags.
const (
	On Tag = iota
	Conic
	Cubic
)

// Point is an outline point in font units.
type Point struct{ X, Y int64 }

// Outline is a glyph as FreeType loads it unscaled: its points, their
// tags, and the index of each contour's last point.
type Outline struct {
	Points []Point
	Tags   []Tag
	Ends   []int
}

type direction int8

const (
	dirNone  direction = 4
	dirRight direction = 1
	dirLeft  direction = -1
	dirUp    direction = 2
	dirDown  direction = -2
)

func (d direction) abs() direction {
	if d < 0 {
		return -d
	}
	return d
}

const (
	flagConic             uint16 = 1 << 0
	flagCubic             uint16 = 1 << 1
	flagControl                  = flagConic | flagCubic
	flagTouchY            uint16 = 1 << 3
	flagWeakInterpolation uint16 = 1 << 4
	flagNear              uint16 = 1 << 5
	flagIgnore            uint16 = 1 << 6
)

const (
	edgeRound   uint8 = 1 << 0
	edgeSerif   uint8 = 1 << 1
	edgeDone    uint8 = 1 << 2
	edgeNeutral uint8 = 1 << 3
	edgeNoBlue  uint8 = 1 << 4
)

const none = -1

type point struct {
	flags         uint16
	inDir, outDir direction
	ox, oy        int64
	fx, fy        int64
	x, y          int64
	u, v          int64
	next, prev    int
}

type segment struct {
	flags              uint8
	dir                direction
	pos, delta         int16
	minCoord, maxCoord int16
	height             int16
	edge               int
	edgeNext           int
	link, serif        int
	score              int64
	first, last        int
}

type edge struct {
	fpos      int16
	opos, pos int64
	flags     uint8
	dir       direction
	scale     int64
	blueEdge  *width
	link      int
	serif     int
	first     int
	last      int
}

type glyphHints struct {
	upem       int64
	yScale     int64
	xScale     int64
	points     []point
	contours   []int
	yMin, yMax []int64
	segments   []segment
	edges      []edge
	majorDir   direction
}

func direction2(dx, dy int64) direction {
	var dir direction
	var ll, ss int64
	switch {
	case dy >= dx && dy >= -dx:
		dir, ll, ss = dirUp, dy, dx
	case dy >= dx:
		dir, ll, ss = dirLeft, -dx, dy
	case dy >= -dx:
		dir, ll, ss = dirRight, dx, dy
	default:
		dir, ll, ss = dirDown, -dy, dx
	}
	if ll <= 14*abs(ss) {
		return dirNone
	}
	return dir
}

func orientationIsPostScript(o Outline) bool {
	if len(o.Points) == 0 {
		return false
	}
	xMin, xMax, yMin, yMax := o.Points[0].X, o.Points[0].X, o.Points[0].Y, o.Points[0].Y
	for _, p := range o.Points {
		xMin, xMax = min(xMin, p.X), max(xMax, p.X)
		yMin, yMax = min(yMin, p.Y), max(yMax, p.Y)
	}
	if xMin == xMax || yMin == yMax {
		return false
	}
	if xMin < -0x1000000 || yMin < -0x1000000 || xMax > 0x1000000 || yMax > 0x1000000 {
		return false
	}
	xShift := max(msb(uint32(abs(xMax)|abs(xMin)))-14, 0) //nolint:gosec // bounded by the 0x1000000 check above
	yShift := max(msb(uint32(yMax-yMin))-14, 0)           //nolint:gosec // bounded by the 0x1000000 check above
	var area int64
	first := 0
	for _, last := range o.Ends {
		prevX, prevY := o.Points[last].X>>xShift, o.Points[last].Y>>yShift
		for n := first; n <= last; n++ {
			curX, curY := o.Points[n].X>>xShift, o.Points[n].Y>>yShift
			area += (curY - prevY) * (curX + prevX)
			prevX, prevY = curX, curY
		}
		first = last + 1
	}
	return area > 0
}

func (h *glyphHints) reload(o Outline) {
	n := len(o.Points)
	h.points = h.points[:0]
	h.contours = h.contours[:0]
	h.segments = h.segments[:0]
	h.edges = h.edges[:0]
	h.majorDir = dirLeft
	if orientationIsPostScript(o) {
		h.majorDir = dirRight
	}
	if n == 0 {
		return
	}
	h.points = append(h.points, make([]point, n)...)
	points := h.points
	nearLimit := 20 * h.upem / 2048

	contourIndex := 0
	end := o.Ends[0]
	prev := end
	for i := range n {
		p := &points[i]
		p.inDir, p.outDir = dirNone, dirNone
		p.fx, p.fy = int64(short(o.Points[i].X)), int64(short(o.Points[i].Y))
		p.ox = mulFix(o.Points[i].X, h.xScale)
		p.x = p.ox
		p.oy = mulFix(o.Points[i].Y, h.yScale)
		p.y = p.oy
		points[end].fx, points[end].fy = int64(short(o.Points[end].X)), int64(short(o.Points[end].Y))
		switch o.Tags[i] {
		case Conic:
			p.flags = flagConic
		case Cubic:
			p.flags = flagCubic
		default:
			p.flags = 0
		}
		outX, outY := p.fx-points[prev].fx, p.fy-points[prev].fy
		if abs(outX)+abs(outY) < nearLimit {
			points[prev].flags |= flagNear
		}
		p.prev = prev
		points[prev].next = i
		prev = i
		if i == end {
			contourIndex++
			if contourIndex < len(o.Ends) {
				end = o.Ends[contourIndex]
				prev = end
			}
		}
	}

	first := 0
	for _, e := range o.Ends {
		h.contours = append(h.contours, first)
		first = e + 1
	}

	nearLimit2 := 2*nearLimit - 1
	for _, c := range h.contours {
		first := c
		pt := first
		pv := points[first].prev
		for pv != first {
			outX, outY := points[pt].fx-points[pv].fx, points[pt].fy-points[pv].fy
			if abs(outX)+abs(outY) >= nearLimit2 {
				break
			}
			pt = pv
			pv = points[pv].prev
		}
		first = pt
		curr := first
		points[curr].u = int64(first - curr)
		points[first].v = -points[curr].u
		var outX, outY int64
		next := first
		for {
			pt = next
			next = points[pt].next
			outX += points[next].fx - points[pt].fx
			outY += points[next].fy - points[pt].fy
			if abs(outX)+abs(outY) < nearLimit {
				points[next].flags |= flagWeakInterpolation
				if next == first {
					break
				}
				continue
			}
			points[curr].u = int64(next - curr)
			points[next].v = -points[curr].u
			outDir := direction2(outX, outY)
			points[curr].outDir = outDir
			for curr = points[curr].next; curr != next; curr = points[curr].next {
				points[curr].inDir = outDir
				points[curr].outDir = outDir
			}
			points[next].inDir = outDir
			points[curr].u = int64(first - curr)
			points[first].v = -points[curr].u
			outX, outY = 0, 0
			if next == first {
				break
			}
		}
	}

	for i := range points {
		p := &points[i]
		if p.flags&flagWeakInterpolation != 0 {
			continue
		}
		if p.inDir == dirNone && p.outDir == dirNone {
			nextU, prevV := i+int(p.u), i+int(p.v)
			inX, inY := p.fx-points[prevV].fx, p.fy-points[prevV].fy
			outX, outY := points[nextU].fx-p.fx, points[nextU].fy-p.fy
			if (inX^outX) >= 0 && (inY^outY) >= 0 {
				p.flags |= flagWeakInterpolation
				points[prevV].u = int64(nextU - prevV)
				points[nextU].v = -points[prevV].u
			}
		}
	}

	for i := range points {
		p := &points[i]
		if p.flags&flagWeakInterpolation != 0 {
			continue
		}
		switch {
		case p.flags&flagControl != 0:
			p.flags |= flagWeakInterpolation
		case p.outDir == p.inDir:
			if p.outDir != dirNone {
				p.flags |= flagWeakInterpolation
				continue
			}
			nextU, prevV := i+int(p.u), i+int(p.v)
			if cornerIsFlat(p.fx-points[prevV].fx, p.fy-points[prevV].fy, points[nextU].fx-p.fx, points[nextU].fy-p.fy) {
				points[prevV].u = int64(nextU - prevV)
				points[nextU].v = -points[prevV].u
				p.flags |= flagWeakInterpolation
			}
		case p.inDir == -p.outDir:
			p.flags |= flagWeakInterpolation
		}
	}
}

func (h *glyphHints) alignEdgePoints() {
	for i := range h.segments {
		s := &h.segments[i]
		if s.edge == none {
			continue
		}
		pos := h.edges[s.edge].pos
		for p := s.first; ; p = h.points[p].next {
			h.points[p].y = pos
			h.points[p].flags |= flagTouchY
			if p == s.last {
				break
			}
		}
	}
}

func (h *glyphHints) alignStrongPoints() {
	edges := h.edges
	if len(edges) == 0 {
		return
	}
	for i := range h.points {
		p := &h.points[i]
		if p.flags&flagTouchY != 0 || p.flags&flagWeakInterpolation != 0 {
			continue
		}
		u, ou := p.fy, p.oy
		fu := u
		p.y = h.strongPosition(u, ou, fu)
		p.flags |= flagTouchY
	}
}

func (h *glyphHints) strongPosition(u, ou, fu int64) int64 {
	edges := h.edges
	first := &edges[0]
	if int64(first.fpos)-u >= 0 {
		return first.pos - (first.opos - ou)
	}
	last := &edges[len(edges)-1]
	if u-int64(last.fpos) >= 0 {
		return last.pos + (ou - last.opos)
	}
	lo, hi := 0, len(edges)
	if hi <= 8 {
		nn := 0
		for ; nn < hi; nn++ {
			if int64(edges[nn].fpos) >= u {
				break
			}
		}
		if int64(edges[nn].fpos) == u {
			return edges[nn].pos
		}
		lo = nn
	} else {
		for lo < hi {
			mid := (hi + lo) >> 1
			fpos := int64(edges[mid].fpos)
			switch {
			case u < fpos:
				hi = mid
			case u > fpos:
				lo = mid + 1
			default:
				return edges[mid].pos
			}
		}
	}
	before, after := &edges[lo-1], &edges[lo]
	if before.scale == 0 {
		before.scale = divFix(after.pos-before.pos, int64(after.fpos)-int64(before.fpos))
	}
	return before.pos + mulFix(fu-int64(before.fpos), before.scale)
}

func (h *glyphHints) iupShift(p1, p2, ref int) {
	delta := h.points[ref].u - h.points[ref].v
	if delta == 0 {
		return
	}
	for p := p1; p < ref; p++ {
		h.points[p].u = h.points[p].v + delta
	}
	for p := ref + 1; p <= p2; p++ {
		h.points[p].u = h.points[p].v + delta
	}
}

func (h *glyphHints) iupInterp(p1, p2, ref1, ref2 int) {
	if p1 > p2 {
		return
	}
	pts := h.points
	if pts[ref1].v > pts[ref2].v {
		ref1, ref2 = ref2, ref1
	}
	v1, v2 := pts[ref1].v, pts[ref2].v
	u1, u2 := pts[ref1].u, pts[ref2].u
	d1, d2 := u1-v1, u2-v2
	if u1 == u2 || v1 == v2 {
		for p := p1; p <= p2; p++ {
			u := pts[p].v
			switch {
			case u <= v1:
				u += d1
			case u >= v2:
				u += d2
			default:
				u = u1
			}
			pts[p].u = u
		}
		return
	}
	scale := divFix(u2-u1, v2-v1)
	for p := p1; p <= p2; p++ {
		u := pts[p].v
		switch {
		case u <= v1:
			u += d1
		case u >= v2:
			u += d2
		default:
			u = u1 + mulFix(u-v1, scale)
		}
		pts[p].u = u
	}
}

func (h *glyphHints) alignWeakPoints() {
	pts := h.points
	for i := range pts {
		pts[i].u = pts[i].y
		pts[i].v = pts[i].oy
	}
	for _, c := range h.contours {
		p := c
		endPoint := pts[p].prev
		firstPoint := p
		for p <= endPoint && pts[p].flags&flagTouchY == 0 {
			p++
		}
		if p > endPoint {
			continue
		}
		firstTouched := p
		lastTouched := p
		for {
			for p < endPoint && pts[p+1].flags&flagTouchY != 0 {
				p++
			}
			lastTouched = p
			p++
			for p <= endPoint && pts[p].flags&flagTouchY == 0 {
				p++
			}
			if p > endPoint {
				break
			}
			h.iupInterp(lastTouched+1, p-1, lastTouched, p)
		}
		if lastTouched == firstTouched {
			h.iupShift(firstPoint, endPoint, firstTouched)
			continue
		}
		if lastTouched < endPoint {
			h.iupInterp(lastTouched+1, endPoint, lastTouched, firstTouched)
		}
		if firstTouched > 0 {
			h.iupInterp(firstPoint, firstTouched-1, lastTouched, firstTouched)
		}
	}
	for i := range pts {
		pts[i].y = pts[i].u
	}
}
