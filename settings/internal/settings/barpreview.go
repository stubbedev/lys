package settings

import (
	"fmt"
	"math"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/barview"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
	"github.com/stubbedev/lys/ui/widget/css"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	barPreviewScreenH         = 150
	barPreviewVerticalScreenH = 260
)

// livePreview is a lys-shell surface drawn from the config the window
// edits, redrawn whenever that config changes. It only reads the config.
type livePreview struct {
	*widget.Box
	k    *kit
	draw func(k *kit, cfg *config.Config, font render.Font) widget.Widget
}

func newLivePreview(k *kit, name string, draw func(k *kit, cfg *config.Config, font render.Font) widget.Widget) *livePreview {
	p := &livePreview{Box: widget.NewBox(widget.Column, 0, 0), k: k, draw: draw}
	p.AddClass("settings-live-preview")
	p.SetAccessibleName(locale.T().Get(name))
	p.configChanged()
	return p
}

func (p *livePreview) configChanged() {
	p.Clear()
	cfg := p.k.store.svc.Config()
	font, err := p.k.barFont(cfg)
	if err != nil {
		p.Append(previewFailed(p.k, err), false)
		return
	}
	p.Append(p.draw(p.k, cfg, font), false)
}

// previewState is the inline state a preview shows in place of, or
// beside, what it draws.
func previewState(k *kit) *inlineState {
	s := k.inlineState()
	s.AddClass("settings-preview-state")
	return s
}

// previewFailed says a preview cannot be drawn, err why.
func previewFailed(k *kit, err error) *inlineState {
	s := previewState(k)
	s.Error(locale.T().Get("settings-preview-failed"), err, nil)
	return s
}

func drawBarOnScreen(k *kit, cfg *config.Config, font render.Font, monitor string) widget.Widget {
	screenH := barPreviewScreenH
	if cfg.Bar.Location.IsVertical() {
		screenH = barPreviewVerticalScreenH
	}
	shown := wallpaperScreen(k, screenH)
	bar, ok := barview.Preview(cfg, font, monitor, sessionCompositor())
	if !ok {
		none := previewState(k)
		none.Empty("", locale.T().Get("settings-bar-preview-none"))
		shown.AppendAligned(none, widget.AlignCenter, widget.AlignCenter)
		return shown
	}
	h, v := edgeAlign(bar.Edge())
	shown.AppendAligned(bar, h, v)
	return newFitWidth(shown)
}

// fitWidth draws its child at the child's natural width, scaled down as
// a whole to the width it is given where that is narrower, so a bar
// wider than the column shows whole, smaller, instead of cut at the
// column's edge.
type fitWidth struct {
	*widget.Box
	child widget.Widget
}

func newFitWidth(child widget.Widget) *fitWidth {
	f := &fitWidth{Box: widget.NewBox(widget.Column, 0, 0), child: child}
	f.Append(child, true)
	return f
}

// scaleFor is how far the child scales down to fit w.
func (f *fitWidth) scaleFor(w int) float64 {
	nat := f.child.Measure(widget.Row, -1).Nat
	if w <= 0 || nat <= w {
		return 1
	}
	return float64(w) / float64(nat)
}

// Measure asks for no width of its own, and the child's height at its
// natural width scaled as the width given scales it.
func (f *fitWidth) Measure(axis widget.Axis, forSize int) widget.Sizes {
	if axis == widget.Row {
		return widget.Sizes{Nat: f.child.Measure(widget.Row, -1).Nat}
	}
	nat := f.child.Measure(widget.Row, -1).Nat
	h := f.child.Measure(widget.Column, nat).Nat
	return widget.FixedSizes(int(math.Ceil(float64(h) * f.scaleFor(forSize))))
}

// Arrange lays itself and the child out at the child's natural width,
// and scales the drawing down to r about its top left corner.
func (f *fitWidth) Arrange(r render.Rect) {
	s := f.scaleFor(r.W)
	f.SetInlineStyle(fmt.Sprintf("transform-origin: 0 0; transform: scale(%.4f);", s))
	w := max(r.W, f.child.Measure(widget.Row, -1).Nat)
	f.Box.Arrange(render.Rect{X: r.X, Y: r.Y, W: w, H: int(math.Round(float64(r.H) / s))})
}

