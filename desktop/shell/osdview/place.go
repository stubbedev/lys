package osdview

import (
	"github.com/stubbedev/lys/ui/widget"

	"github.com/stubbedev/lys/desktop/config"
)

// Placement is where osd.position puts the card on its output along
// each axis: a corner at both ends, an edge's middle centered along it.
func Placement(p config.OsdPosition) (h, v widget.Align) {
	switch p {
	case config.OsdTopLeft:
		return widget.AlignStart, widget.AlignStart
	case config.OsdTop:
		return widget.AlignCenter, widget.AlignStart
	case config.OsdTopRight:
		return widget.AlignEnd, widget.AlignStart
	case config.OsdRight:
		return widget.AlignEnd, widget.AlignCenter
	case config.OsdBottomRight:
		return widget.AlignEnd, widget.AlignEnd
	case config.OsdBottomLeft:
		return widget.AlignStart, widget.AlignEnd
	case config.OsdLeft:
		return widget.AlignStart, widget.AlignCenter
	}
	return widget.AlignCenter, widget.AlignEnd
}
