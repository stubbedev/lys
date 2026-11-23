package autofit

import (
	"cmp"
	"slices"
	"sync"
	"sync/atomic"
)

// Source is the font a Face analyses.
type Source interface {
	Upem() int64
	Lookup(r rune) (uint32, bool)
	Mappings() []Mapping
	LoadOutline(o *Outline, gid uint32) bool
}

// Mapping is one cmap entry.
type Mapping struct {
	Rune  rune
	Glyph uint32
}

const (
	styleMask    uint16 = 0x1FFF
	styleNonBase uint16 = 0x4000
)

type styleMetrics struct {
	once        sync.Once
	latin       *latinMetrics
	cjk         *cjkMetrics
	scaledLatin atomic.Pointer[scaledAxis]
	scaledCJK   atomic.Pointer[scaledCJK]
}

var hintsPool sync.Pool

func takeHints() *glyphHints {
	if h, ok := hintsPool.Get().(*glyphHints); ok {
		return h
	}
	return new(glyphHints)
}

// Face is FreeType's light autohinter for one font: the style of each
// glyph and, computed when first needed, each style's blue zones and
// stem widths.
type Face struct {
	src     Source
	upem    int64
	glyphs  map[uint32]uint16
	reverse map[uint32]rune
	styles  [len(styles)]styleMetrics
}

// NewFace assigns src's glyphs to scripts as FreeType does.
func NewFace(src Source) *Face {
	f := &Face{src: src, upem: src.Upem()}
	mappings := src.Mappings()
	slices.SortFunc(mappings, func(a, b Mapping) int { return cmp.Compare(a.Rune, b.Rune) })
	f.glyphs = coverage(mappings)
	f.reverse = reverseCharmap(src)
	return f
}

func coverage(mappings []Mapping) map[uint32]uint16 {
	glyphs := make(map[uint32]uint16, len(mappings))
	inRanges := func(ranges []uniRange, visit func(gid uint32)) {
		for _, r := range ranges {
			k, _ := slices.BinarySearchFunc(mappings, r.first, func(m Mapping, first rune) int { return cmp.Compare(m.Rune, first) })
			for ; k < len(mappings) && mappings[k].Rune <= r.last; k++ {
				if gid := mappings[k].Glyph; gid != 0 {
					visit(gid)
				}
			}
		}
	}
	for i, st := range styles {
		ss := uint16(i)
		sc := &scripts[st.script]
		inRanges(sc.ranges, func(gid uint32) {
			if _, ok := glyphs[gid]; !ok {
				glyphs[gid] = ss
			}
		})
		inRanges(sc.nonbase, func(gid uint32) {
			if g, ok := glyphs[gid]; ok && g&styleMask == ss {
				glyphs[gid] = g | styleNonBase
			}
		})
	}
	glyphs[0] = fallbackStyle
	for _, m := range mappings {
		if _, ok := glyphs[m.Glyph]; !ok {
			glyphs[m.Glyph] = fallbackStyle
		}
	}
	return glyphs
}

var (
	latinStyle    = styleOf(latinScript)
	fallbackStyle = styleOf(haniScript)
)

func styleOf(script int) uint16 {
	i := slices.IndexFunc(styles[:], func(st style) bool { return st.script == script })
	if i < 0 || i > int(styleMask) {
		panic("autofit: no default style for a script the tables name")
	}
	return uint16(i)
}

func (f *Face) style(gid uint32) uint16 {
	if s, ok := f.glyphs[gid]; ok {
		return s
	}
	return latinStyle
}

func reverseCharmap(src Source) map[uint32]rune {
	reverse := make(map[uint32]rune, len(adjustments))
	for _, a := range adjustments {
		if gid, ok := src.Lookup(a.codepoint); ok && gid != 0 {
			reverse[gid] = a.codepoint
		}
	}
	return reverse
}

func (f *Face) metrics(ss int) *styleMetrics {
	sm := &f.styles[ss]
	sm.once.Do(func() {
		switch st := styles[ss]; st.writing {
		case writingLatin:
			if m, ok := newLatinMetrics(f.src, st); ok {
				sm.latin = m
			}
		case writingCJK, writingIndic:
			sm.cjk = newCJKMetrics(f.src, st)
		}
	})
	return sm
}

type scaledAxis struct {
	nominal int64
	scale   int64
	blues   []blue
}

func (sm *styleMetrics) latinAxis(nominal int64) *scaledAxis {
	if a := sm.scaledLatin.Load(); a != nil && a.nominal == nominal {
		return a
	}
	a := sm.latin.scale(nominal)
	sm.scaledLatin.Store(a)
	return a
}

func (sm *styleMetrics) cjkAxes(nominal int64) *scaledCJK {
	if a := sm.scaledCJK.Load(); a != nil && a.nominal == nominal {
		return a
	}
	a := sm.cjk.scale(nominal)
	sm.scaledCJK.Store(a)
	return a
}

