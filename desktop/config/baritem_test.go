package config

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestUpdatesIsAnUnknownBarModule(t *testing.T) {
	var module BarModule
	err := module.UnmarshalConfig("updates")
	if err == nil || !strings.HasPrefix(err.Error(), "unknown variant `updates`, expected one of `audio`") {
		t.Fatalf("updates = %v, want the unknown-variant error", err)
	}
	if _, removed := errors.AsType[*RemovedModuleError](err); removed {
		t.Errorf("updates = %v, want an unknown module, not a removed one", err)
	}
	if strings.Contains(err.Error(), "`updates`, `") || strings.Contains(err.Error(), ", `updates`") {
		t.Errorf("updates = %v, which still offers updates", err)
	}
	if err := module.UnmarshalConfig("weather"); err != nil || module != ModuleWeather {
		t.Errorf("weather = %q, %v, want the weather module", module, err)
	}
}

func TestUpdatesInABarLayoutIsRefused(t *testing.T) {
	for shape, left := range map[string]string{
		"plain":   "updates",
		"classed": "{module: updates, class: x}",
		"grouped": "{name: g, modules: [clock, updates]}",
	} {
		cfg, err := LoadFile(writeConfig(t, "bar:\n  layout:\n    - left: [clock, "+left+"]\n"))
		diags := diagnosticsTitled(err, "invalid config value")
		if len(diags) != 1 || diags[0].Field("Error") != "data did not match any variant of untagged enum BarItem" {
			t.Errorf("%s: diagnostics %+v from %v, want the untagged-enum BarItem error", shape, diags, err)
		}
		if len(cfg.Bar.Layout) != 1 || !slices.Equal(cfg.Bar.Layout[0].Left, DefaultBarLayout().Left) {
			t.Errorf("%s: a refused layout = %+v, want the default", shape, cfg.Bar.Layout)
		}
	}
	if _, err := LoadFile(writeConfig(t, "bar:\n  layout:\n    - left: [clock, weather]\n")); err != nil {
		t.Errorf("a layout without updates = %v", err)
	}
}
