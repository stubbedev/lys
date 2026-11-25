package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
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

// A display's Reserved space cross was wider than the card's control
// column, so it started left of the column's fields: it takes its row's
// width under its text, as the Scale chips do. Its Hot corners choice
// stood on a line of its own under its text while Motion's hot corners
// end their title line: the choice ends the row's title line on the
// control edge, the corners' fields under the text.
func TestWideDisplayControlsStackUnderTheirText(t *testing.T) {
	k := testKit(t, "")
	empty := func() slot { var v any; return memSlot(&v) }
	laidOut := func(name string, ctl widget.Widget) (*settingRow, *widget.Entry) {
		row := k.row(name, "").withControl(ctl)
		field := k.entry()
		box := widget.NewBox(widget.Column, 0, 0)
		box.Append(widget.NewItemGroup(row, k.row("Name", "").withControl(field)), false)
		styledLayOut(k, box, 900)
		return row, field
	}
	sides := newSidesEditor(k, empty(), false)
	row, field := laidOut("reserved space", sides)
	if fit := row.ControlFit(); fit != widget.ItemControlWide {
		t.Fatalf("the reserved space row holds its control at fit %d, want under its text", fit)
	}
	if c, f := sides.Bounds(), field.Bounds(); c.X >= f.X || c.X+c.W != f.X+f.W {
		t.Errorf("the reserved space spans %d..%d beside a field at %d..%d", c.X, c.X+c.W, f.X, f.X+f.W)
	}
	sw := newWMSwitch(k, empty(), nil)
	row, field = laidOut("hot corners", sw)
	if fit := row.ControlFit(); fit != widget.ItemControlNatural {
		t.Fatalf("the hot corners row holds its choice at fit %d, want on its title line", fit)
	}
	c, f, title := sw.choice.Bounds(), field.Bounds(), titleLabel(row).Bounds()
	if c.X+c.W != f.X+f.W {
		t.Errorf("the hot corners choice ends at %d, the field beside it at %d", c.X+c.W, f.X+f.W)
	}
	if c.Y > title.Y+title.H || c.Y+c.H < title.Y {
		t.Errorf("the hot corners choice spans y %d..%d, off its title's line %d..%d", c.Y, c.Y+c.H, title.Y, title.Y+title.H)
	}
	if !treetest.Contains(row, sw.body) || treetest.Contains(sw, sw.body) {
		t.Error("the hot corners fields are not under the row's text")
	}
}
