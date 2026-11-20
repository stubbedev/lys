package settings

import (
	"reflect"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"

	"github.com/stubbedev/lys/settings/internal/locale"
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

func negateChar(t *testing.T, k *kit) *valueSelect {
	t.Helper()
	c, ok := fieldControl(k, k.store, pageRow(t, "launcher", "launcher.negate-char")).(*valueSelect)
	if !ok {
		t.Fatal("the negation character is not picked from a list")
	}
	return c
}

func TestTheNegationCharacterIsOneOfThreeAndWritesThePickedOne(t *testing.T) {
	k := testKit(t, "")
	c := negateChar(t, k)
	if len(c.Items()) != 3 || c.shown[c.Selected()] != "-" {
		t.Fatalf("offers %q with %q selected", c.Items(), c.shown[c.Selected()])
	}
	c.OnSelect(slices.Index(c.shown, "~"))
	if v := k.store.value("launcher.negate-char"); v != "~" {
		t.Fatalf("stored %v", v)
	}
}

func TestANegationCharacterOutsideTheThreeIsKeptAndShown(t *testing.T) {
	k := testKit(t, "launcher:\n  negate-char: \"^\"\n")
	c := negateChar(t, k)
	if len(c.Items()) != 4 || c.shown[c.Selected()] != "^" {
		t.Fatalf("offers %q with %q selected", c.Items(), c.shown[c.Selected()])
	}
	if v := k.store.value("launcher.negate-char"); v != "^" {
		t.Fatalf("an unlisted character was rewritten to %v", v)
	}
	c.OnSelect(slices.Index(c.shown, "!"))
	c.refresh()
	if len(c.Items()) != 3 || k.store.value("launcher.negate-char") != "!" {
		t.Fatalf("after picking ! it offers %q, stored %v", c.Items(), k.store.value("launcher.negate-char"))
	}
}

func fileExclude(t *testing.T, k *kit) *nameChips {
	t.Helper()
	c, ok := fieldControl(k, k.store, pageRow(t, "launcher-modes", "launcher.search.file-exclude")).(*nameChips)
	if !ok {
		t.Fatal("the skipped folders are not chips")
	}
	return c
}

func TestSkippedFoldersAreExampleChipsThatToggleTheirName(t *testing.T) {
	k := testKit(t, "launcher:\n  search:\n    file-exclude: [build-cache]\n")
	c := fileExclude(t, k)
	if want := append(slices.Clone(skippedFolderExamples), "build-cache"); !slices.Equal(c.labels, want) {
		t.Fatalf("chips %q, want %q", c.labels, want)
	}
	if !c.toggles[len(c.labels)-1].Active() || c.toggles[0].Active() {
		t.Fatal("the chips do not show what is skipped")
	}
	c.toggles[slices.Index(c.labels, "Downloads")].SetActive(true)
	if got := k.store.value("launcher.search.file-exclude"); !reflect.DeepEqual(got, []any{"build-cache", "Downloads"}) {
		t.Fatalf("stored %v", got)
	}
	c.toggles[slices.Index(c.labels, "build-cache")].SetActive(false)
	if got := k.store.value("launcher.search.file-exclude"); !reflect.DeepEqual(got, []any{"Downloads"}) {
		t.Fatalf("stored %v", got)
	}
	if slices.Contains(c.labels, "build-cache") {
		t.Fatal("a removed name of the user's own is still a chip")
	}
}

func TestATypedFolderNameIsAddedOnlyWhenItIsAName(t *testing.T) {
	k := testKit(t, "")
	c := fileExclude(t, k)
	tr := locale.T()
	for typed, key := range map[string]string{"": "settings-folder-name-empty", "src/gen": "settings-folder-name-slash"} {
		c.entry.SetText(typed)
		c.add()
		if !c.problem.Visible() || c.problem.Text() != tr.Get(key) {
			t.Errorf("%q shows %q", typed, c.problem.Text())
		}
		if got := k.store.value("launcher.search.file-exclude"); !reflect.DeepEqual(got, []any{}) {
			t.Fatalf("%q stored %v", typed, got)
		}
	}
	c.entry.SetText(" gen ")
	c.add()
	if got := k.store.value("launcher.search.file-exclude"); !reflect.DeepEqual(got, []any{"gen"}) || c.problem.Visible() || c.entry.Text() != "" {
		t.Fatalf("stored %v, problem %v, entry %q", got, c.problem.Visible(), c.entry.Text())
	}
	c.entry.SetText("gen")
	c.add()
	if c.problem.Text() != tr.Get("settings-folder-name-listed") {
		t.Fatalf("a repeat shows %q", c.problem.Text())
	}
}

func TestTheTerminalIsSetOnDefaultAppsAlone(t *testing.T) {
	if slices.Contains(rowPaths(bothParts, "launcher"), "launcher.terminal") {
		t.Error("the launcher page still holds the terminal")
	}
	p, _ := pageByID(layout(config.Defaults(), bothParts), defaultAppsID)
	if !slices.ContainsFunc(p.panel, func(s panelSetting) bool { return s.path == terminalPath }) {
		t.Error("Default Apps lost the terminal")
	}
}
