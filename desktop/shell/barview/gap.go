package barview

import (
	"math"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/styling"
)

// TrayItemGapPx is the systray's item-gap in pixels at the bar scale,
// the gap between the tray's, the task list's and the dock's entries.
func TrayItemGapPx(cfg *config.Config) int {
	return int(math.Round(cfg.Systray.ItemGap.ResolvePx(styling.RemBase, float64(cfg.Bar.Scale))))
}
