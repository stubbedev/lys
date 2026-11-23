package settings

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/settings/internal/locale"
	"github.com/stubbedev/lys/settings/internal/wmconfig"
	"github.com/stubbedev/lys/settings/internal/wmtest"
)

func TestAWorkspaceRuleIsACardTitledByItsNameNumberOrMatch(t *testing.T) {
	k, w, _, _ := wmKit(t, "workspaces:\n  - {id: 3, name: web}\n  - {id: 4}\n  - {match: single_tiled}\n")
	c := fieldControl(k, w, wmWorkspacesRow()).(*cardList)
	var titles []string
	for _, cd := range c.cards {
		titles = append(titles, unisolated(cd.title.Text()))
	}
	if want := []string{"web", "Workspace 4", "With one tiled window"}; !slices.Equal(titles, want) {
		t.Fatalf("titles %q, want %q", titles, want)
	}
}

func TestANewWorkspaceRuleTakesTheLowestFreeNumberAndKeepsUnknownKeys(t *testing.T) {
	k, w, fake, files := wmKit(t, "workspaces:\n  - {id: 1, name: web, frobnicate: 7}\n  - {id: 2}\n")
	c := fieldControl(k, w, wmWorkspacesRow()).(*cardList)
	c.appendItem(c.spec.blank())
	got := fileText(t, files.runtime)
	if !strings.Contains(got, "id: 3") || !strings.Contains(got, "frobnicate: 7") || fake.checks == 0 {
		t.Fatalf("adding a rule wrote %q after %d checks", got, fake.checks)
	}
	if n := nextWorkspaceID([]any{map[string]any{"id": int64(2)}, map[string]any{"match": "any"}}); n != 1 {
		t.Errorf("the lowest free number is %d, want 1", n)
	}
}

func TestAWorkspaceRuleLysWMRefusesIsHeldAsADraft(t *testing.T) {
	k, w, fake, files := wmKit(t, "workspaces:\n  - {id: 1}\n")
	c := fieldControl(k, w, wmWorkspacesRow()).(*cardList)
	fake.reject = "workspaces[1]: needs exactly one of id and match"
	c.appendItem(map[string]any{"id": int64(2), "match": "any"})
	noFile(t, files.runtime)
	if !failedWith(c.draft, locale.T().Get("settings-list-draft"), "needs exactly one of id and match") {
		t.Fatalf("the refused rule shows %q over %q (visible %v)", c.draft.Text(), c.draft.detail.Text(), c.draft.Visible())
	}
}

func TestAWorkspaceRuleTakesItsEditorsFromTheSchema(t *testing.T) {
	k, w, _, files := wmKit(t, "workspaces:\n  - {id: 1}\n")
	c := fieldControl(k, w, wmWorkspacesRow()).(*cardList)
	openCard(c, 0)
	byLabel := map[string]control{}
	for i, f := range c.spec.fields {
		byLabel[f.label] = c.cards[0].controls[i]
	}
	if _, ok := byLabel["wm-workspaces-gaps-outer"].(*sidesEditor); !ok {
		t.Errorf("gaps.outer takes %T, want the sides editor", byLabel["wm-workspaces-gaps-outer"])
	}
	if _, ok := byLabel["wm-workspaces-output"].(*choiceEditor); !ok {
		t.Errorf("the output takes %T, want a choice of displays", byLabel["wm-workspaces-output"])
	}
	if _, ok := byLabel["wm-workspaces-on-created-empty"].(*choiceEditor); !ok {
		t.Errorf("on_created_empty takes %T, want a choice of apps", byLabel["wm-workspaces-on-created-empty"])
	}
	layout, ok := byLabel["wm-workspaces-layout"].(*choiceControl)
	if !ok || layout.choice.unset == "" {
		t.Fatalf("the layout takes %T, want the inherit-or-pick choice", byLabel["wm-workspaces-layout"])
	}
	if got := shownValues(layout)[1:]; !slices.Equal(got, []string{"scroll", "tree"}) {
		t.Errorf("the layout offers %q", got)
	}
	sides := byLabel["wm-workspaces-gaps-outer"].(*sidesEditor)
	sides.spins[0].OnValueChanged(12)
	if got := fileText(t, files.runtime); !strings.Contains(got, "outer:") {
		t.Fatalf("setting the top gap wrote %q", got)
	}
}

func TestAnAppCommandChoiceListsInstalledAppsByName(t *testing.T) {
	dockAppsFixture(t)
	choices := appCommands()
	i := slices.IndexFunc(choices, func(c choice) bool { return c.label == "Firefox" })
	if i < 0 || choices[i].value != "true" {
		t.Fatalf("the installed apps offered as commands: %+v", choices)
	}
	if slices.ContainsFunc(choices, func(c choice) bool { return c.label == "Hidden" }) {
		t.Error("an app hidden from launchers is offered")
	}
}

