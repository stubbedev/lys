package material

import (
	"image"

	"github.com/stubbedev/lys/lib/intconv"
)

// Sample reads img at a grid of at most side samples along its longer
// edge, keeping its aspect, one pixel at the center of each cell, and
// returns the opaque ones. Its work and its one allocation are bounded
// by the grid, whatever the image's resolution.
func Sample(img *image.RGBA, side int) []ARGB {
	return sampleGrid(img.Rect, side, func(x, y int) (ARGB, bool) {
		o := img.PixOffset(x, y)
		p := img.Pix[o : o+4 : o+4]
		return RGB(p[0], p[1], p[2]), p[3] == 0xff
	})
}

// SampleImage is Sample over any decoded image; an *image.RGBA is read
// through Sample itself.
func SampleImage(img image.Image, side int) []ARGB {
	if rgba, ok := img.(*image.RGBA); ok {
		return Sample(rgba, side)
	}
	return sampleGrid(img.Bounds(), side, func(x, y int) (ARGB, bool) {
		r, g, b, a := img.At(x, y).RGBA()
		return RGB(intconv.Must[uint8](r>>8), intconv.Must[uint8](g>>8), intconv.Must[uint8](b>>8)), a == 0xffff
	})
}

func sampleGrid(b image.Rectangle, side int, at func(x, y int) (ARGB, bool)) []ARGB {
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 || side <= 0 {
		return nil
	}
	cols, rows := min(side, w), min(side, h)
	if w >= h {
		rows = max(1, min(rows, h*cols/w))
	} else {
		cols = max(1, min(cols, w*rows/h))
	}
	out := make([]ARGB, 0, cols*rows)
	for row := range rows {
		y := (2*row + 1) * h / (2 * rows)
		for col := range cols {
			x := (2*col + 1) * w / (2 * cols)
			if c, opaque := at(b.Min.X+x, b.Min.Y+y); opaque {
				out = append(out, c)
			}
		}
	}
	return out
}
