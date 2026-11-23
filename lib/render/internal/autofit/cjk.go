package autofit

import "strings"

const (
	cjkBlueTop   uint16 = 1 << 0
	cjkBlueHoriz uint16 = 1 << 1
)

const (
	cjkZoneActive uint32 = 1 << 0
	cjkZoneTop    uint32 = 1 << 1
)

const (
	dimHorz = 0
	dimVert = 1
)

type cjkBlue struct {
	ref, shoot width
	flags      uint32
}

type cjkAxis struct {
	widths                []width
	edgeDistanceThreshold int64
	blues                 []cjkBlue
}

type cjkMetrics struct {
	upem int64
	axis [2]cjkAxis
}

type scaledCJKAxis struct {
	scale int64
	blues []cjkBlue
}

type scaledCJK struct {
	nominal int64
	axis    [2]scaledCJKAxis
}

func newCJKMetrics(src Source, st style) *cjkMetrics {
	m := &cjkMetrics{upem: src.Upem()}
	m.initWidths(src, scripts[st.script].standard)
	if st.writing == writingCJK {
		m.initBlues(src, blueStringsets[st.blues])
	}
	return m
}

func (m *cjkMetrics) initWidths(src Source, standard string) {
	var gid uint32
	found := false
	clusters(standard, func(r rune, single bool) {
		if found || !single {
			return
		}
		if g, ok := src.Lookup(r); ok && g != 0 {
			gid, found = g, true
		}
	})
	if found {
		m.axis[dimHorz].widths, m.axis[dimVert].widths = cjkStandardWidths(src, gid, m.upem)
	}
	for d := range m.axis {
		a := &m.axis[d]
		stdw := latinConstant(m.upem, 50)
		if len(a.widths) > 0 {
			stdw = a.widths[0].org
		}
		a.edgeDistanceThreshold = stdw / 5
	}
}

func cjkStandardWidths(src Source, gid uint32, upem int64) (horz, vert []width) {
	var o Outline
	if !src.LoadOutline(&o, gid) || len(o.Points) == 0 {
		return nil, nil
	}
	h := &glyphHints{upem: upem, xScale: 0x10000, yScale: 0x10000}
	h.reload(o)
	vert = h.stemWidths(upem)
	h.transpose()
	horz = h.stemWidths(upem)
	return horz, vert
}

func (h *glyphHints) stemWidths(upem int64) []width {
	h.computeSegments()
	h.linkSegments(nil)
	widths := make([]width, 0, maxWidths)
	for i := range h.segments {
		s := &h.segments[i]
		link := s.link
		if link != none && h.segments[link].link == i && link > i && len(widths) < maxWidths {
			widths = append(widths, width{org: abs(int64(s.pos) - int64(h.segments[link].pos))})
		}
	}
	return sortAndQuantizeWidths(widths, upem/100)
}

func cjkClusters(text string, yield func(r rune, single, fill bool)) {
	fill := true
	for field := range strings.FieldsSeq(text) {
		for strings.HasPrefix(field, "|") {
			fill = false
			field = field[1:]
		}
		if field == "" {
			continue
		}
		clusters(field, func(r rune, single bool) { yield(r, single, fill) })
	}
}

func (m *cjkMetrics) initBlues(src Source, set []blueString) {
	var o Outline
	for _, bs := range set {
		horiz := bs.properties&cjkBlueHoriz != 0
		top := bs.properties&cjkBlueTop != 0
		axis := &m.axis[dimVert]
		if horiz {
			axis = &m.axis[dimHorz]
		}
		var fills, flats []int64
		cjkClusters(bs.text, func(r rune, single, fill bool) {
			if !single {
				return
			}
			gid, ok := src.Lookup(r)
			if !ok || gid == 0 || !src.LoadOutline(&o, gid) || len(o.Points) <= 2 {
				return
			}
			best := cjkExtremum(o, horiz, top)
			if fill {
				fills = append(fills, best)
			} else {
				flats = append(flats, best)
			}
		})
		if len(fills) == 0 && len(flats) == 0 {
			continue
		}
		sortPos(fills)
		sortPos(flats)
		var b cjkBlue
		switch {
		case len(flats) == 0:
			b.ref.org, b.shoot.org = fills[len(fills)/2], fills[len(fills)/2]
		case len(fills) == 0:
			b.ref.org, b.shoot.org = flats[len(flats)/2], flats[len(flats)/2]
		default:
			b.ref.org, b.shoot.org = fills[len(fills)/2], flats[len(flats)/2]
		}
		if b.shoot.org != b.ref.org {
			underRef := b.shoot.org < b.ref.org
			if top != underRef {
				mid := (b.shoot.org + b.ref.org) / 2
				b.ref.org, b.shoot.org = mid, mid
			}
		}
		if top {
			b.flags |= cjkZoneTop
		}
		axis.blues = append(axis.blues, b)
	}
}

