package settings

import (
	"image"
	"image/color"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/lib/golden"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

const backdropShotW, backdropShotH = 160, 100

type countingPainter struct{ invalidated int }

func (p *countingPainter) Invalidate() { p.invalidated++ }

func fixtureWallpaper(t *testing.T, name string, w, h int, at func(x, y int) color.NRGBA) string {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetNRGBA(x, y, at(x, y))
		}
	}
	path := filepath.Join(t.TempDir(), name)
	savePNG(t, path, img)
	return path
}

func duskWallpaper(t *testing.T) string {
	return fixtureWallpaper(t, "dusk.png", 64, 40, func(x, y int) color.NRGBA {
		return color.NRGBA{R: uint8(40 + x*3), G: uint8(30 + y*2), B: uint8(150 - y), A: 255}
	})
}

func solidWallpaper(t *testing.T, name string, c color.NRGBA) string {
	return fixtureWallpaper(t, name, 40, 40, func(int, int) color.NRGBA { return c })
}

func withTheme(t *testing.T, th *widget.Theme) {
	t.Helper()
	prev := widget.Current()
	widget.SetTheme(th)
	t.Cleanup(func() { widget.SetTheme(prev) })
}

func setWallpaper(t *testing.T, k *kit, path string) {
	t.Helper()
	if err := k.store.set("wallpaper.wallpaper", path); err != nil {
		t.Fatal(err)
	}
}

func paintBackdropScreen(bd screenBackdrop, inset int) ([]byte, render.Rect) {
	stride := render.Stride(backdropShotW)
	data := make([]byte, stride*backdropShotH)
	cv := render.New(data, stride, backdropShotW, backdropShotH)
	cv.Clear(cv.Rect(), widget.Current().CardColor())
	r := render.Rect{X: inset, Y: inset, W: backdropShotW - 2*inset, H: backdropShotH - 2*inset}
	backdropScreen(cv, r, bd, func(*render.Canvas, render.Rect) {})
	return data, r
}

func shotPixel(data []byte, x, y int) render.Color {
	i := y*render.Stride(backdropShotW) + 4*x
	return render.ColorFromBytes(data[i : i+4])
}

var backdropThemes = []struct {
	name string
	th   func() *widget.Theme
}{{"dark", widget.DarkTheme}, {"light", widget.LightTheme}}

func TestTheBackdropIsTheWallpaperElseTheThemesFillInEitherTheme(t *testing.T) {
	for _, theme := range backdropThemes {
		for _, source := range []string{"wallpaper", "theme"} {
			name := "backdrop-" + source + "-" + theme.name
			t.Run(name, func(t *testing.T) {
				withTheme(t, theme.th())
				k := testKit(t, "")
				if source == "wallpaper" {
					setWallpaper(t, k, duskWallpaper(t))
				}
				data, _ := paintBackdropScreen(k.backdropFor(&countingPainter{}), 0)
				stride := render.Stride(backdropShotW)
				golden.Check(t, filepath.Join("testdata", "golden"), name, render.NRGBA(data, stride, backdropShotW, backdropShotH), golden.Tolerance{})
			})
		}
	}
}

func TestAFramedPreviewsCornersAreTheCardsColorInEitherTheme(t *testing.T) {
	for _, theme := range backdropThemes {
		for _, source := range []string{"wallpaper", "theme"} {
			t.Run(source+"-"+theme.name, func(t *testing.T) {
				withTheme(t, theme.th())
				k := testKit(t, "")
				if source == "wallpaper" {
					setWallpaper(t, k, solidWallpaper(t, "red.png", color.NRGBA{R: 200, G: 40, B: 40, A: 255}))
				}
				data, r := paintBackdropScreen(k.backdropFor(&countingPainter{}), 4)
				card := widget.Current().CardColor()
				for _, at := range [][2]int{{r.X, r.Y}, {r.X + r.W - 1, r.Y}, {r.X, r.Y + r.H - 1}, {r.X + r.W - 1, r.Y + r.H - 1}} {
					if got := shotPixel(data, at[0], at[1]); got != card {
						t.Errorf("the corner pixel at %v is %08x, want the card's %08x", at, uint32(got), uint32(card))
					}
				}
				if mid := shotPixel(data, backdropShotW/2, backdropShotH/2); mid == card {
					t.Error("the screen's middle is the card's color, not the backdrop")
				}
			})
		}
	}
}

