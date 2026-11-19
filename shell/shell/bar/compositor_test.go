package bar

import (
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/compositor"
	"github.com/stubbedev/lys/desktop/config"
)

// advertise sets exactly the named compositor variables, hiding the
// rest the host session exports.
func advertise(t *testing.T, set ...string) {
	t.Helper()
	clearCompositorEnv(t)
	for _, name := range set {
		t.Setenv(name, "/run/test-"+name)
	}
}

// A Hyprland session that also exports SWAYSOCK is Hyprland: the
// workspaces module is Hyprland's, never sway's.
func TestWorkspacesUnderHyprlandIsNotSways(t *testing.T) {
	fakeI3(t, "[]", `{"id":1,"type":"root","nodes":[],"floating_nodes":[]}`)
	ctx := newTestContext(t, config.Defaults())
	if _, err := createOnHeadlessLoop("workspaces", ctx); err != nil {
		t.Fatalf("workspaces under sway: %v", err)
	}
	t.Setenv(compositor.HyprlandSocketEnv, "/run/test-hyprland")
	_, err := createOnHeadlessLoop("workspaces", ctx)
	if err == nil || !strings.Contains(err.Error(), "Hyprland") {
		t.Errorf("workspaces under Hyprland without its connection: %v, want Hyprland's error", err)
	}
}

func TestDetectCompositorReadsTheSessionEnvironment(t *testing.T) {
	advertise(t, compositor.NiriSocketEnv, compositor.SwaySocketEnv)
	if got := detectCompositor(); got != compositor.Niri {
		t.Errorf("detected %v, want niri", got)
	}
}

func TestNewWorkspacesFollowsTheCompositor(t *testing.T) {
	for _, env := range []string{compositor.LysWMSocketEnv, compositor.HyprlandSocketEnv, compositor.NiriSocketEnv, compositor.MangoSocketEnv, compositor.SwaySocketEnv} {
		t.Setenv(env, "")
	}
	_, err := newWorkspaces(ModuleContext{})
	if err == nil || !strings.Contains(err.Error(), "unknown") {
		t.Errorf("no compositor: %v, want an error naming it", err)
	}
	t.Setenv(compositor.SwaySocketEnv, "/nonexistent/sway.sock")
	if _, err = newWorkspaces(newTestContext(t, config.Defaults())); err != nil {
		t.Errorf("under sway the workspaces module was refused: %v", err)
	}
}
