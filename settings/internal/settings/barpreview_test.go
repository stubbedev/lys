package settings

import (
	"errors"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/barview"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func previewedBar(t *testing.T, p *settingsPage) *widget.Bar {
	t.Helper()
	previews := treetest.All[*livePreview](p)
	if len(previews) != 1 {
		t.Fatalf("the page holds %d previews, want one", len(previews))
	}
	bars := treetest.All[*widget.Bar](previews[0])
	if len(bars) != 1 {
		t.Fatalf("the preview shows %v instead of one bar", treetest.Labels(previews[0]))
	}
	return bars[0]
}

func previewedButton(t *testing.T, bar *widget.Bar, module string) *barview.Button {
	t.Helper()
	for _, w := range treetest.WithClass(bar, module) {
		if b := treetest.All[*barview.Button](w); len(b) == 1 {
			return b[0]
		}
	}
	t.Fatalf("the previewed bar has no %s button", module)
	return nil
}

func set(t *testing.T, k *kit, p *settingsPage, path string, v any) {
	t.Helper()
	if err := k.store.set(path, v); err != nil {
		t.Fatalf("set %s: %v", path, err)
	}
	p.refresh()
}

func TestTheBarPagePreviewsTheBarItsDraftDraws(t *testing.T) {
	k := testKit(t, "")
	p := buildPage(k, barPage(nil))
	bar := previewedBar(t, p)
	if bar.Edge() != widget.BarTop {
		t.Errorf("the default bar previews on the %v edge, want the top", bar.Edge())
	}
	if b := previewedButton(t, bar, "battery"); b.HasClass("icon-only") {
		t.Fatalf("the default battery previews icon-only: %v", b.Classes())
	}
	set(t, k, p, "bar.location", "left")
	set(t, k, p, "modules.battery.label-show", false)
	set(t, k, p, "bar.button-variant", "block-prefix")
	bar = previewedBar(t, p)
	if bar.Edge() != widget.BarLeft {
		t.Errorf("bar.location left previews on the %v edge", bar.Edge())
	}
	b := previewedButton(t, bar, "battery")
	if !b.HasClass("icon-only") || !b.HasClass("block-prefix") {
		t.Errorf("label-show off under block-prefix previews the battery as %v", b.Classes())
	}
}

func TestABarItemPagePreviewsItsButton(t *testing.T) {
	k := testKit(t, "")
	p := buildPage(k, batteryPage(nil))
	bar := previewedBar(t, p)
	if got := treetest.All[*barview.Button](bar); len(got) != 1 {
		t.Fatalf("the battery page previews %d buttons, want its own alone", len(got))
	}
	if b := previewedButton(t, bar, "battery"); b.HasClass("label-only") {
		t.Fatalf("the default battery previews without its icon: %v", b.Classes())
	}
	set(t, k, p, "modules.battery.icon-show", false)
	if b := previewedButton(t, previewedBar(t, p), "battery"); !b.HasClass("label-only") {
		t.Errorf("icon-show off previews the battery as %v", b.Classes())
	}
}

func TestABarItemPreviewLabelsItsButtonInItsFormat(t *testing.T) {
	k := testKit(t, "")
	p := buildPage(k, batteryPage(nil))
	if got := previewedButton(t, previewedBar(t, p), "battery").Label().Text(); got != "45%" {
		t.Fatalf("the default battery format previews %q, want 45%%", got)
	}
	set(t, k, p, "modules.battery.format", "Bat {{ percent }}")
	if got := previewedButton(t, previewedBar(t, p), "battery").Label().Text(); got != "Bat 45" {
		t.Errorf("the format Bat {{ percent }} previews %q, want Bat 45", got)
	}
	set(t, k, p, "modules.battery.format", "Bat {{ percent")
	if got := previewedButton(t, previewedBar(t, p), "battery").Label().Text(); got != "" {
		t.Errorf("a format that does not parse previews %q, want the empty label lys-shell shows", got)
	}
}

