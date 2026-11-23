package autofit

import (
	"math"
	"strings"
	"unicode/utf8"
)

const maxWidths = 16

const (
	blueTop          uint16 = 1 << 0
	blueSubTop       uint16 = 1 << 1
	blueNeutral      uint16 = 1 << 2
	blueXHeight      uint16 = 1 << 3
	blueLong         uint16 = 1 << 4
	blueCapitalBotom uint16 = 1 << 5
	blueSmallBottom  uint16 = 1 << 6
)

const (
	zoneActive      uint32 = 1 << 0
	zoneTop         uint32 = 1 << 1
	zoneSubTop      uint32 = 1 << 2
	zoneNeutral     uint32 = 1 << 3
	zoneAdjustment  uint32 = 1 << 4
	zoneBottom      uint32 = 1 << 5
	zoneBottomSmall uint32 = 1 << 6
)

type width struct{ org, cur, fit int64 }

type blue struct {
	ref, shoot          width
	ascender, descender int64
	flags               uint32
}

type latinMetrics struct {
	upem                  int64
	widths                []width
	edgeDistanceThreshold int64
	blues                 []blue
}

func latinConstant(upem, c int64) int64 { return c * upem / 2048 }

func flatThreshold(upem int64) int64 { return upem / 14 }

func clusters(text string, yield func(r rune, single bool)) {
	for field := range strings.FieldsSeq(text) {
		r, size := utf8.DecodeRuneInString(field)
		yield(r, size == len(field))
	}
}

func newLatinMetrics(src Source, st style) (*latinMetrics, bool) {
	m := &latinMetrics{upem: src.Upem()}
	m.initWidths(src, scripts[st.script].standard)
	if !m.initBlues(src, blueStringsets[st.blues]) {
		return nil, false
	}
	return m, true
}

func (m *latinMetrics) initWidths(src Source, standard string) {
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
		m.widths = standardWidths(src, gid, m.upem)
	}
	stdw := latinConstant(m.upem, 50)
	if len(m.widths) > 0 {
		stdw = m.widths[0].org
	}
	m.edgeDistanceThreshold = stdw / 5
}

func standardWidths(src Source, gid uint32, upem int64) []width {
	var o Outline
	if !src.LoadOutline(&o, gid) || len(o.Points) == 0 {
		return nil
	}
	h := &glyphHints{upem: upem, xScale: 0x10000, yScale: 0x10000}
	h.reload(o)
	h.computeSegments()
	h.linkSegments(nil)
	widths := make([]width, 0, maxWidths)
	for i := range h.segments {
		s := &h.segments[i]
		link := s.link
		if link != none && h.segments[link].link == i && link > i {
			if len(widths) < maxWidths {
				widths = append(widths, width{org: abs(int64(s.pos) - int64(h.segments[link].pos))})
			}
		}
	}
	return sortAndQuantizeWidths(widths, upem/100)
}

func sortAndQuantizeWidths(table []width, threshold int64) []width {
	count := len(table)
	if count == 0 {
		return []width{{}}
	}
	if count == 1 {
		return table
	}
	for i := 1; i < count; i++ {
		for j := i; j > 0 && table[j].org < table[j-1].org; j-- {
			table[j], table[j-1] = table[j-1], table[j]
		}
	}
	curIdx := 0
	curVal := table[curIdx].org
	for i := 1; i < count; i++ {
		if table[i].org-curVal > threshold || i == count-1 {
			var sum int64
			if table[i].org-curVal <= threshold && i == count-1 {
				i++
			}
			j := curIdx
			for ; j < i; j++ {
				sum += table[j].org
				table[j].org = 0
			}
			table[curIdx].org = sum / int64(j)
			if i < count-1 {
				curIdx = i + 1
				curVal = table[curIdx].org
			}
		}
	}
	out := table[:1]
	for i := 1; i < count; i++ {
		if table[i].org != 0 {
			out = append(out, table[i])
		}
	}
	return out
}

func contourBounds(o Outline, nn int) (first, last int) {
	if nn > 0 {
		first = o.Ends[nn-1] + 1
	}
	return first, o.Ends[nn]
}

