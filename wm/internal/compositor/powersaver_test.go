package compositor

import (
	"image"
	"testing"

	"github.com/stubbedev/lys/wm/internal/backend"
)

func TestLowRefreshModeTakesTheSlowestModeOfTheSizeAtOrAboveTheRate(t *testing.T) {
	big, small := image.Pt(2880, 1800), image.Pt(1920, 1200)
	mode := func(size image.Point, hz int32) backend.Mode { return backend.Mode{Size: size, RefreshMHz: hz * 1000} }
	for _, tc := range []struct {
		name   string
		modes  []backend.Mode
		within ModePolicy
		want   backend.Mode
	}{
		{"120 and 60 Hz", []backend.Mode{mode(big, 120), mode(big, 60)}, HighResMode{}, mode(big, 60)},
		{"nothing at or above the rate below the chosen", []backend.Mode{mode(big, 144), mode(big, 48)}, HighResMode{}, mode(big, 144)},
		{"the slowest above the rate", []backend.Mode{mode(big, 165), mode(big, 120), mode(big, 90), mode(big, 40)}, HighResMode{}, mode(big, 90)},
		{"59.94 Hz counts as 60", []backend.Mode{mode(big, 120), {Size: big, RefreshMHz: 59940}}, HighResMode{}, backend.Mode{Size: big, RefreshMHz: 59940}},
		{"never another size", []backend.Mode{mode(big, 120), mode(small, 60)}, HighResMode{}, mode(big, 120)},
		{"within an exact size", []backend.Mode{mode(big, 120), mode(small, 120), mode(small, 60)}, ExactMode{Size: small, RefreshMHz: 120000}, mode(small, 60)},
	} {
		got, err := lowRefreshMode{within: tc.within, hz: DefaultPowerSaverRate}.choose(tc.modes, tc.modes[0])
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got != tc.want {
			t.Errorf("%s: chose %+v, want %+v", tc.name, got, tc.want)
		}
	}
	if _, err := (lowRefreshMode{within: ExactMode{Size: small}, hz: DefaultPowerSaverRate}).choose([]backend.Mode{mode(big, 120)}, mode(big, 120)); err == nil {
		t.Error("a missing exact mode chose a mode under power saver, want the exact policy's error")
	}
}
