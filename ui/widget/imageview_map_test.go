package widget

import (
	"math"
	"testing"

	"github.com/stubbedev/lys/lib/exif"
	"github.com/stubbedev/lys/lib/render"
)

// PictureAt and ViewAt map a point between the root and the picture
// both ways, fitted, zoomed about a point and panned, and a point
// outside the box maps outside the picture.
func TestImageViewMapsPointsBetweenTheRootAndThePicture(t *testing.T) {
	v := NewImageView(imageViewPicture(), exif.TopLeft)
	v.Arrange(render.Rect{X: 10, Y: 20, W: 80, H: 50})
	if x, y := v.PictureAt(Point{X: 10, Y: 20}); x != 0 || y != 0 {
		t.Errorf("the fitted picture's top left shows %g, %g at the box's", x, y)
	}
	if x, y := v.ViewAt(160, 100); x != 90 || y != 70 {
		t.Errorf("the fitted picture's bottom right lies at %g, %g, want the box's 90, 70", x, y)
	}
	if x, y := v.PictureAt(Point{X: 5, Y: 20}); x >= 0 || y != 0 {
		t.Errorf("a point left of the box shows %g, %g, want left of the picture", x, y)
	}
	v.ZoomAt(4, Point{X: 50, Y: 45})
	for _, p := range []Point{{X: 50, Y: 45}, {X: 15, Y: 30}, {X: 85, Y: 65}} {
		x, y := v.PictureAt(p)
		if vx, vy := v.ViewAt(x, y); math.Abs(vx-float64(p.X)) > 1e-9 || math.Abs(vy-float64(p.Y)) > 1e-9 {
			t.Errorf("%v maps to %g, %g and back to %g, %g", p, x, y, vx, vy)
		}
	}
	before, _ := v.PictureAt(Point{X: 50, Y: 45})
	v.ScrollPixels(20, 0)
	if after, _ := v.PictureAt(Point{X: 50, Y: 45}); math.Abs(after-before-5) > 1e-9 {
		t.Errorf("panning 20 px at 4x moved the picture under the point by %g, want 5", after-before)
	}
}
