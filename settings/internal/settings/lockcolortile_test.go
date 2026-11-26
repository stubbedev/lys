package settings

import (
	"testing"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/ui/widget"
)

// The lock background's Color card drew its screen in the dark preview
// fill, about the card's own color, so the card looked empty. Its screen
// stands off the card as a control's boundary must.
func TestTheLockBackgroundsColorCardShowsItsScreen(t *testing.T) {
	k := testKit(t, "")
	themed(k, gapBox(widget.Column))
	r := render.Rect{W: 88, H: 48}
	stride := render.Stride(r.W)
	data := make([]byte, stride*r.H)
	lockBackgroundPictures[string(config.BackgroundColor)].draw(render.New(data, stride, r.W, r.H), r, k.backdropFor(widget.NewDrawingArea(1, 1)))
	i := (r.H/2)*stride + 4*(r.W/2)
	screen := render.RGB(data[i+2], data[i+1], data[i])
	if c := contrastOf(screen, widget.Current().CardColor()); c < minUIContrast {
		t.Errorf("the Color card's screen stands %.2f:1 off the card, want %d:1", c, minUIContrast)
	}
}