// wallpaperScreen is a small screen showing the backdrop, h pixels
// tall, for a surface to be placed over.
func wallpaperScreen(k *kit, h int) *widget.Overlay {
	screen := widget.NewDrawingArea(0, h)
	bd := k.backdropFor(screen)
	screen.OnDraw = func(cv *render.Canvas, r render.Rect) {
		previewScreen(cv, r, func(cv *render.Canvas) { bd.paint(cv, r) })
	}
	return widget.NewOverlay().Append(screen)
}

// offScreen is the wallpaper screen of a surface that is off, the
// caption the message key gives saying so on a plate of the card's
// color, so it reads whatever the wallpaper behind it.
func offScreen(k *kit, h int, key string) *widget.Overlay {
	caption := k.label(locale.T().Get(key), "settings-preview-state", css.Caption)
	screen := widget.NewDrawingArea(0, h)
	bd := k.backdropFor(screen)
	screen.OnDraw = func(cv *render.Canvas, r render.Rect) {
		previewScreen(cv, r, func(cv *render.Canvas) { bd.paint(cv, r) })
		th := widget.Current()
		plate := render.Insets{Top: th.Padding / 2, Right: th.Padding, Bottom: th.Padding / 2, Left: th.Padding}.Grow(caption.Bounds())
		cv.RoundedRect(plate, th.Radius, th.CardColor())
	}
	return widget.NewOverlay().Append(screen).AppendAligned(caption, widget.AlignCenter, widget.AlignCenter)
}

// previewScreen paints a preview's screen into r: what paint draws,
// clipped to r with the corners the theme rounds a card with, inside
// its border, so a preview sits among the cards as one of them.
func previewScreen(cv *render.Canvas, r render.Rect, paint func(cv *render.Canvas)) {
	maskedScreen(cv, r, render.UniformCorners(widget.Current().Radius), paint)
}

func edgeAlign(edge widget.BarEdge) (h, v widget.Align) {
	switch edge {
	case widget.BarBottom:
		return widget.AlignFill, widget.AlignEnd
	case widget.BarLeft:
		return widget.AlignStart, widget.AlignFill
	case widget.BarRight:
		return widget.AlignEnd, widget.AlignFill
	}
	return widget.AlignFill, widget.AlignStart
}

func overflowing(w widget.Widget) *widget.Scroll {
	s := widget.NewScroll(w)
	s.FillX, s.FillY, s.PropagateNaturalHeight = true, true, true
	return s
}

// moduleSample is module's button alone on a strip of the bar under
// cfg, in the first of states it is for. The strip stands on no screen,
// so the bar's insets from a screen's edges leave it spanning the
// column it sits in.
func moduleSample(cfg *config.Config, font render.Font, module config.BarModule, states ...barview.State) widget.Widget {
	strip := *cfg
	strip.Bar.InsetEdge, strip.Bar.InsetEnds = config.Scale(0), config.Scale(0)
	return barview.SampleBar(&strip, font, config.BarLayout{Monitor: barview.AllMonitors, Show: true, Center: []config.BarItem{{Module: module}}}, sessionCompositor(), states...)
}

// barItemPreview is the button of the module that module names, on a
// strip of the bar as the config sets it; empty while it names none.
func barItemPreview(k *kit, module barModuleOf) *livePreview {
	return newLivePreview(k, "settings-bar-item-preview", func(_ *kit, cfg *config.Config, font render.Font) widget.Widget {
		m, ok := module()
		if !ok {
			return widget.NewBox(widget.Column, 0, 0)
		}
		return moduleSample(cfg, font, m)
	})
}

// moduleSection is the config section of a built-in bar module.
func moduleSection(m config.BarModule) string { return "modules." + string(m) }