func TestScratchpadsAreCardsNamedByTheirKey(t *testing.T) {
	k, w, _, files := wmKit(t, "scratchpads:\n  term: {exec: foot, size: 80%x60%, frobnicate: 1}\n")
	c := fieldControl(k, w, wmScratchpadsRow()).(*cardList)
	if len(c.cards) != 1 || c.cards[0].identity.(*liveText).Text() != "term" {
		t.Fatalf("%d cards", len(c.cards))
	}
	if err := c.fieldSlot(0, itemNameKey).set("console"); err != nil {
		t.Fatal(err)
	}
	got := fileText(t, files.runtime)
	if !strings.Contains(got, "console:") || !strings.Contains(got, "frobnicate: 1") || !strings.Contains(got, "exec: foot") {
		t.Fatalf("renaming wrote %q", got)
	}
	c.appendItem(c.spec.blank())
	if got := fileText(t, files.runtime); strings.Count(got, "exec:") != 1 || strings.Contains(got, `"":`) {
		t.Fatalf("an unnamed scratchpad was written: %q", got)
	}
}

func TestNamedItemsRefuseANamelessOrRepeatedItem(t *testing.T) {
	if _, err := namedItems([]any{map[string]any{"exec": "foot"}}); err == nil {
		t.Error("an item without a name was taken")
	}
	if _, err := namedItems([]any{map[string]any{"name": "a"}, map[string]any{"name": "a"}}); err == nil {
		t.Error("a name given twice was taken")
	}
	var stored any
	s := namedItemsSlot(slot{get: func() any { return stored }, set: func(v any) error { stored = v; return nil }, unset: func() { stored = nil }})
	if err := s.set([]any{map[string]any{"name": "b", "x": 1}, map[string]any{"name": "a"}}); err != nil {
		t.Fatal(err)
	}
	if want := map[string]any{"a": map[string]any{}, "b": map[string]any{"x": 1}}; !reflect.DeepEqual(stored, want) {
		t.Fatalf("stored %v, want %v", stored, want)
	}
	if got := s.get().([]any); asMap(got[0])["name"] != "a" || asMap(got[1])["name"] != "b" {
		t.Errorf("read back %v, want the names in order", got)
	}
	if err := s.set([]any{}); err != nil || stored != nil {
		t.Errorf("no items left %v (%v), want the key dropped", stored, err)
	}
}

func TestAScratchpadSizeIsTwoPercentages(t *testing.T) {
	var stored any = "80%x60%"
	s := slot{get: func() any { return stored }, set: func(v any) error { stored = v; return nil }, unset: func() { stored = nil }}
	k := testKit(t, "")
	c := newPercentSizeEditor(k, s, nil).(*percentSizeEditor)
	if c.sides[0].Value() != 80 || c.sides[1].Value() != 60 || c.unknown.Visible() {
		t.Fatalf("80%%x60%% shows %v by %v (unknown %v)", c.sides[0].Value(), c.sides[1].Value(), c.unknown.Visible())
	}
	c.sides[1].SetValue(45)
	c.write()
	if stored != "80%x45%" {
		t.Fatalf("stored %v", stored)
	}
	stored = "800x600"
	c.refresh()
	if !c.unknown.Visible() || stored != "800x600" {
		t.Errorf("a size of another form shows unknown %v and became %v", c.unknown.Visible(), stored)
	}
	for _, bad := range []string{"0%x50%", "101%x50%", "50%", "50%x"} {
		if _, _, ok := parsePercentSize(bad); ok {
			t.Errorf("%q was taken", bad)
		}
	}
}

func TestWorkspaceAndScratchpadCardsWriteWhatLysWMAccepts(t *testing.T) {
	wmtest.Need(t)
	k := testKit(t, "")
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	w, err := openWM(k, wmconfig.Exec)
	if err != nil {
		t.Fatal(err)
	}
	k.wm = w
	ws := fieldControl(k, w, wmWorkspacesRow()).(*cardList)
	ws.appendItem(ws.spec.blank())
	if ws.draft.Visible() {
		t.Fatalf("a new workspace rule is refused: %s", ws.draft.Text())
	}
	for key, v := range map[string]any{"layout": "scroll", "name": "web", "persistent": true} {
		if err := ws.fieldSlot(0, key).set(v); err != nil {
			t.Fatal(err)
		}
	}
	if err := nestedSlot(ws.fieldSlot(0, "gaps"), []string{"outer"}).set([]any{int64(1), int64(2), int64(3), int64(4)}); err != nil {
		t.Fatal(err)
	}
	if ws.draft.Visible() {
		t.Fatalf("lys-wm refuses the rule the card wrote: %s", ws.draft.Text())
	}
	sp := fieldControl(k, w, wmScratchpadsRow()).(*cardList)
	sp.appendItem(map[string]any{"name": "term", "exec": "foot", "size": "80%x60%"})
	if sp.draft.Visible() {
		t.Fatalf("lys-wm refuses the scratchpad the card wrote: %s", sp.draft.Text())
	}
	if err := w.file.Check(nil); err != nil {
		t.Fatalf("lys-wm refuses what the cards wrote: %v", err)
	}
	ws.appendItem(map[string]any{"id": int64(0)})
	var rejected *wmconfig.RejectedError
	if !ws.draft.Visible() || !errors.As(w.file.Check([]wmconfig.Edit{{Path: []string{"workspaces"}, Value: []any{map[string]any{"id": 0}}}}), &rejected) {
		t.Errorf("workspace 0 is not refused: draft %q", ws.draft.Text())
	}
}
