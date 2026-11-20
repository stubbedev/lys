package launcherview

import (
	"slices"
	"strconv"
	"testing"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/treetest"
)

func testSession(t *testing.T, look Look) *Session {
	t.Helper()
	cfg := config.Defaults()
	st := rowStyle(t)
	return NewSession(Spec{
		Font: st.Font, Config: cfg, Look: look,
		Row: func(pos int) widget.Widget { return NewRow(Row{Text: "row " + strconv.Itoa(pos)}, false, st) },
	})
}

func TestASessionGrowsWithItsRowsUpToItsLines(t *testing.T) {
	s := testSession(t, Look{Width: 500, Lines: 3})
	out := render.Rect{W: 1600, H: 1000}
	w, empty := s.TargetIn(out, 0)
	if w != 500 {
		t.Errorf("the panel is %d wide, want its look's 500", w)
	}
	if s.Sep.Visible() || s.Footer.Visible() {
		t.Error("a session with no rows shows the separator or the footer")
	}
	s.SetRows(2)
	_, two := s.TargetIn(out, 0)
	if two <= empty || !s.Sep.Visible() || !s.Footer.Visible() {
		t.Errorf("two rows are %d tall against %d empty, separator %v footer %v", two, empty, s.Sep.Visible(), s.Footer.Visible())
	}
	s.SetRows(3)
	_, three := s.TargetIn(out, 0)
	s.SetRows(30)
	if _, many := s.TargetIn(out, 0); many != three {
		t.Errorf("30 rows are %d tall, want the 3 lines' %d", many, three)
	}
	if w, _ := s.TargetIn(out, 100); w != 600 {
		t.Errorf("a side pane of 100 makes the panel %d wide, want 600", w)
	}
	if _, h := s.TargetIn(render.Rect{W: 1600, H: 300}, 0); h != RoomBelow(render.Rect{H: 300}) {
		t.Errorf("a short output holds the panel at %d, want the room below, %d", h, RoomBelow(render.Rect{H: 300}))
	}
}

func TestAFixedSessionKeepsItsLinesWhateverItShows(t *testing.T) {
	out := render.Rect{W: 1600, H: 1000}
	fixed := testSession(t, Look{Width: 500, Lines: 4, Fixed: true})
	fixed.SetRows(1)
	_, one := fixed.TargetIn(out, 0)
	fixed.SetRows(4)
	if _, four := fixed.TargetIn(out, 0); four != one {
		t.Errorf("a fixed list is %d tall for one row and %d for four", one, four)
	}
	loose := testSession(t, Look{Width: 500, Lines: 4})
	loose.SetRows(1)
	if _, h := loose.TargetIn(out, 0); h >= one {
		t.Errorf("a list that is not fixed is %d tall for one row, as tall as a fixed one", h)
	}
}

func TestASessionShowsItsTabsOnlyWithTheSidebar(t *testing.T) {
	if s := testSession(t, Look{Width: 500, Lines: 3}); s.Tabs.Visible() {
		t.Error("the tabs show without the sidebar")
	}
	s := testSession(t, Look{Width: 500, Lines: 3, Sidebar: true})
	picked := -1
	s.SetTabs([]string{"drun", "run"}, map[string]string{"drun": "Apps"}, 1, func(i int) { picked = i })
	if !s.Tabs.Visible() || !slices.Equal(treetest.Labels(s.Tabs), []string{"Apps", "run"}) {
		t.Fatalf("the tabs read %v", treetest.Labels(s.Tabs))
	}
	buttons := treetest.All[*widget.Button](s.Tabs)
	if buttons[0].HasState(widget.StateChecked) || !buttons[1].HasState(widget.StateChecked) {
		t.Error("the active tab is not the one marked")
	}
	buttons[0].OnClick()
	if picked != 0 {
		t.Errorf("a click on the first tab picked %d", picked)
	}
}

func TestAStillRestsThePanelWhereItsLookPutsIt(t *testing.T) {
	out := render.Rect{W: 1200, H: 800}
	at := func(look Look) render.Rect {
		s := testSession(t, look)
		s.SetRows(2)
		still := NewStill(s)
		still.Arrange(out)
		return still.Panel().Bounds()
	}
	centre := at(Look{Width: 400, Lines: 5, Location: config.LauncherCenter})
	if centre.X != 400 || centre.Y != int(800*topFraction) || centre.W != 400 {
		t.Errorf("a centred panel rests at %v", centre)
	}
	corner := at(Look{Width: 400, Lines: 5, Location: config.LauncherSouthEast})
	if corner.X+corner.W != out.W || corner.Y+corner.H != out.H || corner.H != centre.H {
		t.Errorf("a south-east panel rests at %v in %v", corner, out)
	}
	if wide := at(Look{Width: 5000, Lines: 5}); wide.W != WidthIn(out, 5000) {
		t.Errorf("a panel wider than its output rests %d wide, want %d", wide.W, WidthIn(out, 5000))
	}
}
