package settings

import (
	"slices"
	"strings"
	"testing"
)

// A picture card's name broke off at the card's width ("Catppuccin…",
// "Tokyo Nigh…"). Every theme preset's name now reads whole on its
// 104px card, on two lines where one is too narrow, while a name that
// cannot fit two lines still ends in an ellipsis.
func TestEveryThemePresetNameReadsWholeOnItsCard(t *testing.T) {
	k := testKit(t, "")
	c := themeCards(k)
	themed(k, c)
	w, px := c.cards[0].labelRect().W, k.textPx()
	wrappedNames := 0
	for _, card := range c.cards {
		lines := cardLabelLines(k.face, card.label, w, px)
		if len(lines) > 2 || strings.Join(lines, " ") != strings.Join(strings.Fields(card.label), " ") {
			t.Errorf("%q reads %q on its card", card.label, lines)
		}
		if len(lines) == 2 {
			wrappedNames++
		}
	}
	if wrappedNames == 0 {
		t.Error("no theme preset's name takes two lines; the test no longer covers a wrapped name")
	}
	if got := cardLabelLines(k.face, "Dark", w, px); !slices.Equal(got, []string{"Dark"}) {
		t.Errorf("a short name reads %q", got)
	}
	long := cardLabelLines(k.face, "An extraordinarily long theme name that cannot fit", w, px)
	if len(long) != 2 || !strings.HasSuffix(long[1], "…") {
		t.Errorf("a name past two lines reads %q, want two lines ending in an ellipsis", long)
	}
	word := cardLabelLines(k.face, "Supercalifragilisticexpialidocious", w, px)
	if len(word) != 1 || !strings.HasSuffix(word[0], "…") {
		t.Errorf("one word wider than the card reads %q, want one cut line", word)
	}
}
