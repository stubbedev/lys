package render

import (
	"image"
	"image/color"
	"slices"
	"testing"
)

type pixelsBehindAt struct{ img *image.RGBA }

func (o pixelsBehindAt) ColorModel() color.Model { return o.img.ColorModel() }
func (o pixelsBehindAt) Bounds() image.Rectangle { return o.img.Bounds() }
func (o pixelsBehindAt) At(x, y int) color.Color { return o.img.At(x, y) }

func TestTilingAnRGBAImagePaintsWhatItsPixelsPaint(t *testing.T) {
	img := image.NewRGBA(image.Rect(2, 3, 9, 8))
	for i := range img.Pix {
		img.Pix[i] = uint8(i * 37)
	}
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i+3] = max(img.Pix[i+3], img.Pix[i], img.Pix[i+1], img.Pix[i+2])
	}
	paint := func(src image.Image) []byte {
		buf := make([]byte, Stride(40)*30)
		cv := New(buf, Stride(40), 40, 30)
		cv.PaintImageTiles(src, Tile{X: 1, W: 9, H: 7, GapX: 2, RepeatX: true, RepeatY: true}, Rect{X: 2, Y: 1, W: 35, H: 26}, Corners{TopLeft: 4})
		return buf
	}
	if !slices.Equal(paint(img), paint(pixelsBehindAt{img})) {
		t.Error("tiling an RGBA image paints other pixels than the same pixels behind At")
	}
}
