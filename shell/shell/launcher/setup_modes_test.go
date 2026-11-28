package launcher

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"

	"github.com/stubbedev/lys/shell/internal/launcheripc"
	"github.com/stubbedev/lys/shell/service/fileindex"
	engine "github.com/stubbedev/lys/shell/service/launcher"
	"github.com/stubbedev/lys/shell/service/launcher/modes"
)

func TestEveryBuiltinModeNamedToSettingsIsBuilt(t *testing.T) {
	cfg := config.Defaults()
	deps := setupDeps{vault: func() engine.Mode { return modes.NewCalc() }, files: emptyFiles}
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

type fixedFiles struct{ snap *fileindex.Snapshot }

func (f fixedFiles) Snapshot() *fileindex.Snapshot { return f.snap }

func emptyFiles() modes.FileIndex { return fixedFiles{snap: &fileindex.Snapshot{Ready: true}} }

// combiOf builds the combi mode cfg describes.
func combiOf(t *testing.T, cfg *config.Config, deps setupDeps) *modes.Combi {
	t.Helper()
	m, ok := buildMode("combi", launcheripc.SessionOptions{}, cfg, nil, nil, deps).(*modes.Combi)
	if !ok {
		t.Fatal("combi built no combi mode")
	}
	return m
}

func TestCombiFindsFilesUnlessTheSettingTurnsThemOff(t *testing.T) {
	cfg := config.Defaults()
	deps := setupDeps{files: emptyFiles}
	if !combiOf(t, cfg, deps).Restricts("files") {
		t.Error("the default combi has no files")
	}
	cfg.Launcher.Combi.Files = false
	cfg.Launcher.Combi.Modes = append(cfg.Launcher.Combi.Modes, "files")
	if combiOf(t, cfg, deps).Restricts("files") {
		t.Error("combi.files off still finds files, though combi.modes names them")
	}
	cfg.Launcher.Combi.Files = true
	if !combiOf(t, cfg, deps).Restricts("files") {
		t.Error("files named in combi.modes and on are missing")
	}
}

func TestCombiWithoutAFileIndexLeavesFilesOut(t *testing.T) {
	cfg := config.Defaults()
	if combiOf(t, cfg, setupDeps{files: func() modes.FileIndex { return nil }}).Restricts("files") {
		t.Error("combi offers files with no index to search")
	}
}
