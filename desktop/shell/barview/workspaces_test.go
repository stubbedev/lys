package barview

import (
	"reflect"
	"testing"

	"github.com/stubbedev/lys/desktop/compositor"
	"github.com/stubbedev/lys/desktop/config"
)

func occupiedWS(id uint64, num int, output string) Workspace {
	return Workspace{ID: id, Num: num, Output: output, HasOutput: true, HasWindows: true}
}

func emptyWS(id uint64, num int, output string) Workspace {
	return Workspace{ID: id, Num: num, Output: output, HasOutput: true}
}

func wsIDs(list []Workspace) []uint64 {
	out := make([]uint64, len(list))
	for i, ws := range list {
		out[i] = ws.ID
	}
	return out
}

func TestWorkspaceShowsEmptyWorkspacesWithoutHideTrailing(t *testing.T) {
	named := emptyWS(3, 3, "DP-1")
	named.Name, named.HasName = "web", true
	got := collectDisplayed([]Workspace{occupiedWS(1, 1, "DP-1"), emptyWS(2, 2, "DP-1"), named}, workspaceFilter{})
	if !reflect.DeepEqual(wsIDs(got), []uint64{1, 2, 3}) {
		t.Errorf("ids = %v", wsIDs(got))
	}
}

func TestWorkspaceHideTrailingEmptyDropsOnlyTheTail(t *testing.T) {
	all := []Workspace{occupiedWS(1, 1, "DP-1"), emptyWS(2, 2, "DP-1"), occupiedWS(3, 3, "DP-1"), emptyWS(4, 4, "DP-1"), emptyWS(5, 1, "HDMI-A-1")}
	got := collectDisplayed(all, workspaceFilter{hideTrailingEmpty: true})
	if !reflect.DeepEqual(wsIDs(got), []uint64{1, 2, 3}) {
		t.Errorf("ids = %v, want the trailing empties of both outputs dropped", wsIDs(got))
	}
	got = collectDisplayed([]Workspace{emptyWS(1, 1, "DP-1"), occupiedWS(2, 2, "DP-1")}, workspaceFilter{hideTrailingEmpty: true})
	if !reflect.DeepEqual(wsIDs(got), []uint64{1, 2}) {
		t.Errorf("ids = %v, want an occupied tail kept", wsIDs(got))
	}
}

func TestWorkspaceIgnoreByNameNumberAndID(t *testing.T) {
	scratch := emptyWS(3, 3, "DP-1")
	scratch.Name, scratch.HasName = "scratch", true
	all := []Workspace{occupiedWS(1, 1, "DP-1"), occupiedWS(2, 2, "DP-1"), scratch, occupiedWS(40, 9, "DP-1")}
	if got := collectDisplayed(all, workspaceFilter{ignore: []string{"scr*"}}); !reflect.DeepEqual(wsIDs(got), []uint64{1, 2, 40}) {
		t.Errorf("name glob: %v", wsIDs(got))
	}
	if got := collectDisplayed(all, workspaceFilter{ignore: []string{"?"}}); !reflect.DeepEqual(wsIDs(got), []uint64{}) {
		t.Errorf("single-digit number glob: %v, want all ignored", wsIDs(got))
	}
	if got := collectDisplayed(all, workspaceFilter{ignore: []string{"40"}}); !reflect.DeepEqual(wsIDs(got), []uint64{1, 2, 3}) {
		t.Errorf("stable id: %v", wsIDs(got))
	}
	if got := collectDisplayed(all, workspaceFilter{ignore: []string{"SCRATCH"}}); len(got) != 4 {
		t.Errorf("case-insensitive match: %v", wsIDs(got))
	}
}

func TestWorkspaceOrdersByOutputThenNumber(t *testing.T) {
	unnumbered := occupiedWS(9, -1, "DP-1")
	all := []Workspace{occupiedWS(3, 2, "DP-2"), unnumbered, occupiedWS(1, 2, "DP-1"), occupiedWS(2, 1, "DP-2")}
	if got := collectDisplayed(all, workspaceFilter{}); !reflect.DeepEqual(wsIDs(got), []uint64{1, 9, 2, 3}) {
		t.Errorf("ids = %v, want output order, number order, unnumbered last", wsIDs(got))
	}
}

func TestWorkspaceMonitorSpecific(t *testing.T) {
	all := []Workspace{occupiedWS(1, 1, "DP-1"), occupiedWS(2, 1, "DP-2"), {ID: 3, Num: 1}}
	if got := collectDisplayed(all, workspaceFilter{monitorSpecific: true, barMonitor: "DP-1"}); !reflect.DeepEqual(wsIDs(got), []uint64{1}) {
		t.Errorf("scoped = %v, want DP-1's only (an outputless workspace is not on any bar)", wsIDs(got))
	}
	if got := collectDisplayed(all, workspaceFilter{monitorSpecific: false, barMonitor: "DP-1"}); len(got) != 3 {
		t.Errorf("unscoped = %v", wsIDs(got))
	}
	if got := collectDisplayed(all, workspaceFilter{monitorSpecific: true}); len(got) != 3 {
		t.Errorf("no bar monitor = %v", wsIDs(got))
	}
}

