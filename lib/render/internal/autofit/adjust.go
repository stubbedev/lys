package autofit

import "math"

const (
	adjustUp            uint32 = 0x01
	adjustDown          uint32 = 0x02
	adjustUp2           uint32 = 0x04
	adjustDown2         uint32 = 0x08
	adjustTildeTop      uint32 = 0x10
	adjustTildeBottom   uint32 = 0x20
	adjustTildeTop2     uint32 = 0x40
	adjustTildeBottom2  uint32 = 0x80
	ignoreCapitalTop    uint32 = 0x100
	ignoreCapitalBottom uint32 = 0x200
	ignoreSmallTop      uint32 = 0x400
	ignoreSmallBottom   uint32 = 0x800
	adjustNoHeightCheck uint32 = 0x1000
)

func adjustmentFlags(codepoint rune) uint32 {
	lo, hi := 0, len(adjustments)-1
	for hi >= lo {
		mid := (lo + hi) / 2
		switch c := adjustments[mid].codepoint; {
		case c < codepoint:
			lo = mid + 1
		case c > codepoint:
			hi = mid - 1
		default:
			return adjustments[mid].flags
		}
	}
	return 0
}

func (h *glyphHints) moveContour(c int, movement int64) {
	first := h.contours[c]
	for p := first; ; {
		h.points[p].y += movement
		p = h.points[p].next
		if p == first {
			break
		}
	}
}

func (h *glyphHints) moveContoursUp(limit, delta int64) {
	for c := range h.contours {
		if h.yMin[c] < h.yMax[c] && h.yMin[c] > limit {
			h.moveContour(c, delta)
		}
	}
}

func (h *glyphHints) moveContoursDown(limit, delta int64) {
	for c := range h.contours {
		if h.yMin[c] < h.yMax[c] && h.yMax[c] < limit {
			h.moveContour(c, -delta)
		}
	}
}

func (h *glyphHints) computeVerticalExtrema() {
	h.yMin = append(h.yMin[:0], make([]int64, len(h.contours))...)
	h.yMax = append(h.yMax[:0], make([]int64, len(h.contours))...)
	for c, first := range h.contours {
		lo, hi := int64(math.MaxInt64), int64(math.MinInt64)
		if h.points[h.points[first].next].next != first {
			for p := first; ; {
				lo, hi = min(lo, h.points[p].y), max(hi, h.points[p].y)
				p = h.points[p].next
				if p == first {
					break
				}
			}
		}
		h.yMin[c], h.yMax[c] = lo, hi
	}
}

func (h *glyphHints) highestContour() int {
	best, bestMin, bestMax := 0, int64(math.MaxInt64), int64(math.MinInt64)
	for c := range h.contours {
		if h.yMax[c] > bestMax || h.yMax[c] == bestMax && h.yMin[c] > bestMin {
			best, bestMin, bestMax = c, h.yMin[c], h.yMax[c]
		}
	}
	return best
}

func (h *glyphHints) secondHighestContour() int {
	if len(h.contours) < 3 {
		return 0
	}
	highest := h.highestContour()
	highestMin := h.yMin[highest]
	best, bestMax := 0, int64(math.MinInt64)
	for c := range h.contours {
		if c != highest && h.yMax[c] > bestMax && h.yMin[c] < highestMin {
			best, bestMax = c, h.yMax[c]
		}
	}
	return best
}

func (h *glyphHints) lowestContour() int {
	best, bestMin, bestMax := 0, int64(math.MaxInt64), int64(math.MinInt64)
	for c := range h.contours {
		if h.yMin[c] < bestMin || h.yMin[c] == bestMin && h.yMax[c] < bestMax {
			best, bestMin, bestMax = c, h.yMin[c], h.yMax[c]
		}
	}
	return best
}

