package barview

import (
	"testing"

	"github.com/stubbedev/lys/ui/widget"

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
		Left:   []config.BarItem{{Module: config.ModuleWorkspaces}},
		Center: []config.BarItem{{Module: config.ModuleClock, Class: "mine"}},
		Right: []config.BarItem{
			{Module: config.ModuleBattery},
			{Module: config.ModuleSeparator},
			{Group: &config.BarGroup{Name: "status", Modules: []config.BarItem{{Module: config.ModuleNetwork}, {Module: config.ModuleAudio}}}},
		},
	}}
	bar, ok := Preview(cfg, treetest.Font(t), AllMonitors)
	if !ok {
		t.Fatal("the default layout shows no bar")
	}
	if got := []int{len(bar.Start().Children()), len(bar.Center().Children()), len(bar.End().Children())}; got[0] != 1 || got[1] != 1 || got[2] != 3 {
		t.Fatalf("zones hold %v items, want 1, 1 and 3", got)
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
	if b := sampleButton(t, bar, "workspaces"); b.Label().Text() != ModuleTitle("workspaces", cfg) {
		t.Errorf("a module without an icon of its own shows %q, want its title", b.Label().Text())
	}
}

func TestThePreviewFollowsTheConfig(t *testing.T) {
	cfg := config.Defaults()
	before, _ := Preview(cfg, treetest.Font(t), AllMonitors)
	if b := sampleButton(t, before, "battery"); b.HasClass("icon-only") || !b.Label().Visible() || before.Edge() != widget.BarTop {
		t.Fatalf("the default battery is icon-only %v with its label shown %v on edge %v", b.HasClass("icon-only"), b.Label().Visible(), before.Edge())
	}
	cfg.Battery.LabelShow = false
	cfg.Bar.ButtonVariant = config.ButtonBlockPrefix
	cfg.Bar.Location = config.LocationLeft
	after, _ := Preview(cfg, treetest.Font(t), AllMonitors)
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
	if _, ok := Preview(cfg, treetest.Font(t), AllMonitors); ok {
		t.Error("a layout for DP-1 alone previews a bar on every monitor")
	}
	cfg.Bar.Layout = []config.BarLayout{{Monitor: AllMonitors, Show: false, Center: []config.BarItem{{Module: config.ModuleClock}}}}
	if _, ok := Preview(cfg, treetest.Font(t), AllMonitors); ok {
		t.Error("a hidden layout previews a bar")
	}
	cfg.Bar.Layout = nil
	if _, ok := Preview(cfg, treetest.Font(t), AllMonitors); ok {
		t.Error("no layout previews a bar")
	}
}
