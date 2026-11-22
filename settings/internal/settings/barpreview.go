package settings

import (
	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/barview"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

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
	shown := wallpaperScreen(screenH)
	bar, ok := barview.Preview(cfg, font, monitor, sessionCompositor())
	if !ok {
		none := previewState(k)
		none.Empty("", locale.T().Get("settings-bar-preview-none"))
		shown.AppendAligned(none, widget.AlignCenter, widget.AlignCenter)
		return shown
	}
	h, v := edgeAlign(bar.Edge())
	shown.AppendAligned(bar, h, v)
	return overflowing(shown)
}

// wallpaperScreen is a small screen showing a wallpaper, h pixels tall,
// for a surface to be placed over.
func wallpaperScreen(h int) *widget.Overlay {
	screen := widget.NewDrawingArea(0, h)
	screen.OnDraw = func(cv *render.Canvas, r render.Rect) {
		screenFrame(cv, r, func(s render.Rect) { photo(cv, s) })
	}
	return widget.NewOverlay().Append(screen)
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
// cfg, in the first of states it is for.
func moduleSample(cfg *config.Config, font render.Font, module config.BarModule, states ...barview.State) widget.Widget {
	return barview.SampleBar(cfg, font, config.BarLayout{Monitor: barview.AllMonitors, Show: true, Center: []config.BarItem{{Module: module}}}, sessionCompositor(), states...)
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
