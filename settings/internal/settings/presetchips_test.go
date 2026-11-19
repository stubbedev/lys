package settings

import (
	"slices"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/desktop/shell/treetest"

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
		{after(2 * time.Second), tr.Get("settings-duration-seconds", i18n.Int("n", 2))},
		{zeroAs(off), tr.Get("settings-duration-off")},
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
	for _, c := range []struct{ page, path string }{
		{passwordsID, repromptPath},
		{"privacy", "portal.screen-cast.reminder-days"},
		{"notifications", "modules.notifications.popup-duration"},
		{"osd", "osd.duration"},
		{"lock", "login.lock.grace-period-ms"},
		{"lock", "login.lock.blank-timeout-ms"},
		{"cpu", "modules.cpu.poll-interval-ms"},
		{"netstat", "modules.netstat.poll-interval-ms"},
		{"ram", "modules.ram.poll-interval-ms"},
		{"storage", "modules.storage.poll-interval-ms"},
		{"weather", "modules.weather.refresh-interval-seconds"},
		{"screenshot", "modules.screenshot.thumbnail-ms"},
		{"recorder", "modules.recorder.start-delay-ms"},
		{"recorder", "modules.recorder.replay.seconds"},
	} {
		k := testKit(t, "")
		pickerOf(t, k, pageRow(t, c.page, c.path))
	}
}

func pollRow() rowSpec {
	return field("modules.netstat.poll-interval-ms", durationChips(inMilliseconds, after(time.Second), after(2*time.Second), after(5*time.Second), after(10*time.Second)))
}

func TestAPollIntervalReadsInSecondsAndOffersNoOff(t *testing.T) {
	k := testKit(t, "")
	p := pickerOf(t, k, pollRow())
	tr := locale.T()
	want := []string{
		tr.Get("settings-duration-seconds", i18n.Int("n", 1)), tr.Get("settings-duration-seconds", i18n.Int("n", 2)),
		tr.Get("settings-duration-seconds", i18n.Int("n", 5)), tr.Get("settings-duration-seconds", i18n.Int("n", 10)),
	}
	if got := chipLabels(p); !slices.Equal(got, want) || len(p.chips) != len(want)+1 {
		t.Fatalf("chips %q and %d buttons, want %q and Custom", got, len(p.chips), want)
	}
	if p.active() != 1 {
		t.Fatalf("the default 2 s shows chip %d", p.active())
	}
	p.chips[2].SetActive(true)
	if v := k.store.value("modules.netstat.poll-interval-ms"); v != int64(5000) {
		t.Errorf("5 seconds stored %v", v)
	}
}

func TestACustomPollIntervalStopsAtAQuarterSecond(t *testing.T) {
	k := testKit(t, "modules:\n  netstat:\n    poll-interval-ms: 750\n")
	p := pickerOf(t, k, pollRow())
	if !p.typed.Visible() || p.custom.Value() != 0.75 {
		t.Fatalf("750 ms: number shown %v at %v", p.typed.Visible(), p.custom.Value())
	}
	if lo, _ := p.custom.Range(); lo != shortestPeriod.Seconds() {
		t.Fatalf("the custom number starts at %v, want %v", lo, shortestPeriod.Seconds())
	}
	p.custom.SetValue(0)
	p.custom.OnValueChanged(p.custom.Value())
	if v := k.store.value("modules.netstat.poll-interval-ms"); v != shortestPeriod.Milliseconds() {
		t.Errorf("typing 0 stored %v, want the shortest interval", v)
	}
}

