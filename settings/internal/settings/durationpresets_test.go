package settings

import (
	"testing"

	"github.com/stubbedev/lys/settings/internal/locale"
)

func cyclingInterval(t *testing.T, configYAML string) (*kit, *presetDuration) {
	t.Helper()
	k := testKit(t, configYAML)
	return k, fieldControl(k, k.store, field("wallpaper.cycling-interval-mins", minutePresets(5, 15, 60))).(*presetDuration)
}

func TestTheCyclingIntervalIsChosenAmongPresets(t *testing.T) {
	k, c := cyclingInterval(t, "wallpaper:\n  cycling-interval-mins: 15\n")
	if got, want := c.menu.Items()[c.menu.Selected()], minutesLabel(15); got != want || c.custom.Visible() {
		t.Fatalf("15 minutes reads %q with the number shown %v, want %q alone", got, c.custom.Visible(), want)
	}
	if minutesLabel(60) != durationLabel("1h") {
		t.Errorf("an hour reads %q", minutesLabel(60))
	}
	c.menu.OnSelect(2)
	if got := k.store.value("wallpaper.cycling-interval-mins"); got != int64(60) {
		t.Errorf("choosing an hour stored %v", got)
	}
}

func TestACustomIntervalShowsItsNumber(t *testing.T) {
	k, c := cyclingInterval(t, "wallpaper:\n  cycling-interval-mins: 7\n")
	if got := c.menu.Items()[c.menu.Selected()]; got != locale.T().Get("settings-duration-custom") || !c.custom.Visible() || c.custom.Value() != 7 {
		t.Fatalf("7 minutes reads %q, number shown %v at %v", got, c.custom.Visible(), c.custom.Value())
	}
	_, c = cyclingInterval(t, "")
	c.menu.OnSelect(len(c.presets))
	if !c.custom.Visible() {
		t.Fatal("choosing Custom hid the number")
	}
	c.custom.OnValueChanged(42)
	if got := k.store.value("wallpaper.cycling-interval-mins"); got != int64(7) {
		t.Errorf("the other kit's value changed to %v", got)
	}
	if got := c.slot.get(); got != int64(42) {
		t.Errorf("typing 42 stored %v", got)
	}
}
