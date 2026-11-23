package autofit

func (h *glyphHints) segmentRound(minFlags, maxFlags uint16, minOn, maxOn int64) bool {
	return (minFlags|maxFlags)&flagControl != 0 && maxOn-minOn < flatThreshold(h.upem)
}

type segmentSpan struct {
	minPos, maxPos     int64
	minCoord, maxCoord int64
	minFlags, maxFlags uint16
	minOn, maxOn       int64
}

func (h *glyphHints) setSegment(s *segment, sp segmentSpan, last int) {
	s.last = last
	s.pos = short((sp.minPos + sp.maxPos) >> 1)
	s.delta = short((sp.maxPos - sp.minPos) >> 1)
	if h.segmentRound(sp.minFlags, sp.maxFlags, sp.minOn, sp.maxOn) {
		s.flags |= edgeRound
	}
	s.minCoord = short(sp.minCoord)
	s.maxCoord = short(sp.maxCoord)
	s.height = s.maxCoord - s.minCoord
}

func (h *glyphHints) computeSegments() {
	pts := h.points
	majorDir := h.majorDir.abs()
	segmentDir := majorDir
	h.segments = h.segments[:0]
	for i := range pts {
		pts[i].u = pts[i].fy
		pts[i].v = pts[i].fx
	}
	fresh := segmentSpan{minPos: 32000, maxPos: -32000, minCoord: 32000, maxCoord: -32000, minOn: 32000, maxOn: -32000}
	for _, c := range h.contours {
		pt := c
		last := pts[pt].prev
		onEdge := false
		cur := fresh
		prevSeg := none
		prevSpan := fresh
		seg := none
		if pts[pt].flags&flagIgnore == 0 && pts[last].outDir.abs() == majorDir && pts[pt].outDir.abs() == majorDir {
			last = pt
			for {
				pt = pts[pt].prev
				if pts[pt].outDir.abs() != majorDir {
					pt = pts[pt].next
					break
				}
				if pt == last {
					break
				}
			}
		}
		last = pt
		passed := false
		for {
			if onEdge {
				p := &pts[pt]
				cur.minPos, cur.maxPos = min(cur.minPos, p.u), max(cur.maxPos, p.u)
				if p.v < cur.minCoord {
					cur.minCoord, cur.minFlags = p.v, p.flags
				}
				if p.v > cur.maxCoord {
					cur.maxCoord, cur.maxFlags = p.v, p.flags
				}
				if p.flags&flagControl == 0 {
					cur.minOn, cur.maxOn = min(cur.minOn, p.v), max(cur.maxOn, p.v)
				}
				if p.flags&flagIgnore != 0 || p.outDir != segmentDir || pt == last {
					if p.flags&flagIgnore != 0 || prevSeg == none || h.segments[seg].first != h.segments[prevSeg].last {
						h.setSegment(&h.segments[seg], cur, pt)
						prevSeg, prevSpan = seg, cur
					} else {
						h.mergeSegment(seg, prevSeg, &cur, &prevSpan, pt)
						h.segments = h.segments[:len(h.segments)-1]
					}
					onEdge = false
					seg = none
				}
			}
			if pt == last {
				if passed {
					break
				}
				passed = true
			}
			p := &pts[pt]
			if p.flags&flagIgnore == 0 && !onEdge && (p.outDir.abs() == majorDir || pt == p.prev) {
				if len(h.segments) > 1000 {
					h.segments = h.segments[:0]
					return
				}
				segmentDir = p.outDir
				h.segments = append(h.segments, segment{score: 32000, dir: segmentDir, first: pt, last: pt, edge: none, edgeNext: none, link: none, serif: none})
				seg = len(h.segments) - 1
				cur = segmentSpan{minPos: p.u, maxPos: p.u, minCoord: p.v, maxCoord: p.v, minFlags: p.flags, maxFlags: p.flags}
				if p.flags&flagControl != 0 {
					cur.minOn, cur.maxOn = 32000, -32000
				} else {
					cur.minOn, cur.maxOn = p.v, p.v
				}
				onEdge = true
				if pt == p.prev {
					s := &h.segments[seg]
					s.pos = short(cur.minPos)
					if p.flags&flagControl != 0 {
						s.flags |= edgeRound
					}
					s.minCoord, s.maxCoord = short(p.v), short(p.v)
					s.height = 0
					onEdge = false
					seg = none
				}
			}
			pt = pts[pt].next
		}
	}
	for i := range h.segments {
		s := &h.segments[i]
		firstV, lastV := pts[s.first].v, pts[s.last].v
		before, after := pts[pts[s.first].prev].v, pts[pts[s.last].next].v
		if firstV < lastV {
			if before < firstV {
				s.height += short((firstV - before) >> 1)
			}
			if after > lastV {
				s.height += short((after - lastV) >> 1)
			}
		} else {
			if before > firstV {
				s.height += short((before - firstV) >> 1)
			}
			if after < lastV {
				s.height += short((lastV - after) >> 1)
			}
		}
	}
}