func cjkExtremum(o Outline, horiz, top bool) int64 {
	bestPoint := -1
	var best int64
	for nn := range o.Ends {
		first, last := contourBounds(o, nn)
		if last <= first {
			continue
		}
		for pp := first; pp <= last; pp++ {
			v := o.Points[pp].Y
			if horiz {
				v = o.Points[pp].X
			}
			if bestPoint < 0 || top && v > best || !top && v < best {
				bestPoint, best = pp, v
			}
		}
	}
	return best
}

func (m *cjkMetrics) scale(nominal int64) *scaledCJK {
	s := &scaledCJK{nominal: nominal}
	for d := range m.axis {
		a := &s.axis[d]
		a.scale = nominal
		a.blues = make([]cjkBlue, len(m.axis[d].blues))
		copy(a.blues, m.axis[d].blues)
		for i := range a.blues {
			b := &a.blues[i]
			b.ref.cur = mulFix(b.ref.org, nominal)
			b.ref.fit = b.ref.cur
			b.shoot.cur = mulFix(b.shoot.org, nominal)
			b.shoot.fit = b.shoot.cur
			b.flags &^= cjkZoneActive
			dist := mulFix(b.ref.org-b.shoot.org, nominal)
			if dist > 48 || dist < -48 {
				continue
			}
			b.ref.fit = pixRound(b.ref.cur)
			delta1 := divFix(b.ref.fit, nominal) - b.shoot.org
			delta2 := mulFix(abs(delta1), nominal)
			if delta2 < 32 {
				delta2 = 0
			} else {
				delta2 = pixRound(delta2)
			}
			if delta1 < 0 {
				delta2 = -delta2
			}
			b.shoot.fit = b.ref.fit - delta2
			b.flags |= cjkZoneActive
		}
	}
	return s
}

func (h *glyphHints) linkSegmentsCJK() {
	segs := h.segments
	lenThreshold := latinConstant(h.upem, 8)
	distThreshold := divFix(64*3, h.yScale)
	for i := range segs {
		s1 := &segs[i]
		if s1.dir != h.majorDir {
			continue
		}
		for j := range segs {
			s2 := &segs[j]
			if j == i || s1.dir+s2.dir != 0 {
				continue
			}
			dist := int64(s2.pos) - int64(s1.pos)
			if dist < 0 {
				continue
			}
			lo := max(int64(s1.minCoord), int64(s2.minCoord))
			hi := min(int64(s1.maxCoord), int64(s2.maxCoord))
			length := hi - lo
			if length < lenThreshold {
				continue
			}
			if dist*8 < s1.score*9 && (dist*8 < s1.score*7 || s1.len < length) {
				s1.score, s1.len, s1.link = dist, length, j
			}
			if dist*8 < s2.score*9 && (dist*8 < s2.score*7 || s2.len < length) {
				s2.score, s2.len, s2.link = dist, length, i
			}
		}
	}
	for i := range segs {
		s1 := &segs[i]
		link1 := s1.link
		if link1 == none || segs[link1].link != i || segs[link1].pos <= s1.pos || s1.score >= distThreshold {
			continue
		}
		for j := range segs {
			s2 := &segs[j]
			if s2.pos > s1.pos || i == j {
				continue
			}
			link2 := s2.link
			if link2 == none || segs[link2].link != j || segs[link2].pos < segs[link1].pos {
				continue
			}
			if s1.pos == s2.pos && segs[link1].pos == segs[link2].pos {
				continue
			}
			if s2.score <= s1.score || s1.score*4 <= s2.score {
				continue
			}
			if s1.len >= s2.len*3 {
				for k := range segs {
					switch segs[k].link {
					case j:
						segs[k].link, segs[k].serif = none, link1
					case link2:
						segs[k].link, segs[k].serif = none, i
					}
				}
				continue
			}
			s1.link, segs[link1].link = none, none
			break
		}
	}
	for i := range segs {
		s1 := &segs[i]
		s2 := s1.link
		if s2 == none || segs[s2].link == i {
			continue
		}
		s1.link = none
		if segs[s2].score < distThreshold || s1.score < segs[s2].score*4 {
			s1.serif = segs[s2].link
		}
	}
}

