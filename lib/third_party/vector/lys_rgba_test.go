package vector

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"testing"
)

type genericRGBA struct{ *image.RGBA }

type stripes struct{ bounds image.Rectangle }

func (s stripes) ColorModel() color.Model { return color.NRGBAModel }
func (s stripes) Bounds() image.Rectangle { return s.bounds }
func (s stripes) At(x, y int) color.Color {
	return color.NRGBA{uint8(x * 7), uint8(y * 13), uint8(x ^ y), uint8(40 + x*3)}
}

type stripes64 struct{ stripes }

func (s stripes64) RGBA64At(x, y int) color.RGBA64 {
	r, g, b, a := s.At(x, y).RGBA()
	return color.RGBA64{uint16(r), uint16(g), uint16(b), uint16(a)}
}

func TestAnRGBADestinationPaintsWhatTheGenericPathPaints(t *testing.T) {
	bounds := image.Rect(0, 0, 100, 100)
	for _, op := range []draw.Op{draw.Over, draw.Src} {
		for _, src := range []image.Image{stripes{bounds}, stripes64{stripes{bounds}}} {
			paint := func(dst draw.Image) {
				z := NewRasterizer(120, 110)
				z.DrawOp = op
				curvePath(z)
				z.Draw(dst, image.Rect(-10, -5, 110, 105), src, image.Point{3, 1})
			}
			fast, generic := image.NewRGBA(bounds), image.NewRGBA(bounds)
			for i := range fast.Pix {
				fast.Pix[i], generic.Pix[i] = uint8(i*31), uint8(i*31)
			}
			paint(fast)
			paint(genericRGBA{generic})
			if !bytes.Equal(fast.Pix, generic.Pix) {
				t.Errorf("op %v, source %T: the RGBA path paints other pixels than the generic one", op, src)
			}
		}
	}
}
