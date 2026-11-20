package config

import (
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/compositor"
)

func TestAWorkspaceKeyIsHonouredOnlyWhereItsCompositorReadsIt(t *testing.T) {
	for _, tc := range []struct {
		key  string
		on   compositor.Kind
		want bool
	}{
		{"display-mode", compositor.Mango, true},
		{"display-mode", compositor.Hyprland, true},
		{"show-special", compositor.Hyprland, true},
		{"show-special", compositor.Niri, false},
		{"label-strategy", compositor.Sway, true},
		{"label-strategy", compositor.LysWM, false},
		{"monitor-specific", compositor.Mango, false},
		{"monitor-specific", compositor.LysWM, true},
		{"min-tag-count", compositor.Mango, true},
		{"min-tag-count", compositor.Hyprland, false},
	} {
		if got := WorkspaceKeyHonored(tc.key, tc.on); got != tc.want {
			t.Errorf("%s on %v: %v, want %v", tc.key, tc.on, got, tc.want)
		}
	}
}

func TestEveryCompositorOnlyWorkspaceKeyIsAKey(t *testing.T) {
	for key := range workspaceKeyCompositors {
		if _, ok := Field("modules.workspaces." + key); !ok {
			t.Errorf("%s is tied to compositors but is no modules.workspaces key", key)
		}
	}
}

func TestTheViewLeavesOutWhatTheCompositorDoesNotHonour(t *testing.T) {
	c := DefaultsWorkspaces()
	c.LabelStrategy, c.MonitorSpecific, c.WorkspaceIgnore, c.HideTrailingEmpty = LabelIndexAndName, true, []string{"scratch"}, true
	niri := c.View(compositor.Niri)
	if niri.LabelStrategy != LabelIndexAndName || !niri.MonitorSpecific || len(niri.WorkspaceIgnore) != 1 || !niri.HideTrailingEmpty {
		t.Errorf("niri view %+v drops keys niri honours", niri)
	}
	mango := c.View(compositor.Mango)
	if mango.MonitorSpecific || mango.WorkspaceIgnore != nil || mango.HideTrailingEmpty || mango.LabelStrategy != LabelIndex {
		t.Errorf("mango view %+v keeps keys mango does not read", mango)
	}
	if lys := c.View(compositor.LysWM); lys.LabelStrategy != LabelIndex || !lys.MonitorSpecific {
		t.Errorf("lys-wm view %+v: want index labels and its own monitor", lys)
	}
}

func TestAnOldWorkspaceModuleNamesItsReplacement(t *testing.T) {
	for _, old := range []string{"hyprland-workspaces", "niri-workspaces", "sway-workspaces", "mango-workspaces", "lys-wm-workspaces"} {
		_, err := LoadFile(writeConfig(t, "modules:\n  "+old+":\n    divider: \"|\"\n"))
		if err == nil || !strings.Contains(err.Error(), "modules.workspaces") {
			t.Errorf("modules.%s: %v, want an error naming modules.workspaces", old, err)
		}
	}
	if _, err := LoadFile(writeConfig(t, "modules:\n  workspaces:\n    divider: \"|\"\n")); err != nil {
		t.Errorf("modules.workspaces: %v", err)
	}
}

func TestWorkspaceWindowPreviewsAreOffUnlessTurnedOn(t *testing.T) {
	kinds := []compositor.Kind{compositor.LysWM, compositor.Hyprland, compositor.Niri, compositor.Mango, compositor.Sway}
	for _, on := range kinds {
		if Defaults().Workspaces.View(on).WindowPreviews {
			t.Errorf("%v: window previews are on by default", on)
		}
	}
	cfg, err := LoadFile(writeConfig(t, "modules:\n  workspaces:\n    window-previews: true\n"))
	if err != nil {
		t.Fatal(err)
	}
	for _, on := range kinds {
		if !cfg.Workspaces.View(on).WindowPreviews {
			t.Errorf("%v: window-previews: true leaves previews off", on)
		}
	}
	if _, err := LoadFile(writeConfig(t, "modules:\n  workspaces:\n    window-previews: sometimes\n")); err == nil || !strings.Contains(err.Error(), "window-previews") {
		t.Errorf("window-previews: sometimes: %v, want an error naming the key", err)
	}
}
