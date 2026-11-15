package styling

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/tokens"
)

func TestMetricsCSSDeclaresEveryTokenOnce(t *testing.T) {
	css := MetricsCSS()
	var names []string
	for _, m := range spaces {
		names = append(names, "--space-"+m.name, "--bar-space-"+m.name)
	}
	for _, m := range radii {
		names = append(names, "--radius-"+m.name, "--bar-radius-"+m.name, "--bar-button-radius-"+m.name)
	}
	names = append(names, "--radius-full", "--field-fill", "--field-border")
	for _, name := range names {
		if n := strings.Count(css, "    "+name+": "); n != 1 {
			t.Errorf("MetricsCSS declares %s %d times, want once", name, n)
		}
	}
	for _, want := range []string{
		"--space-sm: calc(0.5rem * var(--global-scale));",
		"--bar-space-xl: calc(2rem * var(--bar-scale));",
		"--radius-md: calc(0.625rem * var(--global-scale));",
		"--bar-button-radius-xl: calc(1.125rem * var(--bar-scale));",
		"--radius-full: 9999px;",
		"--field-fill: color-mix(in srgb, var(--palette-fg) 16%, transparent);",
		"--field-border: color-mix(in srgb, var(--palette-fg) 12%, transparent);",
	} {
		if !strings.Contains(css, want) {
			t.Errorf("MetricsCSS lacks %q:\n%s", want, css)
		}
	}
}

func TestRoundingRadiusPxIsTheRadiusToken(t *testing.T) {
	for level, rem := range map[config.RoundingLevel]tokens.Rem{
		config.RoundingSm: tokens.RadiusSM,
		config.RoundingMd: tokens.RadiusMD,
		config.RoundingLg: tokens.RadiusLG,
	} {
		if got, want := RoundingRadiusPx(level, 2), int(rem.Px(tokens.RemPx, 2)+0.5); got != want {
			t.Errorf("RoundingRadiusPx(%s, 2) = %d, want the token's %d", level, got, want)
		}
	}
	if got := RoundingRadiusPx(config.RoundingFull, 3); got != tokens.RadiusFullPx {
		t.Errorf("RoundingRadiusPx(full, 3) = %d, want the pill's %d", got, tokens.RadiusFullPx)
	}
}

func paintProbe(t *testing.T, bundle, value string) [4]byte {
	t.Helper()
	var warns []string
	widget.SetParseWarn(func(msg string) { warns = append(warns, msg) })
	sheet := widget.NewStylesheet(bundle+"\n.probe { background-color: "+value+"; }", 200)
	widget.SetParseWarn(nil)
	if len(warns) > 0 {
		t.Fatalf("the bundle trips the CSS engine: %v", warns)
	}
	box := widget.NewBox(widget.Row, 0, 0)
	box.AddClass("probe")
	box.SetSizeRequest(10, 10)
	box.AttachStylesheet(sheet)
	widget.SizeWithin(box, widget.Size{W: 10, H: 10})
	box.Arrange(render.Rect{W: 10, H: 10})
	data := make([]byte, render.Stride(10)*10)
	box.Paint(render.NewScaled(data, render.Stride(10), 10, 10, 1, 1))
	i := 5*render.Stride(10) + 5*4
	return [4]byte{data[i+2], data[i+1], data[i], data[i+3]}
}

func TestFieldTokensPaintTheFieldMix(t *testing.T) {
	cfg := config.Defaults()
	palette := cfg.Styling.ActivePalette()
	p, err := PaletteFromHex(palette)
	if err != nil {
		t.Fatal(err)
	}
	bundle := BundleCSS(cfg, palette, p)
	for name, pct := range map[string]tokens.Percent{"--field-fill": tokens.FieldFill, "--field-border": tokens.FieldBorder} {
		want := ColorMix(p.Fg, 0, int(pct))
		if got := paintProbe(t, bundle, "var("+name+")"); got != [4]byte{want.R(), want.G(), want.B(), uint8(uint32(want) >> 24)} {
			t.Errorf("var(%s) paints %v, want the text color at %d%% %#08x", name, got, pct, uint32(want))
		}
	}
}

var lockFieldRule = regexp.MustCompile(`(?s)\.lock-window \.lock-credentials entry,[^{]*\{([^}]*)\}`)

func TestTheLoginFieldRestsOnTheFieldTokens(t *testing.T) {
	m := lockFieldRule.FindStringSubmatch(StaticCSS)
	if m == nil {
		t.Fatal("StaticCSS has no rule for the login screen's entries")
	}
	for _, want := range []string{"background-color: var(--field-fill);", "box-shadow: inset 0 0 0 1px var(--field-border);"} {
		if !strings.Contains(m[1], want) {
			t.Errorf("the login field's rule lacks %q:\n%s", want, m[1])
		}
	}
}
