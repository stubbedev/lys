package widget

import (
	"testing"

	"github.com/stubbedev/lys/lib/render"
)

// A swatch's edge reads on the surfaces it sits on, a window or a menu,
// whatever its own color: a swatch of the background's own shade (a
// palette's base, crust or mantle) still shows its outline, 3:1 off
// the surface on either palette (#271).
func TestASwatchsEdgeReadsOnAnySurface(t *testing.T) {
	// over composites fg, premultiplied as render colors are, on bg.
	over := func(fg, bg render.Color) render.Color {
		a := float64(fg.A()) / 255
		mix := func(f, b uint8) uint8 { return uint8(float64(f) + float64(b)*(1-a) + 0.5) }
		return render.RGB(mix(fg.R(), bg.R()), mix(fg.G(), bg.G()), mix(fg.B(), bg.B()))
	}
	for _, th := range []*Theme{DarkTheme(), LightTheme()} {
		SetTheme(th)
		for _, bg := range []render.Color{th.Bg, th.PopoverColor()} {
			s := NewColorSwatch(bg)
			arrangeTree(t, s, 20, 20)
			edge := over(borderColors(s.style(s))[0], bg)
			if borderOf(s.style(s)).Top == 0 {
				t.Fatalf("dark %v: the swatch has no edge", th.IsDark())
			}
			if r := contrastRatio(edge, bg); r < 3 {
				t.Errorf("dark %v: on %08x a swatch of that shade has an edge %.2f:1 off it, want 3:1", th.IsDark(), uint32(bg), r)
			}
		}
	}
	SetTheme(nil)
}
