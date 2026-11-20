package widgets

import (
	"fmt"

	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/desktop/config"
)

// Placement is where p puts a surface on its output along each axis: a
// corner at both ends, an edge's middle centered along it.
func Placement(p config.ScreenPosition) (h, v widget.Align) {
	switch p {
	case config.ScreenTopLeft:
		return widget.AlignStart, widget.AlignStart
	case config.ScreenTopCenter:
		return widget.AlignCenter, widget.AlignStart
	case config.ScreenTopRight:
		return widget.AlignEnd, widget.AlignStart
	case config.ScreenCenterLeft:
		return widget.AlignStart, widget.AlignCenter
	case config.ScreenCenterRight:
		return widget.AlignEnd, widget.AlignCenter
	case config.ScreenBottomLeft:
		return widget.AlignStart, widget.AlignEnd
	case config.ScreenBottomCenter:
		return widget.AlignCenter, widget.AlignEnd
	case config.ScreenBottomRight:
		return widget.AlignEnd, widget.AlignEnd
	}
	panic(fmt.Sprintf("widgets.Placement: %q is no screen position", p))
}
