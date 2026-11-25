package settings

import (
	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/osdview"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/settings/internal/locale"
)

const (
	osdPreviewScreenH = 220
	osdPreviewPercent = 70
)

// osdPreview is lys-shell's OSD on a small wallpaper, where
// osd.position puts it, showing the toast Show OSD flashes.
func osdPreview(k *kit) widget.Widget {
	return newLivePreview(k, "settings-osd-preview", drawOsdOnScreen)
}

func drawOsdOnScreen(k *kit, cfg *config.Config, font render.Font) widget.Widget {
	if !cfg.Osd.Enabled {
		return offScreen(k, osdPreviewScreenH, "settings-osd-preview-off")
	}
	shown := wallpaperScreen(osdPreviewScreenH)
	t := osdSampleToast()
	card := osdview.Preview(cfg, font, osdview.Toast(t.label, t.icon).WithPercent(float64(t.percentage)))
	inset := widget.NewBox(widget.Column, 0, widget.Current().Padding)
	inset.Append(card.Root(), false)
	h, v := widgets.Placement(cfg.Osd.Position)
	shown.AppendAligned(inset, h, v)
	return overflowing(shown)
}

func osdSampleToast() shellToast {
	return shellToast{label: locale.T().Get("settings-osd-preview-label"), icon: osdview.VolumeIcon(osdPreviewPercent, false), percentage: osdPreviewPercent}
}
