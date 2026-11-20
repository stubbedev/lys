package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const allowAppsPath = "modules.notifications.dnd.allow-apps"

func allowAppsGrid(t *testing.T, k *kit) *appGridEditor {
	t.Helper()
	g, ok := fieldControl(k, k.store, pageRow(t, "notifications", allowAppsPath)).(*appGridEditor)
	if !ok {
		t.Fatal("the allowed apps are not an app grid")
	}
	return g
}

func allowedApps(k *kit) []any {
	v, _ := k.store.value(allowAppsPath).([]any)
	return v
}

func tileLabels(g *appGridEditor) [][]string {
	var out [][]string
	for i := range g.Len() - 1 {
		out = append(out, treetest.Labels(g.ChildAt(i)))
	}
	return out
}

func tileIcons(t *testing.T, g *appGridEditor) []string {
	t.Helper()
	var out []string
	for i := range g.Len() - 1 {
		out = append(out, treetest.All[*widget.Icon](g.ChildAt(i))[0].Name())
	}
	return out
}

func TestTheAllowedAppsAreTilesWithTheirIconsAndNames(t *testing.T) {
	dockAppsFixture(t)
	k := testKit(t, "modules:\n  notifications:\n    dnd:\n      allow-apps: [firefox, gone-app, \"*chromium*\"]\n")
	g := allowAppsGrid(t, k)
	if got := tileLabels(g); len(got) != 3 || got[0][0] != "Firefox" || got[1][0] != "gone-app" || got[2][0] != "*chromium*" {
		t.Fatalf("the tiles read %q, want Firefox by name, the uninstalled app by its id and the pattern as written", got)
	}
	if got := tileIcons(t, g); !slices.Equal(got, []string{"firefox", appFallbackIcon, appFallbackIcon}) {
		t.Fatalf("the tiles show the icons %q", got)
	}
	if !slices.Equal(allowedApps(k), []any{"firefox", "gone-app", "*chromium*"}) {
		t.Fatalf("showing the grid rewrote the list: %v", allowedApps(k))
	}
}

func TestAnAllowedAppIsPickedFromTheAppsNotYetAllowedAndRemovedFromItsTile(t *testing.T) {
	dockAppsFixture(t)
	k := testKit(t, "modules:\n  notifications:\n    dnd:\n      allow-apps: [firefox]\n")
	g := allowAppsGrid(t, k)
	f := &fakePickers{}
	k.pickers = f
	g.add.ClickAt(widget.Point{})
	if f.content == nil || f.anchor != widget.Widget(g.add) {
		t.Fatal("the app picker did not open beside the add button")
	}
	flow := treetest.First[*widget.FlowBox](t, f.content)
	if got := shownLabels(flow); !slices.Equal(got, []string{"Calculator", "Fichiers"}) {
		t.Fatalf("the picker offers %q, want the apps not yet allowed", got)
	}
	flow.OnActivate(slices.Index(shownLabels(flow), "Calculator"))
	if got := allowedApps(k); !slices.Equal(got, []any{"firefox", "org.gnome.Calculator"}) {
		t.Fatalf("picking Calculator wrote %v", got)
	}
	if got := tileLabels(g); len(got) != 2 || got[1][0] != "Calculator" {
		t.Fatalf("after the pick the tiles read %q", got)
	}
	remove := locale.T().Get("settings-app-grid-remove", i18n.Str("name", "Firefox"))
	var clicked bool
	for _, b := range treetest.All[*widget.Button](g) {
		if b.AccessibleName() == remove {
			b.OnClick()
			clicked = true
			break
		}
	}
	if !clicked {
		t.Fatalf("no button named %q", remove)
	}
	if got := allowedApps(k); !slices.Equal(got, []any{"org.gnome.Calculator"}) {
		t.Fatalf("removing Firefox wrote %v", got)
	}
}

func TestTheAllowedAppsOfferNoPickUntilTheAppsAreKnownAndFollowTheConfig(t *testing.T) {
	dockAppsFixture(t)
	k := testKit(t, "modules:\n  notifications:\n    dnd:\n      allow-apps: [firefox]\n")
	var scan func()
	k.background = func(fn func()) { scan = fn }
	g := allowAppsGrid(t, k)
	if scan == nil {
		t.Fatal("the grid scanned the applications on the loop")
	}
	if g.add.Visible() {
		t.Fatal("the add button shows before the apps are known")
	}
	scan()
	if !g.add.Visible() {
		t.Fatal("the add button stays hidden after the scan")
	}
	if err := k.store.set(allowAppsPath, []any{"org.gnome.Nautilus"}); err != nil {
		t.Fatal(err)
	}
	g.refresh()
	if got := tileLabels(g); len(got) != 1 || got[0][0] != "Fichiers" {
		t.Fatalf("after the config changed the tiles read %q", got)
	}
}
