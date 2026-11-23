package harfbuzz

import (
	"slices"

	"github.com/stubbedev/lys/lib/third_party/typesetting/font"
	"github.com/stubbedev/lys/lib/third_party/typesetting/font/opentype/tables"
	"github.com/stubbedev/lys/lib/third_party/typesetting/language"
)

// AppendOTScriptTags appends to dst the OpenType script tags of script,
// at most three, as hb_ot_tags_from_script_and_language gives them
// without a language.
func AppendOTScriptTags(dst []tables.Tag, script language.Script) []tables.Tag {
	return appendTagsFromScript(dst, script)
}

// CollectAllLookups returns, ascending, the lookups that any feature of
// any language system of scripts reaches in layout, feature variations
// included, as hb_ot_layout_collect_lookups with no language and no
// feature filter.
func CollectAllLookups(layout *font.Layout, scripts []tables.Tag) []uint16 {
	return collectLookups(layout, scripts, nil)
}

// CollectFeatureLookups returns, ascending, the lookups that feature
// reaches in any language system of scripts in layout, as
// hb_ot_layout_collect_lookups with no language and one feature.
func CollectFeatureLookups(layout *font.Layout, scripts []tables.Tag, feature tables.Tag) []uint16 {
	return collectLookups(layout, scripts, &feature)
}

func collectLookups(layout *font.Layout, scripts []tables.Tag, feature *tables.Tag) []uint16 {
	if !slices.ContainsFunc(scripts, func(tag tables.Tag) bool { return layout.FindScript(tag) >= 0 }) {
		return nil
	}
	var filter []uint16
	if feature != nil {
		for i, f := range layout.Features {
			if f.Tag == *feature {
				filter = append(filter, uint16(i))
			}
		}
		if len(filter) == 0 {
			return nil
		}
	}
	var features []uint16
	collectLangSys := func(l *tables.LangSys) {
		if filter == nil {
			if l.RequiredFeatureIndex != 0xFFFF {
				features = append(features, l.RequiredFeatureIndex)
			}
			features = append(features, l.FeatureIndices...)
			return
		}
		for _, fi := range l.FeatureIndices {
			if slices.Contains(filter, fi) {
				features = append(features, fi)
			}
		}
	}
	var visited []int
	for _, tag := range scripts {
		si := layout.FindScript(tag)
		if si < 0 || slices.Contains(visited, si) {
			continue
		}
		visited = append(visited, si)
		s := &layout.Scripts[si]
		if s.DefaultLangSys != nil {
			collectLangSys(s.DefaultLangSys)
		}
		for i := range s.LangSys {
			collectLangSys(&s.LangSys[i])
		}
	}
	slices.Sort(features)
	features = slices.Compact(features)
	var lookups []uint16
	for _, fi := range features {
		if int(fi) < len(layout.Features) {
			lookups = append(lookups, layout.Features[fi].LookupListIndices...)
		}
	}
	for _, record := range layout.FeatureVariations {
		for _, sub := range record.Substitutions.Substitutions {
			if _, ok := slices.BinarySearch(features, sub.FeatureIndex); ok {
				lookups = append(lookups, sub.AlternateFeature.LookupListIndices...)
			}
		}
	}
	slices.Sort(lookups)
	return slices.Compact(lookups)
}

const collectNestingLevel = 64

// CollectGSUBOutput passes to add every glyph that lookup of gsub can
// substitute in, lookups its contextual subtables call included, as
// hb_ot_layout_lookup_collect_glyphs gives them in its output set.
func CollectGSUBOutput(gsub *font.GSUB, lookup uint16, add func(GID)) {
	c := gsubOutputCollector{gsub: gsub, add: add, recursed: map[uint16]bool{}}
	c.collect(lookup, collectNestingLevel)
}

type gsubOutputCollector struct {
	gsub     *font.GSUB
	add      func(GID)
	recursed map[uint16]bool
}

func (c *gsubOutputCollector) recurse(records []tables.SequenceLookupRecord, nestingLeft int) {
	for _, r := range records {
		if nestingLeft == 0 || c.recursed[r.LookupListIndex] {
			continue
		}
		c.collect(r.LookupListIndex, nestingLeft-1)
		c.recursed[r.LookupListIndex] = true
	}
}

func (c *gsubOutputCollector) addAll(glyphs []tables.GlyphID) {
	for _, g := range glyphs {
		c.add(GID(g))
	}
}