func TestWorkspaceLabelStrategies(t *testing.T) {
	for _, tc := range []struct {
		num      int
		name     string
		hasName  bool
		strategy config.LabelStrategy
		want     string
		ok       bool
	}{
		{3, "web", true, config.LabelIndex, "3", true},
		{3, "web", true, config.LabelNameOrIndex, "web", true},
		{3, "", false, config.LabelNameOrIndex, "3", true},
		{3, "", false, config.LabelNameOnly, "", false},
		{3, "web", true, config.LabelIndexAndName, "3: web", true},
		{3, "", false, config.LabelIndexAndName, "3", true},
		{-1, "chat", true, config.LabelIndex, "", false},
		{-1, "chat", true, config.LabelIndexAndName, "chat", true},
		{-1, "", false, config.LabelIndexAndName, "", false},
	} {
		got, ok := workspaceLabel(tc.num, tc.name, tc.hasName, tc.strategy)
		if got != tc.want || ok != tc.ok {
			t.Errorf("label(%d, %q, %s) = %q %v, want %q %v", tc.num, tc.name, tc.strategy, got, ok, tc.want, tc.ok)
		}
	}
}

func TestWorkspaceStylePrefersNameThenID(t *testing.T) {
	m := map[string]config.WorkspaceStyle{
		"web": {Label: new("W")},
		"5":   {Label: new("five")},
	}
	named := Workspace{ID: 5, Name: "web", HasName: true}
	if s, ok := styleFor(named, m); !ok || *s.Label != "W" {
		t.Errorf("named = %+v %v, want the name entry", s, ok)
	}
	unmatched := Workspace{ID: 5, Name: "mail", HasName: true}
	if s, ok := styleFor(unmatched, m); !ok || *s.Label != "five" {
		t.Errorf("fallback = %+v %v, want the id entry", s, ok)
	}
	if _, ok := styleFor(Workspace{ID: 6}, m); ok {
		t.Error("no entry matched but a style came back")
	}
}

func TestWorkspaceNameClassSanitizes(t *testing.T) {
	if got := nameClass("2:web ü"); got != "ws-name-2_web__" {
		t.Errorf("class = %q", got)
	}
	if got := idClass(5); got != "ws-id-5" {
		t.Errorf("id class = %q", got)
	}
}

func TestWorkspaceOverrideColorCascade(t *testing.T) {
	red, _ := config.ParseColorValue("#ff0000")
	green, _ := config.ParseColorValue("#00ff00")
	m := map[string]config.WorkspaceStyle{
		"5":   {Color: &red},
		"web": {Color: &green},
		"x":   {Icon: new("no-color")},
	}
	if c, ok := overrideColor([]string{"ws-id-5"}, m); !ok || c != red {
		t.Errorf("id match = %v %v", c, ok)
	}
	if c, ok := overrideColor([]string{"ws-id-5", "ws-name-web"}, m); !ok || c != green {
		t.Errorf("both match = %v %v, want the later key (web)", c, ok)
	}
	if c, ok := overrideColor([]string{"ws-id-99", "ws-name-5"}, m); !ok || c != red {
		t.Errorf("numeric name = %v %v", c, ok)
	}
	if _, ok := overrideColor([]string{"ws-id-7", "ws-name-x"}, m); ok {
		t.Error("a colorless entry produced an override")
	}
}

func TestWorkspaceAppIconsDedupeOrderAndUrgency(t *testing.T) {
	cfg := config.DefaultsWorkspaces().View(compositor.Niri)
	windows := []WorkspaceWindow{
		{ID: 3, Workspace: 1, HasWorkspace: true, Icon: "firefox", Order: [2]int{2, 1}},
		{ID: 1, Workspace: 1, HasWorkspace: true, Icon: "foot", Order: [2]int{1, 1}},
		{ID: 2, Workspace: 1, HasWorkspace: true, Icon: "firefox", Order: [2]int{3, 1}, Urgent: true},
		{ID: 9, Workspace: 2, HasWorkspace: true},
	}
	on := windowsOn(windows, 1)
	if len(on) != 3 || on[0].ID != 1 || on[1].ID != 3 || on[2].ID != 2 {
		t.Fatalf("windows on 1 = %+v, want layout order", on)
	}
	icons := collectAppIcons(on, cfg.AppIconsDedupe, map[uint64]bool{2: true})
	if len(icons) != 2 || !reflect.DeepEqual(icons[1].WindowIDs, []uint64{3, 2}) || !icons[1].Urgent || icons[0].Urgent {
		t.Fatalf("deduped = %+v", icons)
	}
	cfg.AppIconsDedupe = false
	if icons := collectAppIcons(on, cfg.AppIconsDedupe, nil); len(icons) != 3 {
		t.Fatalf("no dedupe = %+v, want one icon per window", icons)
	}
}

