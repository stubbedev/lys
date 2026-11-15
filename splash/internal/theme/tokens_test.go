package theme

import (
	"encoding/binary"
	"image"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/desktop/splashframe"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/desktop/tokens"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/third_party/typesetting/font"
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

func TestTheLysLogoAndTheThrobberShareTheAccent(t *testing.T) {
	t.Parallel()
	primary := Default().Accent
	accented, err := Parse("theme.yaml", []byte("accent: \"#7aa2f7\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	for name, th := range map[string]Theme{"default": Default(), "accent override": accented} {
		size := image.Pt(800, 600)
		f := frame(size)
		r := renderer(t, th, nil)
		draw(t, r, f, 1, nil, State{Phase: 3})
		l := r.layout(size, 1)
		count := func(box image.Rectangle, c render.Color) int {
			n := 0
			for y := box.Min.Y; y < box.Max.Y; y++ {
				for x := box.Min.X; x < box.Max.X; x++ {
					if render.Color(binary.LittleEndian.Uint32(f.Pix[y*f.Stride+x*4:])) == c {
						n++
					}
				}
			}
			return n
		}
		for part, box := range map[string]image.Rectangle{"logo": l.Logo, "throbber": l.Throb} {
			if count(box, th.Accent) == 0 {
				t.Errorf("%s: no pixel of the %s %v is the accent %#08x", name, part, box, uint32(th.Accent))
			}
			if th.Accent != primary && count(box, primary) != 0 {
				t.Errorf("%s: the %s %v is drawn in the palette's primary %#08x, not the accent %#08x", name, part, box, uint32(primary), uint32(th.Accent))
			}
		}
		if got := r.LastFrame([]splashframe.Output{{Name: "Virtual-1", Size: size, Scale: 1}}).LogoColor; got != th.Accent {
			t.Errorf("%s: the last frame records the logo in %#08x, want the accent %#08x", name, uint32(got), uint32(th.Accent))
		}
	}
}

func TestALogoInAnotherColourIsNotTheRenderersLogo(t *testing.T) {
	t.Parallel()
	th := Default()
	th.Accent = render.RGB(0x7a, 0xa2, 0xf7)
	r := renderer(t, th, nil)
	stale, err := RasterizeLogo(1, Default().Accent)
	if err != nil {
		t.Fatal(err)
	}
	r.AddLogo(stale)
	if !r.NeedsLogo(1) {
		t.Error("a logo rasterized in another colour stands in for the theme's")
	}
	fresh, err := RasterizeLogo(1, r.LogoColor())
	if err != nil {
		t.Fatal(err)
	}
	r.AddLogo(fresh)
	if r.NeedsLogo(1) {
		t.Error("a logo rasterized in the theme's accent is still needed")
	}
}

func TestTheSplashEmbedsTheGreetersDefaultTypeface(t *testing.T) {
	t.Parallel()
	r := renderer(t, Default(), nil)
	greeter := config.Defaults().General.FontSans
	if got := r.face.Family(); got != greeter {
		t.Errorf("the splash embeds %q, the greeter's default font is %q", got, greeter)
	}
	if greeter != tokens.FontSans {
		t.Errorf("the greeter's default font is %q, not the token's %q", greeter, tokens.FontSans)
	}
	if d := r.face.Describe(); d.Aspect.Style != font.StyleNormal || d.Aspect.Weight != font.WeightNormal {
		t.Errorf("the embedded face is %+v, want the regular face", d)
	}
}