func TestAnOffChipStoresZeroAndLetsCustomReachIt(t *testing.T) {
	k := testKit(t, "")
	p := pickerOf(t, k, field("modules.screenshot.thumbnail-ms", durationChips(inMilliseconds, after(3*time.Second), after(5*time.Second), after(10*time.Second), zeroAs(off))))
	if got := chipLabels(p); got[3] != locale.T().Get("settings-duration-off") {
		t.Fatalf("chips %q, want Off after the presets", got)
	}
	p.chips[3].SetActive(true)
	if v := k.store.value("modules.screenshot.thumbnail-ms"); v != int64(0) {
		t.Errorf("Off stored %v", v)
	}
	if lo, _ := p.custom.Range(); lo != 0 {
		t.Errorf("with an Off chip the custom number starts at %v, want 0", lo)
	}
}

func TestSecondChipsStoreSecondsAndCustomTakesSeconds(t *testing.T) {
	k := testKit(t, "")
	p := pickerOf(t, k, field("modules.weather.refresh-interval-seconds", durationChips(inSeconds, after(15*time.Minute), after(time.Hour))))
	if got := chipLabels(p); got[1] != locale.T().Get("settings-duration-hours", i18n.Int("n", 1)) {
		t.Errorf("3600 s reads %q", got[1])
	}
	p.chips[1].SetActive(true)
	if v := k.store.value("modules.weather.refresh-interval-seconds"); v != int64(3600) {
		t.Errorf("an hour stored %v", v)
	}
	p.chips[2].SetActive(true)
	p.custom.OnValueChanged(90)
	if v := k.store.value("modules.weather.refresh-interval-seconds"); v != int64(90) {
		t.Errorf("typing 90 seconds stored %v", v)
	}
	if lo, _ := p.custom.Range(); lo != 1 {
		t.Errorf("without a zero chip the custom seconds start at %v, want 1", lo)
	}
}

func TestTheCustomNumberNamesItsUnit(t *testing.T) {
	tr := locale.T()
	for _, c := range []struct {
		spec rowSpec
		unit string
	}{
		{lockGraceRow(), "settings-duration-unit-seconds"},
		{cyclingRow(), "settings-duration-unit-minutes"},
		{reminderRow(), "settings-duration-unit-days"},
		{field("launcher.history.max-size", countChips(inEntries, 25)), "settings-unit-entries"},
	} {
		k := testKit(t, "")
		p := pickerOf(t, k, c.spec)
		if labels := treetest.Labels(p.typed); !slices.Contains(labels, tr.Get(c.unit)) {
			t.Errorf("%s's custom number reads %q, want %q beside it", c.spec.path, labels, tr.Get(c.unit))
		}
	}
}

func TestACountChipWritesItsNumberAndCustomTakesAnyOther(t *testing.T) {
	k := testKit(t, "")
	p := pickerOf(t, k, field("launcher.history.max-size", countChips(inEntries, 25, 50, 100, 500)))
	if got := chipLabels(p); len(got) != 4 || got[0] != "25" || got[3] != "500" {
		t.Errorf("count chips read %q", got)
	}
	if p.active() != 0 || p.typed.Visible() {
		t.Fatalf("the default 25 shows chip %d, custom shown %v", p.active(), p.typed.Visible())
	}
	p.chips[2].SetActive(true)
	if v := k.store.value("launcher.history.max-size"); v != int64(100) {
		t.Errorf("choosing 100 stored %v", v)
	}
	p.chips[4].SetActive(true)
	p.custom.OnValueChanged(30)
	if v := k.store.value("launcher.history.max-size"); v != int64(30) {
		t.Errorf("typing 30 stored %v", v)
	}
	if p.active() != 4 || !p.typed.Visible() {
		t.Errorf("30 entries shows chip %d, custom shown %v", p.active(), p.typed.Visible())
	}
}