func segmentDist(a, b *segment) int64 { return abs(int64(a.pos) - int64(b.pos)) }

func (h *glyphHints) computeEdgesCJK(edgeDistanceThreshold int64) {
	scale := h.yScale
	segs := h.segments
	h.edges = h.edges[:0]
	threshold := edgeDistanceThreshold
	if mulFix(edgeDistanceThreshold, scale) > 64/4 {
		threshold = divFix(64/4, scale)
	}
	for i := range segs {
		s := &segs[i]
		found := -1
		best := int64(0xFFFF)
		for k := range h.edges {
			e := &h.edges[k]
			if e.dir != s.dir {
				continue
			}
			dist := abs(int64(s.pos) - int64(e.fpos))
			if dist >= threshold || dist >= best {
				continue
			}
			if s.link != none {
				link := &segs[s.link]
				var dist2 int64
				for s1 := e.first; ; {
					if l1 := segs[s1].link; l1 != none {
						dist2 = segmentDist(link, &segs[l1])
						if dist2 >= threshold {
							break
						}
					}
					s1 = segs[s1].edgeNext
					if s1 == e.first {
						break
					}
				}
				if dist2 >= threshold {
					continue
				}
			}
			best, found = dist, k
		}
		if found >= 0 {
			e := &h.edges[found]
			s.edgeNext = e.first
			segs[e.last].edgeNext = i
			e.last = i
			continue
		}
		k := h.insertEdge(s.pos, s.dir)
		opos := mulFix(int64(s.pos), scale)
		h.edges[k] = edge{first: i, last: i, dir: s.dir, fpos: s.pos, opos: opos, pos: opos, link: none, serif: none}
		s.edgeNext = i
	}
	for k := range h.edges {
		e := &h.edges[k]
		for s := e.first; ; {
			segs[s].edge = k
			s = segs[s].edgeNext
			if s == e.first {
				break
			}
		}
	}
	for k := range h.edges {
		e := &h.edges[k]
		isRound, isStraight := 0, 0
		for s := e.first; ; {
			seg := &segs[s]
			if seg.flags&edgeRound != 0 {
				isRound++
			} else {
				isStraight++
			}
			isSerif := seg.serif != none && segs[seg.serif].edge != k
			if seg.link != none || isSerif {
				edge2, seg2 := e.link, seg.link
				if isSerif {
					seg2, edge2 = seg.serif, e.serif
				}
				if edge2 != none {
					if segmentDist(seg, &segs[seg2]) < abs(int64(e.fpos)-int64(h.edges[edge2].fpos)) {
						edge2 = segs[seg2].edge
					}
				} else {
					edge2 = segs[seg2].edge
				}
				if isSerif {
					e.serif = edge2
					h.edges[edge2].flags |= edgeSerif
				} else {
					e.link = edge2
				}
			}
			s = seg.edgeNext
			if s == e.first {
				break
			}
		}
		e.flags = 0
		if isRound > 0 && isRound >= isStraight {
			e.flags |= edgeRound
		}
		if e.serif != none && e.link != none {
			e.serif = none
		}
	}
}