func (h *glyphHints) secondLowestContour() int {
	if len(h.contours) < 3 {
		return 0
	}
	lowest := h.lowestContour()
	lowestMax := h.yMax[lowest]
	best, bestMin := 0, int64(math.MaxInt64)
	for c := range h.contours {
		if c != lowest && h.yMin[c] < bestMin && h.yMax[c] > lowestMax {
			best, bestMin = c, h.yMin[c]
		}
	}
	return best
}

func (h *glyphHints) preventBlueAlignment(above bool, pos int64) {
	for k := range h.edges {
		e := &h.edges[k]
		if above && e.pos > pos || !above && e.pos < pos {
			e.flags |= edgeNoBlue
		}
	}
}

func (axis *scaledAxis) baseGlyphBlues(capital bool) (top, bottom *blue) {
	topFlag, bottomFlag := zoneAdjustment|zoneActive, zoneBottomSmall|zoneActive
	if capital {
		topFlag, bottomFlag = zoneTop|zoneActive, zoneBottom|zoneActive
	}
	for i := range axis.blues {
		if axis.blues[i].flags&topFlag == topFlag {
			top = &axis.blues[i]
			break
		}
	}
	for i := range axis.blues {
		if axis.blues[i].flags&bottomFlag == bottomFlag {
			bottom = &axis.blues[i]
			break
		}
	}
	return top, bottom
}

func (h *glyphHints) ignoreTop(top, bottom *blue) {
	height := top.shoot.cur - bottom.shoot.cur
	h.preventBlueAlignment(true, top.shoot.cur+height/7+16)
}

func (h *glyphHints) ignoreBottom(top, bottom *blue) {
	height := top.shoot.cur - bottom.shoot.cur
	h.preventBlueAlignment(false, bottom.shoot.cur-height/7-16)
}

func (h *glyphHints) touchContour(c int) {
	first := h.contours[c]
	for p := first; ; {
		p = h.points[p].next
		h.points[p].flags |= flagIgnore
		if h.points[p].flags&flagControl == 0 {
			h.points[p].flags |= flagTouchY
		}
		if p == first {
			break
		}
	}
}

func (h *glyphHints) touchContours(top bool, limitContour int) {
	limit := h.yMin[limitContour]
	for c := range h.contours {
		if h.yMin[c] >= h.yMax[c] {
			continue
		}
		if top && h.yMin[c] >= limit || !top && h.yMax[c] <= limit {
			h.touchContour(c)
		}
	}
}

func (h *glyphHints) tildeWiggle(c int, extremumThreshold int64) (int64, bool) {
	pts := h.points
	minY, maxY := h.yMin[c], h.yMax[c]
	first := h.contours[c]
	var best int64
	taken := false
	for p := first; ; {
		p = pts[p].next
		pt := &pts[p]
		prev, next := &pts[pt.prev], &pts[pt.next]
		if pt.flags&flagControl == 0 && prev.y == pt.y && next.y == pt.y && pt.y != minY && pt.y != maxY && prev.flags&flagControl != 0 && next.flags&flagControl != 0 {
			prevOn, nextOn := pt.prev, pt.next
			for pts[prevOn].flags&flagControl != 0 {
				prevOn = pts[prevOn].prev
			}
			for pts[nextOn].flags&flagControl != 0 {
				nextOn = pts[nextOn].next
			}
			var measurement int64
			ok := true
			switch {
			case pts[nextOn].y > pt.y && pts[prevOn].y > pt.y:
				measurement = pt.y - minY
			case pts[nextOn].y < pt.y && pts[prevOn].y < pt.y:
				measurement = maxY - pt.y
			default:
				ok = false
			}
			if ok && measurement >= extremumThreshold && (!taken || measurement < best) {
				best, taken = measurement, true
			}
		}
		if p == first {
			break
		}
	}
	return best, taken
}

