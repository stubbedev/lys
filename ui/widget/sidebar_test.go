package widget

import (
	"testing"

	"github.com/stubbedev/lys/ui/anim"
)

// newTestSidebar builds a sidebar of a group with an active item, a
// badge and a submenu, over a content label, laid out width wide.
func newTestSidebar(t *testing.T, width int) (*Sidebar, *[]string) {
	t.Helper()
	t.Cleanup(anim.SetInstant(true))
	face := goldenFace(t)
	var picked []string
	pick := func(s string) func() { return func() { picked = append(picked, s) } }
	s := NewSidebar(face, NewLabel(face, 14, "Content", 0))
	s.SetHeader(NewLabel(face, 14, "Acme Inc", 0))
	s.AddGroup("Platform",
		SidebarItem{Label: "Home", Icon: NewSymbol(SymbolMaximize, 16), Active: true, OnActivate: pick("home")},
		SidebarItem{Label: "Inbox", Icon: NewSymbol(SymbolSearch, 16), Badge: "24", OnActivate: pick("inbox")},
		SidebarItem{Label: "Settings", Icon: NewSymbol(SymbolCrosshair, 16), Items: []SidebarItem{
			{Label: "General", OnActivate: pick("general")},
			{Label: "Team", OnActivate: pick("team")},
		}},
	)
	s.SetFooter(NewLabel(face, 14, "lys", 0))
	arrangeTree(t, s, width, 400)
	arrangeTree(t, s, width, 400)
	return s, &picked
}

// Picking an item makes it the only active one; a parent toggles its
// submenu.
func TestSidebarItems(t *testing.T) {
	s, picked := newTestSidebar(t, 900)
	home, inbox, settings := s.rows[0], s.rows[1], s.rows[2]
	inbox.button.OnClick()
	if !inbox.button.HasState(StateChecked) || home.button.HasState(StateChecked) || len(*picked) != 1 {
		t.Fatalf("after picking Inbox: home %v inbox %v picked %v", home.button.HasState(StateChecked), inbox.button.HasState(StateChecked), *picked)
	}
	settings.button.OnClick()
	if !settings.sub.Visible() {
		t.Error("the submenu did not open")
	}
	s.rows[3].button.OnClick()
	if (*picked)[1] != "general" || !s.rows[3].button.HasState(StateChecked) {
		t.Errorf("submenu pick %v", *picked)
	}
}

// A wide sidebar collapses off canvas or to an icon rail and expands
// back; ToggleAction toggles it.
func TestSidebarCollapses(t *testing.T) {
	s, _ := newTestSidebar(t, 900)
	if s.dock.Bounds().W != sidebarWidth {
		t.Fatalf("expanded dock %d wide", s.dock.Bounds().W)
	}
	s.ToggleAction().Activate()
	arrangeTree(t, s, 900, 400)
	if s.Expanded() || s.dock.Visible() {
		t.Fatal("an off-canvas collapse left the dock showing")
	}
	s.Toggle()
	s.SetCollapse(SidebarIcon)
	s.Toggle()
	arrangeTree(t, s, 900, 400)
	if s.dock.Bounds().W != sidebarRail || s.rows[0].label.Visible() {
		t.Errorf("the icon rail is %d wide with labels %v", s.dock.Bounds().W, s.rows[0].label.Visible())
	}
	s.Toggle()
	arrangeTree(t, s, 900, 400)
	if !s.rows[0].label.Visible() || s.dock.Bounds().W != sidebarWidth {
		t.Error("expanding did not bring the labels and width back")
	}
}

