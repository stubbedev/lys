// Package webcamframe is where the recorder's webcam overlay sits on a
// recording, shared by the recorder, its dropdown and lys-settings.
package webcamframe

import (
	"image"

	"github.com/stubbedev/lys/lib/intconv"
)

const (
	edgeMarginPct = 5
	minW          = 80
	minH          = 60
	aspectW       = 16
	aspectH       = 9
	fullPct       = 100
)

// Placement is the frame's place in percentages: X and Y of the free
// space inside the margin, Size of the recording's width.
type Placement struct {
	X, Y, Size uint32
}

// Margin is the uniform inset, a percentage of the shorter side.
func Margin(w, h int32) int32 { return min(w, h) * edgeMarginPct / 100 }

func frameSize(size uint32, w int32) (int32, int32) {
	camW := max(int32(float64(w)*float64(min(max(size, 1), fullPct))/fullPct), minW)
	return camW, max(camW*aspectH/aspectW, minH)
}

func travel(camW, camH, w, h int32) (margin, x, y int32) {
	margin = Margin(w, h)
	return margin, max(w-camW-2*margin, 0), max(h-camH-2*margin, 0)
}

// Rect is where the frame sits on a w x h recording: Size percent of the
// width, 16:9 and no smaller than 80x60, at X and Y percent of the travel
// inside the margin.
func Rect(p Placement, w, h int32) image.Rectangle {
	camW, camH := frameSize(p.Size, w)
	margin, tw, th := travel(camW, camH, w, h)
	x := margin + tw*int32(min(p.X, fullPct))/fullPct
	y := margin + th*int32(min(p.Y, fullPct))/fullPct
	return image.Rect(int(x), int(y), int(x+camW), int(y+camH))
}

func percentOf(offset, span int32) uint32 {
	if span <= 0 {
		return 0
	}
	return intconv.Clamp[uint32]((min(max(offset, 0), span)*fullPct + span/2) / span)
}

// At is the placement of a frame camW wide whose top-left corner is at
// x, y on a w x h recording, the inverse of Rect to its rounding.
func At(x, y, camW, w, h int32) Placement {
	size := max(percentOf(camW, w), 1)
	fw, fh := frameSize(size, w)
	margin, tw, th := travel(fw, fh, w, h)
	return Placement{X: percentOf(x-margin, tw), Y: percentOf(y-margin, th), Size: size}
}
