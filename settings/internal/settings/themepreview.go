package settings

import (
	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const themePreviewH = 180

// themePreview is a small desktop in the colors the draft gives
// lys-shell's own palette.
func themePreview(k *kit) widget.Widget {
	return newLivePreview(k, "settings-theme-preview", drawThemeDesktop)
}

// themeDesktop is the mini desktop drawn in colors, paletteKeys' order.
type themeDesktop struct {
	*widget.DrawingArea
	colors []render.Color
}

func drawThemeDesktop(k *kit, cfg *config.Config, _ render.Font) widget.Widget {
	t := locale.T()
	if cfg.Styling.ColorExtractor.ThemeProvider != config.ThemeProviderLysShell {
		return k.note(t.Get("settings-theme-preview-generated"), "settings-preview-state")
	}
	colors, err := paletteColors(cfg.Styling.ActivePalette())
	if err != nil {
		return previewFailed(k, err)
	}
	d := &themeDesktop{DrawingArea: widget.NewDrawingArea(0, themePreviewH), colors: colors}
	d.AddClass("settings-theme-preview")
	desktop := miniDesktop(colors)
	d.OnDraw = func(cv *render.Canvas, r render.Rect) {
		previewScreen(cv, r, func(cv *render.Canvas) { desktop(cv, r) })
	}
	return d
}
