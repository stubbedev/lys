package extract

import (
	"testing"

	"github.com/stubbedev/lys/desktop/config"
)

func TestMatugenSchemeCLIValue(t *testing.T) {
	if got := config.MatugenSchemeFruitSalad.CLIValue(); got != "scheme-fruit-salad" {
		t.Errorf("CLIValue = %q", got)
	}
}

func TestWallustPaletteIsLight(t *testing.T) {
	for _, p := range []config.WallustPalette{config.WallustPaletteLight, config.WallustPaletteSoftlightcomp16} {
		if !p.IsLight() {
			t.Errorf("%s is a light palette", p)
		}
	}
	for _, p := range []config.WallustPalette{config.WallustPaletteDark16, config.WallustPaletteAnsidark, config.WallustPaletteHarddarkcomp} {
		if p.IsLight() {
			t.Errorf("%s is not a light palette", p)
		}
	}
}

func TestFromConfigPicksTheProvidersTool(t *testing.T) {
	c := config.DefaultsColorExtractor()
	if got := FromConfig(c).Tool; got != None {
		t.Errorf("the lys-shell provider extracts with %s, want none", got)
	}
	c.ThemeProvider, c.WallustSaturation, c.PywalContrast = config.ThemeProviderWallust, 40, 7.5
	ex := FromConfig(c)
	if ex.Tool != Wallust || ex.WallustSaturation != 40 || ex.PywalContrast != 7.5 || ex.WallustPalette != config.WallustPaletteDark16 {
		t.Errorf("FromConfig = %+v", ex)
	}
	for p, want := range map[config.ThemeProvider]Tool{config.ThemeProviderMatugen: Matugen, config.ThemeProviderPywal: Pywal} {
		if got := ToolFor(p); got != want {
			t.Errorf("ToolFor(%s) = %s, want %s", p, got, want)
		}
	}
}

func TestFormatFloatIsShortestPlainDecimal(t *testing.T) {
	for in, want := range map[float64]string{3: "3", 0: "0", 0.05: "0.05", -0.25: "-0.25", 21: "21", 1e-7: "0.0000001"} {
		if got := formatFloat(in); got != want {
			t.Errorf("formatFloat(%v) = %q, want %q", in, got, want)
		}
	}
}
