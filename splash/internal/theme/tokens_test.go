package theme

import (
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/desktop/tokens"
	"github.com/stubbedev/lys/lib/render"
)

func TestTheSplashDrawsWithTheGreetersTokens(t *testing.T) {
	t.Parallel()
	greeter, err := styling.PaletteFromHex(config.Defaults().Styling.ActivePalette())
	if err != nil {
		t.Fatal(err)
	}
	token := func(tok config.CssToken) render.Color {
		c, ok := greeter.Token(tok)
		if !ok {
			t.Fatalf("the greeter's palette has no %s", tok)
		}
		return c
	}
	d := Default()
	for _, c := range []struct {
		name      string
		got, want render.Color
	}{
		{"foreground", d.Foreground, token(config.TokenFgDefault)},
		{"secondary", d.Secondary, token(config.TokenFgMuted)},
		{"accent", d.Accent, token(config.TokenAccent)},
		{"error", d.Error, token(config.TokenStatusError)},
	} {
		if c.got != c.want {
			t.Errorf("the splash's %s is %#08x, the greeter's %#08x", c.name, uint32(c.got), uint32(c.want))
		}
	}
	if d.Background != render.RGB(0, 0, 0) {
		t.Errorf("the splash's background is %#08x, want black for the firmware's logo", uint32(d.Background))
	}
	r := renderer(t, d, nil)
	fill := styling.ColorMix(greeter.Fg, d.Background, int(tokens.FieldFill))
	if got := r.fieldFill(); got != fill {
		t.Errorf("the splash's field is %#08x, the greeter's over black %#08x", uint32(got), uint32(fill))
	}
	if got, want := r.fieldBorder(), styling.ColorMix(greeter.Fg, fill, int(tokens.FieldBorder)); got != want {
		t.Errorf("the splash's field ring is %#08x, the greeter's over black %#08x", uint32(got), uint32(want))
	}
	for _, scale := range []int{1, 2} {
		for _, s := range []struct {
			name  string
			got   float64
			greet styling.TextRole
		}{
			{"title", textPx(tokens.Title, scale), styling.Title},
			{"secondary", textPx(tokens.Caption, scale), styling.Caption},
			{"body", textPx(tokens.Body, scale), styling.Body},
		} {
			if want := widgets.TextPx(s.greet, float64(scale)); s.got != want {
				t.Errorf("at scale %d the splash's %s text is %vpx, the greeter's %vpx", scale, s.name, s.got, want)
			}
		}
	}
	if got, want := tokens.RadiusFullPx, styling.RoundingRadiusPx(config.RoundingFull, 1); got != want {
		t.Errorf("the splash's pill radius is %d, the greeter's %d", got, want)
	}
}