func TestAMissingWallpaperFallsBackToTheThemeWithoutAToast(t *testing.T) {
	k := testKit(t, "")
	var toasts []string
	k.toast = func(text string, _ time.Duration) (*widget.Toast, error) {
		toasts = append(toasts, text)
		return nil, nil
	}
	fallback, _ := paintBackdropScreen(k.backdropFor(&countingPainter{}), 0)
	setWallpaper(t, k, filepath.Join(t.TempDir(), "gone.png"))
	missing, _ := paintBackdropScreen(k.backdropFor(&countingPainter{}), 0)
	if !slices.Equal(missing, fallback) {
		t.Error("a missing wallpaper does not paint the theme's backdrop")
	}
	if len(toasts) != 0 {
		t.Errorf("a missing wallpaper toasted %q", toasts)
	}
	setWallpaper(t, k, duskWallpaper(t))
	if shown, _ := paintBackdropScreen(k.backdropFor(&countingPainter{}), 0); slices.Equal(shown, fallback) {
		t.Error("a readable wallpaper paints the theme's backdrop")
	}
}

func TestAChangedWallpaperRepaintsThePreviewsOnceLoaded(t *testing.T) {
	k := testKit(t, "")
	var queued []func()
	k.background = func(fn func()) { queued = append(queued, fn) }
	run := func() {
		for len(queued) > 0 {
			fn := queued[0]
			queued = queued[1:]
			fn()
		}
	}
	red, blue := color.NRGBA{R: 220, A: 255}, color.NRGBA{B: 220, A: 255}
	painter := &countingPainter{}
	bd := k.backdropFor(painter)
	middle := func() render.Color {
		data, _ := paintBackdropScreen(bd, 0)
		return shotPixel(data, backdropShotW/2, backdropShotH/2)
	}
	fallback := middle()
	setWallpaper(t, k, solidWallpaper(t, "red.png", red))
	if got := middle(); got != fallback {
		t.Fatalf("before the wallpaper loads the preview paints %08x, want the theme's %08x", uint32(got), uint32(fallback))
	}
	run()
	if painter.invalidated != 1 {
		t.Fatalf("the loaded wallpaper repainted the preview %d times, want once", painter.invalidated)
	}
	wantRed := render.RGB(red.R, red.G, red.B)
	if got := middle(); got != wantRed {
		t.Fatalf("the loaded wallpaper paints %08x, want %08x", uint32(got), uint32(wantRed))
	}
	setWallpaper(t, k, solidWallpaper(t, "blue.png", blue))
	if got := middle(); got != wantRed {
		t.Errorf("while the new wallpaper loads the preview paints %08x, want the last one %08x", uint32(got), uint32(wantRed))
	}
	run()
	if painter.invalidated != 2 {
		t.Fatalf("the changed wallpaper repainted the preview %d times in all, want twice", painter.invalidated)
	}
	if got, want := middle(), render.RGB(blue.R, blue.G, blue.B); got != want {
		t.Errorf("the changed wallpaper paints %08x, want %08x", uint32(got), uint32(want))
	}
	run()
	if painter.invalidated != 2 {
		t.Errorf("an unchanged wallpaper loaded again: %d repaints", painter.invalidated)
	}
}

func TestTheFitPicturesPlaceTheWallpaperAsTheirModeDoes(t *testing.T) {
	k := testKit(t, "")
	setWallpaper(t, k, solidWallpaper(t, "red.png", color.NRGBA{R: 220, A: 255}))
	red := render.RGB(220, 0, 0)
	bd := k.backdropFor(&countingPainter{})
	paint := func(fit config.FitMode) []byte {
		stride := render.Stride(backdropShotW)
		data := make([]byte, stride*backdropShotH)
		fitPictures[string(fit)].draw(render.New(data, stride, backdropShotW, backdropShotH), render.Rect{W: backdropShotW, H: backdropShotH}, bd)
		return data
	}
	corner := func(data []byte) render.Color { return shotPixel(data, 6, 6) }
	for _, fit := range []config.FitMode{config.FitFill, config.FitStretch} {
		if got := corner(paint(fit)); got != red {
			t.Errorf("%s leaves the screen's corner %08x, want the wallpaper", fit, uint32(got))
		}
	}
	for _, fit := range []config.FitMode{config.FitFit, config.FitCenter} {
		if got := corner(paint(fit)); got == red {
			t.Errorf("%s covers the screen's corner with a wallpaper of another shape", fit)
		}
	}
}
