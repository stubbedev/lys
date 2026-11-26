package preview

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stubbedev/lys/lib/golden"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/anim"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/apptheme"
)

// goldenPad is the margin around the view in a shot.
const goldenPad = 8

// goldenFiles makes the file each renderer's shot shows, every one of a
// fixed size and modification time, so the detail lines never change.
func goldenFiles(t *testing.T) map[string]string {
	t.Helper()
	dir := t.TempDir()
	picture := pngData(t, gradient(160, 90))
	picture = append(picture, make([]byte, 32<<10-len(picture))...)
	folder := filepath.Join(dir, "Holiday")
	for i, size := range []int{1500, 2500, 4096} {
		writeFile(t, filepath.Join(folder, "photo-"+string(rune('a'+i))+".jpg"), make([]byte, size))
	}
	if err := os.Chtimes(folder, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	return map[string]string{
		"image":    writeFile(t, filepath.Join(dir, "sunset.png"), picture),
		"text":     writeFile(t, filepath.Join(dir, "main.go"), []byte("package main\n\nimport \"fmt\"\n\n// main greets.\nfunc main() {\n\tfmt.Println(\"hello\", 42)\n}\n")),
		"folder":   folder,
		"fallback": writeFile(t, filepath.Join(dir, "notes.thing"), make([]byte, 700)),
	}
}

// Each renderer's view under the shell's stylesheet, in the dark and
// the light variant of its palette, as the goldens show it: the
// picture fitted, the code highlighted, the folder counted, and a file
// nothing renders as its icon over its name and detail line.
func TestTheRenderersInEitherTheme(t *testing.T) {
	t.Cleanup(anim.SetInstant(true))
	files := goldenFiles(t)
	for _, renderer := range []string{"image", "text", "folder", "fallback"} {
		for _, palette := range []struct {
			name       string
			appearance config.Appearance
		}{{"dark", config.AppearanceDark}, {"light", config.AppearanceLight}} {
			name := renderer + "-" + palette.name
			t.Run(name, func(t *testing.T) {
				prev := widget.Current()
				t.Cleanup(func() { widget.SetTheme(prev) })
				cfg := config.Defaults()
				cfg.Styling.PaletteBaseTheme, cfg.Styling.Appearance = "catppuccin-mocha", palette.appearance
				sheet := apptheme.New(cfg).Sheet
				h := newHarness(t)
				v := h.view(nil)
				root := v.Root()
				root.AttachStylesheet(sheet)
				h.show(v, files[renderer])
				size := widget.SizeWithin(root, widget.Size{W: 1200, H: 900})
				root.Arrange(render.Rect{X: goldenPad, Y: goldenPad, W: size.W, H: size.H})
				w, hgt := size.W+2*goldenPad, size.H+2*goldenPad
				stride := render.Stride(w)
				buf := make([]byte, stride*hgt)
				cv := render.New(buf, stride, w, hgt)
				cv.Clear(cv.Rect(), widget.Current().Bg)
				root.Paint(cv)
				golden.Check(t, filepath.Join("testdata", "golden"), name, render.NRGBA(buf, stride, w, hgt), golden.Tolerance{})
			})
		}
	}
}
