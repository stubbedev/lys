package settings

import (
	"maps"
	"slices"
	"testing"
)

func aliasEditor(t *testing.T, stored *any, codes []string) *choiceListEditor {
	t.Helper()
	k := testKit(t, "")
	names := listFixtures[listLayoutNames]
	return newChoiceListEditor(k, memorySlot(stored, nil), func() []choice { return pinLayouts(names, codes) }, namedChoices)
}

func aliasRows(c *choiceListEditor) map[string]namedChoiceItem {
	out := map[string]namedChoiceItem{}
	for _, it := range c.items {
		row := it.(namedChoiceItem)
		out[row.key] = row
	}
	return out
}

func TestTheConfiguredLayoutsAreListedForAShortName(t *testing.T) {
	var stored any = map[string]any{}
	c := aliasEditor(t, &stored, []string{"us", "de(nodeadkeys)"})
	rows := aliasRows(c)
	if len(rows) != 2 {
		t.Fatalf("rows %v, want the two configured layouts", slices.Collect(maps.Keys(rows)))
	}
	if _, ok := rows["German"]; ok {
		t.Error("German is listed though only its no-dead-keys variant is configured")
	}
	if got := stored.(map[string]any); len(got) != 0 {
		t.Errorf("listing the layouts stored %v", got)
	}
	row := rows["English (US)"]
	row.name.SetText("EN")
	row.name.OnChanged("EN")
	if got := stored.(map[string]any); len(got) != 1 || got["English (US)"] != "EN" {
		t.Errorf("naming English (US) stored %v", got)
	}
}

func TestAnAliasForALayoutNoLongerConfiguredStaysListed(t *testing.T) {
	var stored any = map[string]any{"Czech (QWERTY)": "CZ"}
	c := aliasEditor(t, &stored, []string{"us"})
	rows := aliasRows(c)
	if _, ok := rows["Czech (QWERTY)"]; !ok || len(rows) != 2 {
		t.Fatalf("rows %v, want the configured layout and the old alias", rows)
	}
	if rows["Czech (QWERTY)"].name.Text() != "CZ" {
		t.Errorf("the old alias reads %q", rows["Czech (QWERTY)"].name.Text())
	}
}

func TestLayoutCodesPairEachLayoutWithItsVariant(t *testing.T) {
	got := layoutCodes(keyboardNames{layout: "us,de,fr", variant: ",nodeadkeys"})
	if want := []string{"us", "de(nodeadkeys)", "fr"}; !slices.Equal(got, want) {
		t.Errorf("codes %v, want %v", got, want)
	}
	if got := layoutCodes(keyboardNames{}); len(got) != 0 {
		t.Errorf("no layout gave codes %v", got)
	}
}

func TestLayoutNamesAreTheDescriptionsUnderTheirCodes(t *testing.T) {
	x := xkbLists{
		layouts:  []choice{{value: "us", label: "English (US)"}},
		variants: []choice{{value: "intl", label: "English (US, intl., with dead keys)", group: "us"}},
	}
	got := layoutNames(x)
	if len(got) != 2 || got[0].value != "English (US)" || got[0].group != "us" || got[1].group != "us(intl)" {
		t.Errorf("names %+v", got)
	}
}

func TestTheKeyboardInputAliasesArePickedNotTyped(t *testing.T) {
	k := testKit(t, "")
	if _, ok := fieldControl(k, k.store, pageRow(t, "keyboard-input", "modules.keyboard-input.layout-alias-map")).(*choiceListEditor); !ok {
		t.Error("the layout aliases are a free map")
	}
}
