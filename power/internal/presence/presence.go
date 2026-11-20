// Package presence is the pure core of the attention check before the
// idle dim: whether one greyscale camera frame shows a face turned to the
// screen, is too dark or too small to tell, or neither, and what a few
// frames say together. Frames are only read: nothing is kept.
package presence

import (
	"fmt"
	"image"

	"github.com/stubbedev/lys/power/internal/config"
	"github.com/stubbedev/lys/power/third_party/pico"
)

type Look uint8

const (
	Blind Look = iota + 1
	NoFace
	Face
)

func (l Look) String() string {
	switch l {
	case Blind:
		return "blind"
	case NoFace:
		return "no face"
	case Face:
		return "face"
	}
	return fmt.Sprintf("Look(%d)", uint8(l))
}

type Params struct {
	Quality   float32
	DarkLuma  float64
	MinHeight int
	Window    pico.Params
}

type Judge struct {
	cascade pico.Cascade
	params  Params
}

func NewJudge(cascade pico.Cascade, p Params) (Judge, error) {
	if !(p.Quality > 0) || !(p.DarkLuma >= 0 && p.DarkLuma < 255) || p.MinHeight < 1 {
		return Judge{}, fmt.Errorf("face judge at quality %v, dark below luma %v, frames from %d rows: want quality > 0, 0 <= dark luma < 255, at least 1 row", p.Quality, p.DarkLuma, p.MinHeight)
	}
	return Judge{cascade: cascade, params: p}, nil
}

func NewFaceJudge() (Judge, error) {
	cascade, err := pico.Facefinder()
	if err != nil {
		return Judge{}, err
	}
	window, err := pico.NewParams(config.AttentionFaceShare, config.AttentionWindowShift, config.AttentionWindowScale)
	if err != nil {
		return Judge{}, fmt.Errorf("face judge: %w", err)
	}
	return NewJudge(cascade, Params{
		Quality:   config.AttentionFaceQuality,
		DarkLuma:  config.AttentionDarkLuma,
		MinHeight: config.AttentionMinHeight,
		Window:    window,
	})
}

func (j Judge) Look(frame *image.Gray) Look {
	if min(frame.Rect.Dx(), frame.Rect.Dy()) < j.params.MinHeight || mean(frame) < j.params.DarkLuma {
		return Blind
	}
	for _, d := range j.cascade.Find(frame, j.params.Window) {
		if d.Q >= j.params.Quality {
			return Face
		}
	}
	return NoFace
}

func Of(looks []Look) Look {
	out := Blind
	for _, l := range looks {
		out = max(out, l)
	}
	return out
}

func mean(frame *image.Gray) float64 {
	r := frame.Rect
	var sum uint64
	for y := r.Min.Y; y < r.Max.Y; y++ {
		start := frame.PixOffset(r.Min.X, y)
		for _, v := range frame.Pix[start : start+r.Dx()] {
			sum += uint64(v)
		}
	}
	return float64(sum) / float64(r.Dx()*r.Dy())
}
