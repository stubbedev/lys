package bar

import (
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/barview"

	"github.com/stubbedev/lys/shell/internal/appicons"
)

// cwsWindow is one compositor window as a backend reports it. order is
// the backend's window sort key ahead of the id: niri's scrolling
// (column, tile), zero for sway (id order).
type cwsWindow struct {
	id           uint64
	workspace    uint64
	hasWorkspace bool
	app          appicons.Window
	urgent       bool
	order        [2]int
}

// cwsShownWindows is windows as the strip shows them, each with the app
// icon cfg resolves for it when the strip shows app icons.
func cwsShownWindows(windows []cwsWindow, cfg config.CompositorWorkspacesConfig) []barview.WorkspaceWindow {
	out := make([]barview.WorkspaceWindow, len(windows))
	for i, w := range windows {
		out[i] = barview.WorkspaceWindow{ID: w.id, Workspace: w.workspace, HasWorkspace: w.hasWorkspace, Urgent: w.urgent, Order: w.order}
		if cfg.AppIconsShow {
			out[i].Icon = appicons.Resolve(w.app, cfg.AppIconMap, cfg.AppIconsFallback)
		}
	}
	return out
}

func cwsFollowsTitles(cfg config.CompositorWorkspacesConfig) bool {
	if !cfg.AppIconsShow {
		return false
	}
	for key := range cfg.AppIconMap {
		if strings.HasPrefix(key, "title:") {
			return true
		}
	}
	return false
}
