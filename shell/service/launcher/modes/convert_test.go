package modes

import "testing"

func TestAConversionSpacesItsUnitAsTheLocaleDoes(t *testing.T) {
	for _, c := range []struct {
		locale string
		value  float64
		unit   string
		want   string
	}{
		{"en-US", 3.5, "mi", "3.5 mi"},
		{"de", 3.5, "mi", "3,5 mi"},
		{"fr", 3.5, "mi", "3,5 mi"},
		{"fr", 1000, "MB", "1 000 MB"},
		{"pt", 3.5, "km/h", "3,5 km/h"},
	} {
		if got := (Conversion{Value: c.value, Unit: c.unit}).Show(loaderIn(t, c.locale)); got != c.want {
			t.Errorf("%s %v %s = %q, want %q", c.locale, c.value, c.unit, got, c.want)
		}
	}
}

func TestADegreeConversionTakesNoSpaceInAnyLocale(t *testing.T) {
	for locale, want := range map[string]string{"en-US": "37.5°C", "de": "37,5°C", "fr": "37,5°C"} {
		if got := (Conversion{Value: 37.5, Unit: "°C"}).Show(loaderIn(t, locale)); got != want {
			t.Errorf("%s 37.5 °C = %q, want %q", locale, got, want)
		}
	}
	if got := (Conversion{Value: 37.5, Unit: "K"}).Show(loaderIn(t, "fr")); got != "37,5 K" {
		t.Errorf("fr 37.5 K = %q, want the locale's space before a unit that is no degree", got)
	}
}
