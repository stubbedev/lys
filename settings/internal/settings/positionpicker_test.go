package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

func TestEveryPositionNamesItsOwnPlaceOnTheScreen(t *testing.T) {
	for value, want := range map[string]cell{
		"top": {1, 0}, "bottom": {1, 2}, "left": {0, 1}, "right": {2, 1}, "center": {1, 1},
		"top-left": {0, 0}, "top-center": {1, 0}, "bottom-right": {2, 2}, "center-left": {0, 1},
		"north-west": {0, 0}, "south": {1, 2}, "east": {2, 1},
	} {
		if got := cellOf(value); got != want {
			t.Errorf("%s: %v, want %v", value, got, want)
		}
	}
}

func TestEveryPositionRowGivesEachValueItsOwnCell(t *testing.T) {
	for _, path := range []string{"bar.location", "dock.edge", "osd.position", "modules.notifications.popup-position", "launcher.location", "modules.screenshot.thumbnail-position"} {
		meta, ok := config.Field(path)
		if !ok || len(meta.Variants) == 0 {
			t.Fatalf("%s is no enum", path)
		}
		var seen []cell
		for _, v := range meta.Variants {
			c := cellOf(v)
			if slices.Contains(seen, c) {
				t.Errorf("%s: %s shares a cell with another value", path, v)
			}
			seen = append(seen, c)
		}
	}
}

func TestThePositionPickerWritesTheChosenPlace(t *testing.T) {
	k := testKit(t, "")
	c := fieldControl(k, k.store, field("osd.position", screenPosition)).(*positionPicker)
	c.Arrange(render.Rect{W: positionW, H: positionH})
	var topLeft *spot[string]
	for _, h := range c.handles {
		if h.key == "top-left" {
			topLeft = h
		}
	}
	b := topLeft.Bounds()
	c.PressAt(widget.Point{X: b.X + b.W/2, Y: b.Y + b.H/2})
	if v := k.store.value("osd.position"); v != "top-left" {
		t.Fatalf("a press on the top-left cell stored %v", v)
	}
	c.handles[len(c.handles)-1].KeyAction(widget.KeyEnter, 0)
	if v := k.store.value("osd.position"); v != c.handles[len(c.handles)-1].key {
		t.Errorf("Enter on the last cell stored %v", v)
	}
	c.handles[0].KeyAction(widget.KeySpace, 0)
	if v := k.store.value("osd.position"); v != c.handles[0].key {
		t.Errorf("Space on the first cell stored %v", v)
	}
	c.handles[len(c.handles)-1].KeyAction(widget.KeyEnter, 0)
	c.PressAt(widget.Point{X: c.screenRect().X + c.screenRect().W/2, Y: c.screenRect().Y + c.screenRect().H/2})
	if v := k.store.value("osd.position"); v != c.handles[len(c.handles)-1].key {
		t.Errorf("a press on the empty centre changed the position to %v", v)
	}
}
