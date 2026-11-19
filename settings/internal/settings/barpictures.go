package settings

import (
	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

const (
	pictureBarH   = 12
	pictureInset  = 4
	pictureShadow = 6
	pictureDot    = 6
)

func pictureBar(screen render.Rect, inset int) render.Rect {
	return render.Rect{X: screen.X + inset, Y: screen.Y + inset, W: screen.W - 2*inset, H: pictureBarH}
}

func barOnScreen(paint func(cv *render.Canvas, screen render.Rect)) picture {
	return drawn(func(cv *render.Canvas, r render.Rect) {
		screenFrame(cv, r, func(s render.Rect) {
			photo(cv, s)
			paint(cv, s)
		})
	})
}

var shadowPictures = map[string]picture{
	string(config.ShadowNone): barOnScreen(func(cv *render.Canvas, s render.Rect) {
		cv.FillRect(pictureBar(s, 0), previewDark)
	}),
	string(config.ShadowDrop): barOnScreen(func(cv *render.Canvas, s render.Rect) {
		bar := pictureBar(s, 0)
		cv.Shadow(bar, 0, pictureShadow, render.RGBA(0, 0, 0, 0xa0))
		cv.FillRect(bar, previewDark)
	}),
	string(config.ShadowFloating): barOnScreen(func(cv *render.Canvas, s render.Rect) {
		bar := pictureBar(s, pictureInset)
		cv.Shadow(render.Rect{X: bar.X, Y: bar.Y + pictureInset/2, W: bar.W, H: bar.H}, bar.H/2, 2*pictureShadow, render.RGBA(0, 0, 0, 0xa0))
		cv.RoundedRect(bar, bar.H/2, previewDark)
	}),
}

func roundedPanel(level config.RoundingLevel) picture {
	return barOnScreen(func(cv *render.Canvas, s render.Rect) {
		panel := render.Rect{X: s.X + pictureInset, Y: s.Y + pictureInset, W: s.W - 2*pictureInset, H: s.H - 2*pictureInset}
		cv.RoundedRect(panel, min(styling.RoundingRadiusPx(level, 1), panel.H/2), previewDark)
	})
}

var roundingPictures = func() map[string]picture {
	out := map[string]picture{}
	for _, level := range []config.RoundingLevel{config.RoundingDefault, config.RoundingNone, config.RoundingSm, config.RoundingMd, config.RoundingLg, config.RoundingFull} {
		out[string(level)] = roundedPanel(level)
	}
	return out
}()

func sampleButton(cv *render.Canvas, r render.Rect, variant config.BarButtonVariant, iconEnd bool) {
	th := widget.Current()
	h := pictureBarH + pictureInset
	button := render.Rect{X: r.X + pictureInset, Y: r.Y + (r.H-h)/2, W: r.W - 2*pictureInset, H: h}
	cv.RoundedRect(button, h/2, th.MutedColor())
	icon := render.Rect{X: button.X, Y: button.Y, W: h, H: h}
	if iconEnd {
		icon.X = button.X + button.W - h
	}
	switch variant {
	case config.ButtonBlockPrefix:
		cv.RoundedRect(icon, h/2, th.Accent)
	case config.ButtonIconSquare:
		icon = render.Rect{X: icon.X + 1, Y: icon.Y + 1, W: h - 2, H: h - 2}
		cv.RoundedRect(icon, tileRadius/2, th.Accent)
	}
	dot := render.Rect{X: icon.X + (h-pictureDot)/2, Y: icon.Y + (h-pictureDot)/2, W: pictureDot, H: pictureDot}
	cv.RoundedRect(dot, pictureDot/2, th.Text)
	label := render.Rect{X: button.X + h + pictureInset, Y: button.Y + h/2 - 1, W: button.W - h - 3*pictureInset, H: 3}
	if iconEnd {
		label.X = button.X + 2*pictureInset
	}
	cv.RoundedRect(label, 1, th.Text)
}

func buttonPicture(variant config.BarButtonVariant, iconEnd bool) picture {
	return drawn(func(cv *render.Canvas, r render.Rect) { sampleButton(cv, r, variant, iconEnd) })
}

var buttonVariantPictures = map[string]picture{
	string(config.ButtonBasic):       buttonPicture(config.ButtonBasic, false),
	string(config.ButtonBlockPrefix): buttonPicture(config.ButtonBlockPrefix, false),
	string(config.ButtonIconSquare):  buttonPicture(config.ButtonIconSquare, false),
}

var iconPositionPictures = map[string]picture{
	string(config.IconStart): buttonPicture(config.ButtonBlockPrefix, false),
	string(config.IconEnd):   buttonPicture(config.ButtonBlockPrefix, true),
}