func TestTheFramerateIsPickedFromChipsInFramesPerSecond(t *testing.T) {
	k := testKit(t, "")
	p := pickerOf(t, k, pageRow(t, "recorder", "modules.recorder.framerate"))
	tr := locale.T()
	want := []string{tr.Get("settings-framerate", i18n.Int("n", 30)), tr.Get("settings-framerate", i18n.Int("n", 60)), tr.Get("settings-framerate", i18n.Int("n", 120))}
	if got := chipLabels(p); !slices.Equal(got, want) {
		t.Fatalf("chips %q, want %q", got, want)
	}
	if p.active() != 1 {
		t.Errorf("the default 60 fps shows chip %d", p.active())
	}
	p.chips[2].SetActive(true)
	if v := k.store.value("modules.recorder.framerate"); v != int64(120) {
		t.Errorf("120 fps stored %v", v)
	}
	k = testKit(t, "modules:\n  recorder:\n    framerate: 24\n")
	p = pickerOf(t, k, pageRow(t, "recorder", "modules.recorder.framerate"))
	if !p.typed.Visible() || p.custom.Value() != 24 || !slices.Contains(treetest.Labels(p.typed), tr.Get("settings-framerate-unit")) {
		t.Errorf("24 fps: number shown %v at %v, beside %q", p.typed.Visible(), p.custom.Value(), treetest.Labels(p.typed))
	}
	if lo, _ := p.custom.Range(); lo != 1 {
		t.Errorf("the custom framerate starts at %v, want 1", lo)
	}
}

func TestModuleDurationsAreChipsInTheirFieldsUnit(t *testing.T) {
	tr := locale.T()
	for _, tc := range []struct {
		page, path string
		first      string
		pick       int
		stored     int64
	}{
		{"bitwarden", "modules.bitwarden.clipboard-clear", tr.Get("settings-duration-seconds", i18n.Int("n", 10)), 2, 60},
		{"idle-inhibit", "modules.idle-inhibit.startup-duration", tr.Get("settings-duration-minutes", i18n.Int("n", 15)), 3, 120},
		{"nightlight", "modules.nightlight.transition-duration", tr.Get("settings-duration-instant"), 0, 0},
		{"cpu", "modules.cpu.poll-interval-ms", tr.Get("settings-duration-seconds", i18n.Int("n", 1)), 0, 1000},
		{"ram", "modules.ram.poll-interval-ms", tr.Get("settings-duration-seconds", i18n.Int("n", 1)), 3, 10000},
	} {
		k := testKit(t, "")
		p := pickerOf(t, k, pageRow(t, tc.page, tc.path))
		if got := chipLabels(p)[0]; got != tc.first {
			t.Errorf("%s: the first chip reads %q, want %q", tc.path, got, tc.first)
		}
		p.chips[tc.pick].SetActive(true)
		if v := k.store.value(tc.path); v != tc.stored {
			t.Errorf("%s: chip %d stored %v, want %d", tc.path, tc.pick, v, tc.stored)
		}
	}
}

func TestZeroReadsAsWhatItMeansOrIsNoChoice(t *testing.T) {
	tr := locale.T()
	k := testKit(t, "")
	labels := chipLabels(pickerOf(t, k, pageRow(t, "idle-inhibit", "modules.idle-inhibit.startup-duration")))
	if got := labels[len(labels)-1]; got != tr.Get("settings-duration-until-off") {
		t.Errorf("an indefinite keep-awake reads %q", got)
	}
	poll := pickerOf(t, k, pageRow(t, "cpu", "modules.cpu.poll-interval-ms"))
	if slices.ContainsFunc(poll.presets, func(pr preset) bool { return pr.value == 0 }) {
		t.Errorf("the poll interval offers 0 among %q", chipLabels(poll))
	}
	if lo, _ := poll.custom.Range(); lo <= 0 {
		t.Errorf("a custom poll interval reaches %v", lo)
	}
}

func TestEveryMonitorsDefaultPollIntervalIsAPreset(t *testing.T) {
	for _, m := range []string{"cpu", "ram", "storage", "netstat"} {
		k := testKit(t, "")
		p := pickerOf(t, k, pageRow(t, m, "modules."+m+".poll-interval-ms"))
		if p.typed.Visible() {
			t.Errorf("%s's default poll interval %v shows as custom", m, k.store.value("modules."+m+".poll-interval-ms"))
		}
	}
}
