package settings

import (
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func pickerOf(t *testing.T, k *kit, spec rowSpec) *presetPicker {
	t.Helper()
	p, ok := fieldControl(k, k.store, spec).(*presetPicker)
	if !ok {
		t.Fatalf("%s is not preset chips", spec.path)
	}
	return p
}

func lockGraceRow() rowSpec {
	return field("login.lock.grace-period-ms", durationChips(inMilliseconds, zeroAs(immediately), after(5*time.Second), after(time.Minute)))
}

func reminderRow() rowSpec {
	return field("portal.screen-cast.reminder-days", durationChips(inDays, after(7*day), after(30*day), after(90*day), zeroAs(never)))
}

func cyclingRow() rowSpec {
	return field("wallpaper.cycling-interval-mins", durationChips(inMinutes, after(5*time.Minute), after(15*time.Minute), after(time.Hour)))
}

func chipLabels(p *presetPicker) []string {
	labels := make([]string, len(p.presets))
	for i, pr := range p.presets {
		labels[i] = pr.label()
	}
	return labels
}

func TestADurationChipWritesItsMilliseconds(t *testing.T) {
	k := testKit(t, "")
	p := pickerOf(t, k, lockGraceRow())
	if p.active() != 0 || p.typed.Visible() {
		t.Fatalf("the default 0 shows chip %d, custom shown %v", p.active(), p.typed.Visible())
	}
	p.chips[2].SetActive(true)
	if v := k.store.value("login.lock.grace-period-ms"); v != int64(60000) {
		t.Errorf("choosing 1 minute stored %v", v)
	}
	p.chips[0].SetActive(true)
	if v := k.store.value("login.lock.grace-period-ms"); v != int64(0) {
		t.Errorf("choosing Immediately stored %v", v)
	}
}

func TestADurationOffTheChipsShowsAsCustomSeconds(t *testing.T) {
	k := testKit(t, "login:\n  lock:\n    grace-period-ms: 1500\n")
	p := pickerOf(t, k, lockGraceRow())
	if p.active() != 3 || !p.typed.Visible() {
		t.Fatalf("1500 ms shows chip %d, custom shown %v", p.active(), p.typed.Visible())
	}
	if got := p.custom.Value(); got != 1.5 {
		t.Errorf("1500 ms reads %v seconds", got)
	}
	p.custom.OnValueChanged(2.5)
	if v := k.store.value("login.lock.grace-period-ms"); v != int64(2500) {
		t.Errorf("typing 2.5 seconds stored %v", v)
	}
}

func TestPickingCustomKeepsTheValueUntilOneIsTyped(t *testing.T) {
	k := testKit(t, "login:\n  lock:\n    grace-period-ms: 5000\n")
	p := pickerOf(t, k, lockGraceRow())
	if p.typed.Visible() {
		t.Fatal("a preset value shows the custom field")
	}
	p.chips[3].SetActive(true)
	if !p.typed.Visible() {
		t.Error("choosing Custom hides the field to type in")
	}
	if v := k.store.value("login.lock.grace-period-ms"); v != int64(5000) {
		t.Errorf("choosing Custom changed the value to %v", v)
	}
	p.refresh()
	if p.active() != 3 || !p.typed.Visible() {
		t.Errorf("a refresh while choosing Custom shows chip %d, custom shown %v", p.active(), p.typed.Visible())
	}
	p.chips[1].SetActive(true)
	p.refresh()
	if p.active() != 1 || p.typed.Visible() {
		t.Errorf("picking a preset after Custom shows chip %d, custom shown %v", p.active(), p.typed.Visible())
	}
}

func TestDayChipsReadAsWordsAndStoreDays(t *testing.T) {
	k := testKit(t, "")
	p := pickerOf(t, k, reminderRow())
	tr := locale.T()
	want := []string{
		tr.Get("settings-duration-days", i18n.Int("n", 7)), tr.Get("settings-duration-days", i18n.Int("n", 30)),
		tr.Get("settings-duration-days", i18n.Int("n", 90)), tr.Get("settings-duration-never"),
	}
	got := chipLabels(p)
	if len(got) != len(want) || len(p.chips) != len(want)+1 {
		t.Fatalf("chips %q and %d buttons, want %q and Custom", got, len(p.chips), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("chip %d reads %q, want %q", i, got[i], want[i])
		}
	}
	if p.active() != 1 || p.typed.Visible() {
		t.Fatalf("the default 30 days shows chip %d, custom shown %v", p.active(), p.typed.Visible())
	}
	p.chips[3].SetActive(true)
	if v := k.store.value("portal.screen-cast.reminder-days"); v != int64(0) {
		t.Errorf("Never stored %v", v)
	}
	p.chips[0].SetActive(true)
	if v := k.store.value("portal.screen-cast.reminder-days"); v != int64(7) {
		t.Errorf("7 days stored %v", v)
	}
}

func TestADayCountOffTheChipsShowsAsCustomDays(t *testing.T) {
	k := testKit(t, "portal:\n  screen-cast:\n    reminder-days: 12\n")
	p := pickerOf(t, k, reminderRow())
	if p.active() != 4 || !p.typed.Visible() || p.custom.Value() != 12 {
		t.Fatalf("12 days shows chip %d, number shown %v at %v", p.active(), p.typed.Visible(), p.custom.Value())
	}
	p.chips[2].SetActive(true)
	if p.typed.Visible() || k.store.value("portal.screen-cast.reminder-days") != int64(90) {
		t.Fatalf("90 days: number shown %v, stored %v", p.typed.Visible(), k.store.value("portal.screen-cast.reminder-days"))
	}
	p.custom.OnValueChanged(45)
	p.refresh()
	if p.active() != 4 || k.store.value("portal.screen-cast.reminder-days") != int64(45) {
		t.Fatalf("typing 45 shows chip %d, stored %v", p.active(), k.store.value("portal.screen-cast.reminder-days"))
	}
}

func TestMinuteChipsStoreMinutesAndCustomTakesAnyOther(t *testing.T) {
	k := testKit(t, "wallpaper:\n  cycling-interval-mins: 15\n")
	p := pickerOf(t, k, cyclingRow())
	if p.active() != 1 || p.typed.Visible() {
		t.Fatalf("15 minutes shows chip %d, custom shown %v", p.active(), p.typed.Visible())
	}
	p.chips[2].SetActive(true)
	if got := k.store.value("wallpaper.cycling-interval-mins"); got != int64(60) {
		t.Errorf("choosing an hour stored %v", got)
	}
	p.chips[3].SetActive(true)
	p.custom.OnValueChanged(42)
	if got := k.store.value("wallpaper.cycling-interval-mins"); got != int64(42) {
		t.Errorf("typing 42 stored %v", got)
	}
	k = testKit(t, "wallpaper:\n  cycling-interval-mins: 7\n")
	p = pickerOf(t, k, cyclingRow())
	if p.active() != 3 || !p.typed.Visible() || p.custom.Value() != 7 {
		t.Fatalf("7 minutes shows chip %d, number shown %v at %v", p.active(), p.typed.Visible(), p.custom.Value())
	}
}

func TestADurationChipReadsAsWords(t *testing.T) {
	tr := locale.T()
	for _, tc := range []struct {
		choice durationChoice
		want   string
	}{
		{zeroAs(never), tr.Get("settings-duration-never")},
		{zeroAs(immediately), tr.Get("settings-duration-immediately")},
		{after(time.Second), "1 second"},
		{after(1500 * time.Millisecond), "\u20681.5\u2069 seconds"},
		{after(5 * time.Minute), "\u20685\u2069 minutes"},
		{after(time.Hour), tr.Get("settings-duration-hours", i18n.Int("n", 1))},
		{after(90 * time.Minute), tr.Get("settings-duration-minutes", i18n.Int("n", 90))},
		{after(day), tr.Get("settings-duration-days", i18n.Int("n", 1))},
		{after(36 * time.Hour), tr.Get("settings-duration-hours", i18n.Int("n", 36))},
	} {
		if got := tc.choice.label(); got != tc.want {
			t.Errorf("%v reads %q, want %q", tc.choice, got, tc.want)
		}
	}
}

func TestAChipOffItsFieldsUnitIsRefused(t *testing.T) {
	for name, build := range map[string]func(){
		"a second in minutes": func() { durationChips(inMinutes, after(time.Second)) },
		"an hour in days":     func() { durationChips(inDays, after(time.Hour)) },
		"a zero duration":     func() { after(0) },
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Errorf("%s was accepted", name)
				}
			}()
			build()
		})
	}
}

func TestTheTimesThatWereTypedArePickedFromChips(t *testing.T) {
	cfg := config.Defaults()
	for page, paths := range map[string][]string{
		passwordsID:     {repromptPath},
		"privacy":       {"portal.screen-cast.reminder-days"},
		"notifications": {"modules.notifications.popup-duration"},
		"osd":           {"osd.duration"},
		"lock":          {"login.lock.grace-period-ms", "login.lock.blank-timeout-ms"},
	} {
		p, ok := pageByID(layout(cfg, bothParts), page)
		if !ok {
			t.Fatalf("no page %s", page)
		}
		for _, path := range paths {
			spec, found := rowSpec{}, false
			for _, s := range p.sections {
				for _, r := range s.rows {
					if r.path == path {
						spec, found = r, true
					}
				}
			}
			if !found {
				t.Errorf("%s has no row %s", page, path)
				continue
			}
			k := testKit(t, "")
			if _, ok := fieldControl(k, k.store, spec).(*presetPicker); !ok {
				t.Errorf("%s on %s is not preset chips", path, page)
			}
		}
	}
}
