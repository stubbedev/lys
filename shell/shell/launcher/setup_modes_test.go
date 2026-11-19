package launcher

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"

	"github.com/stubbedev/lys/shell/internal/launcheripc"
	engine "github.com/stubbedev/lys/shell/service/launcher"
	"github.com/stubbedev/lys/shell/service/launcher/modes"
)

func TestEveryBuiltinModeNamedToSettingsIsBuilt(t *testing.T) {
	cfg := config.Defaults()
	deps := setupDeps{vault: func() engine.Mode { return modes.NewCalc() }}
	aliases := []string{VaultModeName, "windowcd", "recursivebrowser"}
	for _, name := range config.LauncherBuiltinModes {
		m := buildMode(name, launcheripc.SessionOptions{}, cfg, nil, nil, deps)
		if m == nil {
			t.Errorf("%s is listed as a built-in mode but builds none", name)
			continue
		}
		if !slices.Contains(aliases, name) && m.Name() != name {
			t.Errorf("%s builds the %s mode", name, m.Name())
		}
	}
	if m := buildMode("no-such-mode", launcheripc.SessionOptions{}, cfg, nil, nil, deps); m != nil {
		t.Errorf("an unlisted name without a script built %s", m.Name())
	}
}
