package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/widget"
)

func TestARowFollowsTheSwitchThatEnablesIt(t *testing.T) {
	k := testKit(t, "bar:\n  dropdown-shadow: false\n")
	spec := pageSpec{id: "deps", header: "settings-page-overview", sections: []sectionSpec{{title: "settings-section-general", rows: []rowSpec{
		field("bar.dropdown-shadow"), field("bar.dropdown-opacity", enabledBy("bar.dropdown-shadow")),
	}}}}
	p := buildPage(k, spec)
	dependent := p.rows[1]
	if widget.IsEnabled(dependent) {
		t.Fatal("a row is enabled while its switch is off")
	}
	if err := p.rows[0].be.slot("bar.dropdown-shadow").set(true); err != nil {
		t.Fatal(err)
	}
	if !widget.IsEnabled(dependent) || !widget.IsEnabled(p.rows[0]) {
		t.Errorf("with the switch on the row is enabled %v, the switch %v", widget.IsEnabled(dependent), widget.IsEnabled(p.rows[0]))
	}
}

func fractionOver(t *testing.T, stored *any) *slider {
	t.Helper()
	var r rowSpec
	fraction(&r)
	s := slot{get: func() any { return *stored }, set: func(v any) error { *stored = v; return nil }, unset: func() {}}
	return r.editor(testKit(t, ""), s, config.FieldMeta{}).(*slider)
}

func TestAFractionShowsAndWritesAWholePercent(t *testing.T) {
	var stored any = 0.35
	c := fractionOver(t, &stored)
	if got := c.Value(); got != 35 {
		t.Fatalf("0.35 shows as %v", got)
	}
	c.OnCommit(80)
	if stored != 0.8 {
		t.Errorf("80%% stored %v, want 0.8", stored)
	}
	stored = int64(1)
	c = fractionOver(t, &stored)
	if got := c.Value(); got != 100 {
		t.Errorf("a whole 1 shows as %v", got)
	}
}

func TestSoftPixelsKeepAValuePastTheSlider(t *testing.T) {
	var r rowSpec
	softPixels(0, 48)(&r)
	var stored any = int64(400)
	s := slot{get: func() any { return stored }, set: func(v any) error { stored = v; return nil }, unset: func() {}}
	c := r.editor(testKit(t, ""), s, config.FieldMeta{Min: 0, Max: 1 << 31}).(*pixelControl)
	if got := c.number.Value(); got != 400 {
		t.Fatalf("400 px reads %v in the number", got)
	}
	c.slider.OnCommit(12)
	if stored != int64(12) {
		t.Errorf("the slider stored %v", stored)
	}
}

func rowOf(t *testing.T, p *settingsPage, path string) *settingRow {
	t.Helper()
	i := slices.IndexFunc(p.rows, func(r *settingRow) bool { return r.path == path })
	if i < 0 {
		t.Fatalf("no %s row", path)
	}
	return p.rows[i]
}

func setRow(t *testing.T, p *settingsPage, path string, v any) {
	t.Helper()
	r := rowOf(t, p, path)
	if err := r.be.slot(path).set(v); err != nil {
		t.Fatal(err)
	}
}

func layoutPage(t *testing.T, k *kit, id string) *settingsPage {
	t.Helper()
	spec, ok := pageByID(layout(k.store.svc.Config(), bothParts), id)
	if !ok {
		t.Fatalf("no %s page", id)
	}
	return buildPage(k, spec)
}

func TestARowWithNoEffectHidesUntilItHasOne(t *testing.T) {
	k := testKit(t, "bar:\n  button-group-border-location: none\n  button-variant: basic\n")
	p := layoutPage(t, k, "bar")
	width, color := rowOf(t, p, "bar.button-group-border-width"), rowOf(t, p, "bar.button-group-border-color")
	padding := rowOf(t, p, "bar.button-icon-padding")
	if width.Visible() || color.Visible() {
		t.Fatalf("without a group border its width shows %v and its color %v", width.Visible(), color.Visible())
	}
	if padding.Visible() {
		t.Fatal("the icon padding shows on a basic button, which has no icon container")
	}
	setRow(t, p, "bar.button-group-border-location", "all")
	if !width.Visible() || !color.Visible() {
		t.Errorf("with a border its width shows %v and its color %v", width.Visible(), color.Visible())
	}
	setRow(t, p, "bar.button-variant", "icon-square")
	if !padding.Visible() {
		t.Error("the icon padding stays hidden on an icon-square button")
	}
}

func TestRevealingAHiddenRowLandsOnTheRowThatHidesIt(t *testing.T) {
	k := testKit(t, "bar:\n  border-location: none\n")
	p := layoutPage(t, k, "bar")
	i := slices.IndexFunc(p.targets, func(r *settingRow) bool { return r != nil && r.path == "bar.border-width" })
	if got := p.reveal(i, nil); got == nil || got.path != "bar.border-location" {
		t.Fatalf("revealing the hidden border width landed on %v", got)
	}
	setRow(t, p, "bar.border-location", "top")
	if got := p.reveal(i, nil); got == nil || got.path != "bar.border-width" {
		t.Errorf("revealing the shown border width landed on %v", got)
	}
}

func TestARowFollowingAFieldWithoutARowIsAPageBug(t *testing.T) {
	k := testKit(t, "")
	spec := pageSpec{id: "deps", header: "settings-page-overview", sections: []sectionSpec{{title: "settings-section-general", rows: []rowSpec{
		field("bar.border-width", shownWhen("bar.border-location", valueNot("none"))),
	}}}}
	defer func() {
		if recover() == nil {
			t.Error("a row following a field the page has no row for built")
		}
	}()
	buildPage(k, spec)
}

func TestTheDockRowsFollowTheDockSwitch(t *testing.T) {
	k := testKit(t, "dock:\n  enabled: false\n  auto-hide: false\n  hover-zoom: true\n")
	p := layoutPage(t, k, "dock")
	icon, delay, zoom := rowOf(t, p, "dock.icon-size"), rowOf(t, p, "dock.auto-hide-delay-ms"), rowOf(t, p, "dock.hover-zoom-size")
	if widget.IsEnabled(icon) {
		t.Error("the dock's icon size is editable while the dock is off")
	}
	if delay.Visible() || !zoom.Visible() {
		t.Errorf("without auto-hide the delay shows %v; with hover zoom its size shows %v", delay.Visible(), zoom.Visible())
	}
	setRow(t, p, "dock.enabled", true)
	setRow(t, p, "dock.auto-hide", true)
	if !widget.IsEnabled(icon) || !delay.Visible() {
		t.Errorf("with the dock on the icon size is enabled %v; with auto-hide the delay shows %v", widget.IsEnabled(icon), delay.Visible())
	}
	setRow(t, p, "dock.hover-zoom", false)
	if zoom.Visible() {
		t.Error("the zoom size shows with hover zoom off")
	}
}
