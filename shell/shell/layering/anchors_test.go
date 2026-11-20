package layering

import (
	"testing"

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
