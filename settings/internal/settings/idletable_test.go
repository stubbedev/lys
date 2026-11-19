package settings

import (
	"errors"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func TestADurationReadsAsWords(t *testing.T) {
	for value, want := range map[string]string{"never": "Never", "1m": "1 minute", "5m": "\u20685\u2069 minutes", "1h": "1 hour", "90s": "\u206890\u2069 seconds", "2h": "\u20682\u2069 hours"} {
		if got := durationLabel(value); got != want {
			t.Errorf("%s reads %q, want %q", value, got, want)
		}
	}
}

func TestAnIdleRowSetsPluggedInAndBatteryTimes(t *testing.T) {
	k := testKit(t, "idle:\n  lock:\n    ac: 7m\n")
	p := fieldControl(k, k.store, idleTimeoutsRow("lock")).(*idlePair)
	if got := p.ac.values[p.ac.Selected()]; got != "7m" {
		t.Fatalf("a custom time shows as %q", got)
	}
	p.battery.SetSelected(len(p.battery.values) - 1)
	if v := k.store.value("idle.lock.battery"); v != "never" {
		t.Errorf("choosing Never on battery stored %v", v)
	}
	p.ac.SetSelected(0)
	if v := k.store.value("idle.lock.ac"); v != "1m" {
		t.Errorf("choosing 1 minute plugged in stored %v", v)
	}
	if p.ac.values[len(p.ac.values)-1] == "7m" {
		t.Error("a custom time stays offered after it was replaced")
	}
}

func TestTheBatteryTimeHasItsOwnSettingLink(t *testing.T) {
	if err := checkSetting(bothParts, "idle", "idle.suspend.battery"); err != nil {
		t.Error(err)
	}
}

func TestAStageReachedOnlyAfterALaterOneIsOvertaken(t *testing.T) {
	at := func(d time.Duration) idleTime { return idleTime{after: d} }
	never := idleTime{never: true}
	ordered := map[idleStage]idleTime{idleDim: at(time.Minute), idleLock: at(5 * time.Minute), idleScreenOff: at(3 * time.Minute), idleSuspend: at(time.Hour)}
	for _, stage := range idleStages {
		if then, ok := idleOvertaken(ordered, stage); ok {
			t.Errorf("in order, %s is overtaken by %s", stage, then)
		}
	}
	for name, c := range map[string]struct {
		times       map[idleStage]idleTime
		stage, then idleStage
	}{
		"dim with the screens off":  {map[idleStage]idleTime{idleDim: at(10 * time.Minute), idleScreenOff: at(10 * time.Minute), idleSuspend: never}, idleDim, idleScreenOff},
		"dim after suspend":         {map[idleStage]idleTime{idleDim: at(20 * time.Minute), idleScreenOff: never, idleSuspend: at(15 * time.Minute)}, idleDim, idleSuspend},
		"lock after suspend":        {map[idleStage]idleTime{idleLock: at(time.Hour), idleSuspend: at(15 * time.Minute)}, idleLock, idleSuspend},
		"screens off after suspend": {map[idleStage]idleTime{idleScreenOff: at(30 * time.Minute), idleSuspend: at(30 * time.Minute)}, idleScreenOff, idleSuspend},
	} {
		if then, ok := idleOvertaken(c.times, c.stage); !ok || then != c.then {
			t.Errorf("%s: overtaken by %q (%v), want %q", name, then, ok, c.then)
		}
	}
	if _, ok := idleOvertaken(map[idleStage]idleTime{idleDim: never, idleScreenOff: at(time.Minute), idleSuspend: never}, idleDim); ok {
		t.Error("a stage that never runs is overtaken")
	}
	if _, ok := idleOvertaken(map[idleStage]idleTime{idleLock: at(time.Hour), idleSuspend: never}, idleLock); ok {
		t.Error("a stage is overtaken by one that never runs")
	}
}

func idlePairOf(t *testing.T, k *kit, stage idleStage) *idlePair {
	t.Helper()
	return fieldControl(k, k.store, idleTimeoutsRow(stage)).(*idlePair)
}

func TestAnIdleRowWarnsWhenALaterStageComesFirst(t *testing.T) {
	k := testKit(t, "idle:\n  dim:\n    ac: 10m\n    battery: 1m\n  screen-off:\n    ac: 5m\n    battery: 5m\n")
	p := idlePairOf(t, k, idleDim)
	want := locale.T().Get("settings-idle-overtaken", i18n.Str("source", "ac"), i18n.Str("then", "screen-off"))
	if w := p.warnings[idleOnAC]; !w.Visible() || w.Text() != want {
		t.Fatalf("plugged in the dim row warns %v: %q", w.Visible(), w.Text())
	}
	if p.warnings[idleOnBattery].Visible() {
		t.Error("on battery, dimming before the screens turn off is warned about")
	}
	if err := k.store.set("idle.dim.ac", "2m"); err != nil {
		t.Fatal(err)
	}
	p.refresh()
	if p.warnings[idleOnAC].Visible() {
		t.Error("the warning stays once the order is fixed")
	}
}

func TestTheBatteryTimesHideWithoutABattery(t *testing.T) {
	desktop := laptop()
	desktop.state.battery.IsPresent = false
	for name, c := range map[string]struct {
		power func() (powerControl, error)
		shown bool
	}{
		"a laptop":         {func() (powerControl, error) { return laptop(), nil }, true},
		"a desktop":        {func() (powerControl, error) { return desktop, nil }, false},
		"no UPower to ask": {func() (powerControl, error) { return nil, errors.New("no UPower") }, true},
	} {
		k := testKit(t, "idle:\n  lock:\n    battery: 1h\n  suspend:\n    battery: 15m\n")
		asked := 0
		k.power = func() (powerControl, error) { asked++; return c.power() }
		lock, suspend := idlePairOf(t, k, idleLock), idlePairOf(t, k, idleSuspend)
		if asked != 1 {
			t.Errorf("%s: two rows asked for the battery %d times", name, asked)
		}
		for _, p := range []*idlePair{lock, suspend} {
			if p.battery.Visible() != c.shown || p.batteryIcon.Visible() != c.shown {
				t.Errorf("%s: the %s battery time shown %v, want %v", name, p.stage, p.battery.Visible(), c.shown)
			}
		}
		if lock.warnings[idleOnBattery].Visible() != c.shown {
			t.Errorf("%s: the battery order warning shown %v, want %v", name, lock.warnings[idleOnBattery].Visible(), c.shown)
		}
	}
}

func TestIdleRowsFollowTheBuiltInIdleSwitch(t *testing.T) {
	k := testKit(t, "idle:\n  enable: false\n")
	p := buildPage(k, idlePage(nil))
	for _, r := range p.rows {
		if r.path == "idle.enable" || r.path == "lid_action" {
			if !widget.IsEnabled(r) {
				t.Errorf("%s, which does not hang on the built-in idle, is disabled", r.path)
			}
			continue
		}
		if widget.IsEnabled(r) {
			t.Errorf("%s is enabled while the built-in idle is off", r.path)
		}
	}
	if err := k.store.set("idle.enable", true); err != nil {
		t.Fatal(err)
	}
	p.showSections()
	for _, r := range p.rows {
		if !widget.IsEnabled(r) {
			t.Errorf("%s stays disabled with the built-in idle on", r.path)
		}
	}
}
