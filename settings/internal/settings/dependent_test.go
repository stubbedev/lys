package settings

import (
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
