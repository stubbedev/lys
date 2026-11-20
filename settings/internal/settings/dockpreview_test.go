package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/shell/dockview"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func dockPagePreview(t *testing.T, p *settingsPage) *dockPreview {
	t.Helper()
	previews := treetest.All[*dockPreview](p)
	if len(previews) != 1 {
		t.Fatalf("the dock page holds %d dock previews, want one", len(previews))
	}
	return previews[0]
}

type previewedEntry struct {
	icon, name string
	running    bool
}

func previewedDock(t *testing.T, p *settingsPage) (*widget.Box, []previewedEntry) {
	t.Helper()
	preview := dockPagePreview(t, p)
	panels := treetest.WithClass(preview, "dock")
	if len(panels) != 1 {
		t.Fatalf("the preview shows %v instead of one dock", treetest.Labels(preview))
	}
	var out []previewedEntry
	for _, e := range treetest.All[*dockview.TaskButton](preview) {
		out = append(out, previewedEntry{icon: e.Icon(), name: e.TooltipText(), running: e.HasClass("running")})
	}
	return panels[0].(*widget.Box), out
}

func TestTheDockPagePreviewsThePinnedAppsAndAnOpenOne(t *testing.T) {
	dockAppsFixture(t)
	k := testKit(t, "dock:\n  enabled: true\n  pinned: [firefox, gone-app]\n")
	p := buildPage(k, dockPage(nil))
	panel, entries := previewedDock(t, p)
	want := []previewedEntry{
		{icon: "firefox", name: "Firefox"},
		{icon: k.store.svc.Config().Dock.Icon, name: "gone-app"},
		{icon: "accessories-calculator", name: "Calculator", running: true},
	}
	if !slices.Equal(entries, want) {
		t.Fatalf("the preview shows %+v, want %+v", entries, want)
	}
	if !panel.HasClass("bottom") || panel.Axis() != widget.Row {
		t.Errorf("the default dock previews with classes %v along %v, want the bottom edge", panel.Classes(), panel.Axis())
	}

	set(t, k, p, "dock.edge", "left")
	set(t, k, p, "dock.pinned", []any{"org.gnome.Calculator"})
	panel, entries = previewedDock(t, p)
	if !panel.HasClass("left") || panel.Axis() != widget.Column {
		t.Errorf("dock.edge left previews with classes %v along %v", panel.Classes(), panel.Axis())
	}
	want = []previewedEntry{
		{icon: "accessories-calculator", name: "Calculator"},
		{icon: "org.gnome.Nautilus", name: "Fichiers", running: true},
	}
	if !slices.Equal(entries, want) {
		t.Fatalf("after the edits the preview shows %+v, want %+v", entries, want)
	}
}

func TestADisabledDockPreviewsThatNothingShows(t *testing.T) {
	dockAppsFixture(t)
	k := testKit(t, "dock:\n  enabled: false\n  pinned: [firefox]\n")
	p := buildPage(k, dockPage(nil))
	preview := dockPagePreview(t, p)
	if docks := treetest.WithClass(preview, "dock"); len(docks) != 0 {
		t.Error("a disabled dock still previews its panel")
	}
	if want := locale.T().Get("settings-dock-preview-off"); !slices.Contains(treetest.Labels(preview), want) {
		t.Errorf("the preview says %v, want %q", treetest.Labels(preview), want)
	}
	set(t, k, p, "dock.enabled", true)
	previewedDock(t, p)
}

func TestTheOpenAppOfTheDockPreviewIsASampleWhenEveryAppIsPinned(t *testing.T) {
	dockAppsFixture(t)
	k := testKit(t, "dock:\n  enabled: true\n  pinned: [firefox, org.gnome.Nautilus, org.gnome.Calculator]\n")
	p := buildPage(k, dockPage(nil))
	_, entries := previewedDock(t, p)
	want := previewedEntry{icon: k.store.svc.Config().Dock.Icon, name: locale.T().Get("settings-dock-preview-running"), running: true}
	if len(entries) != 4 || entries[3] != want || slices.ContainsFunc(entries[:3], func(e previewedEntry) bool { return e.running }) {
		t.Fatalf("with every app pinned the preview shows %+v, want the pinned apps then %+v", entries, want)
	}
}

func TestTheDockPreviewScansOffTheLoopAndRedrawsWithTheApps(t *testing.T) {
	dockAppsFixture(t)
	k := testKit(t, "dock:\n  enabled: true\n  pinned: [firefox]\n")
	var scans []func()
	k.background = func(fn func()) { scans = append(scans, fn) }
	p := buildPage(k, dockPage(nil))
	_, entries := previewedDock(t, p)
	fallback := k.store.svc.Config().Dock.Icon
	if len(entries) != 2 || entries[0] != (previewedEntry{icon: fallback, name: "firefox"}) {
		t.Fatalf("before the scan the preview shows %+v, want firefox by id with the dock's icon", entries)
	}
	for _, scan := range scans {
		scan()
	}
	if _, entries = previewedDock(t, p); entries[0] != (previewedEntry{icon: "firefox", name: "Firefox"}) {
		t.Fatalf("after the scan the preview shows %+v", entries)
	}
}