func (h *glyphHints) stretchTilde(c int, top bool) int64 {
	minY, maxY := h.yMin[c], h.yMax[c]
	if minY == maxY {
		return 0
	}
	height := maxY - minY
	measurement, _ := h.tildeWiggle(c, height/8)
	h.touchContours(top, c)
	target := measurement + 64
	if height >= target {
		return 0
	}
	anchor := minY
	if !top {
		anchor = maxY
	}
	first := h.contours[c]
	for p := first; ; {
		p = h.points[p].next
		h.points[p].y = (h.points[p].y-anchor)*target/height + anchor
		if p == first {
			break
		}
	}
	return target - height
}

func (h *glyphHints) alignTilde(c int, top bool) int64 {
	first := h.contours[c]
	lo, hi := h.points[first].y, h.points[first].y
	for p := first; ; {
		p = h.points[p].next
		lo, hi = min(lo, h.points[p].y), max(hi, h.points[p].y)
		if p == first {
			break
		}
	}
	height := hi - lo
	var delta int64
	if top {
		delta = pixRound(lo) - lo
		if height < 64*3 {
			delta += (pixRound(height) - height) / 2
		}
	} else {
		delta = pixRound(hi) - hi
		if height < 64*3 {
			delta -= (pixRound(height) - height) / 2
		}
	}
	h.moveContour(c, delta)
	return delta
}

func (h *glyphHints) contourOverlapsHorizontally(index int) bool {
	cMin, cMax := int64(math.MaxInt64), int64(math.MinInt64)
	oMin, oMax := int64(math.MaxInt64), int64(math.MinInt64)
	for c, first := range h.contours {
		if h.points[h.points[first].next].next == first {
			continue
		}
		for p := first; ; {
			p = h.points[p].next
			x := h.points[p].x
			if c == index {
				cMin, cMax = min(cMin, x), max(cMax, x)
			} else {
				oMin, oMax = min(oMin, x), max(oMax, x)
			}
			if p == first {
				break
			}
		}
	}
	return oMin <= cMax && cMax <= oMax || oMin <= cMin && cMin <= oMax || cMax >= oMax && cMin <= oMin
}

func (h *glyphHints) applyTilde(adj uint32) {
	topTilde := adj&adjustTildeTop != 0
	bottomTilde := adj&adjustTildeBottom != 0
	belowTopTilde := adj&adjustTildeTop2 != 0
	aboveBottomTilde := adj&adjustTildeBottom2 != 0
	if !topTilde && !bottomTilde && !belowTopTilde && !aboveBottomTilde {
		return
	}
	h.computeVerticalExtrema()
	if belowTopTilde {
		c := h.secondHighestContour()
		offset := h.stretchTilde(c, true)
		offset += h.alignTilde(c, true)
		h.moveContoursUp(h.yMin[c], offset)
	}
	if aboveBottomTilde {
		c := h.secondLowestContour()
		offset := h.stretchTilde(c, false)
		offset -= h.alignTilde(c, false)
		h.moveContoursDown(h.yMax[c], offset)
	}
	if topTilde {
		c := h.highestContour()
		h.stretchTilde(c, true)
		h.alignTilde(c, true)
	}
	if bottomTilde {
		c := h.lowestContour()
		h.stretchTilde(c, false)
		h.alignTilde(c, false)
	}
}

func (h *glyphHints) separateVertically(adj uint32, accentHeightLimit int64) {
	adjustTop := adj&adjustUp != 0
	adjustBelowTop := adj&adjustUp2 != 0
	adjustBottom := adj&adjustDown != 0
	adjustAboveBottom := adj&adjustDown2 != 0
	n := len(h.contours)
	topCase := adjustTop && n >= 2 || adjustBelowTop && n >= 3
	bottomCase := adjustBottom && n >= 2 || adjustAboveBottom && n >= 3
	if !topCase && !bottomCase {
		return
	}
	h.computeVerticalExtrema()
	if topCase && !h.separateTop(adj, adjustTop, adjustBelowTop, accentHeightLimit) {
		return
	}
	if bottomCase {
		h.separateBottom(adj, adjustBottom, adjustAboveBottom, accentHeightLimit)
	}
}

