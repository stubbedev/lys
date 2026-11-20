package barview

import (
	"slices"
	"strconv"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
)

// HyprNumbering labels Hyprland's workspaces on one monitor's bar as
// the workspaces module's workspace-map, numbering and label-use-name
// say.
type HyprNumbering struct {
	styles    map[string]config.WorkspaceStyle
	numbering config.Numbering
	useName   bool
	monitor   string
	onMonitor []int
}

// NewHyprNumbering labels the workspaces on monitor's bar, rules being
// Hyprland's workspace rules: the monitor each workspace id is bound
// to.
func NewHyprNumbering(cfg config.WorkspacesConfig, monitor string, rules map[int]string) HyprNumbering {
	return HyprNumbering{
		styles: cfg.WorkspaceMap, numbering: cfg.Numbering, useName: cfg.LabelUseName,
		monitor: monitor, onMonitor: hyprMonitorWorkspaces(monitor, rules),
	}
}

// Label is the label of the workspace id named name: its workspace-map
// label, else its name or number.
func (n HyprNumbering) Label(id int, name string) string {
	if style, ok := n.styles[strconv.Itoa(id)]; ok && style.Label != nil {
		return *style.Label
	}
	return hyprLabel(hyprDisplayID(id, n.numbering, n.monitor, n.onMonitor), id, name, n.useName)
}

// hyprMonitorWorkspaces is the positive rule ids bound to one monitor.
func hyprMonitorWorkspaces(barMonitor string, rules map[int]string) []int {
	var out []int
	for id, monitor := range rules {
		if monitor == barMonitor && id > 0 {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}

func hyprDisplayID(id int, numbering config.Numbering, barMonitor string, monitorWorkspaces []int) int {
	if numbering != config.NumberingRelative || id <= 0 || barMonitor == "" || len(monitorWorkspaces) == 0 {
		return id
	}
	for pos, wsID := range monitorWorkspaces {
		if wsID == id {
			return pos + 1
		}
	}
	return id
}

// hyprLabel is a workspace's label, except for special workspaces:
// Hyprland numbers those -99 upward in creation order, an id nobody
// chose or recognizes, so a scratchpad shows its name ("special:magic"
// as "magic") instead.
func hyprLabel(displayID, id int, name string, useName bool) string {
	if special, ok := strings.CutPrefix(name, "special:"); ok && id < 0 && special != "" {
		return special
	}
	if useName && name != "" && name != strconv.Itoa(id) {
		return name
	}
	return strconv.Itoa(displayID)
}