func (c *gsubOutputCollector) collect(lookup uint16, nestingLeft int) {
	if int(lookup) >= len(c.gsub.Lookups) {
		return
	}
	for _, subtable := range c.gsub.Lookups[lookup].Subtables {
		covered := 0
		if cov := subtable.Cov(); cov != nil {
			covered = cov.Len()
		}
		switch s := subtable.(type) {
		case tables.SingleSubs:
			switch d := s.Data.(type) {
			case tables.SingleSubstData1:
				eachCovered(d.Coverage, func(g tables.GlyphID) bool {
					c.add(GID((uint32(g) + uint32(d.DeltaGlyphID)) & 0xFFFF))
					return true
				})
			case tables.SingleSubstData2:
				c.addAll(d.SubstituteGlyphIDs[:min(covered, len(d.SubstituteGlyphIDs))])
			}
		case tables.MultipleSubs:
			for _, seq := range s.Sequences[:min(covered, len(s.Sequences))] {
				c.addAll(seq.SubstituteGlyphIDs)
			}
		case tables.AlternateSubs:
			for _, set := range s.AlternateSets[:min(covered, len(s.AlternateSets))] {
				c.addAll(set.AlternateGlyphIDs)
			}
		case tables.LigatureSubs:
			for _, set := range s.LigatureSets[:min(covered, len(s.LigatureSets))] {
				for _, lig := range set.Ligatures {
					c.add(GID(lig.LigatureGlyph))
				}
			}
		case tables.ContextualSubs:
			switch d := s.Data.(type) {
			case tables.ContextualSubs1:
				c.recurseRuleSets(d.SeqRuleSet, nestingLeft)
			case tables.ContextualSubs2:
				c.recurseRuleSets(d.ClassSeqRuleSet, nestingLeft)
			case tables.ContextualSubs3:
				c.recurse(d.SeqLookupRecords, nestingLeft)
			}
		case tables.ChainedContextualSubs:
			switch d := s.Data.(type) {
			case tables.ChainedContextualSubs1:
				c.recurseChainedRuleSets(d.ChainedSeqRuleSet, nestingLeft)
			case tables.ChainedContextualSubs2:
				c.recurseChainedRuleSets(d.ChainedClassSeqRuleSet, nestingLeft)
			case tables.ChainedContextualSubs3:
				c.recurse(d.SeqLookupRecords, nestingLeft)
			}
		case tables.ReverseChainSingleSubs:
			c.addAll(s.SubstituteGlyphIDs)
		}
	}
}

func (c *gsubOutputCollector) recurseRuleSets(sets []tables.SequenceRuleSet, nestingLeft int) {
	for _, set := range sets {
		for _, rule := range set.SeqRule {
			c.recurse(rule.SeqLookupRecords, nestingLeft)
		}
	}
}

func (c *gsubOutputCollector) recurseChainedRuleSets(sets []tables.ChainedSequenceRuleSet, nestingLeft int) {
	for _, set := range sets {
		for _, rule := range set.ChainedSeqRules {
			c.recurse(rule.SeqLookupRecords, nestingLeft)
		}
	}
}

