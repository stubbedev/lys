package launcherview

import (
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
)

func TestLookOfIsWhatTheLauncherSectionSets(t *testing.T) {
	cfg := config.Defaults()
	cfg.Launcher.Location = config.LauncherSouthWest
	cfg.Launcher.Lines = 4
	cfg.Launcher.FixedNumLines = true
	cfg.Launcher.SidebarMode = true
	cfg.Launcher.ShowIcons = false
	cfg.Launcher.Font = "Inter 12"
	cfg.Launcher.Styles = map[string]string{"compact": ".launcher-row { padding: 0; }"}
	got := LookOf(cfg)
	want := Look{Location: config.LauncherSouthWest, Width: WidthPx(cfg), Lines: 4, Fixed: true, Sidebar: true, Font: "Inter 12"}
	if got != want {
		t.Errorf("look = %+v, want %+v without a style preset", got, want)
	}
}

func TestALooksCSSIsItsFontThenItsStyle(t *testing.T) {
	if css := (Look{}).CSS(); css != "" {
		t.Errorf("neither font nor style added %q", css)
	}
	if _, ok := (Look{Font: "  "}).Stylesheet(0); ok {
		t.Error("a blank font made a stylesheet")
	}
	css := Look{Font: "Inter 12"}.CSS()
	if !strings.Contains(css, `font-family: "Inter"`) || !strings.Contains(css, "font-size: 12pt") || !strings.HasPrefix(css, ".launcher-surface {") {
		t.Errorf("font css = %q (a preset must not reach the rest of the shell)", css)
	}
	if got := (Look{Style: ".launcher-row { padding: 0; }"}).CSS(); got != ".launcher-row { padding: 0; }" {
		t.Errorf("preset = %q", got)
	}
	both := Look{Font: "Inter 12", Style: ".launcher-row { padding: 0; }"}.CSS()
	if !strings.Contains(both, `font-family: "Inter"`) || !strings.HasSuffix(both, ".launcher-row { padding: 0; }") {
		t.Errorf("font and preset = %q", both)
	}
}

func TestFontProperties(t *testing.T) {
	p := fontProperties("Monospace 20")
	if !strings.Contains(p, `font-family: "Monospace"`) || !strings.Contains(p, "font-size: 20pt") || strings.Contains(p, "font:") {
		t.Errorf("Monospace 20 = %q", p)
	}
	p = fontProperties("Inter Bold Italic 11")
	if !strings.Contains(p, "font-style: italic") || !strings.Contains(p, "font-weight: 700") {
		t.Errorf("bold italic = %q", p)
	}
	if plain := fontProperties("Inter 11"); strings.Contains(plain, "font-style") || strings.Contains(plain, "font-weight") {
		t.Errorf("a plain description overrode the theme: %q", plain)
	}
	if got := fontProperties("Inter"); got != `font-family: "Inter";` {
		t.Errorf("sizeless = %q", got)
	}
	if got := fontProperties("Inter 14px"); !strings.Contains(got, "font-size: 14px") {
		t.Errorf("absolute size = %q", got)
	}
}