func (m *latinMetrics) initBlues(src Source, set []blueString) bool {
	flatLimit := flatThreshold(m.upem)
	var outline Outline
	for _, bs := range set {
		top := bs.properties&blueTop != 0
		subTop := bs.properties&blueSubTop != 0
		var flats, rounds []int64
		var ascender, descender int64
		clusters(bs.text, func(r rune, single bool) {
			if !single {
				return
			}
			bestYExtremum := int64(math.MaxInt64)
			if top {
				bestYExtremum = math.MinInt64
			}
			bestRound := false
			gid, ok := src.Lookup(r)
			if ok && gid != 0 {
				if y, round, ok := m.blueExtremum(src, &outline, gid, bs.properties, flatLimit, &ascender, &descender); ok {
					if top && y > bestYExtremum || !top && y < bestYExtremum {
						bestYExtremum, bestRound = y, round
					}
				}
			}
			if bestYExtremum == math.MinInt64 || bestYExtremum == math.MaxInt64 {
				return
			}
			if bestRound {
				rounds = append(rounds, bestYExtremum)
			} else {
				flats = append(flats, bestYExtremum)
			}
		})
		if len(flats) == 0 && len(rounds) == 0 {
			continue
		}
		sortPos(rounds)
		sortPos(flats)
		var b blue
		switch {
		case len(flats) == 0:
			b.ref.org, b.shoot.org = rounds[len(rounds)/2], rounds[len(rounds)/2]
		case len(rounds) == 0:
			b.ref.org, b.shoot.org = flats[len(flats)/2], flats[len(flats)/2]
		default:
			b.ref.org, b.shoot.org = flats[len(flats)/2], rounds[len(rounds)/2]
		}
		if b.shoot.org != b.ref.org {
			overRef := b.shoot.org > b.ref.org
			if (top || subTop) != overRef {
				mid := (b.shoot.org + b.ref.org) / 2
				b.ref.org, b.shoot.org = mid, mid
			}
		}
		b.ascender, b.descender = ascender, descender
		if top {
			b.flags |= zoneTop
		}
		if subTop {
			b.flags |= zoneSubTop
		}
		if bs.properties&blueNeutral != 0 {
			b.flags |= zoneNeutral
		}
		if bs.properties&blueCapitalBotom != 0 {
			b.flags |= zoneBottom
		}
		if bs.properties&blueSmallBottom != 0 {
			b.flags |= zoneBottomSmall
		}
		if bs.properties&blueXHeight != 0 {
			b.flags |= zoneAdjustment
		}
		m.blues = append(m.blues, b)
	}
	if len(m.blues) == 0 {
		return false
	}
	sorted := make([]*blue, len(m.blues))
	for i := range m.blues {
		sorted[i] = &m.blues[i]
	}
	sortBlues(sorted)
	for i := range len(sorted) - 1 {
		a := &sorted[i].ref.org
		if sorted[i].flags&(zoneTop|zoneSubTop) != 0 {
			a = &sorted[i].shoot.org
		}
		b := &sorted[i+1].ref.org
		if sorted[i+1].flags&(zoneTop|zoneSubTop) != 0 {
			b = &sorted[i+1].shoot.org
		}
		if *a > *b {
			*a = *b
		}
	}
	return true
}

func sortPos(table []int64) {
	for i := 1; i < len(table); i++ {
		for j := i; j > 0 && table[j] < table[j-1]; j-- {
			table[j], table[j-1] = table[j-1], table[j]
		}
	}
}

func blueSortKey(b *blue) int64 {
	if b.flags&(zoneTop|zoneSubTop) != 0 {
		return b.ref.org
	}
	return b.shoot.org
}

func sortBlues(table []*blue) {
	for i := 1; i < len(table); i++ {
		for j := i; j > 0 && blueSortKey(table[j]) < blueSortKey(table[j-1]); j-- {
			table[j], table[j-1] = table[j-1], table[j]
		}
	}
}

func (m *latinMetrics) blueExtremum(src Source, buf *Outline, gid uint32, props uint16, flatLimit int64, ascender, descender *int64) (int64, bool, bool) {
	if !src.LoadOutline(buf, gid) || len(buf.Points) <= 2 {
		return 0, false, false
	}
	o := *buf
	points := o.Points
	top := props&(blueTop|blueSubTop) != 0
	bestPoint, bestContourFirst, bestContourLast := -1, -1, -1
	var bestY int64
	for nn := range o.Ends {
		first, last := contourBounds(o, nn)
		if last <= first {
			continue
		}
		for pp := first; pp <= last; pp++ {
			y := points[pp].Y
			if top {
				if bestPoint < 0 || y > bestY {
					bestPoint, bestY = pp, y
					*ascender = max(*ascender, bestY)
				} else {
					*descender = min(*descender, y)
				}
			} else {
				if bestPoint < 0 || y < bestY {
					bestPoint, bestY = pp, y
					*descender = min(*descender, bestY)
				} else {
					*ascender = max(*ascender, y)
				}
			}
		}
		if bestPoint > bestContourLast {
			bestContourFirst, bestContourLast = first, last
		}
	}
	round := false
	if bestPoint >= 0 {
		var skip bool
		bestY, round, skip = m.blueSegment(o, bestPoint, bestContourFirst, bestContourLast, bestY, props, flatLimit)
		if skip {
			return 0, false, false
		}
	}
	return bestY, round, true
}

