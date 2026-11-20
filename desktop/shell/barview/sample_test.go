package barview

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/desktop/compositor"
	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"
)

func sampleButton(t *testing.T, bar *widget.Bar, module string) *Button {
	t.Helper()
	for _, w := range treetest.WithClass(bar, module) {
		if b := treetest.All[*Button](w); len(b) == 1 {
			return b[0]
		}
	}
	t.Fatalf("the preview has no %s button", module)
	return nil
}

func TestThePreviewDrawsEveryItemOfTheLayout(t *testing.T) {
	cfg := config.Defaults()
	cfg.Bar.Layout = []config.BarLayout{{
		Monitor: AllMonitors, Show: true,
		Left:   []config.BarItem{{Module: config.ModuleWorkspaces}, {Module: config.ModuleCava}},
		Center: []config.BarItem{{Module: config.ModuleClock, Class: "mine"}},
		Right: []config.BarItem{
			{Module: config.ModuleBattery},
			{Module: config.ModuleSeparator},
			{Group: &config.BarGroup{Name: "status", Modules: []config.BarItem{{Module: config.ModuleNetwork}, {Module: config.ModuleAudio}}}},
		},
	}}
	bar, ok := Preview(cfg, treetest.Font(t), AllMonitors, compositor.Niri)
	if !ok {
		t.Fatal("the default layout shows no bar")
	}
	if got := []int{len(bar.Start().Children()), len(bar.Center().Children()), len(bar.End().Children())}; got[0] != 2 || got[1] != 1 || got[2] != 3 {
		t.Fatalf("zones hold %v items, want 2, 1 and 3", got)
	}
	clock := sampleButton(t, bar, "clock")
	want := ClockLabel(cfg.Clock, sampleTime, ClockNames(i18n.Shell()), func() string { return "" })
	if clock.Label().Text() != want || want == "" {
		t.Errorf("the clock shows %q, want the sample time in its format, %q", clock.Label().Text(), want)
	}
	if len(treetest.WithClass(bar, "mine")) != 1 {
		t.Error("the clock's instance class is missing")
	}
	if b := sampleButton(t, bar, "battery"); b.Label().Text() != "82%" || b.Icon() == nil || b.Icon().Name() != cfg.Battery.Icon().Name {
		t.Errorf("the battery shows %q beside %v, want its sample level beside its config icon", b.Label().Text(), b.Icon())
	}
	if len(treetest.All[*widget.BarSeparator](bar)) != 1 {
		t.Error("the separator is not a rule")
	}
	group := treetest.WithClass(bar, "bar-group")
	if len(group) != 1 || len(treetest.All[*Button](group[0])) != 2 {
		t.Errorf("the status group holds %d groups, want one with two buttons", len(group))
	}
	if b := sampleButton(t, bar, "cava"); b.Label().Text() != ModuleTitle("cava", cfg) {
		t.Errorf("a module without an icon of its own shows %q, want its title", b.Label().Text())
	}
	strips := treetest.All[*widget.SegmentIndicator](bar)
	if len(strips) != 1 || !strips[0].HasClass("workspaces") || !strips[0].HasClass("module") || len(treetest.All[*Button](strips[0])) != 0 {
		t.Fatalf("the workspaces module previews as %d strips, want one workspace strip in the module's chrome", len(strips))
	}
}

func TestThePreviewFollowsTheConfig(t *testing.T) {
	cfg := config.Defaults()
	before, _ := Preview(cfg, treetest.Font(t), AllMonitors, compositor.Niri)
	if b := sampleButton(t, before, "battery"); b.HasClass("icon-only") || !b.Label().Visible() || before.Edge() != widget.BarTop {
		t.Fatalf("the default battery is icon-only %v with its label shown %v on edge %v", b.HasClass("icon-only"), b.Label().Visible(), before.Edge())
	}
	cfg.Battery.LabelShow = false
	cfg.Bar.ButtonVariant = config.ButtonBlockPrefix
	cfg.Bar.Location = config.LocationLeft
	after, _ := Preview(cfg, treetest.Font(t), AllMonitors, compositor.Niri)
	b := sampleButton(t, after, "battery")
	if !b.HasClass("icon-only") || b.Label().Visible() {
		t.Errorf("label-show off: icon-only %v, label shown %v", b.HasClass("icon-only"), b.Label().Visible())
	}
	if !b.HasClass(config.ButtonBlockPrefix.CSSClass()) || after.Edge() != widget.BarLeft {
		t.Errorf("the block-prefix variant on the left: classes %v, edge %v", b.Classes(), after.Edge())
	}
}

