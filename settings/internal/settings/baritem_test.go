package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

func barItemPaths(s sectionSpec) []string {
	var out []string
	for _, r := range s.rows {
		out = append(append(out, r.path), r.also...)
	}
	return out
}

func TestTheBarItemSectionHoldsOnlyTheKeysAModuleHas(t *testing.T) {
	battery := barItemPaths(barItemSection("modules.battery", actionChoices("battery")))
	for _, want := range []string{"modules.battery.icon-show", "modules.battery.tooltip", "modules.battery.icon-color", "modules.battery.border-color", "modules.battery.left-click", "modules.battery.scroll-down"} {
		if !slices.Contains(battery, want) {
			t.Errorf("battery's bar item lacks %s", want)
		}
	}
	separator := barItemPaths(barItemSection("modules.separator", nil))
	if !slices.Equal(separator, []string{"modules.separator.color"}) {
		t.Errorf("the separator's bar item holds %v, want its colour alone", separator)
	}
}

func TestTheMouseHotspotsChooseWhichActionIsEdited(t *testing.T) {
	k := testKit(t, "")
	row := mouseRow("modules.battery", actionChoices("battery"))
	m := fieldControl(k, k.store, row).(*mouseActions)
	m.mouse.Arrange(render.Rect{W: mouseW, H: mouseH})
	right := m.mouse.parts()[2]
	m.mouse.PressAt(widget.Point{X: right.X + right.W - 4, Y: right.Y + right.H/2})
	if mouseButtons[m.selected] != "right-click" {
		t.Fatalf("a press on the right button chose %s", mouseButtons[m.selected])
	}
	left := k.store.value("modules.battery.left-click")
	ed := treetest.First[*actionEditor](t, m.editor)
	ed.drop.SetSelected(ed.noneIndex())
	ed.drop.SetSelected(0)
	if v := k.store.value("modules.battery.right-click"); v != actionChoices("battery")[0].command {
		t.Errorf("choosing the first action for the right button stored %v", v)
	}
	if v := k.store.value("modules.battery.left-click"); v != left {
		t.Errorf("the right button's choice changed the left one from %v to %v", left, v)
	}
	m.mouse.handles[3].KeyAction(widget.KeyEnter, 0)
	if mouseButtons[m.selected] != "scroll-up" {
		t.Errorf("Enter on the wheel's top chose %s", mouseButtons[m.selected])
	}
}

func TestTheColourStripWritesEachColourToItsOwnKey(t *testing.T) {
	k := testKit(t, "")
	paths := []string{"modules.battery.icon-color", "modules.battery.label-color"}
	c := fieldControl(k, k.store, colorRow(paths)).(*colorStrip)
	if len(c.editors) != 2 {
		t.Fatalf("%d editors for two colours", len(c.editors))
	}
	if err := c.editors[1].slot.set("#123456"); err != nil {
		t.Fatal(err)
	}
	if v := k.store.value("modules.battery.label-color"); v != "#123456" {
		t.Errorf("the label colour stored %v", v)
	}
	if v := k.store.value("modules.battery.icon-color"); v == "#123456" {
		t.Error("the label colour also set the icon colour")
	}
}