// CollectGPOSInput passes to add every glyph that lookup of gpos
// positions or matches, as hb_ot_layout_lookup_collect_glyphs gives them
// in its input set.
func CollectGPOSInput(gpos *font.GPOS, lookup uint16, add func(GID)) {
	if int(lookup) >= len(gpos.Lookups) {
		return
	}
	for _, subtable := range gpos.Lookups[lookup].Subtables {
		switch s := subtable.(type) {
		case tables.SinglePos, tables.CursivePos:
			collectCoverage(s.Cov(), add)
		case tables.PairPos:
			if !collectCoverage(s.Cov(), add) {
				continue
			}
			switch d := s.Data.(type) {
			case tables.PairPosData1:
				for _, set := range d.PairSets {
					for g := range set.SecondGlyphs() {
						add(GID(g))
					}
				}
			case tables.PairPosData2:
				collectClassDefCoverage(d.ClassDef2, add)
			}
		case tables.MarkBasePos:
			if collectCoverage(s.Cov(), add) {
				collectCoverage(s.BaseCoverage, add)
			}
		case tables.MarkLigPos:
			if collectCoverage(s.MarkCoverage, add) {
				collectCoverage(s.LigatureCoverage, add)
			}
		case tables.MarkMarkPos:
			if collectCoverage(s.Mark1Coverage, add) {
				collectCoverage(s.Mark2Coverage, add)
			}
		case tables.ContextualPos:
			collectCoverage(s.Cov(), add)
			switch d := s.Data.(type) {
			case tables.ContextualPos1:
				for _, set := range d.SeqRuleSet {
					for _, rule := range set.SeqRule {
						for _, g := range rule.InputSequence {
							add(GID(g))
						}
					}
				}
			case tables.ContextualPos2:
				for _, set := range d.ClassSeqRuleSet {
					for _, rule := range set.SeqRule {
						for _, class := range rule.InputSequence {
							collectClass(d.ClassDef, uint16(class), add)
						}
					}
				}
			case tables.ContextualPos3:
				for _, cov := range d.Coverages[min(1, len(d.Coverages)):] {
					collectCoverage(cov, add)
				}
			}
		case tables.ChainedContextualPos:
			collectCoverage(s.Cov(), add)
			switch d := s.Data.(type) {
			case tables.ChainedContextualPos1:
				for _, set := range d.ChainedSeqRuleSet {
					for _, rule := range set.ChainedSeqRules {
						for _, g := range rule.InputSequence {
							add(GID(g))
						}
					}
				}
			case tables.ChainedContextualPos2:
				for _, set := range d.ChainedClassSeqRuleSet {
					for _, rule := range set.ChainedSeqRules {
						for _, class := range rule.InputSequence {
							collectClass(d.InputClassDef, uint16(class), add)
						}
					}
				}
			case tables.ChainedContextualPos3:
				for _, cov := range d.InputCoverages[min(1, len(d.InputCoverages)):] {
					collectCoverage(cov, add)
				}
			}
		}
	}
}

func eachCovered(cov tables.Coverage, yield func(tables.GlyphID) bool) {
	switch cov := cov.(type) {
	case tables.Coverage1:
		for _, g := range cov.Glyphs {
			if !yield(g) {
				return
			}
		}
	case tables.Coverage2:
		for _, r := range cov.Ranges {
			for g := uint32(r.StartGlyphID); g <= uint32(r.EndGlyphID); g++ {
				if !yield(tables.GlyphID(g)) {
					return
				}
			}
		}
	}
}

func addRange(first, last uint32, add func(GID)) bool {
	if first > last {
		return false
	}
	for g := first; g <= last; g++ {
		add(GID(g))
	}
	return true
}

func collectCoverage(cov tables.Coverage, add func(GID)) bool {
	switch cov := cov.(type) {
	case tables.Coverage1:
		for i, g := range cov.Glyphs {
			if i > 0 && g < cov.Glyphs[i-1] {
				return false
			}
			add(GID(g))
		}
		return true
	case tables.Coverage2:
		for _, r := range cov.Ranges {
			if !addRange(uint32(r.StartGlyphID), uint32(r.EndGlyphID), add) {
				return false
			}
		}
		return true
	}
	return false
}

func collectClassDefCoverage(cd tables.ClassDef, add func(GID)) bool {
	switch cd := cd.(type) {
	case tables.ClassDef1:
		start := uint32(cd.StartGlyphID)
		run := 0
		for i, class := range cd.ClassValueArray {
			if class != 0 {
				continue
			}
			if run != i && !addRange(start+uint32(run), start+uint32(i), add) {
				return false
			}
			run = i + 1
		}
		if n := len(cd.ClassValueArray); run != n {
			return addRange(start+uint32(run), start+uint32(n), add)
		}
		return true
	case tables.ClassDef2:
		for _, r := range cd.ClassRangeRecords {
			if r.Class != 0 && !addRange(uint32(r.StartGlyphID), uint32(r.EndGlyphID), add) {
				return false
			}
		}
		return true
	}
	return false
}

func collectClass(cd tables.ClassDef, class uint16, add func(GID)) {
	switch cd := cd.(type) {
	case tables.ClassDef1:
		for i, c := range cd.ClassValueArray {
			if c == class {
				add(GID(uint32(cd.StartGlyphID) + uint32(i)))
			}
		}
	case tables.ClassDef2:
		for _, r := range cd.ClassRangeRecords {
			if r.Class == class && !addRange(uint32(r.StartGlyphID), uint32(r.EndGlyphID), add) {
				return
			}
		}
	}
}

// WouldSubstitute reports whether GSUB lookup of f would substitute
// glyphs, as hb_ot_layout_lookup_would_substitute.
func (f *Font) WouldSubstitute(lookup uint16, glyphs []GID, zeroContext bool) bool {
	return otLayoutLookupWouldSubstitute(f, lookup, glyphs, zeroContext)
}
