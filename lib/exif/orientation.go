// Package exif reads the orientation a JPEG, TIFF, WebP or PNG file
// records in its EXIF metadata, and applies orientations: the eight
// ways a stored raster maps onto the picture as it is meant to be seen.
package exif

import (
	"fmt"
	"image"
)

// Orientation is an EXIF orientation, TIFF tag 274: where the stored
// raster's first row and first column belong in the picture as shown.
// The eight values are the rotations and mirror images of a rectangle,
// so they compose (Then) and every one has an inverse in the set.
type Orientation uint8

// The orientations, named as TIFF names them: the side the stored first
// row shows on, then the side the stored first column shows on.
const (
	TopLeft Orientation = iota + 1
	TopRight
	BottomRight
	BottomLeft
	LeftTop
	RightTop
	RightBottom
	LeftBottom
)

type matrix [2][2]int

var matrices = [9]matrix{
	TopLeft:     {{1, 0}, {0, 1}},
	TopRight:    {{-1, 0}, {0, 1}},
	BottomRight: {{-1, 0}, {0, -1}},
	BottomLeft:  {{1, 0}, {0, -1}},
	LeftTop:     {{0, 1}, {1, 0}},
	RightTop:    {{0, -1}, {1, 0}},
	RightBottom: {{0, -1}, {-1, 0}},
	LeftBottom:  {{0, 1}, {-1, 0}},
}

// Valid reports whether o is one of the eight orientations.
func (o Orientation) Valid() bool { return o >= TopLeft && o <= LeftBottom }

func (o Orientation) matrix() matrix {
	if !o.Valid() {
		panic(fmt.Sprintf("exif: orientation %d is not one of 1-8", o))
	}
	return matrices[o]
}

// Transposed reports whether o shows the stored rows as columns: the
// quarter turns and the transpositions, 5 to 8.
func (o Orientation) Transposed() bool { return o.matrix()[0][0] == 0 }

// Size is the shown size of a stored w x h raster.
func (o Orientation) Size(w, h int) (int, int) {
	if o.Transposed() {
		return h, w
	}
	return w, h
}

// Then is o followed by next, next applied to the picture o shows: a
// photo stored RightTop and turned once more clockwise shows
// RightTop.Then(RightTop), BottomRight.
func (o Orientation) Then(next Orientation) Orientation {
	a, b := o.matrix(), next.matrix()
	var m matrix
	for i := range 2 {
		for j := range 2 {
			m[i][j] = b[i][0]*a[0][j] + b[i][1]*a[1][j]
		}
	}
	for c := TopLeft; c <= LeftBottom; c++ {
		if matrices[c] == m {
			return c
		}
	}
	panic(fmt.Sprintf("exif: %d then %d composes to no orientation", o, next))
}

// Shown is where o shows the point (x, y) of a stored w x h raster,
// in the continuous plane its pixels tile: the raster's corners map
// onto the picture's corners.
func (o Orientation) Shown(x, y, w, h float64) (float64, float64) {
	m := o.matrix()
	sw, sh := w, h
	if o.Transposed() {
		sw, sh = h, w
	}
	dx, dy := x-float64(w/2), y-float64(h/2)
	sx := float64(float64(m[0][0])*dx) + float64(float64(m[0][1])*dy) + float64(sw/2)
	sy := float64(float64(m[1][0])*dx) + float64(float64(m[1][1])*dy) + float64(sh/2)
	return sx, sy
}

// StoredRect is the rect of a stored w x h raster that o shows as r,
// r in the shown picture's coordinates.
func (o Orientation) StoredRect(r image.Rectangle, w, h int) image.Rectangle {
	m := o.matrix()
	sw, sh := o.Size(w, h)
	stored := func(x, y int) image.Point {
		dx, dy := 2*x-sw, 2*y-sh
		return image.Pt((m[0][0]*dx+m[1][0]*dy+w)/2, (m[0][1]*dx+m[1][1]*dy+h)/2)
	}
	return image.Rectangle{Min: stored(r.Min.X, r.Min.Y), Max: stored(r.Max.X, r.Max.Y)}.Canon()
}

// Draw writes src into dst as o shows it. dst is o.Size of src's size;
// its pixels are all written.
func (o Orientation) Draw(dst, src *image.RGBA) error {
	w, h := src.Rect.Dx(), src.Rect.Dy()
	if sw, sh := o.Size(w, h); dst.Rect.Dx() != sw || dst.Rect.Dy() != sh {
		return fmt.Errorf("exif: drawing a %dx%d raster oriented %d into %dx%d, not %dx%d", w, h, o, dst.Rect.Dx(), dst.Rect.Dy(), sw, sh)
	}
	if w == 0 || h == 0 {
		return nil
	}
	m := o.matrix()
	sw, sh := o.Size(w, h)
	shown := func(x, y int) (int, int) {
		dx, dy := 2*x+1-w, 2*y+1-h
		return (m[0][0]*dx + m[0][1]*dy + sw - 1) / 2, (m[1][0]*dx + m[1][1]*dy + sh - 1) / 2
	}
	stepX := 4*m[0][0] + dst.Stride*m[1][0]
	stepY := 4*m[0][1] + dst.Stride*m[1][1]
	x0, y0 := shown(0, 0)
	row := dst.PixOffset(dst.Rect.Min.X+x0, dst.Rect.Min.Y+y0)
	for y := range h {
		s := src.Pix[src.PixOffset(src.Rect.Min.X, src.Rect.Min.Y+y):]
		d := row
		for x := range w {
			copy(dst.Pix[d:d+4], s[4*x:4*x+4])
			d += stepX
		}
		row += stepY
	}
	return nil
}
