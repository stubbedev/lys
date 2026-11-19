package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func reminderChips(t *testing.T, yaml string) (*kit, *durationChips) {
	t.Helper()
	k := testKit(t, yaml)
	c, ok := fieldControl(k, k.store, field("portal.screen-cast.reminder-days", durationChoice(unitDays, 7, 30, 90))).(*durationChips)
	if !ok {
		t.Fatal("the reminder days are not duration chips")
	}
	return k, c
}

func TestDurationChipsReadAsWordsAndStoreTheFieldsUnit(t *testing.T) {
	k, c := reminderChips(t, "")
	tr := locale.T()
	want := []string{
		tr.Get("settings-duration-days", i18n.Int("n", 7)), tr.Get("settings-duration-days", i18n.Int("n", 30)),
		tr.Get("settings-duration-days", i18n.Int("n", 90)), tr.Get("settings-duration-never"), tr.Get("settings-duration-custom"),
	}
	labels := treetest.Labels(c.chips)
	if len(labels) != len(want) {
		t.Fatalf("chips %q, want %q", labels, want)
	}
	for i := range want {
		if labels[i] != want[i] {
			t.Errorf("chip %d reads %q, want %q", i, labels[i], want[i])
		}
	}
	if c.chips.Active() != 1 || c.custom.Visible() {
		t.Fatalf("the default 30 days shows chip %d, custom shown %v", c.chips.Active(), c.custom.Visible())
	}
	c.chips.SetActive(3)
	if v := k.store.value("portal.screen-cast.reminder-days"); v != int64(0) {
		t.Errorf("Never stored %v", v)
	}
	c.chips.SetActive(0)
	if v := k.store.value("portal.screen-cast.reminder-days"); v != int64(7) {
		t.Errorf("7 days stored %v", v)
	}
}

func TestATimeOffThePresetsShowsAsCustomWithItsNumber(t *testing.T) {
	k, c := reminderChips(t, "portal:\n  screen-cast:\n    reminder-days: 12\n")
	if c.chips.Active() != 4 || !c.custom.Visible() || c.custom.Value() != 12 {
		t.Fatalf("12 days shows chip %d, number shown %v at %v", c.chips.Active(), c.custom.Visible(), c.custom.Value())
	}
	c.chips.SetActive(2)
	if c.custom.Visible() || k.store.value("portal.screen-cast.reminder-days") != int64(90) {
		t.Fatalf("90 days: number shown %v, stored %v", c.custom.Visible(), k.store.value("portal.screen-cast.reminder-days"))
	}
	c.chips.SetActive(4)
	if !c.custom.Visible() || k.store.value("portal.screen-cast.reminder-days") != int64(90) {
		t.Fatalf("choosing Custom: number shown %v, stored %v", c.custom.Visible(), k.store.value("portal.screen-cast.reminder-days"))
	}
	c.custom.OnValueChanged(45)
	c.refresh()
	if c.chips.Active() != 4 || k.store.value("portal.screen-cast.reminder-days") != int64(45) {
		t.Fatalf("typing 45 shows chip %d, stored %v", c.chips.Active(), k.store.value("portal.screen-cast.reminder-days"))
	}
}

func TestMinuteChipsReadInHoursPastAnHour(t *testing.T) {
	tr := locale.T()
	if got := unitMinutes.words(60); got != tr.Get("settings-duration-hours", i18n.Int("n", 1)) {
		t.Errorf("60 minutes reads %q", got)
	}
	if got := unitMinutes.words(15); got != tr.Get("settings-duration-minutes", i18n.Int("n", 15)) {
		t.Errorf("15 minutes reads %q", got)
	}
	if got := unitDays.words(0); got != tr.Get("settings-duration-never") {
		t.Errorf("0 days reads %q", got)
	}
}

func TestThePasswordAndScreenShareTimesAreChips(t *testing.T) {
	for page, path := range map[string]string{passwordsID: repromptPath, "privacy": "portal.screen-cast.reminder-days"} {
		k := testKit(t, "")
		p, ok := pageByID(layout(config.Defaults(), bothParts), page)
		if !ok {
			t.Fatalf("no page %s", page)
		}
		var spec rowSpec
		for _, s := range p.sections {
			for _, r := range s.rows {
				if r.path == path {
					spec = r
				}
			}
		}
		if _, ok := fieldControl(k, k.store, spec).(*durationChips); !ok {
			t.Errorf("%s on %s is not duration chips", path, page)
		}
	}
}
