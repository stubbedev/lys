package settings

import (
	"reflect"
	"testing"
)

const (
	quicklinksPath = "launcher.search.quicklinks"
	snippetsPath   = "launcher.search.snippets"
)

func cardsAt(t *testing.T, k *kit, page, path string) *cardList {
	t.Helper()
	c, ok := fieldControl(k, k.store, pageRow(t, page, path)).(*cardList)
	if !ok {
		t.Fatalf("%s is not edited as cards", path)
	}
	return c
}

func TestQuickLinksAreCardsAndKeepTheirValuesUntouched(t *testing.T) {
	k := testKit(t, "launcher:\n  search:\n    quicklinks:\n      - name: GitHub\n        url: https://github.com/search?q={query}\n        keyword: gh\n")
	before := k.store.value(quicklinksPath)
	c := cardsAt(t, k, "launcher-modes", quicklinksPath)
	if len(c.cards) != 1 || !reflect.DeepEqual(k.store.value(quicklinksPath), before) {
		t.Fatalf("%d cards; stored %v, was %v", len(c.cards), k.store.value(quicklinksPath), before)
	}
	if _, ok := c.cards[0].controls[0].(*labelBuilder); !ok {
		t.Errorf("the address is a %T, not the builder offering {query}", c.cards[0].controls[0])
	}
	if err := c.fieldSlot(0, "keyword").set("hub"); err != nil {
		t.Fatal(err)
	}
	want := []any{map[string]any{"name": "GitHub", "url": "https://github.com/search?q={query}", "keyword": "hub", "icon": ""}}
	if got := k.store.value(quicklinksPath); !reflect.DeepEqual(got, want) {
		t.Errorf("stored %v, want %v", got, want)
	}
}

func TestANewQuickLinkIsWrittenOnceNamed(t *testing.T) {
	k := testKit(t, "")
	c := cardsAt(t, k, "launcher-modes", quicklinksPath)
	c.local = append(c.local, c.spec.blank())
	c.rebuild()
	c.commit()
	if got := k.store.value(quicklinksPath); !sameStored(got, []any{}) {
		t.Fatalf("a nameless quick link was written: %v", got)
	}
	_ = c.fieldSlot(0, "icon").set("")
	_ = c.fieldSlot(0, "url").set("https://example.org/{query}")
	_ = c.fieldSlot(0, "name").set("Example")
	want := []any{map[string]any{"name": "Example", "url": "https://example.org/{query}", "keyword": "", "icon": ""}}
	if got := k.store.value(quicklinksPath); !reflect.DeepEqual(got, want) {
		t.Errorf("stored %v, want %v", got, want)
	}
}

func TestASnippetKeepsItsLinesAndIsEditedInAnArea(t *testing.T) {
	k := testKit(t, "launcher:\n  search:\n    snippets:\n      - name: Sign-off\n        text: \"Regards,\\nAda\"\n")
	c := cardsAt(t, k, "launcher-modes", snippetsPath)
	area, ok := c.cards[0].controls[0].(*liveTextArea)
	if !ok || area.Text() != "Regards,\nAda" {
		t.Fatalf("the text is a %T holding %q", c.cards[0].controls[0], area.Text())
	}
	area.SetText("Best,\nAda")
	want := []any{map[string]any{"name": "Sign-off", "text": "Best,\nAda", "keyword": ""}}
	if got := k.store.value(snippetsPath); !reflect.DeepEqual(got, want) {
		t.Errorf("stored %v, want %v", got, want)
	}
}