func (h *glyphHints) computeBlueEdgesCJK(axis *scaledCJKAxis, upem int64) {
	bestDist0 := min(mulFix(upem/40, axis.scale), 64/2)
	for k := range h.edges {
		e := &h.edges[k]
		var best *width
		bestDist := bestDist0
		for b := range axis.blues {
			bl := &axis.blues[b]
			if bl.flags&cjkZoneActive == 0 {
				continue
			}
			isTopRight := bl.flags&cjkZoneTop != 0
			if isTopRight == (e.dir == h.majorDir) {
				continue
			}
			compare := &bl.ref
			if abs(int64(e.fpos)-bl.ref.org) > abs(int64(e.fpos)-bl.shoot.org) {
				compare = &bl.shoot
			}
			if dist := mulFix(abs(int64(e.fpos)-compare.org), axis.scale); dist < bestDist {
				bestDist, best = dist, compare
			}
		}
		if best != nil {
			e.blueEdge = best
		}
	}
}

const (
	lightMaxHorzGap  = 9
	lightMaxVertGap  = 15
	lightMaxDeltaAbs = 14
)

func (h *glyphHints) hintNormalStem(e, e2 *edge, anchor int64) int64 {
	var threshold int64
	switch {
	case e.flags&edgeRound != 0 && e2.flags&edgeRound != 0 && !h.horizontal:
		threshold = 64 - lightMaxHorzGap
	case e.flags&edgeRound != 0 && e2.flags&edgeRound != 0:
		threshold = 64 - lightMaxVertGap
	case !h.horizontal:
		threshold = 64 - lightMaxHorzGap/3
	default:
		threshold = 64 - lightMaxVertGap/3
	}
	curLen := e2.opos - e.opos
	orgCenter := (e.opos+e2.opos)/2 + anchor
	curPos1 := orgCenter - curLen/2
	delta := stemDelta(curPos1, curLen, threshold)
	delta = max(min(delta, lightMaxDeltaAbs), -lightMaxDeltaAbs)
	curPos1 += delta
	if e.opos < e2.opos {
		e.pos, e2.pos = curPos1, curPos1+curLen
	} else {
		e.pos, e2.pos = curPos1+curLen, curPos1
	}
	return delta
}

func stemDelta(curPos1, curLen, threshold int64) int64 {
	curPos2 := curPos1 + curLen
	dOff1 := curPos1 - curPos1&^63
	dOff2 := curPos2 - curPos2&^63
	uOff1, uOff2 := 64-dOff1, 64-dOff2
	if dOff1 == 0 || dOff2 == 0 {
		return 0
	}
	if curLen <= threshold {
		if dOff2 < curLen {
			if uOff1 <= dOff2 {
				return uOff1
			}
			return -dOff2
		}
		return 0
	}
	if threshold < 64 && (dOff1 >= threshold || uOff1 >= threshold || dOff2 >= threshold || uOff2 >= threshold) {
		return 0
	}
	offset := curLen & 63
	if offset < 32 {
		if uOff1 <= offset || dOff2 <= offset {
			return 0
		}
	} else {
		offset = 64 - threshold
	}
	dOff1 = threshold - uOff1
	uOff1 -= offset
	uOff2 = threshold - dOff2
	dOff2 -= offset
	if dOff1 <= uOff1 {
		uOff1 = -dOff1
	}
	if dOff2 <= uOff2 {
		uOff2 = -dOff2
	}
	if abs(uOff1) <= abs(uOff2) {
		return uOff1
	}
	return uOff2
}

