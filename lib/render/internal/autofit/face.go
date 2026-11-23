package autofit

import (
	"cmp"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/stubbedev/lys/lib/third_party/typesetting/font"
)

// Source is the font a Face analyses.
type Source interface {
	Face() *font.Face
	LoadOutline(o *Outline, gid uint32) bool
}

const (
	styleMask       uint16 = 0x1FFF
	styleUnassigned        = styleMask
	styleNonBase    uint16 = 0x4000
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
	src    Source
	upem   int64
	layout *layout
	glyphs []uint16
	styles [len(styles)]styleMetrics
}

// NewFace assigns src's glyphs to styles as FreeType built with
// HarfBuzz does: by their cmap entries and the OpenType features that
// reach them.
func NewFace(src Source) *Face {
	face := src.Face()
	f := &Face{src: src, upem: int64(face.Upem()), layout: newLayout(face)}
	f.glyphs = f.coverage(face)
	return f
}

func (f *Face) coverage(face *font.Face) []uint16 {
	gstyles := make([]uint16, face.NumGlyphs())
	for i := range gstyles {
		gstyles[i] = styleUnassigned
	}
	if face.Cmap == nil {
		return fallback(gstyles)
	}
	var mappings []mapping
	for it := face.Cmap.Iter(); it.Next(); {
		r, gid := it.Char()
		if gid != 0 && int(gid) < len(gstyles) {
			mappings = append(mappings, mapping{Rune: r, Glyph: uint32(gid)})
		}
	}
	slices.SortFunc(mappings, func(a, b mapping) int { return cmp.Compare(a.Rune, b.Rune) })
	inRanges := func(ranges []uniRange, visit func(gid uint32)) {
		for _, r := range ranges {
			k, _ := slices.BinarySearchFunc(mappings, r.first, func(m mapping, first rune) int { return cmp.Compare(m.Rune, first) })
			for ; k < len(mappings) && mappings[k].Rune <= r.last; k++ {
				visit(mappings[k].Glyph)
			}
		}
	}
	for i := range styles {
		ss := uint16(i)
		st := &styles[i]
		if st.coverage != 0 {
			f.layout.assignCoverage(gstyles, ss, false)
			continue
		}
		sc := &scripts[st.script]
		inRanges(sc.ranges, func(gid uint32) {
			if gstyles[gid] == styleUnassigned {
				gstyles[gid] = ss
			}
		})
		inRanges(sc.nonbase, func(gid uint32) {
			if gstyles[gid]&styleMask == ss {
				gstyles[gid] |= styleNonBase
			}
		})
	}
	for i := range styles {
		if styles[i].coverage == 0 {
			f.layout.assignCoverage(gstyles, uint16(i), false)
		}
	}
	f.layout.assignCoverage(gstyles, latinStyle, true)
	return fallback(gstyles)
}

func fallback(gstyles []uint16) []uint16 {
	for i, gs := range gstyles {
		if gs&styleMask == styleUnassigned {
			gstyles[i] = gs&^styleMask | fallbackStyle
		}
	}
	return gstyles
}

type mapping struct {
	Rune  rune
	Glyph uint32
}

func (f *Face) metrics(ss int) *styleMetrics {
	sm := &f.styles[ss]
	sm.once.Do(func() {
		switch st := &styles[ss]; st.writing {
		case writingLatin:
			if m, ok := f.newLatinMetrics(st); ok {
				sm.latin = m
			}
		case writingCJK, writingIndic:
			sm.cjk = f.newCJKMetrics(st)
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
	if int(gid) >= len(f.glyphs) {
		return dst, false, false
	}
	gs := f.glyphs[gid]
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
	topToBottom := scripts[styles[gs&styleMask].script].topToBottom
	axis := sm.latinAxis(nominal)
	h.upem, h.xScale, h.yScale = f.upem, nominal, axis.scale
	h.reload(o)

	var adj uint32
	if cp, ok := m.reverse[gid]; ok {
		adj = adjustmentFlags(cp)
	}
	h.applyTilde(adj)
	h.computeSegments()
	h.linkSegments(m.widths)
	h.computeEdges(m.edgeDistanceThreshold, topToBottom)

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
	h.hintEdges(topToBottom)
	h.alignEdgePoints()
	h.alignStrongPoints()
	h.alignWeakPoints()
	h.separateVertically(adj, accentHeightLimit)
}