func TestALayoutWithoutABarEverywherePreviewsNoBar(t *testing.T) {
	k := testKit(t, "bar:\n  layout:\n    - monitor: DP-1\n      show: true\n      center: [clock]\n")
	p := buildPage(k, barPage(nil))
	previews := treetest.All[*livePreview](p)
	if len(previews) != 1 {
		t.Fatalf("the page holds %d previews, want one", len(previews))
	}
	if bars := treetest.All[*widget.Bar](previews[0]); len(bars) != 0 {
		t.Error("a layout for DP-1 alone previews a bar on every monitor")
	}
	if want := locale.T().Get("settings-bar-preview-none"); !slices.Contains(treetest.Labels(previews[0]), want) {
		t.Errorf("the preview says %v, want %q", treetest.Labels(previews[0]), want)
	}
	set(t, k, p, "bar.layout", []any{map[string]any{"monitor": "*", "show": true, "center": []any{"clock"}}})
	previewedButton(t, previewedBar(t, p), "clock")
}

func TestAPreviewWhoseFontDoesNotLoadSaysWhy(t *testing.T) {
	k := testKit(t, "")
	err := errors.New("no font Sans in the tests")
	k.barFont = func(*config.Config) (render.Font, error) { return nil, err }
	p := buildPage(k, barPage(nil))
	previews := treetest.All[*livePreview](p)
	if len(previews) != 1 || len(treetest.All[*widget.Bar](previews[0])) != 0 {
		t.Fatal("a bar font that does not load still previews a bar")
	}
	if want := locale.T().Get("settings-preview-failed", i18n.Str("reason", err.Error())); !slices.Contains(treetest.Labels(previews[0]), want) {
		t.Errorf("the preview says %v, want %q", treetest.Labels(previews[0]), want)
	}
}

func TestAPreviewWritesNoConfig(t *testing.T) {
	k := testKit(t, "")
	for _, spec := range []pageSpec{barPage(nil), batteryPage(nil), workspacesPage(nil), osdPage(nil), notificationsPage(nil)} {
		p := buildPage(k, spec)
		p.refresh()
		p.close()
	}
	if got := runtimeFile(t, k); got != "" {
		t.Errorf("building and refreshing the previews wrote %q", got)
	}
}

func previewedWorkspaceLabels(t *testing.T, p *settingsPage) []string {
	t.Helper()
	strips := treetest.All[*widget.SegmentIndicator](previewedBar(t, p))
	if len(strips) != 1 || !strips[0].HasClass("workspaces") {
		t.Fatalf("the preview shows %d segment rows, want one workspace strip", len(strips))
	}
	var labels []string
	for _, w := range treetest.WithClass(strips[0], "workspace-label") {
		labels = append(labels, w.(*widget.Label).Text())
	}
	return labels
}

func TestTheWorkspacesPagePreviewsTheStripItsDraftDraws(t *testing.T) {
	k := testKit(t, "")
	p := buildPage(k, workspacesPage(nil))
	before := previewedWorkspaceLabels(t, p)
	if len(before) == 0 {
		t.Fatal("the default strip previews no labels")
	}
	set(t, k, p, "modules.workspaces.workspace-map", map[string]any{"1": map[string]any{"label": "one"}})
	if got := previewedWorkspaceLabels(t, p); len(got) != len(before) || got[0] != "one" {
		t.Errorf("a workspace-map label for 1 previews %v, want one first", got)
	}
	set(t, k, p, "modules.workspaces.display-mode", "none")
	if got := previewedWorkspaceLabels(t, p); len(got) != 0 {
		t.Errorf("display-mode none previews labels %v", got)
	}
}

func TestARefusedWorkspacesValueLeavesThePreviewAsItWas(t *testing.T) {
	k := testKit(t, "")
	p := buildPage(k, workspacesPage(nil))
	before := previewedWorkspaceLabels(t, p)
	if err := k.store.set("modules.workspaces.display-mode", "sideways"); err == nil {
		t.Fatal("display-mode sideways was accepted")
	}
	p.refresh()
	if got := previewedWorkspaceLabels(t, p); !slices.Equal(got, before) {
		t.Errorf("a refused display-mode previews %v, want %v as before", got, before)
	}
}

func TestTheBarPagePreviewsTheWorkspaceStrip(t *testing.T) {
	k := testKit(t, "bar:\n  layout:\n    - monitor: \"*\"\n      show: true\n      left: [workspaces]\n")
	p := buildPage(k, barPage(nil))
	if got := previewedWorkspaceLabels(t, p); len(got) == 0 {
		t.Error("the bar previews a workspace strip without labels")
	}
	for _, w := range treetest.WithClass(previewedBar(t, p), "workspaces") {
		if _, ok := w.(*widget.SegmentIndicator); !ok || len(treetest.All[*barview.Button](w)) != 0 {
			t.Errorf("the workspaces module previews as %T, want the strip alone", w)
		}
	}
}