func (h *glyphHints) separateTop(adj uint32, adjustTop, adjustBelowTop bool, accentHeightLimit int64) bool {
	isTopTilde := adj&adjustTildeTop != 0
	isBelowTopTilde := adj&adjustTildeTop2 != 0
	high := h.highestContour()
	if adjustBelowTop {
		high = h.secondHighestContour()
	}
	if !h.contourOverlapsHorizontally(high) {
		return false
	}
	highMin, highMax := h.yMin[high], h.yMax[high]
	highHeight := highMax - highMin
	if highHeight > accentHeightLimit {
		return false
	}
	minDistance := int64(64)
	for c := range h.contours {
		if c == high {
			continue
		}
		distance := highMin - h.yMax[c]
		if distance < 64 && distance < minDistance && h.yMin[c] < highMin {
			minDistance = distance
		}
	}
	amount := 64 - minDistance
	var centering int64
	if isTopTilde || isBelowTopTilde {
		tilde := high
		if !adjustTop && !isBelowTopTilde {
			tilde = h.highestContour()
		}
		tildeHeight := h.yMax[tilde] - h.yMin[tilde]
		pos := highMin + amount
		if adjustBelowTop && isTopTilde {
			pos += highHeight
		}
		if pos%64 == 0 && tildeHeight < 3*64 {
			centering = (pixRound(tildeHeight) - tildeHeight) / 2
		}
	}
	calculated := amount
	if adjustTop && isTopTilde || adjustBelowTop && isBelowTopTilde {
		calculated = amount + centering
	}
	if calculated != 0 && calculated >= -2 && (calculated <= 66 || amount <= 66) {
		limit := highMin - highHeight/8
		h.moveContoursUp(limit, amount)
		if adjustBelowTop && isTopTilde {
			h.moveContoursUp(limit+highHeight, centering)
		}
	}
	return true
}

func (h *glyphHints) separateBottom(adj uint32, adjustBottom, adjustAboveBottom bool, accentHeightLimit int64) {
	isBottomTilde := adj&adjustTildeBottom != 0
	isAboveBottomTilde := adj&adjustTildeBottom2 != 0
	low := h.lowestContour()
	if adjustAboveBottom {
		low = h.secondLowestContour()
	}
	if !h.contourOverlapsHorizontally(low) {
		return
	}
	lowMin, lowMax := h.yMin[low], h.yMax[low]
	lowHeight := lowMax - lowMin
	if lowHeight > accentHeightLimit {
		return
	}
	minDistance := int64(64)
	for c := range h.contours {
		if c == low {
			continue
		}
		distance := h.yMin[c] - lowMax
		if distance < 64 && distance < minDistance && h.yMax[c] > lowMax {
			minDistance = distance
		}
	}
	amount := 64 - minDistance
	var centering int64
	if isBottomTilde || isAboveBottomTilde {
		tilde := low
		if !adjustBottom && !isAboveBottomTilde {
			tilde = h.lowestContour()
		}
		tildeHeight := h.yMax[tilde] - h.yMin[tilde]
		pos := lowMax - amount
		if adjustAboveBottom && isBottomTilde {
			pos -= lowHeight
		}
		if pos%64 == 0 && tildeHeight < 3*64 {
			centering = (pixRound(tildeHeight) - tildeHeight) / 2
		}
	}
	calculated := amount
	if adjustBottom && isBottomTilde || adjustAboveBottom && isAboveBottomTilde {
		calculated = amount + centering
	}
	if calculated != 0 && calculated >= -2 && (calculated <= 66 || amount <= 66) {
		limit := lowMax + lowHeight/8
		h.moveContoursDown(limit, amount)
		if adjustAboveBottom && isBottomTilde {
			h.moveContoursDown(limit-lowHeight, centering)
		}
	}
}
