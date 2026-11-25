package config

import (
	"testing"

	"github.com/stubbedev/lys/desktop/i18n"
)

// The night light's bar sample read "On" beside its Icon Off, the icon a
// module shows before its first reading. Its sample is the filter off,
// as that icon and the bar's label then read it.
func TestTheNightlightSampleIsTheFilterOffItsIconShows(t *testing.T) {
	cfg := Defaults()
	icon, ok := cfg.ModuleIcon(ModuleNightlight)
	if !ok || icon != cfg.Nightlight.IconOffView() {
		t.Fatalf("the night light's module icon is %+v, want its Icon Off", icon)
	}
	samples := map[string]string{}
	for _, p := range Placeholders("modules.nightlight.format") {
		samples[p.Name] = p.Sample
	}
	if got, want := samples["status"], i18n.Shell().Get("bar-nightlight-off"); got != want {
		t.Errorf("the status sample reads %q, the bar's label off %q", got, want)
	}
	for _, live := range []string{"temp", "gamma"} {
		if samples[live] != "--" {
			t.Errorf("the %s sample reads %q, what the bar shows with the filter off is --", live, samples[live])
		}
	}
}
