package settings

import (
	"log"
	"slices"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/i18n"
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
		p.Append(p.state("settings-preview-failed", i18n.Str("reason", err.Error())), false)
		return
	}
	p.Append(p.draw(p.k, cfg, font), false)
}

func (p *livePreview) state(key string, args ...i18n.Arg) *widget.Label {
	l := wrapped(p.k.label(locale.T().Get(key, args...), "settings-preview-state", css.Caption, css.DimLabel))
	l.SetAlignment(render.AlignCenter)
	return l
}

// barPreview is the whole bar on a small wallpaper, at the edge the
// config puts it: lys-shell's own bar with every module holding sample
// state.
func barPreview(k *kit) widget.Widget {
	return newLivePreview(k, "settings-bar-preview", drawBarOnScreen)
}

func drawBarOnScreen(k *kit, cfg *config.Config, font render.Font) widget.Widget {
	screenH := barPreviewScreenH
	if cfg.Bar.Location.IsVertical() {
		screenH = barPreviewVerticalScreenH
	}
	screen := widget.NewDrawingArea(0, screenH)
	screen.OnDraw = func(cv *render.Canvas, r render.Rect) {
		screenFrame(cv, r, func(s render.Rect) { photo(cv, s) })
	}
	shown := widget.NewOverlay().Append(screen)
	bar, ok := barview.Preview(cfg, font, barview.AllMonitors)
	if !ok {
		shown.AppendAligned(k.label(locale.T().Get("settings-bar-preview-none"), "settings-preview-state", css.Caption), widget.AlignCenter, widget.AlignCenter)
		return shown
	}
	h, v := edgeAlign(bar.Edge())
	shown.AppendAligned(bar, h, v)
	return overflowing(shown)
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

// barItemPreview is one module's button on a strip of the bar, as the
// module's page sets it.
func barItemPreview(module config.BarModule) func(k *kit) widget.Widget {
	return func(k *kit) widget.Widget {
		return newLivePreview(k, "settings-bar-item-preview", func(_ *kit, cfg *config.Config, font render.Font) widget.Widget {
			return barview.SampleBar(cfg, font, config.BarLayout{Monitor: barview.AllMonitors, Show: true, Center: []config.BarItem{{Module: module}}})
		})
	}
}

// moduleOfSection is the bar module whose config section path names.
func moduleOfSection(path string) config.BarModule {
	name, ok := strings.CutPrefix(path, "modules.")
	if !ok || !slices.Contains(config.BuiltinModules, config.BarModule(name)) {
		log.Panicf("settings: %s is no bar module's section", path)
	}
	return config.BarModule(name)
}
