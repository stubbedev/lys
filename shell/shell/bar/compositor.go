package bar

import (
	"fmt"
	"os"

	"github.com/stubbedev/lys/desktop/compositor"
)

// detectCompositor is the session's compositor as its environment
// advertises it.
func detectCompositor() compositor.Kind { return compositor.Detect(os.Getenv) }

func compositorHasClient() bool {
	switch detectCompositor() {
	case compositor.LysWM, compositor.Niri, compositor.Mango, compositor.Sway:
		return true
	case compositor.Hyprland, compositor.Unknown:
		return false
	}
	return false
}

// newWorkspaces is the running compositor's workspace module, so that a
// layout works on any of them.
func newWorkspaces(ctx ModuleContext) (Module, error) {
	switch c := detectCompositor(); c {
	case compositor.LysWM:
		return newLysWMWorkspaces(ctx)
	case compositor.Hyprland:
		return newHyprlandWorkspaces(ctx)
	case compositor.Niri:
		return newNiriWorkspaces(ctx)
	case compositor.Mango:
		return newMangoWorkspaces(ctx)
	case compositor.Sway:
		return newSwayWorkspaces(ctx)
	default:
		return nil, fmt.Errorf("workspaces: no workspace module for the compositor, which is %s", c)
	}
}
