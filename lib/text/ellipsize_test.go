package text

import (
	"strings"
	"testing"

	"github.com/rivo/uniseg"
)

func TestATextThatFitsIsShownWhole(t *testing.T) {
	for _, tt := range []struct {
		in       string
		maxChars int
	}{
		{"", 0},
		{"", 3},
		{"abc", 3},
		{"abc", 4},
		{strings.Repeat("é", 5), 5},
		{"👨‍👩‍👧🇩🇰", 2},
	} {
		if got := Ellipsize(tt.in, tt.maxChars); got != tt.in {
			t.Errorf("Ellipsize(%q, %d) = %q, want it whole", tt.in, tt.maxChars, got)
		}
	}
}

func TestALongTextIsCutToItsCharactersAndEndsInAnEllipsis(t *testing.T) {
	for _, tt := range []struct {
		in       string
		maxChars int
		want     string
	}{
		{"abcdefgh", 5, "abcd…"},
		{"abcd", 3, "ab…"},
		{"ab", 1, "…"},
		{strings.Repeat("é", 40), 10, strings.Repeat("é", 9) + "…"},
		{"ééé", 2, "é…"},
		{"👨‍👩‍👧👨‍👩‍👧👨‍👩‍👧", 2, "👨‍👩‍👧…"},
		{"🇩🇰🇩🇪🇫🇷", 2, "🇩🇰…"},
		{"日本語のテキスト", 4, "日本語…"},
	} {
		got := Ellipsize(tt.in, tt.maxChars)
		if got != tt.want {
			t.Errorf("Ellipsize(%q, %d) = %q, want %q", tt.in, tt.maxChars, got, tt.want)
		}
		if n := uniseg.GraphemeClusterCount(got); n > tt.maxChars {
			t.Errorf("Ellipsize(%q, %d) = %q is %d characters, past the limit", tt.in, tt.maxChars, got, n)
		}
	}
}

func TestNothingFitsInNoCharacters(t *testing.T) {
	for _, maxChars := range []int{0, -1} {
		if got := Ellipsize("abc", maxChars); got != "" {
			t.Errorf("Ellipsize(%q, %d) = %q, want nothing", "abc", maxChars, got)
		}
	}
}
