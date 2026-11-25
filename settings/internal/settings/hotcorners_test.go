package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func storedHotCorners(t *testing.T) (*kit, *hotCorners, map[string]any, []string) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	k := testKit(t, "")
	if _, err := k.wmBackend(); err != nil {
		t.Fatal(err)
	}
	stored := map[string]any{}
	slots := make([]slot, len(hotCornerKeys))
	paths := make([]string, len(hotCornerKeys))
	for i, c := range hotCornerKeys {
		path := "hot_corners." + c
		paths[i] = path
		slots[i] = slot{get: func() any { return stored[path] }, set: func(v any) error { stored[path] = v; return nil }, unset: func() { delete(stored, path) }}
	}
	return k, newHotCorners(k, paths, slots), stored, paths
}

// The Hot Corners card's first row read "Hot Corners" and its section's
// description again, word for word under the section's own heading.
// The row names what it holds, the corners, in its own words.
func TestTheHotCornersRowDoesNotRepeatItsSection(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	win := laidOutPage(t, "", motionPageID, sweepW)
	tr := locale.T()
	for _, r := range win.page.rows {
		if r.path != "hot_corners.top_left" {
			continue
		}
		title := titleLabel(r).Text()
		if title == tr.Get("wm-section-hot-corners") || title != tr.Get("wm-hot-corners-corners") {
			t.Errorf("the hot corners row reads %q", title)
		}
		return
	}
	t.Fatal("the Motion page shows no hot corners row")
}

// The picked corner differed from the others only by a thicker ring. It
// fills with the accent as every picker's chosen cell does, a corner
// that does something carries a dot whether picked or not, and the
// others keep the card's colour.
func TestThePickedHotCornerFillsWithTheAccent(t *testing.T) {
	_, h, stored, paths := storedHotCorners(t)
	stored[paths[2]] = map[string]any{"action": "overview"}
	themed(h.k, gapBox(widget.Column))
	r := render.Rect{W: cornerScreenW, H: cornerScreenH}
	th := widget.Current()
	centre := func(i int) render.Color {
		d := h.screen.dots()[i]
		return paintedAt(h.screen, r, d.X+d.W/2, d.Y+d.H/2)
	}
	edge := func(i int) render.Color {
		d := h.screen.dots()[i]
		return paintedAt(h.screen, r, d.X+d.W/2, d.Y+d.H/2-cornerMarkR-2)
	}
	h.screen.Arrange(r)
	if got := edge(0); got != th.Accent {
		t.Errorf("the picked corner fills with %v, want the accent %v", got, th.Accent)
	}
	if got := edge(1); got == th.Accent {
		t.Error("an unpicked corner fills with the accent")
	}
	if got := centre(2); got != th.Accent {
		t.Errorf("an unpicked corner that does something marks its centre %v, want the accent", got)
	}
	if got := centre(1); got == th.Accent {
		t.Error("an unpicked corner that does nothing carries a mark")
	}
	h.choose(2)
	if got := centre(2); got != th.OnAccent {
		t.Errorf("the picked corner that does something marks its centre %v, want %v on the accent", got, th.OnAccent)
	}
}

// The top right corner touched the "Top left corner" name beside it,
// and the Inherit, Off and On choices floated mid-card under it. The
// screen stands a section's gap off the side; the picked corner's name
// starts the side's line and its choices end it on the control edge.
func TestTheHotCornersSideStandsOffTheScreenItsChoicesOnTheEdge(t *testing.T) {
	k, h, _, _ := storedHotCorners(t)
	box := widget.NewBox(widget.Column, 0, 0)
	box.Append(h, false)
	styledLayOut(k, box, 700)
	screen, title, side := h.screen.Bounds(), h.title.Bounds(), h.editor.Bounds()
	if gap := title.X - (screen.X + screen.W); gap < sectionGap() {
		t.Errorf("the corner's name stands %dpx off the screen, want %d at least", gap, sectionGap())
	}
	sw := treetest.First[*wmSwitch](t, h.editor)
	choice := sw.choice.Bounds()
	if choice.X+choice.W != side.X+side.W {
		t.Errorf("the choices end at %d, the side at %d", choice.X+choice.W, side.X+side.W)
	}
	if choice.Y >= title.Y+title.H || title.Y >= choice.Y+choice.H {
		t.Errorf("the choices at %+v are not on the name's line at %+v", choice, title)
	}
}
