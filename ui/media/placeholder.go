package media

import (
	"image"
	"image/color"
)

const placeholderSamples = 4

// placeholder is the picture of a sound without a cover: a neutral grey
// square with a quaver on it, side pixels across, its edges smoothed by
// sampling each pixel placeholderSamples² times.
func placeholder(side int) *image.RGBA {
	background := color.RGBA{0x3a, 0x3a, 0x3c, 0xff}
	glyph := color.RGBA{0x8e, 0x8e, 0x93, 0xff}
	img := image.NewRGBA(image.Rect(0, 0, side, side))
	scale := 1 / float64(side)
	for y := range side {
		for x := range side {
			hits := 0
			for sy := range placeholderSamples {
				for sx := range placeholderSamples {
					u := (float64(x) + (float64(sx)+0.5)/placeholderSamples) * scale
					v := (float64(y) + (float64(sy)+0.5)/placeholderSamples) * scale
					if inQuaver(u, v) {
						hits++
					}
				}
			}
			img.SetRGBA(x, y, mix(background, glyph, hits, placeholderSamples*placeholderSamples))
		}
	}
	return img
}

// inQuaver reports whether the point (u, v) of the unit square lies on
// the quaver: a tilted oval head, a stem up from its right and a flag
// down from the stem's top.
func inQuaver(u, v float64) bool {
	const (
		headX, headY = 0.43, 0.70
		headA, headB = 0.13, 0.09
		tiltC, tiltS = 0.89, -0.45
		stemL, stemR = 0.535, 0.575
		stemTop      = 0.22
	)
	dx, dy := u-headX, v-headY
	rx, ry := dx*tiltC+dy*tiltS, -dx*tiltS+dy*tiltC
	if rx*rx/(headA*headA)+ry*ry/(headB*headB) <= 1 {
		return true
	}
	if u >= stemL && u <= stemR && v >= stemTop && v <= headY-0.02 {
		return true
	}
	if u < stemR || u > stemR+0.17 || v < stemTop {
		return false
	}
	t := (u - stemR) / 0.17
	top := stemTop + 0.12*t*t + 0.10*t
	return v >= top && v <= top+0.10*(1-t)+0.02
}

func mix(a, b color.RGBA, n, of int) color.RGBA {
	lerp := func(x, y uint8) uint8 { return uint8((int(x)*(of-n) + int(y)*n + of/2) / of) } //nolint:gosec // a weighted mean of two bytes
	return color.RGBA{lerp(a.R, b.R), lerp(a.G, b.G), lerp(a.B, b.B), 0xff}
}