func (h *glyphHints) hintEdgesCJK() {
	edges := h.edges
	anchor := none
	var delta int64
	skipped := 0
	hasLastStem := false
	var lastStemPos int64
	for k := range edges {
		e := &edges[k]
		if e.flags&edgeDone != 0 {
			continue
		}
		blue := e.blueEdge
		e1, e2 := none, e.link
		switch {
		case blue != nil:
			e1 = k
		case e2 != none && edges[e2].blueEdge != nil:
			blue = edges[e2].blueEdge
			e1, e2 = e2, k
		}
		if e1 == none {
			continue
		}
		edges[e1].pos = blue.fit
		edges[e1].flags |= edgeDone
		if e2 != none && edges[e2].blueEdge == nil {
			h.alignLinkedEdge(e1, e2)
			edges[e2].flags |= edgeDone
		}
		if anchor == none {
			anchor = k
		}
	}
	for k := range edges {
		e := &edges[k]
		if e.flags&edgeDone != 0 {
			continue
		}
		e2 := e.link
		if e2 == none {
			skipped++
			continue
		}
		if hasLastStem && (e.pos < lastStemPos+64 || edges[e2].pos < lastStemPos+64) {
			skipped++
			continue
		}
		if edges[e2].blueEdge != nil {
			h.alignLinkedEdge(e2, k)
			e.flags |= edgeDone
			continue
		}
		if e2 < k {
			h.alignLinkedEdge(e2, k)
			e.flags |= edgeDone
			hasLastStem, lastStemPos = true, e.pos
			continue
		}
		if h.horizontal && anchor == none {
			delta = h.hintNormalStem(e, &edges[e2], 0)
		} else {
			h.hintNormalStem(e, &edges[e2], delta)
		}
		anchor = k
		e.flags |= edgeDone
		edges[e2].flags |= edgeDone
		hasLastStem, lastStemPos = true, edges[e2].pos
	}
	if n := len(edges); h.horizontal && (n == 6 || n == 12) {
		e1, e2, e3 := 0, 2, 4
		if n == 12 {
			e1, e2, e3 = 1, 5, 9
		}
		span := abs((edges[e2].opos - edges[e1].opos) - (edges[e3].opos - edges[e2].opos))
		if edges[e1].link == e1+1 && edges[e2].link == e2+1 && edges[e3].link == e3+1 && span < 8 {
			d := edges[e3].pos - (2*edges[e2].pos - edges[e1].pos)
			edges[e3].pos -= d
			if l := edges[e3].link; l != none {
				edges[l].pos -= d
			}
			if n == 12 {
				edges[8].pos -= d
				edges[11].pos -= d
			}
			edges[e3].flags |= edgeDone
			if l := edges[e3].link; l != none {
				edges[l].flags |= edgeDone
			}
		}
	}
	if skipped == 0 {
		return
	}
	for k := range edges {
		e := &edges[k]
		if e.flags&edgeDone != 0 || e.serif == none {
			continue
		}
		s := &edges[e.serif]
		e.pos = s.pos + (e.opos - s.opos)
		e.flags |= edgeDone
		skipped--
	}
	if skipped == 0 {
		return
	}
	for k := range edges {
		e := &edges[k]
		if e.flags&edgeDone != 0 {
			continue
		}
		before, after := k-1, k+1
		for before >= 0 && edges[before].flags&edgeDone == 0 {
			before--
		}
		for after < len(edges) && edges[after].flags&edgeDone == 0 {
			after++
		}
		switch {
		case before < 0 && after >= len(edges):
		case before < 0:
			a := &edges[after]
			e.pos = a.pos + (e.opos - a.opos)
		case after >= len(edges):
			b := &edges[before]
			e.pos = b.pos + (e.opos - b.opos)
		default:
			b, a := &edges[before], &edges[after]
			if a.fpos == b.fpos {
				e.pos = b.pos
			} else {
				e.pos = b.pos + mulDiv(int64(e.fpos)-int64(b.fpos), a.pos-b.pos, int64(a.fpos)-int64(b.fpos))
			}
		}
	}
}

func (h *glyphHints) alignEdgePointsCJK() {
	for k := range h.edges {
		e := &h.edges[k]
		delta := e.pos - e.opos
		for s := e.first; ; {
			seg := &h.segments[s]
			for p := seg.first; ; p = h.points[p].next {
				h.points[p].y += delta
				h.points[p].flags |= h.touch
				if p == seg.last {
					break
				}
			}
			s = seg.edgeNext
			if s == e.first {
				break
			}
		}
	}
}

func (h *glyphHints) hintAxisCJK(m *cjkMetrics, axis *scaledCJKAxis, d int) {
	h.computeSegments()
	h.linkSegmentsCJK()
	h.computeEdgesCJK(m.axis[d].edgeDistanceThreshold)
	h.computeBlueEdgesCJK(axis, m.upem)
	h.hintEdgesCJK()
	h.alignEdgePointsCJK()
	h.alignStrongPoints()
	h.alignWeakPoints()
}
