package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
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
	battery := barItemPaths(barItemSection(config.ModuleBattery))
	for _, want := range []string{"modules.battery.icon-show", "modules.battery.tooltip", "modules.battery.icon-color", "modules.battery.border-color", "modules.battery.left-click", "modules.battery.scroll-down"} {
		if !slices.Contains(battery, want) {
			t.Errorf("battery's bar item lacks %s", want)
		}
	}
	separator := barItemPaths(barItemSection(config.ModuleSeparator))
	if !slices.Equal(separator, []string{"modules.separator.color"}) {
		t.Errorf("the separator's bar item holds %v, want its colour alone", separator)
	}
}

// barItemRow is the row of a module's Bar Item section labelled key.
func barItemRow(t *testing.T, module config.BarModule, key string) rowSpec {
	t.Helper()
	rows := barItemSection(module).rows
	i := slices.IndexFunc(rows, func(r rowSpec) bool { return r.key == key })
	if i < 0 {
		t.Fatalf("the %s bar item has no %s row", module, key)
	}
	return rows[i]
}

func TestTheMouseHotspotsChooseWhichActionIsEdited(t *testing.T) {
	k := testKit(t, "")
	m := fieldControl(k, k.store, barItemRow(t, config.ModuleBattery, "settings-bar-item-mouse")).(*mouseActions)
	m.mouse.Arrange(render.Rect{W: mouseW, H: mouseH})
	right := m.mouse.parts()[2]
	m.mouse.PressAt(widget.Point{X: right.X + right.W - 4, Y: right.Y + right.H/2})
	if mouseButtons[m.selected] != "right-click" {
		t.Fatalf("a press on the right button chose %s", mouseButtons[m.selected])
	}
	left := k.store.value("modules.battery.left-click")
	ed := treetest.First[*actionEditor](t, m.editor)
	pickChoice(ed.pick.pick, ed.noneIndex())
	pickChoice(ed.pick.pick, 0)
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
	c := fieldControl(k, k.store, barItemRow(t, config.ModuleBattery, "settings-bar-item-colors")).(*colorStrip)
	colors := slices.DeleteFunc(slices.Clone(barItemColors), func(key string) bool { return !hasKey("modules.battery." + key) })
	if len(c.editors) != len(colors) {
		t.Fatalf("%d editors for the battery's %d colours", len(c.editors), len(colors))
	}
	if err := c.editors[slices.Index(colors, "label-color")].slot.set("#123456"); err != nil {
		t.Fatal(err)
	}
	if v := k.store.value("modules.battery.label-color"); v != "#123456" {
		t.Errorf("the label colour stored %v", v)
	}
	if v := k.store.value("modules.battery.icon-color"); v == "#123456" {
		t.Error("the label colour also set the icon colour")
	}
}

func TestTheSystemMonitorsOpenTheDashboard(t *testing.T) {
	for _, module := range []config.BarModule{config.ModuleCPU, config.ModuleRAM, config.ModuleStorage, config.ModuleNetstat} {
		if !slices.ContainsFunc(actionChoices(module), func(c actionChoice) bool { return c.command == "dropdown:dashboard" }) {
			t.Errorf("%s offers no action to open the dashboard", module)
		}
	}
	if len(actionChoices("separator")) != 0 {
		t.Error("the separator offers actions though it has none")
	}
}

// The chosen button's highlight was a crescent reaching neither the
// outline nor the divider. It fills the button up to the body's outline,
// its outer edge following the body's curve, and up to the dividers,
// clear of the wheel.
func TestTheMouseHighlightKeepsInsideTheBody(t *testing.T) {
	k := testKit(t, "")
	m := fieldControl(k, k.store, barItemRow(t, config.ModuleBattery, "settings-bar-item-mouse")).(*mouseActions)
	m.mouse.Arrange(render.Rect{W: mouseW, H: mouseH})
	m.choose(0)
	buf := make([]byte, mouseW*mouseH*4)
	m.mouse.Paint(render.New(buf, mouseW*4, mouseW, mouseH))
	accent := widget.Current().Accent
	at := func(x, y int) bool {
		i := (y*mouseW + x) * 4
		px := buf[i : i+4]
		return closeByte(px[0], accent.B()) && closeByte(px[1], accent.G()) && closeByte(px[2], accent.R())
	}
	left := m.mouse.parts()[0]
	if !at(left.X+left.W/2, left.Y+left.H*2/3) {
		t.Fatal("the chosen left button is not highlighted")
	}
	if at(mouseOutline+2, mouseOutline+2) {
		t.Error("the highlight fills the corner outside the body's curve")
	}
	if !at(left.X+mouseOutline+1, left.Y+left.H*2/3) {
		t.Error("the highlight stops short of the body's outline")
	}
	if !at(left.X+left.W-1, left.Y+left.H-2) || !at(left.X+left.W/3, left.Y+left.H-1) {
		t.Error("the highlight stops short of the dividers")
	}
	wheel := m.mouse.parts()[3]
	if at(wheel.X-1, wheel.Y+wheel.H) {
		t.Error("the highlight runs up to the wheel")
	}
}

// closeByte is a channel within 8 of another.
func closeByte(a, b uint8) bool { return int(a)-int(b) <= 8 && int(b)-int(a) <= 8 }

// The wheel was a muted pill on the card's fill, near invisible. It is
// ringed in the outline's colour, so it reads beside an unlit button.
func TestTheMouseWheelIsOutlined(t *testing.T) {
	k := testKit(t, "")
	m := fieldControl(k, k.store, barItemRow(t, config.ModuleBattery, "settings-bar-item-mouse")).(*mouseActions)
	m.mouse.Arrange(render.Rect{W: mouseW, H: mouseH})
	m.choose(0)
	buf := make([]byte, mouseW*mouseH*4)
	m.mouse.Paint(render.New(buf, mouseW*4, mouseW, mouseH))
	px := func(x, y int) [4]byte {
		i := (y*mouseW + x) * 4
		return [4]byte(buf[i : i+4])
	}
	up, middle := m.mouse.parts()[3], m.mouse.parts()[1]
	y := middle.Y + middle.H/2
	if ring, card := px(middle.X, y), px(middle.X-2, y); ring == card {
		t.Errorf("the wheel's edge %v is the card's fill", ring)
	}
	if ring, inside := px(up.X+up.W/2, up.Y), px(up.X+up.W/2, up.Y+up.H/2); ring == inside {
		t.Errorf("the wheel's top edge %v is its fill", ring)
	}
}
