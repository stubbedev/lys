package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

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
	lead := newPlacementRow(k, only(config.ModuleWeather))
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
	in := newPlacementRow(k, only(config.ModuleClock))
	if treetest.Button(t, in, locale.T().Get("settings-bar-placement-add")).Visible() {
		t.Error("a module in the bar offers to add it again")
	}
}

// A module the bar refuses is not added, and the placement says so in
// the app's words, the refusal in the closed detail.
func TestAModuleTheBarRefusesIsSaidNotAdded(t *testing.T) {
	k := testKit(t, "bar:\n  layout:\n    - monitor: \"*\"\n      right: [clock]\n")
	p := newPlacementRow(k, only(config.BarModule("ghost")))
	p.add.OnClick()
	if layoutsHold(k.store.svc.Config().Bar.Layout, config.BarModule("ghost")) {
		t.Fatal("the refused module is in the bar")
	}
	if !failedWith(p.problem, locale.T().Get("settings-bar-placement-failed"), "bar.layout") || !p.add.Visible() {
		t.Errorf("the refusal says %q over %q, add shown %v", p.problem.Text(), p.problem.detail.Text(), p.add.Visible())
	}
}

func containsLabel(w widget.Widget, text string) bool {
	return slices.Contains(treetest.Labels(w), text)
}

// "In your bar" heads the page's preview as a section's title heads its
// section: the same size, weight and color.
func TestThePlacementReadsAsASectionTitle(t *testing.T) {
	weighted(t)
	win := laidOutPage(t, "", "battery", 1000)
	status := treetest.WithClass(win.page, "settings-bar-placement-status")[0].(*widget.Label)
	section := treetest.WithClass(win.page, "settings-section-title")[0].(*widget.Label)
	twin := win.k.label(status.Text(), "settings-section-title", css.CaptionHeading, css.DimLabel)
	themed(win.k, twin)
	if sw, tw := status.Measure(widget.Row, -1).Nat, twin.Measure(widget.Row, -1).Nat; sw != tw {
		t.Errorf("%q is %dpx wide, as a section title %dpx", status.Text(), sw, tw)
	}
	if sc, tc := widget.CascadeColor(status), widget.CascadeColor(section); sc != tc {
		t.Errorf("%q reads in %#08x, a section title in %#08x", status.Text(), uint32(sc), uint32(tc))
	}
}
