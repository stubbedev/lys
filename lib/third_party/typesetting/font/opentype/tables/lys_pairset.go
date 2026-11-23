package tables

import (
	"encoding/binary"
	"iter"
)

// SecondGlyphs yields the second glyph of every pair of ps, in table
// order.
func (ps PairSet) SecondGlyphs() iter.Seq[GlyphID] {
	return func(yield func(GlyphID) bool) {
		recLen := 2 * (1 + ps.data.fmt1.size() + ps.data.fmt2.size())
		for i := range int(ps.pairValueCount) {
			if !yield(GlyphID(binary.BigEndian.Uint16(ps.data.data[2+i*recLen:]))) {
				return
			}
		}
	}
}
