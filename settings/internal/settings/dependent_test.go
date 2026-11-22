package settings

import (
	"slices"
	"strings"
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
	c := r.editor(testKit(t, ""), s, config.FieldMeta{Min: 0, Max: 1 << 31}).(*softRange)
	if got := c.number.Value(); got != 400 {
		t.Fatalf("400 px reads %v in the number", got)
	}
	c.OnCommit(12)
	if stored != int64(12) {
		t.Errorf("the slider stored %v", stored)
	}
}

func rowOf(t *testing.T, p *settingsPage, path string) *fieldRow {
	t.Helper()
	i := slices.IndexFunc(p.rows, func(r *fieldRow) bool { return r.path == path && !r.action })
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
	i := slices.IndexFunc(p.targets, func(r *fieldRow) bool { return r != nil && r.path == "bar.border-width" })
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

func TestADurationReadsInWholeMilliseconds(t *testing.T) {
	for ms, want := range map[float64]string{0: "0 ms", 1: "1 ms", 150: "150 ms", 1499.6: "1500 ms"} {
		if got := strings.NewReplacer("\u2068", "", "\u2069", "").Replace(unitMilliseconds.format(0)(ms)); got != want {
			t.Errorf("%v ms reads %q, want %q", ms, got, want)
		}
	}
}

func TestDurationsAreSlidersThatKeepALongerValue(t *testing.T) {
	k := testKit(t, "animations:\n  duration: 5000\n  ui-duration: 150\ndock:\n  enabled: true\n  auto-hide: true\n")
	motion := layoutPage(t, k, "motion")
	long, ok := rowOf(t, motion, "animations.duration").ctl.(*softRange)
	if !ok {
		t.Fatalf("the animation duration is a %T", rowOf(t, motion, "animations.duration").ctl)
	}
	if got, end := long.number.Value(), long.Knob.Value(); got != 5000 || end != animationSliderMS {
		t.Errorf("5 s reads %v in the number and %v on the slider", got, end)
	}
	short := rowOf(t, motion, "animations.ui-duration").ctl.(*softRange)
	short.OnCommit(300)
	if got := k.store.value("animations.ui-duration"); got != int64(300) {
		t.Errorf("sliding the UI duration to 300 ms stored %v", got)
	}
	dock := layoutPage(t, k, "dock")
	if _, ok := rowOf(t, dock, "dock.auto-hide-delay-ms").ctl.(*softRange); !ok {
		t.Errorf("the dock's hide delay is a %T, not a slider", rowOf(t, dock, "dock.auto-hide-delay-ms").ctl)
	}
}

func speedCards(t *testing.T, configYAML string) (*kit, *choiceCards) {
	t.Helper()
	k := testKit(t, configYAML)
	return k, fieldControl(k, k.store, speedRow()).(*choiceCards)
}

func TestASpeedSetsTheThreeDurationsTogether(t *testing.T) {
	k, c := speedCards(t, "")
	if c.current != "normal" {
		t.Fatalf("the default durations read as %q, want normal", c.current)
	}
	c.choose("fast")
	want := map[string]int64{"animations.duration": 100, "animations.ui-duration": 125, "animations.interaction-duration": 75}
	for path, ms := range want {
		if got := k.store.value(path); got != ms {
			t.Errorf("fast stored %s = %v, want %d", path, got, ms)
		}
	}
	if c.current != "fast" {
		t.Errorf("after choosing fast the cards show %q", c.current)
	}
	c.choose("instant")
	if got := k.store.value("animations.ui-duration"); got != int64(0) {
		t.Errorf("instant stored a UI duration of %v", got)
	}
}

func TestDurationsNoSpeedGivesShowNoSpeedChosen(t *testing.T) {
	_, c := speedCards(t, "animations:\n  duration: 200\n  ui-duration: 900\n")
	if c.current != "" {
		t.Errorf("hand-set durations read as the speed %q", c.current)
	}
	if err := speedSlot(c.cards[0].k).set("warp"); err == nil {
		t.Error("an unknown speed was accepted")
	}
}

func TestTheMotionRowsRestWhileAnimationsAreOff(t *testing.T) {
	k := testKit(t, "animations:\n  enabled: false\n")
	p := layoutPage(t, k, "motion")
	speed := p.rows[slices.IndexFunc(p.rows, func(r *fieldRow) bool { return r.action && r.path == "animations.duration" })]
	transition, exact := rowOf(t, p, "animations.transition"), rowOf(t, p, "animations.duration")
	if widget.IsEnabled(speed) || widget.IsEnabled(transition) || widget.IsEnabled(exact) {
		t.Fatalf("with animations off the speed is enabled %v, the transition %v, the duration %v", widget.IsEnabled(speed), widget.IsEnabled(transition), widget.IsEnabled(exact))
	}
	if !widget.IsEnabled(rowOf(t, p, "animations.reduced-motion")) {
		t.Error("reduce motion rests with the animations it is about")
	}
	setRow(t, p, "animations.enabled", true)
	if !widget.IsEnabled(speed) || !widget.IsEnabled(transition) {
		t.Error("turning animations on left their rows disabled")
	}
}