func (m *latinMetrics) scale(yScale int64) *scaledAxis {
	scale := yScale
	adjust := slices.IndexFunc(m.blues, func(b blue) bool { return b.flags&zoneAdjustment != 0 })
	if adjust >= 0 {
		scaled := mulFix(m.blues[adjust].shoot.org, scale)
		fitted := (scaled + 40) &^ 63
		if scaled != fitted {
			newScale := mulDiv(scale, fitted, scaled)
			maxHeight := m.upem
			for _, b := range m.blues {
				maxHeight = max(maxHeight, b.ascender, -b.descender)
			}
			if dist := mulFix(maxHeight, newScale-scale); -128 < dist && dist < 128 {
				scale = newScale
			}
		}
	}
	a := &scaledAxis{nominal: yScale, scale: scale, blues: slices.Clone(m.blues)}
	for i := range a.blues {
		b := &a.blues[i]
		b.ref.cur = mulFix(b.ref.org, scale)
		b.ref.fit = b.ref.cur
		b.shoot.cur = mulFix(b.shoot.org, scale)
		b.shoot.fit = b.shoot.cur
		b.flags &^= zoneActive
		dist := mulFix(b.ref.org-b.shoot.org, scale)
		if dist > 48 || dist < -48 {
			continue
		}
		delta := abs(dist)
		switch {
		case delta < 32:
			delta = 0
		case delta < 48:
			delta = 32
		default:
			delta = 64
		}
		if dist < 0 {
			delta = -delta
		}
		b.ref.fit = pixRound(b.ref.cur)
		b.shoot.fit = b.ref.fit - delta
		b.flags |= zoneActive
	}
	for i := range a.blues {
		b := &a.blues[i]
		if b.flags&zoneSubTop == 0 || b.flags&zoneActive == 0 {
			continue
		}
		for j := range a.blues {
			o := &a.blues[j]
			if o.flags&zoneSubTop != 0 || o.flags&zoneActive == 0 {
				continue
			}
			if o.ref.fit <= b.shoot.fit && o.shoot.fit >= b.ref.fit {
				b.flags &^= zoneActive
				break
			}
		}
	}
	return a
}

// Hint appends the hinted position of every point of o, in 26.6 device
// pixels at size26d6 pixels per em, to dst. horizontal reports whether
// x is hinted too, as the CJK and Indic writing systems hint it; Latin
// leaves x scaled. It is false when FreeType would leave the glyph
// unhinted.
func (f *Face) Hint(dst []Point, gid uint32, o Outline, size26d6 int64) (out []Point, horizontal, ok bool) {
	if len(o.Points) == 0 {
		return dst, false, false
	}
	gs := f.style(gid)
	ss := int(gs & styleMask)
	sm := f.metrics(ss)
	nominal := divFix(size26d6, f.upem)
	h := takeHints()
	defer hintsPool.Put(h)
	switch {
	case sm.latin != nil:
		f.hintLatin(h, sm, gid, gs, o, nominal)
	case sm.cjk != nil:
		f.hintCJK(h, sm, o, nominal)
		horizontal = true
	default:
		return dst, false, false
	}
	for i := range h.points {
		dst = append(dst, Point{X: h.points[i].x, Y: h.points[i].y})
	}
	return dst, horizontal, true
}

func (f *Face) hintCJK(h *glyphHints, sm *styleMetrics, o Outline, nominal int64) {
	axes := sm.cjkAxes(nominal)
	h.upem, h.xScale, h.yScale = f.upem, axes.axis[dimHorz].scale, axes.axis[dimVert].scale
	h.reload(o)
	h.transpose()
	h.hintAxisCJK(sm.cjk, &axes.axis[dimHorz], dimHorz)
	h.transpose()
	h.hintAxisCJK(sm.cjk, &axes.axis[dimVert], dimVert)
}

func (f *Face) hintLatin(h *glyphHints, sm *styleMetrics, gid uint32, gs uint16, o Outline, nominal int64) {
	m := sm.latin
	axis := sm.latinAxis(nominal)
	h.upem, h.xScale, h.yScale = f.upem, nominal, axis.scale
	h.reload(o)

	var adj uint32
	if cp, ok := f.reverse[gid]; ok {
		adj = adjustmentFlags(cp)
	}
	h.applyTilde(adj)
	h.computeSegments()
	h.linkSegments(m.widths)
	h.computeEdges(m.edgeDistanceThreshold)

	var accentHeightLimit int64
	if adj != 0 {
		capTop, capBottom := axis.baseGlyphBlues(true)
		smallTop, smallBottom := axis.baseGlyphBlues(false)
		if adj&adjustNoHeightCheck == 0 {
			switch {
			case smallTop != nil && smallBottom != nil:
				accentHeightLimit = 2 * (smallTop.shoot.cur - smallBottom.shoot.cur) / 3
			case capTop != nil && capBottom != nil:
				accentHeightLimit = (capTop.shoot.cur - capBottom.shoot.cur) / 2
			default:
				accentHeightLimit = mulFix(f.upem*4/10, axis.scale)
			}
		}
		if capTop != nil && capBottom != nil {
			if adj&ignoreCapitalTop != 0 {
				h.ignoreTop(capTop, capBottom)
			}
			if adj&ignoreCapitalBottom != 0 {
				h.ignoreBottom(capTop, capBottom)
			}
		}
		if smallTop != nil && smallBottom != nil {
			if adj&ignoreSmallTop != 0 {
				h.ignoreTop(smallTop, smallBottom)
			}
			if adj&ignoreSmallBottom != 0 {
				h.ignoreBottom(smallTop, smallBottom)
			}
		}
	}
	if gs&styleNonBase == 0 {
		h.computeBlueEdges(axis, f.upem)
	}
	h.hintEdges()
	h.alignEdgePoints()
	h.alignStrongPoints()
	h.alignWeakPoints()
	h.separateVertically(adj, accentHeightLimit)
}
