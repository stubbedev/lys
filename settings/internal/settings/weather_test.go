package settings

import (
	"slices"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
)

func TestTheWeatherPageAsksOnlyForTheChosenProvidersKey(t *testing.T) {
	k := testKit(t, "")
	spec, _ := pageByID(layout(k.store.svc.Config(), bothParts), "weather")
	p := buildPage(k, spec)
	keyShown := func(path string) bool {
		i := slices.IndexFunc(p.rows, func(r *fieldRow) bool { return r.path == path })
		if i < 0 {
			t.Fatalf("no row %s", path)
		}
		for _, c := range p.sections {
			if slices.Contains(p.targets[c.first:c.first+c.n], p.rows[i]) {
				return c.box.Visible()
			}
		}
		return true
	}
	const visual, weatherAPI = "modules.weather.visual-crossing-key", "modules.weather.weatherapi-key"
	if keyShown(visual) || keyShown(weatherAPI) {
		t.Fatal("Open-Meteo, which needs no key, shows a key row")
	}
	provider := slices.IndexFunc(p.rows, func(r *fieldRow) bool { return r.path == "modules.weather.provider" })
	for _, c := range []struct {
		provider     config.WeatherProvider
		shown, other string
	}{
		{config.WeatherProviderVisualCrossing, visual, weatherAPI},
		{config.WeatherProviderWeatherApi, weatherAPI, visual},
	} {
		if err := p.rows[provider].be.slot("modules.weather.provider").set(string(c.provider)); err != nil {
			t.Fatal(err)
		}
		if !keyShown(c.shown) || keyShown(c.other) {
			t.Errorf("with %s: its key shown %v, the other's shown %v", c.provider, keyShown(c.shown), keyShown(c.other))
		}
	}
}
