package settings

import (
	"log"
	"strconv"
	"strings"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/widget"
)

const (
	iconCapSmall = 16
	iconCapLarge = 32
	iconCapName  = "ld-app-window-symbolic"
)

// iconScaleControl is an icon size on a slider between a small and a
// large icon, read in pixels and written as a scale of its base.
type iconScaleControl struct {
	*widget.Box
	slider *slider
}

// iconScale is an icon size slid over the scales lo to hi of baseRem.
func iconScale(baseRem, lo, hi float64) rowOpt {
	return withEditor(func(k *kit, s slot, _ config.FieldMeta) control {
		return newIconScale(k, s, baseRem*remBasePx, lo, hi)
	})
}

func newIconScale(k *kit, s slot, basePx, lo, hi float64) *iconScaleControl {
	c := &iconScaleControl{Box: gapBox(widget.Row)}
	px := unitPixels.format(0)
	c.slider = newSlider(k, scaleOfSize(s, basePx), lo, hi, false, func(v float64) string { return px(v * basePx) })
	c.AppendAligned(widget.NewThemeIcon(iconCapName, iconCapSmall), false, widget.AlignCenter)
	c.Append(c.slider, true)
	c.AppendAligned(widget.NewThemeIcon(iconCapName, iconCapLarge), false, widget.AlignCenter)
	return c
}

func (c *iconScaleControl) refresh() { c.slider.refresh() }

// scaleOfSize is a size as the scale of basePx it comes to: a stored
// "<n>px" reads as its scale, and a scale is written.
func scaleOfSize(s slot, basePx float64) slot {
	return slot{
		get: func() any {
			text, ok := s.get().(string)
			if !ok {
				return s.get()
			}
			px, err := strconv.ParseFloat(strings.TrimSuffix(text, "px"), 64)
			if err != nil {
				log.Printf("settings: %q is no pixel size", text)
				return nil
			}
			return px / basePx
		},
		set:   s.set,
		unset: s.unset,
	}
}
