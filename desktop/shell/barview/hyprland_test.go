package barview

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
)

func TestHyprRelativeNumbering(t *testing.T) {
	rules := map[int]string{5: "DP-1", 3: "DP-1", 7: "DP-2", -1: "DP-1"}
	monWS := hyprMonitorWorkspaces("DP-1", rules)
	if !slices.Equal(monWS, []int{3, 5}) {
		t.Fatalf("monitor workspaces = %v, want sorted positives", monWS)
	}
	if got := hyprDisplayID(5, config.NumberingRelative, "DP-1", monWS); got != 2 {
		t.Errorf("relative 5 = %d, want 2", got)
	}
	if got := hyprDisplayID(10, config.NumberingRelative, "DP-1", monWS); got != 10 {
		t.Errorf("unruled 10 = %d, want its id", got)
	}
	if got := hyprDisplayID(5, config.NumberingAbsolute, "DP-1", monWS); got != 5 {
		t.Errorf("absolute = %d", got)
	}
	if got := hyprDisplayID(5, config.NumberingRelative, "", monWS); got != 5 {
		t.Errorf("no bar monitor = %d", got)
	}
}

func TestHyprLabels(t *testing.T) {
	if got := hyprLabel(2, 5, "5", true); got != "2" {
		t.Errorf("default name with use-name = %q, want the display id", got)
	}
	if got := hyprLabel(5, 5, "web", true); got != "web" {
		t.Errorf("custom name = %q", got)
	}
	if got := hyprLabel(5, 5, "web", false); got != "5" {
		t.Errorf("use-name off = %q", got)
	}
	for _, useName := range []bool{false, true} {
		if got := hyprLabel(-98, -98, "special:magic", useName); got != "magic" {
			t.Errorf("special (use-name %v) = %q, want its name, not Hyprland's arbitrary id", useName, got)
		}
	}
	if got := hyprLabel(-99, -99, "special", false); got != "-99" {
		t.Errorf("an unnamed special workspace = %q, want the id it has no name for", got)
	}
}

func TestAWorkspaceMapLabelWinsOverHyprlandsNumbering(t *testing.T) {
	cfg := config.Defaults().Workspaces
	cfg.Numbering = config.NumberingRelative
	label := "mail"
	cfg.WorkspaceMap = map[string]config.WorkspaceStyle{"5": {Label: &label}}
	n := NewHyprNumbering(cfg, "DP-1", map[int]string{4: "DP-1", 5: "DP-1"})
	if got := n.Label(5, "5"); got != "mail" {
		t.Errorf("a mapped workspace reads %q, want its map label", got)
	}
	if got := n.Label(4, "4"); got != "1" {
		t.Errorf("an unmapped workspace reads %q, want its number on the monitor", got)
	}
}