func TestWorkspaceBuildModelsBlinkGatesUrgency(t *testing.T) {
	cfg := config.DefaultsWorkspaces().View(compositor.Sway)
	urgent := occupiedWS(1, 1, "DP-1")
	urgent.Urgent = true
	all := []Workspace{urgent}
	on := WorkspaceButtons(FlavorWorkspaces, all, nil, cfg, "DP-1", false, true)
	if !hasClass(on[0].Classes, "urgent") {
		t.Errorf("blink on: classes %v miss urgent", on[0].Classes)
	}
	off := WorkspaceButtons(FlavorWorkspaces, all, nil, cfg, "DP-1", false, false)
	if hasClass(off[0].Classes, "urgent") {
		t.Errorf("blink off: classes %v carry urgent", off[0].Classes)
	}
	cfg.UrgentShow = false
	if m := WorkspaceButtons(FlavorWorkspaces, all, nil, cfg, "DP-1", false, true); hasClass(m[0].Classes, "urgent") {
		t.Error("urgent-show = false still marks the class")
	}
	cfg.UrgentShow, cfg.UrgentMode = true, config.UrgentApplication
	if m := WorkspaceButtons(FlavorWorkspaces, all, nil, cfg, "DP-1", false, true); !hasClass(m[0].Classes, "urgent-application") {
		t.Error("urgent-mode application misses its class")
	}
}

func TestWorkspaceClassesMatchComputeCSSClasses(t *testing.T) {
	cfg := config.DefaultsWorkspaces().View(compositor.Niri)
	ws := occupiedWS(5, 2, "DP-1")
	ws.Active, ws.Focused, ws.Name, ws.HasName = true, true, "web", true
	m := WorkspaceButtons(FlavorWorkspaces, []Workspace{ws}, nil, cfg, "", true, false)
	want := []string{"workspace", "active", "focused", "indicator-background", "vertical", "ws-id-5", "ws-name-web"}
	if !reflect.DeepEqual(m[0].Classes, want) {
		t.Errorf("classes = %v, want %v", m[0].Classes, want)
	}
}

func TestWorkspaceShowRules(t *testing.T) {
	cfg := config.DefaultsWorkspaces().View(compositor.Niri)
	labelled := WorkspaceButton{Label: "1", HasLabel: true}
	if !labelled.ShowLabel(cfg.DisplayMode) || labelled.ShowIcon(cfg.DisplayMode) || labelled.ShowDivider(cfg) {
		t.Error("a labelled button without app icons: label only")
	}
	iconed := WorkspaceButton{Label: "1", HasLabel: true, Icon: "ld-globe-symbolic"}
	if iconed.ShowLabel(cfg.DisplayMode) || !iconed.ShowIcon(cfg.DisplayMode) {
		t.Error("a mapped icon replaces the label")
	}
	if iconed.ShowIcon(config.DisplayModeNone) || labelled.ShowLabel(config.DisplayModeNone) {
		t.Error("display-mode none shows something")
	}
	cfg.AppIconsShow = true
	if !labelled.ShowDivider(cfg) {
		t.Error("app icons on: the divider shows beside the label")
	}
	cfg.Divider = ""
	if labelled.ShowDivider(cfg) {
		t.Error("an empty divider shows")
	}
}

func TestWorkspaceTagShowRules(t *testing.T) {
	tag := WorkspaceButton{Flavor: FlavorTags, Label: "1", HasLabel: true}
	iconed := WorkspaceButton{Flavor: FlavorTags, Label: "1", HasLabel: true, Icon: "ld-globe-symbolic"}
	if !iconed.ShowLabel(config.DisplayModeLabel) || iconed.ShowIcon(config.DisplayModeLabel) {
		t.Error("label mode: tags show the label even with a mapped icon, and no icon")
	}
	if iconed.ShowLabel(config.DisplayModeIcon) || !iconed.ShowIcon(config.DisplayModeIcon) {
		t.Error("icon mode: the mapped icon replaces the label")
	}
	if !tag.ShowLabel(config.DisplayModeIcon) {
		t.Error("icon mode without a mapped icon falls back to the label")
	}
	if tag.ShowLabel(config.DisplayModeNone) || iconed.ShowIcon(config.DisplayModeNone) {
		t.Error("none shows something")
	}
}

func TestWorkspaceTagOverrideTargetsTagClass(t *testing.T) {
	red, _ := config.ParseColorValue("#ff0000")
	m := map[string]config.WorkspaceStyle{"4": {Color: &red}}
	if c, ok := overrideColor([]string{"tag-4"}, m); !ok || c != red {
		t.Errorf("tag-4 override = %v %v", c, ok)
	}
	if _, ok := overrideColor([]string{"tag-5"}, m); ok {
		t.Error("tag-5 picked up tag 4's color")
	}
}
