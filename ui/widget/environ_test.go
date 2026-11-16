package widget

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stubbedev/lys/lib/render"
)

const envProbeCSS = `.env { background-color: Canvas; border: 4px solid light-dark(#101010, #efefef); }
	@media (min-width: 500px) { .env { border-style: dashed; } }`

func envProbe() *boxLayersLeaf {
	w := &boxLayersLeaf{}
	w.AddClass("env")
	w.AttachStylesheet(NewStylesheet(envProbeCSS, StylePriorityApplication))
	return w
}

func envProbePaint(w *boxLayersLeaf) (bg, border render.Color) {
	SizeWithin(w, Size{W: 30, H: 30})
	w.Arrange(render.Rect{W: 30, H: 30})
	data := paintTree(w, 30, 30)
	return centerOf(data, 30, 30), render.ColorFromBytes(data[15*render.Stride(30)+1*4:])
}

func TestAThemeSwapRacesNoTreeRestylingAndPaintingAtOnce(t *testing.T) {
	themes := [2]*Theme{DarkTheme(), LightTheme()}
	t.Cleanup(func() {
		SetTheme(nil)
		SetMediaViewport(0, 0)
		SetMediaContrast(false)
		SetMediaResolution(96)
		SetRootFontSize(16)
	})
	borders := map[render.Color]render.Color{
		themes[0].Bg: render.RGB(0xef, 0xef, 0xef),
		themes[1].Bg: render.RGB(0x10, 0x10, 0x10),
	}
	const rounds = 60
	atOnce(t, 2, func(w int) error {
		if w == 0 {
			for i := range rounds {
				SetTheme(themes[i%2])
				SetMediaViewport(300+400*(i%2), 200)
				SetMediaContrast(i%3 == 0)
				SetMediaResolution(float64(96 + 48*(i%2)))
				SetRootFontSize(float64(16 + i%2))
			}
			return nil
		}
		probe := envProbe()
		var errs []error
		for i := range rounds {
			bg, border := envProbePaint(probe)
			want, ok := borders[bg]
			if !ok {
				errs = append(errs, fmt.Errorf("round %d: Canvas painted %v, neither theme's background", i, bg))
				continue
			}
			if border != want {
				errs = append(errs, fmt.Errorf("round %d: under the background %v light-dark() painted %v, want %v from the same theme", i, bg, border, want))
			}
		}
		return errors.Join(errs...)
	})

	probe := envProbe()
	for _, th := range themes {
		SetTheme(th)
		if bg, border := envProbePaint(probe); bg != th.Bg || border != borders[th.Bg] {
			t.Errorf("after the swaps, %v's tree painted Canvas %v and light-dark() %v, want %v and %v", th.Bg, bg, border, th.Bg, borders[th.Bg])
		}
	}
	SetMediaViewport(640, 480)
	if w, h := MediaViewportOf(probe); w != 640 || h != 480 {
		t.Errorf("a tree without a viewport of its own reports %dx%d, want the global 640x480", w, h)
	}
	SetMediaViewportFor(probe, 100, 50)
	if w, h := MediaViewportOf(probe); w != 100 || h != 50 {
		t.Errorf("a tree with its own viewport reports %dx%d, want 100x50", w, h)
	}
	SetRootFontSize(-1)
	SetRootFontSize(20)
	if got := RootFontSize(); got != 20 {
		t.Errorf("the root font size is %v after setting 20 and ignoring -1", got)
	}
}
