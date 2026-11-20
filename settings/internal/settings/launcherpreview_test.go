package settings

import (
	"math"
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/launcherview"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func previewedLauncher(t *testing.T, p *settingsPage) (*widget.Box, render.Rect) {
	t.Helper()
	return previewedSurface(t, p, "launcher-panel", launcherPreviewScreenH)
}

// launcherRowTexts are the sample rows the previewed launcher lists.
func launcherRowTexts(t *testing.T, panel *widget.Box) []string {
	t.Helper()
	var texts []string
	for _, row := range treetest.All[*launcherview.RowView](panel) {
		texts = append(texts, treetest.Labels(row)...)
	}
	return texts
}

func sampleName(key string) string { return locale.T().Get(key) }

func previewQuery(t *testing.T, p *settingsPage) *widget.Entry {
	t.Helper()
	entries := treetest.WithClass(onlyPreview(t, p), "launcher-search-entry")
	if len(entries) != 1 {
		t.Fatalf("the preview holds %d query fields, want one", len(entries))
	}
	return entries[0].(*widget.Entry)
}

func TestTheLauncherPagePreviewsTheLauncherItsDraftDraws(t *testing.T) {
	k := testKit(t, "")
	p := buildPage(k, launcherPage(nil))
	panel, at := previewedLauncher(t, p)
	rows := launcherRowTexts(t, panel)
	if len(rows) == 0 || rows[0] != sampleName(launcherSamples[0].key) {
		t.Fatalf("the launcher previews rows %v, want the sample apps", rows)
	}
	if at.W != 720 || at.X != (previewScreenW-720)/2 || at.Y != launcherview.Place(render.Rect{H: launcherPreviewScreenH}, 0, 0, "center", 0, 0).Y {
		t.Errorf("the default launcher previews at %+v, want 720 wide, centred above the middle", at)
	}
	if tabs := treetest.WithClass(panel, "launcher-tabs"); len(tabs) != 1 || widget.IsVisible(tabs[0]) {
		t.Error("the default launcher previews its mode tabs")
	}

	set(t, k, p, "launcher.location", "north-west")
	set(t, k, p, "launcher.width", "400px")
	set(t, k, p, "launcher.sidebar-mode", true)
	panel, at = previewedLauncher(t, p)
	if at.X != 0 || at.Y != 0 || at.W != 400 {
		t.Errorf("a 400px launcher in the north-west previews at %+v", at)
	}
	if tabs := treetest.WithClass(panel, "launcher-tabs"); len(tabs) != 1 || !widget.IsVisible(tabs[0]) {
		t.Error("sidebar mode previews no mode tabs")
	}

	_, tall := previewedLauncher(t, p)
	set(t, k, p, "launcher.lines", int64(3))
	panel, short := previewedLauncher(t, p)
	if got := launcherRowTexts(t, panel); len(got) != 3 || short.H >= tall.H {
		t.Errorf("three lines preview rows %v at %dpx, against %dpx for ten", got, short.H, tall.H)
	}
}

func TestTheLauncherPreviewsQueryFiltersTheSampleApps(t *testing.T) {
	k := testKit(t, "")
	p := buildPage(k, launcherPage(nil))
	terminal := sampleName("settings-launcher-sample-terminal")
	previewQuery(t, p).SetText(terminal[1:4])
	panel, _ := previewedLauncher(t, p)
	if got := launcherRowTexts(t, panel); !slices.Equal(got, []string{terminal}) {
		t.Errorf("the query %q previews %v, want %s alone", terminal[1:4], got, terminal)
	}
	set(t, k, p, "launcher.lines", int64(4))
	if got := previewQuery(t, p).Text(); got != terminal[1:4] {
		t.Errorf("a change to the draft reset the query to %q", got)
	}
	panel, _ = previewedLauncher(t, p)
	if got := launcherRowTexts(t, panel); !slices.Equal(got, []string{terminal}) {
		t.Errorf("after a change to the draft the query previews %v", got)
	}

	written := runtimeFile(t, k)
	previewQuery(t, p).SetText("zzzz")
	panel, _ = previewedLauncher(t, p)
	if got := launcherRowTexts(t, panel); len(got) != 0 {
		t.Errorf("a query nothing matches previews %v", got)
	}
	want := locale.T().Get("settings-launcher-preview-no-match", i18n.Str("query", "zzzz"))
	if said := treetest.LabelReading(t, onlyPreview(t, p), want); !widget.IsVisible(said) {
		t.Errorf("the preview does not say %q", want)
	}
	previewQuery(t, p).SetText("")
	if said := treetest.LabelReading(t, onlyPreview(t, p), want); widget.IsVisible(said) {
		t.Error("an empty query still says nothing matches")
	}
	if got := runtimeFile(t, k); got != written {
		t.Errorf("typing a test query wrote %q over %q", got, written)
	}
}