// Below the narrow width the sidebar hides and Toggle opens it as a
// sheet, which a pick closes.
func TestSidebarNarrow(t *testing.T) {
	s, _ := newTestSidebar(t, 500)
	if !s.Narrow() || s.dock.Visible() {
		t.Fatal("a narrow sidebar still docks")
	}
	s.Toggle()
	if !s.sheet.Open() {
		t.Fatal("Toggle did not open the sheet")
	}
	s.rows[1].button.OnClick()
	if s.sheet.Open() {
		t.Error("a pick left the sheet open")
	}
	s.SetNarrowWidth(400)
	arrangeTree(t, s, 500, 400)
	if s.Narrow() || !s.dock.Visible() {
		t.Error("a lower narrow width did not dock the sidebar at 500px")
	}
}

// TestGoldenSidebar pins the expanded sidebar and the icon rail,
// in both palettes.
func TestGoldenSidebar(t *testing.T) {
	for _, tc := range []struct {
		name string
		th   *Theme
	}{{"dark", DarkTheme()}, {"light", LightTheme()}} {
		t.Run(tc.name, func(t *testing.T) {
			SetTheme(tc.th)
			defer SetTheme(nil)
			wide, _ := newTestSidebar(t, 800)
			rail, _ := newTestSidebar(t, 800)
			rail.SetCollapse(SidebarIcon)
			rail.SetExpanded(false)
			row := NewBox(Row, 16, 0).Append(wide, true).Append(rail, true)
			row.SetHomogeneous(true)
			NewGolden(t, row, "sidebar-"+tc.name, goldenTheme(tc.th), goldenFrame(1616, 320))
		})
	}
}

// A collapsible group folds its items on its label, SetActive picks an
// item from outside and shows it (opening its group and its submenu),
// and the width sets and drags within its bounds (#272).
func TestSidebarSettingsApp(t *testing.T) {
	s, _ := newTestSidebar(t, 900)
	s.AddCollapsibleGroup("Advanced", nil, false,
		SidebarItem{Label: "Developer", ID: "dev"},
		SidebarItem{Label: "Experimental", Items: []SidebarItem{{Label: "Flags", ID: "flags"}}},
	)
	arrangeTree(t, s, 900, 600)
	dev := s.rows[len(s.rows)-3]
	if dev.id != "dev" || IsVisible(dev.group) {
		t.Fatalf("collapsed group: row %q visible %v", dev.id, IsVisible(dev.group))
	}
	head := s.labels[len(s.labels)-1].(*Button)
	head.OnClick()
	if !IsVisible(dev.group) {
		t.Error("clicking the group label left it folded")
	}
	head.OnClick()

	if !s.SetActive("flags") {
		t.Fatal("SetActive did not find flags")
	}
	flags := s.rows[len(s.rows)-1]
	if !flags.button.HasState(StateChecked) || !IsVisible(flags.group) || !IsVisible(flags.parent) {
		t.Errorf("after SetActive: checked %v, group shown %v, submenu shown %v", flags.button.HasState(StateChecked), IsVisible(flags.group), IsVisible(flags.parent))
	}
	if s.rows[0].button.HasState(StateChecked) {
		t.Error("the previously active item stayed active")
	}
	if s.SetActive("nope") {
		t.Error("SetActive found an unknown id")
	}

	s.SetWidth(300)
	arrangeTree(t, s, 900, 600)
	if got := s.dock.Bounds().W; got != 300 {
		t.Fatalf("docked width %d after SetWidth(300)", got)
	}
	reported := 0
	s.OnWidthChanged = func(w int) { reported = w }
	s.SetResizable(200, 400)
	b := s.dock.Bounds()
	edge := Point{X: b.X + b.W, Y: b.Y + b.H/2}
	if s.HitTest(edge) != Widget(s) {
		t.Fatal("the panel's edge is not the resize handle")
	}
	s.PressAt(edge)
	s.DragMove(Point{X: edge.X + 500, Y: edge.Y})
	s.PressEnd()
	if s.Width() != 400 || reported != 400 {
		t.Errorf("dragged past the bound: width %d reported %d, want 400", s.Width(), reported)
	}
}

