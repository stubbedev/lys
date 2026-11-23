package settings

import (
	"reflect"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func presetsOf(t *testing.T, p *settingsPage) *columnPresets {
	t.Helper()
	c, ok := rowOf(t, p, columnPresetsPath).ctl.(*columnPresets)
	if !ok {
		t.Fatalf("the %s row holds %T, want the width cards", columnPresetsPath, rowOf(t, p, columnPresetsPath).ctl)
	}
	return c
}

func TestTheColumnWidthPresetsAreCardsOfLysWMsWidths(t *testing.T) {
	k := lysWMKit(t, "")
	p := layoutPage(t, k, "wm-tiling")
	c := presetsOf(t, p)
	if want := []columnWidth{shareWidth(1.0 / 3), shareWidth(0.5), shareWidth(2.0 / 3)}; !reflect.DeepEqual(c.widths, want) {
		t.Fatalf("lys-wm's default presets read as %v, want %v", c.widths, want)
	}
	if c.cards.Len() != 3 {
		t.Fatalf("%d cards for three widths", c.cards.Len())
	}
	pickChoice(c.unit, presetUnitPixels)
	c.amount.SetValue(1280)
	c.addWidth()
	want := []any{1.0 / 3, 0.5, 2.0 / 3, "1280px"}
	if got := k.wm.value(columnPresetsPath); !reflect.DeepEqual(got, want) {
		t.Fatalf("adding 1280 px wrote %v, want %v", got, want)
	}
	if c.unit.Selected() != presetUnitShare || c.amount.Value() != 50 {
		t.Errorf("after adding, the new width reads unit %d amount %v, want half the screen", c.unit.Selected(), c.amount.Value())
	}
	pickChoice(c.unit, presetUnitPixels)
	c.amount.SetValue(1280)
	c.showAddable()
	if c.add.Enabled() {
		t.Error("a width already listed can be added again")
	}
	c.remove(0)
	if got := k.wm.value(columnPresetsPath); !reflect.DeepEqual(got, want[1:]) {
		t.Fatalf("removing the first width wrote %v, want %v", got, want[1:])
	}
	pickChoice(c.unit, presetUnitShare)
	c.amount.SetValue(75)
	c.addWidth()
	if got := c.widths[len(c.widths)-1]; got != shareWidth(0.75) {
		t.Errorf("adding 75%% added %v", got)
	}
	if labels := treetest.Labels(c.cards); len(labels) == 0 || labels[0] != unitPercent.format(0)(50) {
		t.Errorf("the cards read %q, the first want 50%%", labels)
	}
}

func TestTheLastColumnWidthCannotBeRemoved(t *testing.T) {
	k := lysWMKit(t, "scroll:\n  presets: [0.5]\n")
	c := presetsOf(t, layoutPage(t, k, "wm-tiling"))
	if len(c.removes) != 1 || c.removes[0].Enabled() {
		t.Fatal("the only width offers to be removed")
	}
	c.remove(0)
	if got := k.wm.value(columnPresetsPath); !reflect.DeepEqual(got, []any{0.5}) {
		t.Errorf("removing the only width wrote %v", got)
	}
}

func TestAColumnWidthLysWMWouldRefuseIsNamedNotDrawn(t *testing.T) {
	for _, v := range []any{0.0, 1.5, int64(2), "0px", "080px", "12em", true} {
		if w, err := parseColumnWidth(v); err == nil {
			t.Errorf("%v reads as the width %v", v, w)
		}
	}
	for v, want := range map[any]columnWidth{0.25: shareWidth(0.25), int64(1): shareWidth(1), "1280px": pixelWidth(1280)} {
		if w, err := parseColumnWidth(v); err != nil || w != want {
			t.Errorf("%v reads as %v, %v; want %v", v, w, err, want)
		}
	}
	for _, v := range []any{0.25, 1.0, "1280px"} {
		if w, _ := parseColumnWidth(v); !reflect.DeepEqual(w.value(), v) {
			t.Errorf("%v writes back as %v", v, w.value())
		}
	}
	c := newColumnPresets(testKit(t, ""), slot{get: func() any { return []any{0.5, "wide"} }})
	if c.cards.Visible() || c.adder.Visible() {
		t.Error("a list with a width lys-wm would refuse is drawn instead of named")
	}
	wantClosedFailure(t, "a list with a width lys-wm would refuse", c.problem.stateParts, c.problem.Text(), locale.T().Get("wm-column-widths-unreadable"), "wide")
}

func TestTheLayoutIsPickedFromPictures(t *testing.T) {
	k := lysWMKit(t, "")
	p := layoutPage(t, k, "wm-tiling")
	cards, ok := rowOf(t, p, "layout").ctl.(*choiceCards)
	if !ok {
		t.Fatalf("the layout row holds %T, want picture cards", rowOf(t, p, "layout").ctl)
	}
	cards.choose("scroll")
	if got := k.wm.value("layout"); got != "scroll" {
		t.Errorf("picking the scrolling card wrote %v", got)
	}
}
