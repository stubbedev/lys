package text

import "github.com/rivo/uniseg"

// ellipsis is what Ellipsize ends a shortened text with: one character.
const ellipsis = "…"

// Ellipsize shortens s for display to at most maxChars characters, a
// character being a grapheme cluster, so neither a multi-byte rune nor
// an emoji sequence or an accented letter is ever cut in half. A text
// that fits comes back as it is; a longer one keeps its first
// maxChars-1 characters and ends in an ellipsis. Below one character
// nothing fits, and the result is empty.
func Ellipsize(s string, maxChars int) string {
	if maxChars < 1 {
		return ""
	}
	rest, state, keep := s, -1, 0
	for n := 0; rest != ""; n++ {
		if n == maxChars {
			return s[:keep] + ellipsis
		}
		if n == maxChars-1 {
			keep = len(s) - len(rest)
		}
		_, rest, _, state = uniseg.FirstGraphemeClusterInString(rest, state)
	}
	return s
}