func (h *glyphHints) mergeSegment(seg, prevSeg int, cur, prev *segmentSpan, pt int) {
	ps := &h.segments[prevSeg]
	if h.points[ps.last].inDir == h.points[pt].inDir {
		cur.minPos, cur.maxPos = min(cur.minPos, prev.minPos), max(cur.maxPos, prev.maxPos)
		if prev.minCoord < cur.minCoord {
			cur.minCoord, cur.minFlags = prev.minCoord, prev.minFlags
		}
		if prev.maxCoord > cur.maxCoord {
			cur.maxCoord, cur.maxFlags = prev.maxCoord, prev.maxFlags
		}
		cur.minOn, cur.maxOn = min(cur.minOn, prev.minOn), max(cur.maxOn, prev.maxOn)
		ps.last = pt
		ps.pos = short((cur.minPos + cur.maxPos) >> 1)
		ps.delta = short((cur.maxPos - cur.minPos) >> 1)
		if h.segmentRound(cur.minFlags, cur.maxFlags, cur.minOn, cur.maxOn) {
			ps.flags |= edgeRound
		} else {
			ps.flags &^= edgeRound
		}
		ps.minCoord, ps.maxCoord = short(cur.minCoord), short(cur.maxCoord)
		ps.height = ps.maxCoord - ps.minCoord
		return
	}
	if abs(prev.maxCoord-prev.minCoord) > abs(cur.maxCoord-cur.minCoord) {
		prev.minPos, prev.maxPos = min(prev.minPos, cur.minPos), max(prev.maxPos, cur.maxPos)
		ps.last = pt
		ps.pos = short((prev.minPos + prev.maxPos) >> 1)
		ps.delta = short((prev.maxPos - prev.minPos) >> 1)
		return
	}
	cur.minPos, cur.maxPos = min(cur.minPos, prev.minPos), max(cur.maxPos, prev.maxPos)
	s := &h.segments[seg]
	h.setSegment(s, *cur, pt)
	*ps = *s
	*prev = *cur
}

func (h *glyphHints) linkSegments(widths []width) {
	segs := h.segments
	var maxWidth int64
	if len(widths) > 0 {
		maxWidth = widths[len(widths)-1].org
	}
	lenThreshold := max(latinConstant(h.upem, 8), 1)
	lenScore := latinConstant(h.upem, 6000)
	const distScore = 3000
	for i := range segs {
		s1 := &segs[i]
		if s1.dir != h.majorDir {
			continue
		}
		for j := range segs {
			s2 := &segs[j]
			pos1, pos2 := int64(s1.pos), int64(s2.pos)
			if s1.dir+s2.dir != 0 || pos2 <= pos1 {
				continue
			}
			lo := max(int64(s1.minCoord), int64(s2.minCoord))
			hi := min(int64(s1.maxCoord), int64(s2.maxCoord))
			length := hi - lo
			if length < lenThreshold {
				continue
			}
			dist := pos2 - pos1
			var demerit int64
			if maxWidth != 0 {
				delta := (dist<<10)/maxWidth - (1 << 10)
				switch {
				case delta > 10000:
					demerit = 32000
				case delta > 0:
					demerit = delta * delta / distScore
				}
			} else {
				demerit = dist
			}
			score := demerit + lenScore/length
			if score < s1.score {
				s1.score, s1.link = score, j
			}
			if score < s2.score {
				s2.score, s2.link = score, i
			}
		}
	}
	for i := range segs {
		s1 := &segs[i]
		if s1.link == none {
			continue
		}
		if segs[s1.link].link != i {
			s1.serif = segs[s1.link].link
			s1.link = none
		}
	}
}

func (h *glyphHints) insertEdge(fpos int16, dir direction) int {
	h.edges = append(h.edges, edge{})
	k := len(h.edges) - 1
	for k > 0 {
		prev := &h.edges[k-1]
		if prev.fpos < fpos {
			break
		}
		if prev.fpos == fpos && dir == h.majorDir {
			break
		}
		h.edges[k] = *prev
		k--
	}
	return k
}