func TestTheLauncherPreviewsQueryMatchesAsTheLauncherIsSetToMatch(t *testing.T) {
	k := testKit(t, "")
	p := buildPage(k, launcherPage(nil))
	terminal := sampleName("settings-launcher-sample-terminal")
	editor := sampleName("settings-launcher-sample-editor")
	rowsFor := func(query string) []string {
		t.Helper()
		previewQuery(t, p).SetText(query)
		panel, _ := previewedLauncher(t, p)
		return launcherRowTexts(t, panel)
	}
	if got := rowsFor("trml"); len(got) != 0 {
		t.Errorf("normal matching previews %v for trml, want nothing", got)
	}
	set(t, k, p, "launcher.matching", "fuzzy")
	if got := rowsFor("trml"); !slices.Equal(got, []string{terminal}) {
		t.Errorf("fuzzy matching previews %v for trml, want %s alone", got, terminal)
	}
	set(t, k, p, "launcher.matching", "prefix")
	if got := rowsFor("editor"); len(got) != 0 {
		t.Errorf("prefix matching previews %v for editor, want nothing", got)
	}
	if got := rowsFor("text"); !slices.Equal(got, []string{editor}) {
		t.Errorf("prefix matching previews %v for text, want %s alone", got, editor)
	}
	set(t, k, p, "launcher.case", "sensitive")
	if got := rowsFor("text"); len(got) != 0 {
		t.Errorf("case-sensitive matching previews %v for text, want nothing", got)
	}
	if got := rowsFor("Text"); !slices.Equal(got, []string{editor}) {
		t.Errorf("case-sensitive matching previews %v for Text, want %s alone", got, editor)
	}
}

func TestTheLauncherPreviewDrawsAStylePresetItIsAskedTo(t *testing.T) {
	k := testKit(t, "launcher:\n  styles:\n    huge: \".launcher-row-label { font-size: 40px; }\"\n")
	p := buildPage(k, launcherPage(nil))
	rowPx := func() float64 {
		panel, _ := previewedLauncher(t, p)
		return treetest.RenderedTextPx(treetest.LabelReading(t, panel, sampleName(launcherSamples[0].key)), k.face)
	}
	plain := rowPx()
	if plain > 30 {
		t.Fatalf("the preview draws the preset before it is chosen, rows at %.1fpx", plain)
	}
	chooser := treetest.First[*widget.Dropdown](t, onlyPreview(t, p))
	chooser.SetSelected(1)
	chooser.OnSelect(1)
	if got := rowPx(); got < 30 {
		t.Errorf("the huge preset previews rows at %.1fpx, against %.1fpx without it", got, plain)
	}

	set(t, k, p, "launcher.styles", map[string]any{})
	if got := rowPx(); math.Abs(got-plain) > 0.5 {
		t.Errorf("a preset the draft dropped still previews rows at %.1fpx, want %.1f", got, plain)
	}
	if dropdowns := treetest.All[*widget.Dropdown](onlyPreview(t, p)); len(dropdowns) != 0 {
		t.Error("a draft without presets still offers a style to preview with")
	}
}

func TestARefusedLauncherValueLeavesThePreviewAsItWas(t *testing.T) {
	k := testKit(t, "")
	p := buildPage(k, launcherPage(nil))
	_, before := previewedLauncher(t, p)
	if err := k.store.set("launcher.location", "upside-down"); err == nil {
		t.Fatal("launcher.location upside-down was accepted")
	}
	p.refresh()
	if _, got := previewedLauncher(t, p); got != before {
		t.Errorf("a refused location previews the launcher at %+v, want %+v as before", got, before)
	}
}

func TestTheLauncherSlidersMoveThePreview(t *testing.T) {
	k := testKit(t, "")
	p := buildPage(k, launcherPage(nil))
	lines, ok := rowOf(t, p, "launcher.lines").ctl.(*softRange)
	if !ok {
		t.Fatalf("launcher.lines is a %T, want a slider", rowOf(t, p, "launcher.lines").ctl)
	}
	lines.slider.OnCommit(2)
	p.refresh()
	panel, _ := previewedLauncher(t, p)
	if got := launcherRowTexts(t, panel); len(got) != 2 || k.store.value("launcher.lines") != int64(2) {
		t.Errorf("the lines slider at 2 stored %v and previews %v", k.store.value("launcher.lines"), got)
	}

	width, ok := rowOf(t, p, "launcher.width").ctl.(*sizeEditor)
	if !ok || width.slides == nil {
		t.Fatalf("launcher.width is a %T without a slider", rowOf(t, p, "launcher.width").ctl)
	}
	if !widget.IsVisible(width.slides.scale) || widget.IsVisible(width.slides.px) {
		t.Error("a width given as a scale shows the pixel slider")
	}
	width.slides.scale.OnCommit(0.75)
	p.refresh()
	if _, at := previewedLauncher(t, p); at.W != 540 || k.store.value("launcher.width") != 0.75 {
		t.Errorf("the width slider at 0.75 stored %v and previews %dpx, want 540", k.store.value("launcher.width"), at.W)
	}
	width.mode.SetSelected(sizePx)
	width.mode.OnSelect(sizePx)
	p.refresh()
	if widget.IsVisible(width.slides.scale) || !widget.IsVisible(width.slides.px) {
		t.Error("a width in pixels shows the scale slider")
	}
	width.slides.px.OnCommit(600)
	p.refresh()
	if _, at := previewedLauncher(t, p); at.W != 600 || k.store.value("launcher.width") != "600px" {
		t.Errorf("the width slider at 600px stored %v and previews %dpx", k.store.value("launcher.width"), at.W)
	}
}

func TestTheLauncherPreviewShowsIconsTheAppShips(t *testing.T) {
	listed := listedIcons(t)
	for _, s := range launcherSamples {
		if !slices.Contains(listed, s.icon) {
			t.Errorf("the sample %s shows %s, which resources/icons.txt does not list", s.key, s.icon)
		}
	}
	if !slices.Contains(listed, launcherview.SearchIconName) {
		t.Errorf("the launcher's field shows %s, which resources/icons.txt does not list", launcherview.SearchIconName)
	}
}
