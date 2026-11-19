package config

import "testing"

func TestModuleIconIsTheIconTheModuleStartsWith(t *testing.T) {
	cfg := Defaults()
	cfg.IdleInhibit.IconInactive = "sleep-symbolic"
	cfg.Dashboard.IconOverride = ""
	def := DefaultsCustomModuleDefinition()
	def.Id, def.IconName = "vpn", "shield-symbolic"
	cfg.Custom = append(cfg.Custom, def)
	for _, tc := range []struct {
		module BarModule
		want   IconConfig
	}{
		{ModuleBattery, cfg.Battery.Icon()},
		{ModuleIdleInhibit, IconWith(cfg.IdleInhibit.IconShow, "sleep-symbolic", cfg.IdleInhibit.IconColor)},
		{ModuleDashboard, IconWith(true, DashboardFallbackIcon, cfg.Dashboard.IconColor)},
		{ModuleScreenshot, IconWith(cfg.Screenshot.IconShow, cfg.Screenshot.Icon, cfg.Screenshot.IconColor)},
		{CustomModule("vpn"), IconWith(def.IconShow, "shield-symbolic", def.IconColor)},
	} {
		got, ok := cfg.ModuleIcon(tc.module)
		if !ok || got != tc.want {
			t.Errorf("%s: icon %+v (%v), want %+v", tc.module, got, ok, tc.want)
		}
	}
	cfg.Dashboard.IconOverride = "my-logo"
	if got, _ := cfg.ModuleIcon(ModuleDashboard); got.Name != "my-logo" {
		t.Errorf("the dashboard override shows %q, want my-logo", got.Name)
	}
}

func TestModuleIconRefusesModulesWithoutAnIconOfTheirOwn(t *testing.T) {
	cfg := Defaults()
	for _, m := range []BarModule{ModuleWorkspaces, ModuleSystray, ModuleCava, ModuleSeparator, ModuleMinimizedWindows, CustomModule("undefined")} {
		if got, ok := cfg.ModuleIcon(m); ok {
			t.Errorf("%s has the icon %+v, want none", m, got)
		}
	}
}