func (m *latinMetrics) blueSegment(o Outline, bestPoint, contourFirst, contourLast int, bestY int64, props uint16, flatLimit int64) (int64, bool, bool) {
	points, tags := o.Points, o.Tags
	stepBack := func(i int) int {
		if i > contourFirst {
			return i - 1
		}
		return contourLast
	}
	stepFwd := func(i int) int {
		if i < contourLast {
			return i + 1
		}
		return contourFirst
	}
	bestX := points[bestPoint].X
	segFirst, segLast := bestPoint, bestPoint
	onFirst, onLast := -1, -1
	if tags[bestPoint] == On {
		onFirst, onLast = bestPoint, bestPoint
	}
	prev, next := bestPoint, bestPoint
	for {
		prev = stepBack(prev)
		dist := abs(points[prev].Y - bestY)
		if dist > 5 && abs(points[prev].X-bestX) <= 20*dist {
			break
		}
		segFirst = prev
		if tags[prev] == On {
			onFirst = prev
			if onLast < 0 {
				onLast = prev
			}
		}
		if prev == bestPoint {
			break
		}
	}
	var dist int64
	for {
		next = stepFwd(next)
		dist = abs(points[next].Y - bestY)
		if dist > 5 && abs(points[next].X-bestX) <= 20*dist {
			break
		}
		segLast = next
		if tags[next] == On {
			onLast = next
			if onFirst < 0 {
				onFirst = next
			}
		}
		if next == bestPoint {
			break
		}
	}
	if props&blueLong != 0 {
		lengthThreshold := m.upem / 25
		dist = abs(points[segLast].X - points[segFirst].X)
		if dist < lengthThreshold && segLast-segFirst+2 <= contourLast-contourFirst {
			heightThreshold := m.upem / 4
			prev = bestPoint
			for {
				prev = stepBack(prev)
				if points[prev].X != bestX || prev == bestPoint {
					break
				}
			}
			if prev == bestPoint {
				return 0, false, true
			}
			left2right := points[prev].X < points[bestPoint].X
			first := segLast
			last := first
			hit := false
			pFirst, pLast := 0, 0
			for {
				if !hit {
					first = last
					if tags[first] == On {
						pFirst, pLast = first, first
					} else {
						pFirst, pLast = -1, -1
					}
					hit = true
				}
				last = stepFwd(last)
				if abs(bestY-points[first].Y) > heightThreshold {
					hit = false
					if last == segFirst {
						break
					}
					continue
				}
				dist = abs(points[last].Y - points[first].Y)
				if dist > 5 && abs(points[last].X-points[first].X) <= 20*dist {
					hit = false
					if last == segFirst {
						break
					}
					continue
				}
				if tags[last] == On {
					pLast = last
					if pFirst < 0 {
						pFirst = last
					}
				}
				l2r := points[first].X < points[last].X
				d := abs(points[last].X - points[first].X)
				if l2r == left2right && d >= lengthThreshold {
					for {
						last = stepFwd(last)
						d = abs(points[last].Y - points[first].Y)
						if d > 5 && abs(points[next].X-points[first].X) <= 20*dist {
							last = stepBack(last)
							break
						}
						pLast = last
						if tags[last] == On {
							pLast = last
							if pFirst < 0 {
								pFirst = last
							}
						}
						if last == segFirst {
							break
						}
					}
					bestY = points[first].Y
					segFirst, segLast = first, last
					onFirst, onLast = pFirst, pLast
					break
				}
				if last == segFirst {
					break
				}
			}
		}
	}
	var round bool
	if onFirst >= 0 && onLast >= 0 && abs(points[onLast].X-points[onFirst].X) > flatLimit {
		round = false
	} else {
		round = tags[segFirst] != On || tags[segLast] != On
	}
	if round && props&blueNeutral != 0 {
		return 0, false, true
	}
	return bestY, round, false
}
