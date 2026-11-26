package settings

import (
	"image"
	"log"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

const (
	backdropAngle       = 135
	backdropAccentAlpha = 0x59
)

type invalidator interface{ Invalidate() }

// previewBackdrop is the one backdrop the window's previews stand on:
// the wallpaper lys-shell's config names, decoded off the loop once per
// path and held scaled down, else a calm fill drawn from the theme.
type previewBackdrop struct {
	k        *kit
	want     string
	img      image.Image
	painters map[invalidator]struct{}
}

func (k *kit) previewBackdrop() *previewBackdrop {
	if k.backdrop == nil {
		k.backdrop = &previewBackdrop{k: k, painters: map[invalidator]struct{}{}}
	}
	return k.backdrop
}

// screenBackdrop is the window's backdrop as painter paints it: painter is
// painted again once the wallpaper it showed is loaded or replaced.
type screenBackdrop struct {
	src     *previewBackdrop
	painter invalidator
}

func (k *kit) backdropFor(painter invalidator) screenBackdrop {
	return screenBackdrop{src: k.previewBackdrop(), painter: painter}
}

var themeBackdropSize = image.Point{X: 4, Y: 3}

func (b screenBackdrop) paint(cv *render.Canvas, r render.Rect) { b.paintFit(cv, r, config.FitFill) }

// paintFit paints the backdrop over screen as lys-shell fits a
// wallpaper there, the theme's fill standing in for a 4:3 image.
func (b screenBackdrop) paintFit(cv *render.Canvas, screen render.Rect, fit config.FitMode) {
	img := b.src.image(b.painter)
	size := themeBackdropSize
	if img != nil {
		size = img.Bounds().Size()
	}
	dst := screen
	switch fit {
	case config.FitFill:
		if img != nil {
			cv.DrawImageCover(img, screen, render.Corners{})
			return
		}
	case config.FitFit:
		dst = contained(size, screen)
	case config.FitCenter:
		dst = contained(size, render.Rect{X: screen.X + screen.W/4, Y: screen.Y + screen.H/4, W: screen.W / 2, H: screen.H / 2})
	case config.FitStretch:
	}
	if img != nil {
		cv.DrawImageRect(img, dst)
		return
	}
	paintThemeBackdrop(cv, dst)
}

// contained is the largest rect of size's aspect centred in r.
func contained(size image.Point, r render.Rect) render.Rect {
	w, h := r.W, r.W*size.Y/size.X
	if h > r.H {
		w, h = r.H*size.X/size.Y, r.H
	}
	return render.Rect{X: r.X + (r.W-w)/2, Y: r.Y + (r.H-h)/2, W: w, H: h}
}

// image is the wallpaper to paint, nil while there is none to show; a
// path not held yet starts loading, the last image showing meanwhile.
func (p *previewBackdrop) image(painter invalidator) image.Image {
	p.painters[painter] = struct{}{}
	path := p.k.store.svc.Config().Wallpaper.Wallpaper
	if path == p.want {
		return p.img
	}
	p.want = path
	if path == "" {
		p.img = nil
		return nil
	}
	async(p.k, func() wallpaperThumb { return decodeThumb(path) }, func(th wallpaperThumb) { p.loaded(path, th) })
	return p.img
}

func (p *previewBackdrop) loaded(path string, th wallpaperThumb) {
	if path != p.want {
		return
	}
	if th.err != nil {
		log.Printf("settings: preview backdrop %s: %v", path, th.err)
	}
	p.img = th.img
	for w := range p.painters {
		w.Invalidate()
	}
	clear(p.painters)
}

// paintThemeBackdrop is the backdrop without a wallpaper: the theme's
// muted surface under a soft wash of its accent.
func paintThemeBackdrop(cv *render.Canvas, r render.Rect) {
	th := widget.Current()
	cv.FillRect(r, th.MutedColor())
	a := th.Accent.Straight()
	cv.PaintGradient(r, render.Corners{}, render.Linear(backdropAngle,
		render.GradientStop{Pos: 0, Color: render.RGBA(a[0], a[1], a[2], 0)},
		render.GradientStop{Pos: 1, Color: render.RGBA(a[0], a[1], a[2], backdropAccentAlpha)}))
}

// maskedScreen paints a preview's screen into r: what paint draws on a
// layer masked to r with radii, inside its border, so nothing paint
// draws reaches past the rounded corners.
func maskedScreen(cv *render.Canvas, r render.Rect, radii render.Corners, paint func(cv *render.Canvas)) {
	region := cv.MapRect(r).Intersect(cv.Rect())
	if region.Empty() {
		return
	}
	layer := cv.ScratchLayer(region)
	defer layer.Release()
	paint(layer.Canvas())
	shape := &render.Path{}
	render.RoundedRectPath(shape, r, radii)
	layer.ApplyPathMask(shape, region)
	cv.DrawLayer(layer, region, render.BlendNormal)
	cv.RoundedBorder(r, radii, render.UniformInsets(1), widget.Current().Border)
}
