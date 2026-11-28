package lockview

import (
	"image"
	"math"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

// glassInk is the clock label's class while glass fills its glyphs: the
// stylesheet hides the label's own ink and shadow, which the glass draws
// in their place.
const glassInk = "lock-glass-ink"

// The glass recipe, its lengths in ems of the clock's line box so it
// looks the same at every size and scale: the frost's blur, how far the
// rim bends the backdrop and how wide that bezel runs, the rim light's
// width, and the shadow's blur and drop.
const (
	frostSigma   = 0.05
	bendReach    = 0.08
	bezelSigma   = 0.035
	rimSigma     = 0.008
	shadowSigma  = 0.05
	shadowDrop   = 0.015
	shadowAlpha  = 0.36
	frostSat     = 1.5
	frostWhite   = 0.22
	rimLight     = 0.95
	rimAmbient   = 0.25
	rimShade     = 0.35
	glassOpacity = 0.96
)

// glassScale is how much finer the device is than the frost raster: the
// frost is blurred far past a pixel, so half the resolution loses
// nothing a person could see and quarters the blur's work.
const glassScale = 2

// lightDir is where the rim light comes from: above, a little left, as
// a unit vector in device pixels (y down).
var lightDir = [2]float64{-0.34, -0.94}

// glassClock fills a clock label's glyphs with the backdrop as liquid
// frosted glass: the background under the digits blurred, saturated and
// lifted toward white, bent along each glyph's rim like a lens, under a
// rim light from above and over a soft shadow that keeps the digits
// legible on light and dark backgrounds. It draws through an area over
// the label, which keeps the text, the layout and the accessible name.
//
// The frost depends on where the clock sits over the background, not on
// what it reads, so the clock's minute change reshapes only the glyph
// mask and the composite over it.
type glassClock struct {
	label *widget.Label
	bg    *Backdrop
	// under is the widget the backdrop covers, the box its cover
	// placement is worked out in.
	under Placed
	frost *frostField
	shown *glassFace
}

// frostField is the frosted backdrop over a device region: rgba at
// 1/glassScale of the device resolution, its pixel (0, 0) centered on
// device pixel (region.X + glassScale/2, region.Y + glassScale/2).
type frostField struct {
	key    frostKey
	region render.Rect
	rgba   *image.RGBA
}

type frostKey struct {
	dev, box render.Rect
}

// glassFace is one composited clock: img, at device origin at, still
// once made so a GPU renderer uploads it once; nil pixels when the
// clock shows no glyph.
type glassFace struct {
	key   faceKey
	frost *frostField
	img   render.StillImage
	at    image.Point
}

type faceKey struct {
	text       string
	dev        render.Rect
	num, denom int
}

// newGlassClock fills label with glass over bg, drawn under the
// widget under covers; the returned widget takes the label's place.
func newGlassClock(label *widget.Label, bg *Backdrop, under Placed) (*glassClock, widget.Widget) {
	g := &glassClock{label: label, bg: bg, under: under}
	label.AddClass(glassInk)
	area := widget.NewDrawingArea(0, 0)
	area.OnDraw = g.draw
	return g, widget.NewOverlay().Append(label).Append(area)
}

func (g *glassClock) draw(cv *render.Canvas, r render.Rect) {
	if !g.label.Visible() || g.label.Text() == "" {
		return
	}
	dev := cv.MapRect(r)
	if dev.Empty() {
		return
	}
	num, denom := cv.DeviceScale()
	fk := frostKey{dev: dev, box: cv.MapRect(g.under.Bounds())}
	if g.frost == nil || g.frost.key != fk {
		g.frost = newFrostField(g.bg.Image.RGBA, fk)
	}
	key := faceKey{text: g.label.Text(), dev: dev, num: num, denom: denom}
	if g.shown == nil || g.shown.key != key || g.shown.frost != g.frost {
		g.shown = g.compose(key, r)
	}
	if g.shown.img.RGBA != nil {
		cv.DrawImageDevice(g.shown.img, g.shown.at.X, g.shown.at.Y)
	}
}

// compose draws the label's glyphs as a coverage mask and fills them
// with the frost.
func (g *glassClock) compose(key faceKey, r render.Rect) *glassFace {
	dev := key.dev
	pix := make([]byte, render.Stride(dev.W)*dev.H)
	g.label.PaintGlyphs(render.NewScaled(pix, render.Stride(dev.W), dev.W, dev.H, key.num, key.denom), render.RGB(255, 255, 255), -r.X, -r.Y)
	f := &glassFace{key: key, frost: g.frost}
	f.fill(maskOf(pix, dev))
	return f
}

// glyphMask is coverage in [0, 1] over a device rect.
type glyphMask struct {
	rect render.Rect
	a    []float32
}

// maskOf reads the alpha of an ARGB8888 raster covering dev.
func maskOf(pix []byte, dev render.Rect) glyphMask {
	m := glyphMask{rect: dev, a: make([]float32, dev.W*dev.H)}
	for i := range m.a {
		m.a[i] = float32(pix[4*i+3]) / 255
	}
	return m
}

// ink is the smallest rect, in m's device space, holding every covered
// pixel; empty when nothing is.
func (m glyphMask) ink() render.Rect {
	x0, y0, x1, y1 := m.rect.W, m.rect.H, -1, -1
	for y := range m.rect.H {
		row := m.a[y*m.rect.W : (y+1)*m.rect.W]
		for x, v := range row {
			if v > 0 {
				x0, x1 = min(x0, x), max(x1, x)
				y0, y1 = min(y0, y), max(y1, y)
			}
		}
	}
	if x1 < 0 {
		return render.Rect{}
	}
	return render.Rect{X: m.rect.X + x0, Y: m.rect.Y + y0, W: x1 - x0 + 1, H: y1 - y0 + 1}
}

// crop is m over r, zero where r leaves m.
func (m glyphMask) crop(r render.Rect) glyphMask {
	out := glyphMask{rect: r, a: make([]float32, r.W*r.H)}
	in := r.Intersect(m.rect)
	for y := in.Y; y < in.Y+in.H; y++ {
		src := m.a[(y-m.rect.Y)*m.rect.W+in.X-m.rect.X:]
		copy(out.a[(y-r.Y)*r.W+in.X-r.X:], src[:in.W])
	}
	return out
}

// at is the coverage at device pixel (x, y), zero outside.
func (m glyphMask) at(x, y int) float32 {
	x, y = x-m.rect.X, y-m.rect.Y
	if x < 0 || y < 0 || x >= m.rect.W || y >= m.rect.H {
		return 0
	}
	return m.a[y*m.rect.W+x]
}

// fill composites the glass over the glyphs of mask: the face's image
// covers the glyphs' ink grown by the shadow's reach, inside the
// clock's own device rect so nothing lands where the clock does not
// repaint.
func (f *glassFace) fill(mask glyphMask) {
	ink := mask.ink()
	if ink.Empty() {
		return
	}
	em := float64(f.key.dev.H)
	reach := int(math.Ceil(float64(3*shadowSigma*em) + float64(shadowDrop*em)))
	out := (render.Insets{Top: reach, Right: reach, Bottom: reach, Left: reach}).Grow(ink).Intersect(f.key.dev)
	m := mask.crop(out)
	bezel := gaussian(m, bezelSigma*em)
	rimWidth := max(rimSigma*em, 0.7)
	rim := gaussian(m, rimWidth)
	shadow := gaussian(m, shadowSigma*em)
	drop := int(math.Round(shadowDrop * em))
	bend := float32(bendReach * em)
	rimGain := float32(rimWidth * math.Sqrt(2*math.Pi))
	img := image.NewRGBA(image.Rect(0, 0, out.W, out.H))
	for y := range out.H {
		for x := range out.W {
			dx, dy := out.X+x, out.Y+y
			cov := m.at(dx, dy)
			sh := shadowAlpha * shadow.at(dx, dy-drop) * (1 - cov)
			var c [4]float32
			if cov > 0 {
				gx, gy := grad(bezel, dx, dy)
				col := f.frost.sample(float32(dx)+0.5+float32(bend*gx), float32(dy)+0.5+float32(bend*gy))
				rx, ry := grad(rim, dx, dy)
				c = glassInkAt(col, [2]float32{-rx * rimGain, -ry * rimGain})
				for i := range c {
					c[i] *= cov * glassOpacity
				}
			}
			o := img.PixOffset(x, y)
			for i := range 3 {
				img.Pix[o+i] = unit8(c[i] * 255)
			}
			img.Pix[o+3] = unit8((c[3] + float32(sh*(1-c[3]))) * 255)
		}
	}
	f.img, f.at = render.StillImage{RGBA: img}, image.Pt(out.X, out.Y)
}

// glassInkAt is the glass over one glyph pixel, straight color in [0, 1]
// and opaque: the frost, lifted by the rim light on the edges facing
// lightDir and shaded on those facing away, with an even sheen all
// round. edge is the rim's outward normal scaled by how much of the
// edge band the pixel is in.
func glassInkAt(frost [3]float32, edge [2]float32) [4]float32 {
	band := min(float32(math.Sqrt(float64(float32(edge[0]*edge[0])+float32(edge[1]*edge[1])))), 1)
	facing := float32(0)
	if band > 0 {
		facing = (float32(edge[0]*float32(lightDir[0])) + float32(edge[1]*float32(lightDir[1]))) / max(band, 1e-6)
		facing = max(min(facing, 1), -1)
	}
	light := float32(band * (rimAmbient + float32(rimLight*max(facing, 0))))
	shade := float32(band * rimShade * max(-facing, 0))
	var c [4]float32
	for i, v := range frost {
		c[i] = float32(float32(v*float32(1-light))*float32(1-shade)) + light
	}
	c[3] = 1
	return c
}

// grad is the gradient of m at (x, y), pointing toward more coverage,
// in coverage per pixel.
func grad(m glyphMask, x, y int) (float32, float32) {
	return (m.at(x+1, y) - m.at(x-1, y)) / 2, (m.at(x, y+1) - m.at(x, y-1)) / 2
}

func unit8(v float32) uint8 {
	return uint8(min(max(v, 0), 255) + 0.5)
}

// gaussian is m blurred by a gaussian of standard deviation sigma device
// pixels, three box passes per axis as the backdrop's blur, zero
// coverage assumed past m's edges.
func gaussian(m glyphMask, sigma float64) glyphMask {
	out := glyphMask{rect: m.rect, a: append([]float32(nil), m.a...)}
	if sigma <= 0 {
		return out
	}
	w, h := m.rect.W, m.rect.H
	tmp := make([]float32, len(out.a))
	for _, r := range boxRadii(sigma, 3) {
		boxLines(out.a, tmp, w, h, 1, w, r)
		boxLines(tmp, out.a, h, w, w, 1, r)
	}
	return out
}

// boxLines box-blurs src into dst along lines of n samples step apart,
// lines of them across apart (count of them), radius r, zero past the
// ends.
func boxLines(src, dst []float32, n, count, step, across, r int) {
	inv := 1 / float32(2*r+1)
	for l := range count {
		base := l * across
		var sum float32
		for i := 0; i <= min(r, n-1); i++ {
			sum += src[base+i*step]
		}
		for i := range n {
			dst[base+i*step] = sum * inv
			if j := i + r + 1; j < n {
				sum += src[base+j*step]
			}
			if j := i - r; j >= 0 {
				sum -= src[base+j*step]
			}
		}
	}
}

// newFrostField frosts the backdrop under key.dev, grown by how far the
// rim bends it, as the background shows it in key.box (cover,
// centered): resampled to 1/glassScale of the device resolution,
// blurred, saturated and lifted toward white.
func newFrostField(bg *image.RGBA, key frostKey) *frostField {
	em := float64(key.dev.H)
	grow := int(math.Ceil(bendReach*em)) + glassScale
	region := (render.Insets{Top: grow, Right: grow, Bottom: grow, Left: grow}).Grow(key.dev)
	w, h := (region.W+glassScale-1)/glassScale, (region.H+glassScale-1)/glassScale
	f := &frostField{key: key, region: region, rgba: image.NewRGBA(image.Rect(0, 0, w, h))}
	b := bg.Bounds()
	src, dst := render.PlaceImage(b.Dx(), b.Dy(), key.box.W, key.box.H, render.ImageCover, render.ImageCentered)
	if dst.Empty() {
		return f
	}
	sx := float64(src.Dx()) / float64(dst.W)
	sy := float64(src.Dy()) / float64(dst.H)
	ox, oy := float64(key.box.X+dst.X), float64(key.box.Y+dst.Y)
	for y := range h {
		v := float64(src.Min.Y) + float64((float64(region.Y+y*glassScale)+float64(glassScale)/2-oy)*sy) - 0.5
		for x := range w {
			u := float64(src.Min.X) + float64((float64(region.X+x*glassScale)+float64(glassScale)/2-ox)*sx) - 0.5
			c := bilinear(bg, src, u, v)
			o := f.rgba.PixOffset(x, y)
			for i := range 4 {
				f.rgba.Pix[o+i] = unit8(c[i])
			}
		}
	}
	blur(f.rgba, make([]uint8, len(f.rgba.Pix)), frostSigma*em/glassScale)
	for i := 0; i < len(f.rgba.Pix); i += 4 {
		frosted := frostTone([3]float32{float32(f.rgba.Pix[i]) / 255, float32(f.rgba.Pix[i+1]) / 255, float32(f.rgba.Pix[i+2]) / 255})
		for c := range 3 {
			f.rgba.Pix[i+c] = unit8(frosted[c] * 255)
		}
		f.rgba.Pix[i+3] = 255
	}
	return f
}

// frostTone saturates c by frostSat about its luma and lifts it
// frostWhite of the way to white.
func frostTone(c [3]float32) [3]float32 {
	luma := float32(0.2126*c[0]) + float32(0.7152*c[1]) + float32(0.0722*c[2])
	var out [3]float32
	for i, v := range c {
		s := min(max(luma+float32(frostSat*(v-luma)), 0), 1)
		out[i] = s + float32(frostWhite*(1-s))
	}
	return out
}

// bilinear samples the premultiplied img at (u, v) in its pixels,
// clamped to within, in 0..255.
func bilinear(img *image.RGBA, within image.Rectangle, u, v float64) [4]float32 {
	u = min(max(u, float64(within.Min.X)), float64(within.Max.X-1))
	v = min(max(v, float64(within.Min.Y)), float64(within.Max.Y-1))
	x0, y0 := int(u), int(v)
	x1, y1 := min(x0+1, within.Max.X-1), min(y0+1, within.Max.Y-1)
	fx, fy := float32(u-float64(x0)), float32(v-float64(y0))
	var c [4]float32
	for i := range 4 {
		p00 := float32(img.Pix[img.PixOffset(x0, y0)+i])
		p10 := float32(img.Pix[img.PixOffset(x1, y0)+i])
		p01 := float32(img.Pix[img.PixOffset(x0, y1)+i])
		p11 := float32(img.Pix[img.PixOffset(x1, y1)+i])
		c[i] = float32((float32(p00*(1-fx))+float32(p10*fx))*(1-fy)) + float32((float32(p01*(1-fx))+float32(p11*fx))*fy)
	}
	return c
}

// sample is the frost's straight color in [0, 1] at device point (x, y),
// clamped to the field.
func (f *frostField) sample(x, y float32) [3]float32 {
	u := float64(float64(x-float32(f.region.X))/glassScale) - 0.5
	v := float64(float64(y-float32(f.region.Y))/glassScale) - 0.5
	c := bilinear(f.rgba, f.rgba.Rect, u, v)
	return [3]float32{c[0] / 255, c[1] / 255, c[2] / 255}
}
