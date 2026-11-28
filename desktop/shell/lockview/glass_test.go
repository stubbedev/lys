package lockview

import (
	"image"
	"image/color"
	"math"
	"testing"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/memory-go"
	"golang.org/x/image/font/gofont/gobold"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/apptheme"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/desktop/styling"
)

const (
	shotW, shotH = 1280, 800
)

// wallpaper is a w x h picture whose left half is red and right half
// blue, with soft colored discs over both, so a glyph shows which
// side of it is behind.
func wallpaper(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	discs := []struct {
		x, y, r float64
		c       [3]float64
	}{{0.3, 0.2, 0.18, [3]float64{255, 210, 40}}, {0.62, 0.15, 0.14, [3]float64{40, 230, 160}}, {0.5, 0.6, 0.3, [3]float64{200, 60, 230}}}
	for y := range h {
		for x := range w {
			c := [3]float64{230, 40, 50}
			if x >= w/2 {
				c = [3]float64{30, 70, 230}
			}
			for _, d := range discs {
				dist := math.Hypot(float64(x)/float64(w)-d.x, float64(y)/float64(h)-d.y)
				if k := 1 - dist/d.r; k > 0 {
					for i := range c {
						c[i] = c[i]*(1-k) + d.c[i]*k
					}
				}
			}
			img.SetRGBA(x, y, color.RGBA{R: uint8(c[0]), G: uint8(c[1]), B: uint8(c[2]), A: 255})
		}
	}
	return img
}

