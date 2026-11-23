package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"
)

func iconsShown(w widget.Widget) []string {
	var out []string
	for _, ic := range treetest.All[*widget.Icon](w) {
		out = append(out, ic.Name())
	}
	return out
}

func TestEveryModuleWithAPageShowsItsIconInTheLayout(t *testing.T) {
	for _, m := range config.BuiltinModules {
		icon, ok := moduleIcon(m)
		if !ok || icon == "" {
			t.Errorf("%s shows no icon", m)
		}
	}
	if icon, _ := moduleIcon(config.CustomModule("gpu")); icon != "ld-code-symbolic" {
		t.Errorf("a custom module shows %q, want the custom modules' icon", icon)
	}
}

func TestLayoutChipsAndThePickerShowEachModulesIcon(t *testing.T) {
	_, c := newTestLayout(t)
	icons := iconsShown(c)
	for _, m := range []config.BarModule{config.ModuleClock, config.ModuleCPU, config.ModuleRAM, config.ModuleBattery, config.ModuleAudio} {
		want, _ := moduleIcon(m)
		if !slices.Contains(icons, want) {
			t.Errorf("the layout shows no %s icon for %s: %v", want, m, icons)
		}
	}
	p := c.pickModule(nil, func(config.BarModule) {})
	at := slices.Index(p.all, string(config.ModuleClock))
	if got := iconsShown(p.rows.ChildAt(at)); !slices.Equal(got, []string{"ld-clock-symbolic"}) {
		t.Errorf("the clock's picker row shows %v", got)
	}
}
