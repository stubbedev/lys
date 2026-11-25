package bar

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/barview"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

// squareIconTheme installs a hicolor theme holding name, a symbolic
// glyph inked edge to edge, as the only icon search path for the
// test.
func squareIconTheme(t *testing.T, name string) {
	t.Helper()
	base := t.TempDir()
	dir := filepath.Join(base, "hicolor", "scalable", "apps")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	index := "[Icon Theme]\nName=Hicolor\nDirectories=scalable/apps\n\n[scalable/apps]\nSize=16\nMinSize=1\nMaxSize=512\nType=Scalable\n"
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16"><rect width="16" height="16" fill="#000"/></svg>`
	for path, body := range map[string]string{
		filepath.Join(base, "hicolor", "index.theme"): index,
		filepath.Join(dir, name+".svg"):               svg,
	} {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	prev := widget.IconSearchPaths()
	widget.SetIconSearchPaths([]string{base})
	widget.RefreshIcons()
	t.Cleanup(func() {
		widget.SetIconSearchPaths(prev)
		widget.RefreshIcons()
	})
}

// inkBounds is the device rect covering every pixel of a w x h buffer
// with any alpha.
func inkBounds(data []byte, w, h int) render.Rect {
	x0, y0, x1, y1 := w, h, 0, 0
	for y := range h {
		for x := range w {
			if pixelAt(data, w, x, y)>>24 == 0 {
				continue
			}
			x0, y0, x1, y1 = min(x0, x), min(y0, y), max(x1, x+1), max(y1, y+1)
		}
	}
	if x1 <= x0 {
		return render.Rect{}
	}
	return render.Rect{X: x0, Y: y0, W: x1 - x0, H: y1 - y0}
}

// A bar button's icon covers its configured logical size in device
// pixels exactly once on every output scale: 20px is 30 device pixels
// at 1.5x, not the 45 a device raster mapped through the scale again
// paints, and it never inks past its own box onto the label beside
// it.
func TestBarIconTakesItsLogicalSizeOnFractionalScales(t *testing.T) {
	const name = "lys-test-square-symbolic"
	squareIconTheme(t, name)
	cfg := config.Defaults()
	cfg.Bar.ButtonIconSize = config.Px(20)
	cfg.Bar.ButtonVariant = config.ButtonBasic
	ctx := styledContext(t, cfg)
	icon := barview.Icon(config.IconConfig{Show: true, Name: name})
	b := newBarButton(ctx, icon, "Off")
	host := widget.NewBox(widget.Row, 0, 0).Append(b, false)
	ctx.Theme.Attach(host)
	const lw, lh = 200, 60
	widget.SizeWithin(host, widget.Size{W: lw, H: lh})
	host.Arrange(render.Rect{W: lw, H: lh})
	for _, tc := range []struct {
		num, denom, want int
	}{
		{1, 1, 20},
		{120, 120, 20},
		{180, 120, 30},
		{240, 120, 40},
	} {
		w, h := lw*tc.num/tc.denom, lh*tc.num/tc.denom
		data := make([]byte, render.Stride(w)*h)
		icon.Paint(render.NewScaled(data, render.Stride(w), w, h, tc.num, tc.denom))
		ink := inkBounds(data, w, h)
		if ink.W != tc.want || ink.H != tc.want {
			t.Errorf("scale %d/%d: icon inks %dx%d device px, want %dx%d", tc.num, tc.denom, ink.W, ink.H, tc.want, tc.want)
		}
		if box := render.MapRect(icon.Bounds(), tc.num, tc.denom); ink.Union(box) != box {
			t.Errorf("scale %d/%d: icon inks %+v, past its device box %+v", tc.num, tc.denom, ink, box)
		}
	}
}
