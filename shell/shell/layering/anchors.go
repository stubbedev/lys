package layering

import (
	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/shell/widgets"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"
)

// Anchors are the layer-shell edges a surface placed at h and v on its
// output takes: the edge at each end it sits at, neither along an axis
// it centers on.
func Anchors(h, v widget.Align) app.Anchor {
	var a app.Anchor
	switch h {
	case widget.AlignStart:
		a |= app.AnchorLeft
	case widget.AlignEnd:
		a |= app.AnchorRight
	}
	switch v {
	case widget.AlignStart:
		a |= app.AnchorTop
	case widget.AlignEnd:
		a |= app.AnchorBottom
	}
	return a
}

// ScreenAnchors are the edges a surface at p on its output takes.
func ScreenAnchors(p config.ScreenPosition) app.Anchor { return Anchors(widgets.Placement(p)) }
