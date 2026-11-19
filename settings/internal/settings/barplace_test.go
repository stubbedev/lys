package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func TestLayoutsHoldAModuleInAnyZoneOrGroup(t *testing.T) {
	layouts := []config.BarLayout{{
		Monitor: "*", Left: []config.BarItem{{Module: config.ModuleClock}},
		Right: []config.BarItem{{Group: &config.BarGroup{Name: "status", Modules: []config.BarItem{{Module: config.ModuleBattery}}}}},
	}}
	for m, want := range map[config.BarModule]bool{config.ModuleClock: true, config.ModuleBattery: true, config.ModuleWeather: false} {
		if got := layoutsHold(layouts, m); got != want {
			t.Errorf("%s: %v, want %v", m, got, want)
		}
	}
}

func TestAddingAModuleAppendsItToTheEveryMonitorLayout(t *testing.T) {
	layouts := []config.BarLayout{{Monitor: "DP-1"}, {Monitor: "*", Right: []config.BarItem{{Module: config.ModuleClock}}}}
	got := withModuleAdded(layouts, config.ModuleWeather)
	if !layoutsHold(got[1:], config.ModuleWeather) || layoutsHold(got[:1], config.ModuleWeather) {
		t.Errorf("added to %+v, want the * layout's right zone", got)
	}
	if layoutsHold(layouts, config.ModuleWeather) {
		t.Error("adding changed the layouts it was given")
	}
	if fresh := withModuleAdded(nil, config.ModuleWeather); len(fresh) != 1 || fresh[0].Monitor != "*" || !layoutsHold(fresh, config.ModuleWeather) {
		t.Errorf("with no layout: %+v", fresh)
	}
}

func TestABarItemPageSaysWhetherItIsInTheBarAndAddsIt(t *testing.T) {
	k := testKit(t, "bar:\n  layout:\n    - monitor: \"*\"\n      right: [clock]\n")
	lead := barPlacement(config.ModuleWeather)(k)
	add := treetest.Button(t, lead, locale.T().Get("settings-bar-placement-add"))
	if !add.Visible() {
		t.Fatal("a module outside the bar offers no add")
	}
	add.OnClick()
	if !layoutsHold(k.store.svc.Config().Bar.Layout, config.ModuleWeather) {
		t.Fatal("add did not put the module in the bar")
	}
	if add.Visible() || !containsLabel(lead, locale.T().Get("settings-bar-placement-present")) {
		t.Error("after adding, the page still offers to add it")
	}
	in := barPlacement(config.ModuleClock)(k)
	if treetest.Button(t, in, locale.T().Get("settings-bar-placement-add")).Visible() {
		t.Error("a module in the bar offers to add it again")
	}
}

func containsLabel(w widget.Widget, text string) bool {
	return slices.Contains(treetest.Labels(w), text)
}
