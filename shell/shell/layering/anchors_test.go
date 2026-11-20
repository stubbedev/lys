package layering

import (
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/widget"
)

func TestAnchorsTakeTheEdgesASurfaceSitsAt(t *testing.T) {
	for _, tc := range []struct {
		h, v widget.Align
		want app.Anchor
	}{
		{widget.AlignStart, widget.AlignStart, app.AnchorTop | app.AnchorLeft},
		{widget.AlignEnd, widget.AlignEnd, app.AnchorBottom | app.AnchorRight},
		{widget.AlignCenter, widget.AlignStart, app.AnchorTop},
		{widget.AlignEnd, widget.AlignCenter, app.AnchorRight},
		{widget.AlignCenter, widget.AlignCenter, 0},
	} {
		if got := Anchors(tc.h, tc.v); got != tc.want {
			t.Errorf("Anchors(%v, %v) = %v, want %v", tc.h, tc.v, got, tc.want)
		}
	}
}

func TestACornerTakesBothEdgesAnEdgesMiddleOne(t *testing.T) {
	for pos, want := range map[config.ScreenPosition]app.Anchor{
		config.ScreenTopLeft:      app.AnchorTop | app.AnchorLeft,
		config.ScreenTopCenter:    app.AnchorTop,
		config.ScreenTopRight:     app.AnchorTop | app.AnchorRight,
		config.ScreenCenterLeft:   app.AnchorLeft,
		config.ScreenCenterRight:  app.AnchorRight,
		config.ScreenBottomLeft:   app.AnchorBottom | app.AnchorLeft,
		config.ScreenBottomCenter: app.AnchorBottom,
		config.ScreenBottomRight:  app.AnchorBottom | app.AnchorRight,
	} {
		if got := ScreenAnchors(pos); got != want {
			t.Errorf("%s anchors %v, want %v", pos, got, want)
		}
	}
}
