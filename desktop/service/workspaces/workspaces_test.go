package workspaces

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/sockdir"
	"github.com/stubbedev/lys/lib/testhome"

	"github.com/stubbedev/lys/desktop/compositor"
	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/service/niri"
	"github.com/stubbedev/lys/desktop/service/sway"
	"github.com/stubbedev/lys/desktop/service/workspaces/workspacestest"
	"github.com/stubbedev/lys/desktop/shell/barview"
)

func TestMain(m *testing.M) { testhome.Run(m) }

func TestTheTestsReachNoHostSession(t *testing.T) { testhome.RequireNoHostSession(t) }

func TestSwayOffersEachWorkspaceByItsName(t *testing.T) {
	workspacestest.Sway(t, `[
		{"id":4,"num":10,"name":"10","output":"DP-1"},
		{"id":3,"num":-1,"name":"web","output":"DP-1"},
		{"id":2,"num":2,"name":"2: mail","output":"HDMI-A-1"},
		{"id":1,"num":2,"name":"2","output":"DP-1"}
	]`)
	names, err := IgnoreNames(compositor.Sway)
	if want := []string{"2", "10", "2: mail", "web"}; err != nil || !slices.Equal(names, want) {
		t.Fatalf("sway offers %q (%v), want %q", names, err, want)
	}
}

func TestNiriOffersANamedWorkspaceByNameAndTheOthersByIndexOnce(t *testing.T) {
	workspacestest.Niri(t, `[
		{"id":9,"idx":2,"name":null,"output":"DP-1"},
		{"id":8,"idx":1,"name":"chat","output":"DP-1"},
		{"id":7,"idx":2,"name":null,"output":"HDMI-A-1"},
		{"id":6,"idx":1,"name":null,"output":"HDMI-A-1"}
	]`)
	names, err := IgnoreNames(compositor.Niri)
	if want := []string{"1", "2", "chat"}; err != nil || !slices.Equal(names, want) {
		t.Fatalf("niri offers %q (%v), want %q", names, err, want)
	}
}

func TestHyprlandOffersEachWorkspaceByItsID(t *testing.T) {
	workspacestest.Hyprland(t, `[{"id":3,"name":"3"},{"id":-98,"name":"special:scratch"},{"id":-1337,"name":"web"},{"id":1,"name":"1"}]`)
	names, err := IgnoreNames(compositor.Hyprland)
	if want := []string{"-1337", "-98", "1", "3"}; err != nil || !slices.Equal(names, want) {
		t.Fatalf("Hyprland offers %q (%v), want %q", names, err, want)
	}
}

func TestEveryOfferedNameHidesItsWorkspaceInTheBar(t *testing.T) {
	name := "chat"
	for _, ws := range []barview.Workspace{
		FromSway(sway.Workspace{ID: 3, Num: -1, Name: "web"}, false),
		FromNiri(niri.Workspace{ID: 8, Idx: 1, Name: &name}),
		FromNiri(niri.Workspace{ID: 9, Idx: 2}),
	} {
		other := ws
		other.ID, other.Num, other.Name = ws.ID+100, ws.Num+100, ws.Name+"-other"
		shown := barview.WorkspaceButtons(barview.FlavorWorkspaces, []barview.Workspace{ws, other}, nil, config.CompositorWorkspacesConfig{WorkspaceIgnore: []string{ws.IgnoreName()}}, "", false, false)
		if len(shown) != 1 || shown[0].WS.ID != other.ID {
			t.Errorf("ignoring %q leaves %v", ws.IgnoreName(), shown)
		}
	}
}

func TestACompositorThatIsNotRunningIsAnErrorNamingIt(t *testing.T) {
	for _, tc := range []struct {
		on  compositor.Kind
		env string
	}{
		{compositor.Sway, compositor.SwaySocketEnv},
		{compositor.Niri, compositor.NiriSocketEnv},
		{compositor.Hyprland, compositor.HyprlandSocketEnv},
	} {
		t.Setenv(tc.env, "")
		if names, err := IgnoreNames(tc.on); err == nil || !strings.Contains(err.Error(), tc.on.String()) {
			t.Errorf("%s without its socket offers %q, error %v", tc.on, names, err)
		}
	}
}

func TestADeadSocketIsAnError(t *testing.T) {
	gone := filepath.Join(sockdir.Dir(t), "gone.sock")
	t.Setenv(compositor.SwaySocketEnv, gone)
	t.Setenv(compositor.NiriSocketEnv, gone)
	for _, on := range []compositor.Kind{compositor.Sway, compositor.Niri} {
		if names, err := IgnoreNames(on); err == nil {
			t.Errorf("%s on a dead socket offers %q", on, names)
		}
	}
}

func TestAMalformedReplyIsAnError(t *testing.T) {
	workspacestest.Sway(t, `{"not":"a list"}`)
	workspacestest.Niri(t, `"not a list"`)
	workspacestest.Hyprland(t, `[{"id":"one"}]`)
	for _, on := range []compositor.Kind{compositor.Sway, compositor.Niri, compositor.Hyprland} {
		if names, err := IgnoreNames(on); err == nil {
			t.Errorf("%s offers %q from a malformed reply", on, names)
		}
	}
}

func TestOnlyHyprlandNiriAndSwayAreListed(t *testing.T) {
	for _, on := range []compositor.Kind{compositor.LysWM, compositor.Mango, compositor.Unknown} {
		if names, err := IgnoreNames(on); !errors.Is(err, ErrNotListed) {
			t.Errorf("%s offers %q, error %v", on, names, err)
		}
	}
}
