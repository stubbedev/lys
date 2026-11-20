package barview

import (
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/desktop/compositor"
	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/treetest"
)

func classesIn(w widget.Widget) []string {
	var out []string
	treetest.Walk(w, func(w widget.Widget) {
		if c, ok := w.(interface{ Classes() []string }); ok {
			out = append(out, c.Classes()...)
		}
	})
	return out
}

func testStrip(t *testing.T, cfg *config.Config, c config.CompositorWorkspacesConfig) *WorkspaceStrip {
	t.Helper()
	return NewWorkspaceStrip(cfg, c, "niri", nil, treetest.Font(t), WorkspaceHandlers{})
}

func TestWorkspaceSegmentContent(t *testing.T) {
	cfg := config.Defaults()
	c := cfg.Workspaces.View(compositor.Niri)
	s := testStrip(t, cfg, c)
	labelled := WorkspaceButton{Label: "1", HasLabel: true}
	if got := classesIn(s.segmentContent(labelled, &segmentShown{})); !slices.Contains(got, "workspace-label") || slices.Contains(got, "workspace-icons") || slices.Contains(got, "workspace-divider") {
		t.Errorf("app icons off: content classes %v, want the label alone", got)
	}
	s.cfg.AppIconsShow = true
	apps := WorkspaceButton{Label: "1", HasLabel: true, AppIcons: []WorkspaceAppIcon{{Name: "foot"}, {Name: "firefox", Urgent: true}}}
	got := classesIn(s.segmentContent(apps, &segmentShown{}))
	for _, class := range []string{"workspace-label", "workspace-divider", "workspace-icons", "urgent"} {
		if !slices.Contains(got, class) {
			t.Errorf("app icons on: content classes %v miss %q", got, class)
		}
	}
	if slices.Contains(got, "workspace-icon-empty") || strings.Count(strings.Join(got, " "), "workspace-icon ") != 2 {
		t.Errorf("two apps: content classes %v, want their two icons and no empty one", got)
	}
	if got := classesIn(s.segmentContent(WorkspaceButton{Label: "1", HasLabel: true}, &segmentShown{})); !slices.Contains(got, "workspace-icon-empty") {
		t.Errorf("no apps: content classes %v, want the empty icon", got)
	}
	s.cfg.AppIconsShow = false
	mapped := WorkspaceButton{Label: "1", HasLabel: true, Icon: "ld-globe-symbolic"}
	if got := classesIn(s.segmentContent(mapped, &segmentShown{})); slices.Contains(got, "workspace-label") || !slices.Contains(got, "workspace-icon") {
		t.Errorf("mapped icon: content classes %v, want the icon in the label's place", got)
	}
	mapped.Flavor = FlavorHyprland
	if got := classesIn(s.segmentContent(mapped, &segmentShown{})); !slices.Contains(got, "workspace-custom-icon") || slices.Contains(got, "workspace-icon") {
		t.Errorf("hyprland mapped icon: content classes %v, want the custom icon", got)
	}
}

func TestAStripBindsOnlyTheHandlersItIsGiven(t *testing.T) {
	cfg := config.Defaults()
	c := cfg.Workspaces.View(compositor.Niri)
	var selected []int
	s := NewWorkspaceStrip(cfg, c, "", nil, treetest.Font(t), WorkspaceHandlers{Select: func(i int) { selected = append(selected, i) }})
	s.Set(WorkspaceButtons(FlavorWorkspaces, []Workspace{occupiedWS(1, 1, "DP-1"), occupiedWS(2, 2, "DP-1")}, nil, c, "", false, false))
	router, p := treetest.Pointer(s.Segments(), s.Segments().Segment(1), 500, 40)
	router.Press(widget.BTNLeft, p, 0)
	router.Release(widget.BTNLeft, p)
	if !slices.Equal(selected, []int{1}) {
		t.Errorf("a click on the second segment selected %v, want [1]", selected)
	}
	router.Press(widget.BTNMiddle, p, 0)
	router.Release(widget.BTNMiddle, p)
	router.Axis(0, 1, 0)
	if !slices.Equal(selected, []int{1}) {
		t.Errorf("a middle click and a scroll without handlers selected %v", selected)
	}
}
