package settings

import (
	"errors"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func memorySlot(stored *any, refuse func(v any) bool) slot {
	return slot{
		get: func() any { return *stored },
		set: func(v any) error {
			if refuse != nil && refuse(v) {
				return errors.New("refused")
			}
			*stored = v
			return nil
		},
		unset: func() { *stored = nil },
	}
}

var windowButtons = config.FieldMeta{Kind: config.FieldEnum, Variants: []string{"close", "minimize", "maximize"}}

func TestAnEnumSetListsEachVariantOnceInItsRowsOrder(t *testing.T) {
	k := testKit(t, "")
	var stored any = []any{"maximize", "close"}
	c := newEnumSet(k, memorySlot(&stored, nil), windowButtons)
	if !slices.Equal(c.order, []string{"maximize", "close", "minimize"}) {
		t.Fatalf("rows %v, want the listed values first, then the rest", c.order)
	}
	if c.switches["minimize"].On() || !c.switches["close"].On() {
		t.Fatal("the switches do not follow the list")
	}
	c.switches["minimize"].SetOn(true)
	c.move("minimize", -1)
	if got := stored.([]any); !slices.Equal(got, []any{"maximize", "minimize", "close"}) {
		t.Fatalf("stored %v", got)
	}
	c.switches["minimize"].SetOn(true)
	if got := stored.([]any); len(got) != 3 {
		t.Errorf("switching an on value on again stored %v", got)
	}
}

func TestAnEnumSetDoesNotMovePastItsEnds(t *testing.T) {
	k := testKit(t, "")
	var stored any = []any{"close", "minimize"}
	c := newEnumSet(k, memorySlot(&stored, nil), windowButtons)
	c.move("close", -1)
	c.move("maximize", 1)
	if !slices.Equal(c.order, []string{"close", "minimize", "maximize"}) || !slices.Equal(stored.([]any), []any{"close", "minimize"}) {
		t.Errorf("moving past the ends gave rows %v, stored %v", c.order, stored)
	}
}

func TestAnEnumSetFollowsAnOutsideChangeAndARefusal(t *testing.T) {
	k := testKit(t, "")
	var stored any = []any{"close"}
	c := newEnumSet(k, memorySlot(&stored, func(v any) bool { return len(v.([]any)) == 0 }), windowButtons)
	stored = []any{"maximize", "minimize"}
	c.refresh()
	if !slices.Equal(c.order[:2], []string{"maximize", "minimize"}) || c.switches["close"].On() {
		t.Fatalf("after an outside change the rows are %v, close on %v", c.order, c.switches["close"].On())
	}
	c.switches["maximize"].SetOn(false)
	c.switches["minimize"].SetOn(false)
	if !c.switches["minimize"].On() || !slices.Equal(stored.([]any), []any{"minimize"}) {
		t.Errorf("a refused empty list shows minimize on %v, stored %v", c.switches["minimize"].On(), stored)
	}
}

func TestTheDashboardSessionButtonsAreAnEnumSet(t *testing.T) {
	k := testKit(t, "")
	if _, ok := fieldControl(k, k.store, pageRow(t, "dashboard", "modules.dashboard.user-session.actions")).(*enumSet); !ok {
		t.Error("the session buttons are not switched and ordered")
	}
}

func TestEnumChipsKeepTheEnumsOrder(t *testing.T) {
	k := testKit(t, "")
	var stored any = []any{"maximize"}
	c := newEnumChips(k, memorySlot(&stored, nil), windowButtons, 1)
	pressChip(c.ChoiceChips, 0)
	if got := stored.([]any); !slices.Equal(got, []any{"close", "maximize"}) {
		t.Errorf("choosing close stored %v, want the enum's order", got)
	}
	stored = []any{"minimize"}
	c.refresh()
	if !slices.Equal(c.Checked(), []int{1}) {
		t.Error("the chips do not follow an outside change")
	}
}

func TestEnumChipsHoldTheLastChoiceTheListNeeds(t *testing.T) {
	k := testKit(t, "")
	var stored any = []any{"close", "minimize"}
	c := newEnumChips(k, memorySlot(&stored, nil), windowButtons, 1)
	if !widget.IsEnabled(c.Chip(0)) || !widget.IsEnabled(c.Chip(2)) {
		t.Fatal("a chip is held with two chosen")
	}
	pressChip(c.ChoiceChips, 0)
	if widget.IsEnabled(c.Chip(1)) || !widget.IsEnabled(c.Chip(0)) || !widget.IsEnabled(c.Chip(2)) {
		t.Errorf("with one left the last is enabled %v, the others %v %v", widget.IsEnabled(c.Chip(1)), widget.IsEnabled(c.Chip(0)), widget.IsEnabled(c.Chip(2)))
	}
	pressChip(c.ChoiceChips, 2)
	if !widget.IsEnabled(c.Chip(1)) {
		t.Error("the chip stays held with two chosen again")
	}
}

// Reordering the dashboard's session actions back to the default took
// the reset away but left the custom badge, though nothing was custom
// any more. The row badges a custom order and drops the badge once the
// order is the default again.
func TestSessionActionsBadgeACustomOrderOnlyWhileItDiffers(t *testing.T) {
	const path = "modules.dashboard.user-session.actions"
	k := testKit(t, "")
	p := layoutPage(t, k, "dashboard")
	r := rowOf(t, p, path)
	c := treetest.All[*enumSet](r)[0]
	first := c.order[0]
	c.move(first, 1)
	p.refresh()
	if !r.badge.Visible() || r.badge.Text() != locale.T().Get("settings-source-custom") {
		t.Fatalf("a custom order badges the row %q (shown %v)", r.badge.Text(), r.badge.Visible())
	}
	c.move(first, -1)
	p.refresh()
	if !k.store.isDefault(path) {
		t.Fatalf("test premise: moving %s back left %v", first, k.store.value(path))
	}
	if r.badge.Visible() {
		t.Errorf("the default order still badges the row %q", r.badge.Text())
	}
}