// halves is a w x h picture red on the left half, blue on the right.
func halves(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			c := color.RGBA{R: 220, G: 30, B: 30, A: 255}
			if x >= w/2 {
				c = color.RGBA{R: 30, G: 30, B: 220, A: 255}
			}
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

// glassSurface is a lock screen over src, at rest, with the clock
// reading text.
func glassSurface(t *testing.T, src image.Image, opaque bool, text string) *Surface {
	t.Helper()
	var mem memory.Shared
	bg := newBackdrop(t, &mem, src, 0, image.Pt(shotW*2, shotH*2))
	s := Build(Spec{
		Mode: Lock, Config: config.Defaults().Login, Font: treetest.Font(t), Backdrop: &bg, Opaque: opaque,
		Sheet:    widget.NewStylesheet(bundleCSS(t), apptheme.Priority),
		OnSubmit: func(string) {},
	})
	s.Prompt.SetTime(text, "Saturday 28 November")
	s.Prompt.Reveal.SetRevealed(true)
	s.Prompt.Reveal.Finish()
	return s
}

// bundleCSS is the shell's stylesheet bundle for the default config.
func bundleCSS(t *testing.T) string {
	t.Helper()
	cfg := config.Defaults()
	palette := cfg.Styling.ActivePalette()
	theme, err := styling.PaletteFromHex(palette)
	if err != nil {
		t.Fatal(err)
	}
	return styling.BundleCSS(cfg, palette, theme)
}

// shot paints root at num/denom onto a fresh canvas and returns it as
// premultiplied RGBA.
func shot(root widget.Widget, num, denom int) *image.RGBA {
	widget.SizeWithin(root, widget.Size{W: shotW, H: shotH})
	root.Arrange(render.Rect{W: shotW, H: shotH})
	w, h := shotW*num/denom, shotH*num/denom
	pix := make([]byte, render.Stride(w)*h)
	root.Paint(render.NewScaled(pix, render.Stride(w), w, h, num, denom))
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(pix); i += 4 {
		img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = pix[i+2], pix[i+1], pix[i], pix[i+3]
	}
	return img
}

// solid is the pixels of face's composite that are wholly inside a
// glyph, as device points, left of x or right of it.
func solid(t *testing.T, face *glassFace, x int, left bool) []image.Point {
	t.Helper()
	full := unit8(glassOpacity * 255)
	var out []image.Point
	b := face.img.Bounds()
	for py := range b.Dy() {
		for px := range b.Dx() {
			p := image.Pt(face.at.X+px, face.at.Y+py)
			if face.img.Pix[face.img.PixOffset(px, py)+3] >= full && (p.X < x) == left {
				out = append(out, p)
			}
		}
	}
	if len(out) == 0 {
		t.Fatalf("no glyph pixel lies wholly inside a glyph on that side of %d", x)
	}
	return out
}

// meanColor is the average R, G and B of img at pts.
func meanColor(img *image.RGBA, pts []image.Point) [3]int {
	var sum [3]int
	for _, p := range pts {
		o := img.PixOffset(p.X, p.Y)
		for i := range sum {
			sum[i] += int(img.Pix[o+i])
		}
	}
	for i := range sum {
		sum[i] /= len(pts)
	}
	return sum
}

func TestTheClocksGlyphsShowTheFrostedWallpaperBehindThem(t *testing.T) {
	for _, scale := range [][2]int{{1, 1}, {2, 1}, {5, 4}} {
		num, denom := scale[0], scale[1]
		s := glassSurface(t, halves(800, 500), false, "08:08")
		img := shot(s.Root, num, denom)
		face := s.Prompt.glass.shown
		if face == nil || face.img.RGBA == nil {
			t.Fatalf("scale %d/%d: the glass drew no clock", num, denom)
		}
		mid := img.Bounds().Dx() / 2
		for _, side := range []struct {
			left        bool
			name        string
			strong, off int
		}{{true, "red", 0, 2}, {false, "blue", 2, 0}} {
			c := meanColor(img, solid(t, face, mid, side.left))
			if c[side.strong]-c[side.off] < 80 || c[side.strong]-c[1] < 80 {
				t.Errorf("scale %d/%d: the glyphs over the %s half average %v, not %s glass", num, denom, side.name, c, side.name)
			}
		}
	}
}

func TestTheGlassLeavesThePixelsAwayFromTheGlyphsAlone(t *testing.T) {
	s := glassSurface(t, wallpaper(800, 500), false, "12:34")
	img := shot(s.Root, 1, 1)
	blank := glassSurface(t, wallpaper(800, 500), false, "")
	ref := shot(blank.Root, 1, 1)
	face := s.Prompt.glass.shown
	drawn := image.Rectangle{Min: face.at, Max: face.at.Add(face.img.Bounds().Size())}
	clock := s.Prompt.Clock.Bounds()
	changed := 0
	for y := range img.Bounds().Dy() {
		for x := range img.Bounds().Dx() {
			o := img.PixOffset(x, y)
			if [4]uint8(img.Pix[o:o+4]) == [4]uint8(ref.Pix[o:o+4]) {
				continue
			}
			if !image.Pt(x, y).In(drawn) {
				t.Fatalf("pixel (%d, %d) outside the glass's %v changed", x, y, drawn)
			}
			changed++
		}
	}
	if changed == 0 {
		t.Fatal("the glass clock changed nothing")
	}
	if drawn.Dx() >= clock.W/2 {
		t.Errorf("the glass covers %v, most of the clock row %v rather than the glyphs", drawn, clock)
	}
	for _, corner := range []image.Point{{0, 0}, {face.img.Bounds().Dx() - 1, 0}, {0, face.img.Bounds().Dy() - 1}} {
		if a := face.img.Pix[face.img.PixOffset(corner.X, corner.Y)+3]; a != 0 {
			t.Errorf("the glass's corner %v, past the shadow's reach, has alpha %d", corner, a)
		}
	}
}

func TestTheClockIsInkWithoutGlass(t *testing.T) {
	glass := glassSurface(t, halves(800, 500), false, "08:08")
	shot(glass.Root, 1, 1)
	pts := solid(t, glass.Prompt.glass.shown, shotW/2, true)
	for _, c := range []struct {
		name string
		s    *Surface
	}{
		{"opaque", glassSurface(t, halves(800, 500), true, "08:08")},
		{"no backdrop", Build(Spec{Mode: Lock, Config: config.Defaults().Login, Font: treetest.Font(t), OnSubmit: func(string) {}})},
	} {
		if c.s.Prompt.glass != nil || widget.HasClass(c.s.Prompt.Clock, glassInk) {
			t.Errorf("%s: the clock is glass", c.name)
		}
	}
	opaque := glassSurface(t, halves(800, 500), true, "08:08")
	img := shot(opaque.Root, 1, 1)
	if c := meanColor(img, pts); max(c[0], c[1], c[2])-min(c[0], c[1], c[2]) > 40 || min(c[0], c[1], c[2]) < 150 {
		t.Errorf("the opaque clock's glyphs average %v, not its light ink", c)
	}
}

func TestTheMinuteChangeReshapesOnlyTheGlyphs(t *testing.T) {
	s := glassSurface(t, wallpaper(800, 500), false, "09:41")
	before := shot(s.Root, 2, 1)
	g := s.Prompt.glass
	frost, face := g.frost, g.shown

	shot(s.Root, 2, 1)
	if g.frost != frost || g.shown != face {
		t.Error("a repaint of the same minute rebuilt the glass")
	}

	s.Prompt.SetTime("09:42", "Saturday 28 November")
	after := shot(s.Root, 2, 1)
	if g.frost != frost {
		t.Error("the minute change frosted the backdrop again")
	}
	if g.shown == face {
		t.Error("the minute change kept the old glyphs")
	}
	if string(before.Pix) == string(after.Pix) {
		t.Error("the minute change drew the same clock")
	}

	shot(s.Root, 1, 1)
	if g.frost == frost {
		t.Error("a new scale kept the frost of the old one")
	}
}

func TestAFontChangeReshapesTheGlassAtOnce(t *testing.T) {
	bold, err := render.LoadFont(gobold.TTF)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name   string
		change func(s *Surface)
	}{
		{"face", func(s *Surface) { s.Prompt.Clock.SetFace(bold) }},
		{"theme", func(s *Surface) {
			widget.AttachedStylesheets(s.Root)[0].Load(bundleCSS(t) + ".lock-window .lock-clock { letter-spacing: 0.2em; }")
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			s := glassSurface(t, wallpaper(800, 500), false, "09:41")
			before := shot(s.Root, 1, 1)
			g := s.Prompt.glass
			frost, face := g.frost, g.shown

			shot(s.Root, 1, 1)
			if g.frost != frost || g.shown != face {
				t.Fatal("a repaint of the same font and minute rebuilt the glass")
			}

			c.change(s)
			after := shot(s.Root, 1, 1)
			if g.shown.key != face.key || g.frost != frost {
				t.Fatalf("the %s change moved the clock from %+v to %+v, so it does not show the glass keeps its glyphs", c.name, face.key, g.shown.key)
			}
			if g.shown == face {
				t.Errorf("the %s change kept the old glyphs", c.name)
			}
			if string(before.Pix) == string(after.Pix) {
				t.Errorf("the %s change drew the same clock", c.name)
			}
		})
	}
}
