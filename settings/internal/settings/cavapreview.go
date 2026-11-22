package settings

import (
	"fmt"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/cavaview"
	"github.com/stubbedev/lys/desktop/styling"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

const (
	cavaPreviewScreenH = 120
	cavaPreviewFall    = 0.25
)

// cavaPreview is lys-shell's visualizer on a small wallpaper, drawn from
// the draft at a sample frame.
func cavaPreview(k *kit) previewWidget {
	return newLivePreview(k, "settings-cava-preview", drawCavaOnScreen)
}

func drawCavaOnScreen(k *kit, cfg *config.Config, _ render.Font) widget.Widget {
	shown := wallpaperScreen(cavaPreviewScreenH)
	spectrum, err := cavaSampleSpectrum(cfg, k.palette())
	if err != nil {
		shown.AppendAligned(previewFailed(k, err), widget.AlignCenter, widget.AlignCenter)
		return shown
	}
	shown.AppendAligned(spectrum, widget.AlignCenter, widget.AlignCenter)
	return overflowing(shown)
}

// cavaSampleSpectrum is the visualizer cfg draws, painted in its color
// from p, holding the sample levels; in the peaks style the bars have
// just fallen, so their caps show.
func cavaSampleSpectrum(cfg *config.Config, p styling.Palette) (*cavaview.Spectrum, error) {
	bars, _ := cavaview.Bars(cfg.Cava)
	color, ok := styling.ResolveColor(cfg.Cava.Color, &p)
	if !ok {
		return nil, fmt.Errorf("the visualizer's color %s does not resolve", cfg.Cava.Color.ToCSS())
	}
	s := cavaview.NewSpectrum(cfg.Cava, bars, color, float64(cfg.Bar.Scale), cfg.Bar.Location.IsVertical())
	frame := cavaSampleFrame(bars)
	if cfg.Cava.Style == config.CavaStylePeaks {
		raised := make([]float64, bars)
		for i, v := range frame {
			raised[i] = min(v+cavaPreviewFall, 1)
		}
		s.SetFrame(raised)
	}
	s.SetFrame(frame)
	return s, nil
}

// cavaSampleFrame is the sample levels repeated over bars values.
func cavaSampleFrame(bars int) []float64 {
	frame := make([]float64, bars)
	for i := range frame {
		frame[i] = cavaSampleLevels[i%len(cavaSampleLevels)]
	}
	return frame
}
