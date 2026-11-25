package settings

import (
	"testing"

	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/displays"
	"github.com/stubbedev/lys/settings/internal/locale"
)

// An unselected display's tile was filled in the card's colour with no
// edge, so beside the canvas's muted ground it blended in. It carries
// the border a card's edge has; the selected tile is the accent's fill.
func TestAnUnselectedDisplayTileHasAnEdge(t *testing.T) {
	k := testKit(t, "")
	mode := displays.Mode{Width: 1920, Height: 1080}
	p := &displaysPanel{k: k, draft: []displays.Setting{
		{Name: "HEADLESS-1", Enabled: true, Mode: mode, Scale: 1},
		{Name: "VIRT-1", Enabled: true, Mode: mode, X: 1920, Scale: 1},
	}}
	c := newDisplaysCanvas(p)
	p.canvas = c
	c.layout()
	data := paintedPreview(c)
	for i, at := range c.index {
		if at == p.selected {
			continue
		}
		tile := c.screen(c.rects[i])
		edge, inside := pixelAt(data, tile.X, tile.Y+tile.H/2), pixelAt(data, tile.X+4, tile.Y+tile.H/2)
		if edge == inside {
			t.Errorf("%s's tile edge is its fill %v", p.draft[at].Name, edge)
		}
	}
}

// A display's ICC profile, shader and mirror fields were bare entries,
// empty with nothing in them. The paths say no path is set, and the
// mirror is a display's name, offering the connected ones.
func TestADisplaysEmptyPathAndMirrorFieldsSaySo(t *testing.T) {
	k := testKit(t, "")
	empty := func() slot { var v any; return memSlot(&v) }
	for _, key := range []string{"icc", "shader"} {
		c, ok := wmItemEditors["outputs.[]."+key](k, empty(), nil).(*text)
		if !ok {
			t.Fatalf("%s is not a text field", key)
		}
		if c.Placeholder() != locale.T().Get("settings-file-path-placeholder") {
			t.Errorf("an empty %s reads %q", key, c.Placeholder())
		}
	}
	m, ok := wmItemEditors["outputs.[].mirror"](k, empty(), nil).(*choiceEditor)
	if !ok {
		t.Fatal("mirror is not a choice of displays")
	}
	if m.entry.Placeholder() == "" {
		t.Error("an empty mirror says nothing")
	}
	m.entry.OnActivate("")
	if v := m.slot.get(); v != nil {
		t.Errorf("clearing the mirror stored %v", v)
	}
}

// A display's Hot corners choice and Reserved space cross were wider than
// the card's control column, so each started at its own place left of
// the column's fields. Both take their row's width under its text, as the
// Scale chips do, starting where the row's text starts.
func TestWideDisplayControlsStackUnderTheirText(t *testing.T) {
	k := testKit(t, "")
	empty := func() slot { var v any; return memSlot(&v) }
	for name, ctl := range map[string]widget.Widget{
		"hot corners":    newWMSwitch(k, empty(), nil),
		"reserved space": newSidesEditor(k, empty(), false),
	} {
		t.Run(name, func(t *testing.T) {
			row := k.row(name, "").withControl(ctl)
			field := k.entry()
			box := widget.NewBox(widget.Column, 0, 0)
			box.Append(widget.NewItemGroup(row, k.row("Name", "").withControl(field)), false)
			styledLayOut(k, box, 900)
			if fit := row.ControlFit(); fit != widget.ItemControlWide {
				t.Fatalf("the %s row holds its control at fit %d, want under its text", name, fit)
			}
			if c, f := ctl.(widget.Boundser).Bounds(), field.Bounds(); c.X >= f.X || c.X+c.W != f.X+f.W {
				t.Errorf("the %s control spans %d..%d beside a field at %d..%d", name, c.X, c.X+c.W, f.X, f.X+f.W)
			}
		})
	}
}
