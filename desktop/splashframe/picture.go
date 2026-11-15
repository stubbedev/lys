package splashframe

import (
	_ "embed" // the lys logo
	"fmt"
	"image"

	"github.com/stubbedev/lys/lib/render"

	"github.com/stubbedev/lys/desktop/tokens"
)

//go:embed lys.svg
var lysSVG []byte

// LogoColor is the color the lys logo is drawn in: the palette's primary,
// the accent the splash's throbber and bar default to.
func LogoColor() (render.Color, error) {
	c, err := ParseColor(tokens.LysShell().Primary)
	if err != nil {
		return 0, fmt.Errorf("splashframe: the palette's primary: %w", err)
	}
	return c, nil
}

// RasterizeLogo draws the lys logo for a frame at scale, its shape in
// LogoColor: milliseconds of work, so a frame loop asks for it ahead of
// the frame that shows it.
func RasterizeLogo(scale int) (*render.Icon, error) {
	c, err := LogoColor()
	if err != nil {
		return nil, err
	}
	n := LogoPx * scale
	icon, err := render.LoadSVG(lysSVG, n, n)
	if err != nil {
		return nil, fmt.Errorf("splashframe: rasterize the lys logo at %dpx: %w", n, err)
	}
	return icon.Tint(c), nil
}

// Mark is a logo raster and the box it fills, in device pixels.
type Mark struct {
	Image image.Image
	At    image.Rectangle
}

// LysMark is the lys logo rasterized at scale, placed on a frame.
func LysMark(frame image.Point, scale int, logo *render.Icon) Mark {
	return Mark{Image: logo.Still(), At: LysLogo(frame, scale)}
}

// Mark is the firmware's logo placed on a frame.
func (b *BGRT) Mark(frame image.Point) Mark {
	return Mark{Image: b.Image, At: b.Rect(frame)}
}

// Paint blends the logo onto cv, one raster pixel to one device pixel.
func (m Mark) Paint(cv *render.Canvas) {
	cv.DrawImageDevice(m.Image, m.At.Min.X, m.At.Min.Y)
}

// Picture is what the splash's last frame shows once its status group has
// gone: the logo alone on the background.
type Picture struct {
	Background render.Color
	Mark       Mark
}

// Paint fills cv's clip with the picture.
func (p Picture) Paint(cv *render.Canvas) {
	cv.ClearDevice(cv.Rect(), p.Background)
	p.Mark.Paint(cv)
}
