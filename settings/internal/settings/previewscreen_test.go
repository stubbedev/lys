package settings

import (
	"testing"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

const previewTestW, previewTestH = 320, 160

// paintedPreview lays w out previewTestW by previewTestH and paints it.
func paintedPreview(w widget.Widget) []byte {
	data := make([]byte, render.Stride(previewTestW)*previewTestH)
	w.Arrange(render.Rect{W: previewTestW, H: previewTestH})
	w.Paint(render.New(data, render.Stride(previewTestW), previewTestW, previewTestH))
	return data
}

func pixelAt(data []byte, x, y int) [4]byte {
	i := y*render.Stride(previewTestW) + 4*x
	return [4]byte(data[i : i+4])
}

// A preview's screen has the corners the theme rounds a card with: its
// corner pixel is left empty, while the wallpaper fills its middle.
func TestAPreviewScreenRoundsItsCornersLikeACard(t *testing.T) {
	if widget.Current().Radius < 2 {
		t.Fatalf("the test theme rounds cards by %dpx", widget.Current().Radius)
	}
	data := paintedPreview(wallpaperScreen(testKit(t, ""), previewTestH))
	if a := pixelAt(data, 0, 0)[3]; a != 0 {
		t.Errorf("the screen's corner pixel has alpha %d, want the rounded corner left empty", a)
	}
	if a := pixelAt(data, previewTestW/2, previewTestH/4)[3]; a != 255 {
		t.Errorf("the screen's middle has alpha %d, want the wallpaper", a)
	}
}

// An off surface's caption sits on a plate, not on the wallpaper, so it
// reads on any wallpaper: the pixel just before the caption's text is
// not the wallpaper's.
func TestAnOffSurfacesCaptionSitsOnAPlate(t *testing.T) {
	k := testKit(t, "")
	screen := offScreen(k, previewTestH, "settings-dock-preview-off")
	data := paintedPreview(screen)
	caption := screen.Children()[1].(*widget.Label).Bounds()
	y := caption.Y + caption.H/2
	if caption.X < 8 {
		t.Fatalf("the caption starts at x=%d", caption.X)
	}
	if behind, wallpaper := pixelAt(data, caption.X-2, y), pixelAt(data, 2, y); behind == wallpaper {
		t.Errorf("the caption sits on the wallpaper (%v), want a plate behind it", behind)
	}
}
