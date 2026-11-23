package autofit

import (
	"sync"

	"github.com/stubbedev/lys/lib/third_party/typesetting/font"
	"github.com/stubbedev/lys/lib/third_party/typesetting/font/opentype/tables"
	"github.com/stubbedev/lys/lib/third_party/typesetting/harfbuzz"
	"github.com/stubbedev/lys/lib/third_party/typesetting/language"
)

type shapedGlyph struct {
	gid     uint32
	yOffset int64
}

type layout struct {
	face *font.Face

	once  sync.Once
	font  *harfbuzz.Font
	mu    sync.Mutex
	buf   *harfbuzz.Buffer
	plain *harfbuzz.Buffer
	runes []rune
}

const shapingLanguage language.Language = "c"

func newLayout(face *font.Face) *layout { return &layout{face: face} }

func (l *layout) hbFont() *harfbuzz.Font {
	l.once.Do(func() {
		l.font = harfbuzz.NewFont(font.NewFace(l.face.Font))
		l.buf, l.plain = harfbuzz.NewBuffer(), harfbuzz.NewBuffer()
	})
	return l.font
}

func (l *layout) glyph(r rune) uint32 {
	if l.face.Cmap == nil {
		return 0
	}
	gid, _ := l.face.Cmap.Lookup(r)
	return uint32(gid)
}

func (l *layout) shapeInto(buf *harfbuzz.Buffer, features []harfbuzz.Feature) {
	buf.Clear()
	buf.AddRunes(l.runes, 0, -1)
	buf.Props.Language = shapingLanguage
	buf.GuessSegmentProperties()
	buf.Shape(l.font, features)
}

func (l *layout) cluster(dst []shapedGlyph, text string, coverage tables.Tag) []shapedGlyph {
	l.hbFont()
	l.mu.Lock()
	defer l.mu.Unlock()
	l.runes = l.runes[:0]
	for _, r := range text {
		l.runes = append(l.runes, r)
	}
	var features []harfbuzz.Feature
	if coverage != 0 {
		features = []harfbuzz.Feature{{Tag: coverage, Value: 1, End: harfbuzz.FeatureGlobalEnd}}
	}
	l.shapeInto(l.buf, features)
	if coverage != 0 {
		l.shapeInto(l.plain, nil)
		if sameGlyphs(l.buf.Info, l.plain.Info) {
			return dst
		}
	}
	for i, info := range l.buf.Info {
		dst = append(dst, shapedGlyph{gid: uint32(info.Glyph), yOffset: int64(l.buf.Pos[i].YOffset)})
	}
	return dst
}

func sameGlyphs(a, b []harfbuzz.GlyphInfo) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Glyph != b[i].Glyph {
			return false
		}
	}
	return true
}

const defaultScriptTag tables.Tag = 'D'<<24 | 'F'<<16 | 'L'<<8 | 'T'

func coverageScripts(script int, defaultScript bool) (tags [3]tables.Tag, n int) {
	harfbuzz.AppendOTScriptTags(tags[:0], scripts[script].hb)
	if defaultScript {
		switch {
		case tags[0] == 0:
			tags[0] = defaultScriptTag
		case tags[1] == 0:
			tags[1] = defaultScriptTag
		case tags[1] != defaultScriptTag:
			tags[2] = defaultScriptTag
		}
	} else if tags[0] == defaultScriptTag {
		return tags, 0
	}
	for n < len(tags) && tags[n] != 0 {
		n++
	}
	return tags, n
}

type glyphSet []uint64

func newGlyphSet(n int) glyphSet { return make(glyphSet, (n+63)/64) }

func (s glyphSet) add(g harfbuzz.GID) {
	if w := int(g / 64); w < len(s) {
		s[w] |= 1 << (g % 64)
	}
}

func (s glyphSet) has(g int) bool { return s[g/64]&(1<<(g%64)) != 0 }

func (l *layout) assignCoverage(gstyles []uint16, ss uint16, defaultScript bool) {
	st := &styles[ss]
	scriptTags, n := coverageScripts(st.script, defaultScript)
	tags := scriptTags[:n]
	if n == 0 || l.face.GSUB.Lookups == nil {
		return
	}
	collect := func(layout *font.Layout) []uint16 {
		if st.coverage == 0 {
			return harfbuzz.CollectAllLookups(layout, tags)
		}
		return harfbuzz.CollectFeatureLookups(layout, tags, st.coverage)
	}
	gsubLookups := collect(&l.face.GSUB.Layout)
	if len(gsubLookups) == 0 {
		return
	}
	glyphs := newGlyphSet(len(gstyles))
	for _, li := range gsubLookups {
		harfbuzz.CollectGSUBOutput(&l.face.GSUB, li, glyphs.add)
	}
	if st.coverage != 0 {
		positioned := newGlyphSet(len(gstyles))
		for _, li := range collect(&l.face.GPOS.Layout) {
			harfbuzz.CollectGPOSInput(&l.face.GPOS, li, positioned.add)
		}
		if !l.substitutesBlueCharacter(gsubLookups, st.blues) {
			return
		}
		for i := range glyphs {
			glyphs[i] &^= positioned[i]
		}
	}
	for gid := range gstyles {
		if glyphs.has(gid) && gstyles[gid] == styleUnassigned {
			gstyles[gid] = ss
		}
	}
}

