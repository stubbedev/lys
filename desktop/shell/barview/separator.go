package barview

import (
	"math"
	"strconv"

	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/styling"
)

// NewSeparator is the separator module: lys-ui's BarSeparator, a rule
// across the bar between its items, its color, thickness and length
// the separator config's.
func NewSeparator(cfg *config.Config) *widget.BarSeparator {
	rule := widget.NewBarSeparator()
	rule.SetInlineStyle(separatorCSS(cfg))
	return rule
}

func separatorCSS(cfg *config.Config) string {
	sep := cfg.Separator
	thick := strconv.Itoa(int(sep.Size)) + "px"
	length := strconv.Itoa(int(math.Round(sep.Length.ResolvePx(styling.RemBase, float64(cfg.Bar.Scale))))) + "px"
	width, height := thick, length
	if cfg.Bar.Location.IsVertical() {
		width, height = length, thick
	}
	color := styling.ResolveColorCSS(sep.Color, config.DefaultsSeparator().Color, cfg.Styling.ColorExtractor.ThemeProvider)
	return "color: " + color + "; min-width: " + width + "; min-height: " + height + ";"
}
