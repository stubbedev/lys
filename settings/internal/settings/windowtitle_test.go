package settings

import (
	"testing"

	"github.com/stubbedev/lys/ui/widget"
)

func windowTitleCards(t *testing.T, yaml string) (*kit, *choiceCards) {
	t.Helper()
	k := testKit(t, yaml)
	c, ok := fieldControl(k, k.store, pageRow(t, "window-title", windowTitleFormatPath)).(*choiceCards)
	if !ok {
		t.Fatal("what the window title shows is not picture cards")
	}
	return k, c
}

func chosenCard(c *choiceCards) (string, bool) {
	for _, card := range c.cards {
		if card.value == c.current {
			return card.value, true
		}
	}
	return "", false
}

func TestTheWindowTitleShowsTheAppTheTitleOrBoth(t *testing.T) {
	k, c := windowTitleCards(t, "")
	if v, ok := chosenCard(c); !ok || v != "{{ title }}" {
		t.Fatalf("the default shows the card %q (%v), want Window Title", v, ok)
	}
	c.cards[2].PressAt(widget.Point{})
	if v := k.store.value(windowTitleFormatPath); v != "{{ app }}: {{ title }}" {
		t.Errorf("Both stored %v", v)
	}
}

func TestAWindowTitleFormatOfYourOwnChoosesNoCard(t *testing.T) {
	_, c := windowTitleCards(t, "modules:\n  window-title:\n    format: '[{{ app }}]'\n")
	if v, ok := chosenCard(c); ok {
		t.Errorf("a custom format shows the card %q chosen", v)
	}
}
