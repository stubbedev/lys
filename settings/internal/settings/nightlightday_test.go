package settings

import (
	"errors"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/fakeclock"
	"github.com/stubbedev/lys/desktop/geo"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/shell/service/nightlight"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const customNight = "modules:\n  clock:\n    format: \"%H:%M\"\n  nightlight:\n    auto-schedule: true\n    schedule: custom\n    custom-hours:\n      from: \"22:00\"\n      to: \"07:00\"\n    transition-duration: 0\n"

func mustNightSetting(t *testing.T, temp, gamma int) nightlight.Setting {
	t.Helper()
	s, err := nightlight.NewSetting(temp, gamma)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func nightKit(t *testing.T, yaml string, now time.Time) *kit {
	t.Helper()
	k := testKit(t, yaml)
	k.clock = fakeclock.At(now)
	return k
}

func TestTheNightLightSaysHowLongASwitchHolds(t *testing.T) {
	tr := locale.T()
	evening := time.Date(2026, 6, 21, 20, 0, 0, 0, time.UTC)
	morning := time.Date(2026, 6, 21, 8, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, yaml string
		now        time.Time
		on, off    string
	}{
		{"without a schedule", "modules:\n  nightlight:\n    auto-schedule: false\n", evening, tr.Get("settings-nightlight-is-on"), tr.Get("settings-nightlight-is-off")},
		{"from sunset to sunrise", "modules:\n  nightlight:\n    auto-schedule: true\n    schedule: sunset-to-sunrise\n", evening, tr.Get("settings-nightlight-on-until-sunrise"), tr.Get("settings-nightlight-turn-on-until-sunrise")},
		{
			"custom hours before their end", customNight, evening,
			tr.Get("settings-nightlight-on-until", i18n.Str("time", "Mon 07:00")), tr.Get("settings-nightlight-turn-on-until", i18n.Str("time", "Mon 07:00")),
		},
		{
			"custom hours past their end", customNight, morning,
			tr.Get("settings-nightlight-on-until", i18n.Str("time", "Mon 07:00")), tr.Get("settings-nightlight-turn-on-until", i18n.Str("time", "Mon 07:00")),
		},
	} {
		k := nightKit(t, tc.yaml, tc.now)
		if got, err := nightlightWords(k, true, tc.now); err != nil || got != tc.on {
			t.Errorf("%s, on reads %q, %v, want %q", tc.name, got, err, tc.on)
		}
		if got, err := nightlightWords(k, false, tc.now); err != nil || got != tc.off {
			t.Errorf("%s, off reads %q, %v, want %q", tc.name, got, err, tc.off)
		}
	}
}

func TestTheNightLightSwitchRewordsWhenTheScheduleChanges(t *testing.T) {
	k := testKit(t, "modules:\n  nightlight:\n    auto-schedule: true\n    schedule: sunset-to-sunrise\n")
	k.nightlight = func() (nightlightControl, error) { return &fakeNightlight{on: true}, nil }
	s := nightlightNow(k).(*shellSwitch[bool])
	t.Cleanup(s.close)
	if got := s.Description(); got != locale.T().Get("settings-nightlight-on-until-sunrise") {
		t.Fatalf("on under the default schedule reads %q", got)
	}
	if err := s.k.store.set("modules.nightlight.auto-schedule", false); err != nil {
		t.Fatal(err)
	}
	s.configChanged()
	if got := s.Description(); got != locale.T().Get("settings-nightlight-is-on") {
		t.Errorf("with the schedule off it reads %q", got)
	}
}

func TestTheNightLightDayShowsTheCustomNight(t *testing.T) {
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	k := nightKit(t, customNight, now)
	text, tints, through, err := nightDay(k, k.store.svc.Config().Nightlight, now)
	if err != nil {
		t.Fatal(err)
	}
	if want := locale.T().Get("settings-nightlight-day-times", i18n.Str("start", "07:00"), i18n.Str("end", "22:00")); text != want {
		t.Errorf("the day reads %q, want %q", text, want)
	}
	if through != 0.5 {
		t.Errorf("noon is %v through the day", through)
	}
	white := tint(mustNightSetting(t, 6600, 100))
	if len(tints) != daySamples || tints[0] == white || tints[len(tints)-1] == white || tints[daySamples/2] != white {
		t.Errorf("the strip is %d tints, midnight %#x, noon %#x, want warm nights around a white noon", len(tints), uint32(tints[0]), uint32(tints[daySamples/2]))
	}
}

func TestTheNightLightDayFollowsThePinnedOrFoundPlace(t *testing.T) {
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	sun := "modules:\n  nightlight:\n    auto-schedule: true\n    schedule: sunset-to-sunrise\n"
	k := nightKit(t, sun, now)
	text, tints, _, err := nightDay(k, k.store.svc.Config().Nightlight, now)
	if err != nil || tints != nil || text != locale.T().Get("settings-nightlight-day-no-location") {
		t.Errorf("with no place found yet: %q, %d tints, %v", text, len(tints), err)
	}
	k.nightLocation = func() (geo.Coordinates, error) { return geo.Coordinates{}, errors.New("the record is torn") }
	text, tints, _, err = nightDay(k, k.store.svc.Config().Nightlight, now)
	if want := locale.T().Get("settings-nightlight-day-unreadable"); tints != nil || text != want || err == nil || err.Error() != "the record is torn" {
		t.Errorf("with an unreadable place: %q over %v, %d tints, want %q over the error", text, err, len(tints), want)
	}
	d := nightlightDayPreview(k).(*nightlightDay)
	wantClosedFailure(t, "the day with an unreadable place", d.text.stateParts, d.text.Text(), locale.T().Get("settings-nightlight-day-unreadable"), "the record is torn")
	k.nightLocation = func() (geo.Coordinates, error) { return geo.NewCoordinates(78.2, 15.6) }
	text, tints, _, _ = nightDay(k, k.store.svc.Config().Nightlight, now)
	if tints == nil || text != locale.T().Get("settings-nightlight-day-polar-day") {
		t.Errorf("at Svalbard's midsummer: %q, %d tints", text, len(tints))
	}
	k.nightLocation = func() (geo.Coordinates, error) { return geo.NewCoordinates(55.6, 12.5) }
	text, tints, _, _ = nightDay(k, k.store.svc.Config().Nightlight, now)
	if tints == nil || text == locale.T().Get("settings-nightlight-day-polar-day") || text == "" {
		t.Errorf("in Copenhagen: %q, %d tints", text, len(tints))
	}
}

func TestTheNightLightDayHidesWithTheScheduleOff(t *testing.T) {
	now := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	k := nightKit(t, customNight, now)
	d := nightlightDayPreview(k).(*nightlightDay)
	if !d.Visible() || !d.strip.Visible() {
		t.Fatal("the day is hidden under custom hours")
	}
	if err := k.store.set("modules.nightlight.auto-schedule", false); err != nil {
		t.Fatal(err)
	}
	d.configChanged()
	if d.Visible() {
		t.Error("the day shows with the schedule off")
	}
}
