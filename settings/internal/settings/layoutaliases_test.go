package settings

import (
	"maps"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func aliasEditor(t *testing.T, stored *any, codes []string) *choiceListEditor {
	t.Helper()
	k := testKit(t, "")
	names := listFixtures[listLayoutNames]
	return newChoiceListEditor(k, memorySlot(stored, nil), func() []choice { return pinLayouts(names, codes) }, namedChoices)
}

func layOutChoiceList(c *choiceListEditor) {
	sz := widget.SizeWithin(c, widget.Size{W: 700, H: 4000})
	c.Arrange(render.Rect{W: 700, H: sz.H})
}

func orderedAliasRows(t *testing.T, c *choiceListEditor) []namedChoiceItem {
	t.Helper()
	out := make([]namedChoiceItem, len(c.items))
	for i, it := range c.items {
		out[i] = it.(namedChoiceItem)
	}
	if len(out) < 2 {
		t.Fatalf("%d rows, want at least two to compare", len(out))
	}
	return out
}

func fieldsShareOneColumn(t *testing.T, rows []namedChoiceItem) {
	t.Helper()
	first := rows[0].name.Bounds()
	for _, row := range rows[1:] {
		if b := row.name.Bounds(); b.X != first.X || b.W != first.W {
			t.Errorf("%q's field at x %d, %dpx wide; %q's at x %d, %dpx wide: want one x and one width", row.key, b.X, b.W, rows[0].key, first.X, first.W)
		}
	}
}

func readsWhole(t *testing.T, l *widget.Label) {
	t.Helper()
	if whole := widget.SizeWithin(l, widget.Size{W: 1 << 20, H: 1 << 20}).W; l.Bounds().W < whole {
		t.Errorf("%q is cut to %dpx of %dpx", l.Text(), l.Bounds().W, whole)
	}
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

func TestShortNameFieldsStartAtOneXWhateverTheCodeBesideTheName(t *testing.T) {
	var stored any = map[string]any{"English (US)": "EN", "German (no dead keys)": "DE", "Czech (QWERTY)": "CZ"}
	c := aliasEditor(t, &stored, []string{"us"})
	layOutChoiceList(c)
	rows := orderedAliasRows(t, c)
	fieldsShareOneColumn(t, rows)
	want := map[string]string{"English (US)": "us", "German (no dead keys)": "de(nodeadkeys)", "Czech (QWERTY)": locale.T().Get("settings-choice-unknown")}
	for _, row := range rows {
		hints := treetest.WithClass(row, "choice-hint")
		if len(hints) != 1 {
			t.Fatalf("%q shows %d code hints, want one", row.key, len(hints))
		}
		hint := hints[0].(*widget.Label)
		if hint.Text() != want[row.key] {
			t.Errorf("%q's hint reads %q, want %q", row.key, hint.Text(), want[row.key])
		}
		readsWhole(t, hint)
		name := treetest.LabelReading(t, row, row.key)
		h, n, f := hint.Bounds(), name.Bounds(), row.name.Bounds()
		if h.Y < n.Y+n.H || h.X != n.X {
			t.Errorf("%q's hint at %v is not a caption under its name at %v", row.key, h, n)
		}
		if h.X+h.W > f.X {
			t.Errorf("%q's hint at %v reaches into its field at %v", row.key, h, f)
		}
	}
}

func TestNamedChoicesWithoutCodesKeepOneLineRows(t *testing.T) {
	var stored any = map[string]any{"Alpha": "A", "A much longer choice name": "L"}
	k := testKit(t, "")
	listed := []choice{{value: "Alpha", label: "Alpha", pinned: true}, {value: "A much longer choice name", label: "A much longer choice name", pinned: true}}
	c := newChoiceListEditor(k, memorySlot(&stored, nil), func() []choice { return listed }, namedChoices)
	layOutChoiceList(c)
	rows := orderedAliasRows(t, c)
	fieldsShareOneColumn(t, rows)
	for _, row := range rows {
		if hints := treetest.WithClass(row, "choice-hint"); len(hints) != 0 {
			t.Errorf("%q shows a code hint though its choice has none", row.key)
		}
		name := treetest.LabelReading(t, row, row.key)
		readsWhole(t, name)
		if h := row.Bounds().H; h != max(name.Bounds().H, row.name.Bounds().H) {
			t.Errorf("%q's row is %dpx tall, want one line of name or field", row.key, h)
		}
	}
}