// sidebarScroll is the scroll holding a sidebar's groups.
func sidebarScroll(t *testing.T, s *Sidebar) *Scroll {
	t.Helper()
	for _, c := range s.panel.Children() {
		if sc, ok := c.(*Scroll); ok {
			return sc
		}
	}
	t.Fatal("the sidebar's panel holds no scroll")
	return nil
}

// newGroupedSidebar is a sidebar of n collapsible groups of three items,
// each open when open, laid out width by height.
func newGroupedSidebar(t *testing.T, n int, open bool, width, height int) (*Sidebar, []*Button) {
	t.Helper()
	t.Cleanup(anim.SetInstant(true))
	face := goldenFace(t)
	s := NewSidebar(face, NewLabel(face, 14, "Content", 0))
	s.SetCollapse(SidebarNone)
	heads := make([]*Button, 0, n)
	for range n {
		s.AddCollapsibleGroup("Bar Items: Workspaces & Windows", NewSymbol(SymbolSearch, 16), open,
			SidebarItem{Label: "One"}, SidebarItem{Label: "Two"}, SidebarItem{Label: "Three"})
		heads = append(heads, s.labels[len(s.labels)-1].(*Button))
	}
	arrangeTree(t, s, width, height)
	arrangeTree(t, s, width, height)
	return s, heads
}

// Groups that overflow the panel keep their headers, chevrons included,
// inside the viewport beside the scrollbar's gutter, not cut off under it.
func TestSidebarGroupsThatOverflowStayInsideTheViewport(t *testing.T) {
	s, heads := newGroupedSidebar(t, 12, true, 900, 400)
	sc := sidebarScroll(t, s)
	if sc.nat.H <= sc.viewH {
		t.Fatalf("the groups (%d px) do not overflow the %d px viewport", sc.nat.H, sc.viewH)
	}
	right := sc.view.X + sc.viewW
	for i, h := range heads {
		if b := h.Bounds(); b.X+b.W > right {
			t.Errorf("header %d ends at x=%d, past the viewport's %d", i, b.X+b.W, right)
		}
	}
}

// Groups shorter than the panel, every one folded, sit at its top rather
// than centred in it.
func TestSidebarFoldedGroupsSitAtTheTop(t *testing.T) {
	s, heads := newGroupedSidebar(t, 4, false, 900, 600)
	sc := sidebarScroll(t, s)
	if sc.nat.H >= sc.viewH {
		t.Fatalf("the folded groups (%d px) fill the %d px viewport", sc.nat.H, sc.viewH)
	}
	if got, want := heads[0].Bounds().Y, sc.view.Y; got != want {
		t.Errorf("the first header is at y=%d, want the viewport's top %d", got, want)
	}
	heads[0].OnClick()
	arrangeTree(t, s, 900, 600)
	if got, want := heads[0].Bounds().Y, sc.view.Y; got != want {
		t.Errorf("after opening a group the first header is at y=%d, want %d", got, want)
	}
}

// A collapsible group's icon leads its header; without one the header
// is its label and chevron alone.
func TestSidebarGroupIconLeadsItsHeader(t *testing.T) {
	t.Cleanup(anim.SetInstant(true))
	face := goldenFace(t)
	s := NewSidebar(face, NewLabel(face, 14, "Content", 0))
	icon := NewSymbol(SymbolSearch, 16)
	s.AddCollapsibleGroup("With", icon, true, SidebarItem{Label: "One"})
	s.AddCollapsibleGroup("Without", nil, true, SidebarItem{Label: "Two"})
	with := s.labels[0].(*Button).child.(*Box).Children()
	if len(with) != 3 || with[0] != Widget(icon) {
		t.Errorf("the header with an icon holds %d parts, first %T; want the icon, the label and the chevron", len(with), with[0])
	}
	if without := s.labels[1].(*Button).child.(*Box).Children(); len(without) != 2 {
		t.Errorf("the header without an icon holds %d parts, want the label and the chevron", len(without))
	}
}