func (h *glyphHints) computeEdges(edgeDistanceThreshold int64) {
	scale := h.yScale
	h.edges = h.edges[:0]
	segs := h.segments
	segmentWidthThreshold := divFix(32, scale)
	threshold := min(mulFix(edgeDistanceThreshold, scale), 64/4)
	threshold = divFix(threshold, scale)
	attach := func(s int, e *edge) {
		segs[s].edgeNext = e.first
		segs[e.last].edgeNext = s
		e.last = s
	}
	for i := range segs {
		s := &segs[i]
		if s.height < 0 || int64(s.delta) > segmentWidthThreshold || s.dir == dirNone {
			continue
		}
		if s.serif != none && 2*int64(s.height) < 0 {
			continue
		}
		found := -1
		for k := range h.edges {
			if abs(int64(s.pos)-int64(h.edges[k].fpos)) < threshold && h.edges[k].dir == s.dir {
				found = k
				break
			}
		}
		if found >= 0 {
			attach(i, &h.edges[found])
			continue
		}
		k := h.insertEdge(s.pos, s.dir)
		opos := mulFix(int64(s.pos), scale)
		h.edges[k] = edge{first: i, last: i, dir: s.dir, fpos: s.pos, opos: opos, pos: opos, link: none, serif: none}
		s.edgeNext = i
	}
	for i := range segs {
		s := &segs[i]
		if s.dir != dirNone {
			continue
		}
		for k := range h.edges {
			if abs(int64(s.pos)-int64(h.edges[k].fpos)) < threshold {
				attach(i, &h.edges[k])
				break
			}
		}
	}
	for k := range h.edges {
		e := &h.edges[k]
		if e.first == none {
			continue
		}
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
			isSerif := seg.serif != none && segs[seg.serif].edge != none && segs[seg.serif].edge != k
			if (seg.link != none && segs[seg.link].edge != none) || isSerif {
				edge2 := e.link
				seg2 := seg.link
				if isSerif {
					seg2, edge2 = seg.serif, e.serif
				}
				if edge2 != none {
					edgeDelta := abs(int64(e.fpos) - int64(h.edges[edge2].fpos))
					segDelta := abs(int64(seg.pos) - int64(segs[seg2].pos))
					if segDelta < edgeDelta {
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

func (h *glyphHints) computeBlueEdges(axis *scaledAxis, upem int64) {
	for k := range h.edges {
		e := &h.edges[k]
		if e.flags&edgeNoBlue != 0 {
			continue
		}
		var best *width
		bestNeutral := false
		bestDist := min(mulFix(upem/40, axis.scale), 64/2)
		for b := range axis.blues {
			bl := &axis.blues[b]
			if bl.flags&zoneActive == 0 {
				continue
			}
			isTop := bl.flags&(zoneTop|zoneSubTop) != 0
			isNeutral := bl.flags&zoneNeutral != 0
			isMajor := e.dir == h.majorDir
			if isTop == isMajor && !isNeutral {
				continue
			}
			dist := mulFix(abs(int64(e.fpos)-bl.ref.org), axis.scale)
			if dist < bestDist {
				bestDist, best, bestNeutral = dist, &bl.ref, isNeutral
			}
			if e.flags&edgeRound != 0 && dist != 0 && !isNeutral {
				isUnderRef := int64(e.fpos) < bl.ref.org
				if isTop != isUnderRef {
					dist = mulFix(abs(int64(e.fpos)-bl.shoot.org), axis.scale)
					if dist < bestDist {
						bestDist, best, bestNeutral = dist, &bl.shoot, isNeutral
					}
				}
			}
		}
		if best != nil {
			e.blueEdge = best
			if bestNeutral {
				e.flags |= edgeNeutral
			}
		}
	}
}

func (h *glyphHints) alignLinkedEdge(base, stem int) {
	b, s := &h.edges[base], &h.edges[stem]
	s.pos = b.pos + (s.opos - b.opos)
}

func stemCenter(orgCenter, curLen int64) int64 {
	uOff, dOff := int64(38), int64(26)
	if curLen <= 64 {
		uOff, dOff = 32, 32
	}
	cur := pixRound(orgCenter)
	if abs(orgCenter-(cur-uOff)) < abs(orgCenter-(cur+dOff)) {
		return cur - uOff
	}
	return cur + dOff
}

func (h *glyphHints) hintEdges() {
	edges := h.edges
	anchor := none
	hasNonStem := false
	for k := range edges {
		e := &edges[k]
		if e.flags&edgeDone != 0 {
			continue
		}
		e1, e2 := none, e.link
		if e.blueEdge != nil && e2 != none && edges[e2].blueEdge != nil {
			switch {
			case edges[e2].flags&edgeNeutral != 0:
				edges[e2].blueEdge = nil
				edges[e2].flags &^= edgeNeutral
			case e.flags&edgeNeutral != 0:
				e.blueEdge = nil
				e.flags &^= edgeNeutral
			}
		}
		blue := e.blueEdge
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
			hasNonStem = true
			continue
		}
		if edges[e2].blueEdge != nil {
			h.alignLinkedEdge(e2, k)
			e.flags |= edgeDone
			continue
		}
		if anchor == none {
			orgLen := edges[e2].opos - e.opos
			curLen := orgLen
			if curLen < 96 {
				e.pos = stemCenter(e.opos+(orgLen>>1), curLen) - curLen/2
			} else {
				e.pos = pixRound(e.opos)
			}
			anchor = k
			e.flags |= edgeDone
			h.alignLinkedEdge(k, e2)
		} else {
			a := &edges[anchor]
			orgPos := a.pos + (e.opos - a.opos)
			orgLen := edges[e2].opos - e.opos
			orgCenter := orgPos + (orgLen >> 1)
			curLen := orgLen
			switch {
			case edges[e2].flags&edgeDone != 0:
				e.pos = edges[e2].pos - curLen
			case curLen < 96:
				center := stemCenter(orgCenter, curLen)
				e.pos = center - curLen/2
				edges[e2].pos = center + curLen/2
			default:
				cur1 := pixRound(orgPos)
				delta1 := abs(cur1 + (curLen >> 1) - orgCenter)
				cur2 := pixRound(orgPos+orgLen) - curLen
				delta2 := abs(cur2 + (curLen >> 1) - orgCenter)
				if delta1 < delta2 {
					e.pos = cur1
				} else {
					e.pos = cur2
				}
				edges[e2].pos = e.pos + curLen
			}
			e.flags |= edgeDone
			edges[e2].flags |= edgeDone
			if k > 0 && e.pos < edges[k-1].pos && e.link != none && abs(edges[e.link].pos-edges[k-1].pos) > 16 {
				e.pos = edges[k-1].pos
			}
		}
	}
	if !hasNonStem && anchor != none {
		return
	}
	for k := range edges {
		e := &edges[k]
		if e.flags&edgeDone != 0 {
			continue
		}
		delta := int64(1000)
		if e.serif != none {
			delta = edges[e.serif].opos - e.opos
			top, bottom := e.serif, k
			if delta < 0 {
				delta = -delta
				top, bottom = k, e.serif
			}
			if delta < 64+32 && !h.realSerif(top, bottom) {
				continue
			}
		}
		switch {
		case delta < 64+16:
			s := &edges[e.serif]
			e.pos = s.pos + (e.opos - s.opos)
		case anchor == none:
			e.pos = pixRound(e.opos)
			anchor = k
		default:
			before, after := k-1, k+1
			for before >= 0 && edges[before].flags&edgeDone == 0 {
				before--
			}
			for after < len(edges) && edges[after].flags&edgeDone == 0 {
				after++
			}
			if before >= 0 && after < len(edges) {
				b, a := &edges[before], &edges[after]
				if a.opos == b.opos {
					e.pos = b.pos
				} else {
					e.pos = b.pos + mulDiv(e.opos-b.opos, a.pos-b.pos, a.opos-b.opos)
				}
			} else {
				an := &edges[anchor]
				e.pos = an.pos + ((e.opos - an.opos + 16) &^ 31)
			}
		}
		e.flags |= edgeDone
	}
}

func (h *glyphHints) edgeExtent(e int) (lo, hi int64) {
	ed := &h.edges[e]
	vs := [4]int64{
		h.points[h.segments[ed.first].first].v, h.points[h.segments[ed.first].last].v,
		h.points[h.segments[ed.last].first].v, h.points[h.segments[ed.last].last].v,
	}
	lo, hi = vs[0], vs[0]
	for _, v := range vs[1:] {
		lo, hi = min(lo, v), max(hi, v)
	}
	return lo, hi
}

func (h *glyphHints) realSerif(top, bottom int) bool {
	tLo, tHi := h.edgeExtent(top)
	bLo, bHi := h.edgeExtent(bottom)
	minPos, maxPos := min(tLo, bLo), max(tHi, bHi)
	for e := bottom + 1; e < top; e++ {
		eLo, eHi := h.edgeExtent(e)
		if (eLo >= minPos || eHi >= minPos) && (eLo <= maxPos || eHi <= maxPos) {
			return false
		}
	}
	return true
}
