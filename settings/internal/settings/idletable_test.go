package settings

import "testing"

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