func (l *layout) substitutesBlueCharacter(lookups []uint16, blues int) bool {
	font := l.hbFont()
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, bs := range blueStringsets[blues] {
		for _, r := range bs.text {
			glyphs := []harfbuzz.GID{harfbuzz.GID(l.glyph(r))}
			for _, li := range lookups {
				if font.WouldSubstitute(li, glyphs, true) {
					return true
				}
			}
		}
	}
	return false
}

func (l *layout) reverseCharmap(script int) map[uint32]rune {
	reverse := make(map[uint32]rune, len(adjustments))
	for _, a := range adjustments {
		if gid := l.glyph(a.codepoint); gid != 0 {
			reverse[gid] = a.codepoint
		}
	}
	gsub := &l.face.GSUB
	if gsub.Lookups == nil {
		return reverse
	}
	var buf [3]tables.Tag
	tags := harfbuzz.AppendOTScriptTags(buf[:0], scripts[script].hb)
	tags = tags[:min(1, len(tags))]
	substs := newFTHash()
	for _, li := range harfbuzz.CollectAllLookups(&gsub.Layout, tags) {
		if int(li) < len(gsub.Lookups) && singleOrAlternate(gsub.Lookups[li]) {
			mapLookup(substs, gsub.Lookups[li])
		}
	}
	for key, value := range substs.all() {
		if key&^0xFFFF != 0 {
			continue
		}
		if cp, ok := reverse[key]; ok {
			addSubstitute(reverse, substs, key, value, cp)
		}
	}
	return reverse
}

func addSubstitute(reverse map[uint32]rune, substs *ftHash, glyph, value uint32, cp rune) {
	insert := func(g uint32) {
		if _, ok := reverse[g]; ok {
			return
		}
		reverse[g] = cp
		if subst := substs.lookup(g); subst != nil {
			addSubstitute(reverse, substs, g, *subst, cp)
		}
	}
	insert(value & 0xFFFF)
	for i := range value >> 16 {
		insert(*substs.lookup(glyph + (i+1)<<16))
	}
}

func coverageCount(cov tables.Coverage) (int, bool) {
	switch cov := cov.(type) {
	case tables.Coverage1:
		return len(cov.Glyphs), true
	case tables.Coverage2:
		n := 0
		for _, r := range cov.Ranges {
			if r.StartGlyphID > r.EndGlyphID {
				return 0, false
			}
			n += int(r.EndGlyphID-r.StartGlyphID) + 1
		}
		return n, true
	}
	return 0, false
}

func singleOrAlternate(lookup font.GSUBLookup) bool {
	if len(lookup.Subtables) == 0 {
		return true
	}
	single := false
	for i, subtable := range lookup.Subtables {
		var cov tables.Coverage
		count := -1
		switch s := subtable.(type) {
		case tables.SingleSubs:
			if i > 0 && !single {
				return false
			}
			single = true
			cov = s.Cov()
			if d, ok := s.Data.(tables.SingleSubstData2); ok {
				count = len(d.SubstituteGlyphIDs)
			}
		case tables.AlternateSubs:
			if i > 0 && single {
				return false
			}
			cov, count = s.Coverage, len(s.AlternateSets)
		default:
			return false
		}
		n, ok := coverageCount(cov)
		if !ok || count >= 0 && count != n {
			return false
		}
	}
	return true
}

func eachCoverageGlyph(cov tables.Coverage, yield func(g uint16)) {
	switch cov := cov.(type) {
	case tables.Coverage1:
		for _, g := range cov.Glyphs {
			yield(g)
		}
	case tables.Coverage2:
		for i, r := range cov.Ranges {
			if i == len(cov.Ranges)-1 {
				yield(r.StartGlyphID)
				return
			}
			for g := r.StartGlyphID; ; g++ {
				yield(g)
				if g == r.EndGlyphID {
					break
				}
			}
		}
	}
}

func hashSubstitute(substs *ftHash, glyph, subst uint16) {
	value := substs.lookup(uint32(glyph))
	if value == nil {
		substs.insert(uint32(glyph), uint32(subst), true)
		return
	}
	mask := *value&0xFFFF0000 + 0x10000
	substs.insert(uint32(glyph)|mask, uint32(subst), true)
	*value += 0x10000
}

func mapLookup(substs *ftHash, lookup font.GSUBLookup) {
	for _, subtable := range lookup.Subtables {
		switch s := subtable.(type) {
		case tables.SingleSubs:
			switch d := s.Data.(type) {
			case tables.SingleSubstData1:
				eachCoverageGlyph(d.Coverage, func(g uint16) {
					hashSubstitute(substs, g, deltaGlyph(g, d.DeltaGlyphID))
				})
			case tables.SingleSubstData2:
				i := 0
				eachCoverageGlyph(d.Coverage, func(g uint16) {
					hashSubstitute(substs, g, d.SubstituteGlyphIDs[i])
					i++
				})
			}
		case tables.AlternateSubs:
			i := 0
			eachCoverageGlyph(s.Coverage, func(g uint16) {
				for _, alt := range s.AlternateSets[i].AlternateGlyphIDs {
					hashSubstitute(substs, g, alt)
				}
				i++
			})
		}
	}
}

func deltaGlyph(g uint16, delta int16) uint16 {
	return g + uint16(delta) //nolint:gosec // GSUB adds deltaGlyphID modulo 65536
}