func TestThereIsNoPreviewWhereTheConfigShowsNoBar(t *testing.T) {
	cfg := config.Defaults()
	cfg.Bar.Layout = []config.BarLayout{{Monitor: "DP-1", Show: true, Center: []config.BarItem{{Module: config.ModuleClock}}}}
	if _, ok := Preview(cfg, treetest.Font(t), AllMonitors, compositor.Niri); ok {
		t.Error("a layout for DP-1 alone previews a bar on every monitor")
	}
	cfg.Bar.Layout = []config.BarLayout{{Monitor: AllMonitors, Show: false, Center: []config.BarItem{{Module: config.ModuleClock}}}}
	if _, ok := Preview(cfg, treetest.Font(t), AllMonitors, compositor.Niri); ok {
		t.Error("a hidden layout previews a bar")
	}
	cfg.Bar.Layout = nil
	if _, ok := Preview(cfg, treetest.Font(t), AllMonitors, compositor.Niri); ok {
		t.Error("no layout previews a bar")
	}
}

func stripLabels(s *WorkspaceStrip) []string {
	var out []string
	for _, w := range treetest.WithClass(s.Segments(), "workspace-label") {
		out = append(out, w.(*widget.Label).Text())
	}
	return out
}

func TestTheWorkspaceSampleFollowsTheConfig(t *testing.T) {
	cfg := config.Defaults()
	font := treetest.Font(t)
	if got := stripLabels(SampleWorkspaces(cfg, font, compositor.Niri)); !slices.Equal(got, []string{"web", "2"}) {
		t.Errorf("the default sample shows %v, want web and 2 with the empty last workspace hidden", got)
	}
	if got := stripLabels(SampleWorkspaces(cfg, font, compositor.LysWM)); !slices.Equal(got, []string{"1", "2", "3"}) {
		t.Errorf("lys-wm, which reads neither hide-trailing-empty nor label-strategy, shows %v, want 1, 2 and 3", got)
	}
	if s := SampleWorkspaces(cfg, font, compositor.Niri); !s.Segments().HasClass("workspaces") || s.Segments().Active() != 0 {
		t.Errorf("the sample strip has classes %v and active segment %d, want the workspaces row with the first active", s.Segments().Classes(), s.Segments().Active())
	}
	cfg.Workspaces.HideTrailingEmpty = false
	cfg.Workspaces.LabelStrategy = config.LabelIndex
	if got := stripLabels(SampleWorkspaces(cfg, font, compositor.Niri)); !slices.Equal(got, []string{"1", "2", "3"}) {
		t.Errorf("hide-trailing-empty off under index shows %v, want 1, 2 and 3", got)
	}
	cfg.Workspaces.AppIconsShow = true
	s := SampleWorkspaces(cfg, font, compositor.Niri)
	if icons := treetest.WithClass(s.Segments(), "workspace-icon"); len(icons) != 3 || len(treetest.WithClass(s.Segments(), "workspace-icon-empty")) != 1 {
		t.Errorf("app-icons-show previews %d icons, want the first workspace's deduped icon, the second's and the empty one", len(icons))
	}
	cfg.Workspaces.WorkspaceMap = config.WorkspaceMap{"web": {Label: new("www")}}
	if got := stripLabels(SampleWorkspaces(cfg, font, compositor.Niri)); len(got) == 0 || got[0] != "www" {
		t.Errorf("a workspace-map label previews %v, want www first", got)
	}
	tags := SampleWorkspaces(cfg, font, compositor.Mango)
	if got := stripLabels(tags); !slices.Equal(got, []string{"1", "2", "3"}) || len(treetest.WithClass(tags.Segments(), "tag-1")) != 1 {
		t.Errorf("mango previews tags %v, want its three numbered tags", got)
	}
}

func TestTheWorkspaceSampleLeavesOutWhatTheConfigHides(t *testing.T) {
	cfg := config.Defaults()
	font := treetest.Font(t)
	cfg.Workspaces.LabelStrategy = config.LabelNameOnly
	s := SampleWorkspaces(cfg, font, compositor.Niri)
	if got := stripLabels(s); !slices.Equal(got, []string{"web"}) || s.Segments().Count() != 2 {
		t.Errorf("name-only previews %v on %d segments, want the named workspace's label and a bare second segment", got, s.Segments().Count())
	}
	cfg.Workspaces.DisplayMode = config.DisplayModeNone
	if got := stripLabels(SampleWorkspaces(cfg, font, compositor.Niri)); len(got) != 0 {
		t.Errorf("display-mode none previews labels %v", got)
	}
	cfg.Workspaces.WorkspaceIgnore = []string{"*"}
	if s := SampleWorkspaces(cfg, font, compositor.Niri); s.Segments().Count() != 0 || s.Segments().Active() != -1 {
		t.Errorf("ignoring every workspace previews %d segments, active %d", s.Segments().Count(), s.Segments().Active())
	}
}
