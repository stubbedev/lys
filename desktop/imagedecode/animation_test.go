package imagedecode

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/kettek/apng"
	"github.com/stubbedev/lys/lib/render"
)

// gifOf is a w x h GIF of frames frames, each a different color.
func gifOf(t *testing.T, w, h, frames int) []byte {
	t.Helper()
	pal := color.Palette{color.RGBA{255, 0, 0, 255}, color.RGBA{0, 255, 0, 255}, color.RGBA{0, 0, 255, 255}}
	g := &gif.GIF{}
	for i := range frames {
		p := image.NewPaletted(image.Rect(0, 0, w, h), pal)
		for k := range p.Pix {
			p.Pix[k] = uint8((i + k) % len(pal))
		}
		g.Image = append(g.Image, p)
		g.Delay = append(g.Delay, 10)
	}
	var buf bytes.Buffer
	if err := gif.EncodeAll(&buf, g); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// apngOf is a w x h APNG of frames frames.
func apngOf(t *testing.T, w, h, frames int) []byte {
	t.Helper()
	a := apng.APNG{}
	for i := range frames {
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		for k := range img.Pix {
			img.Pix[k] = uint8(i*40 + k)
		}
		a.Frames = append(a.Frames, apng.Frame{Image: img, DelayNumerator: 1, DelayDenominator: 10})
	}
	var buf bytes.Buffer
	if err := apng.Encode(&buf, a); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// An animated GIF or APNG is sized before it decodes, covering what
// decoding every frame as render.DecodeImage does allocates, up to the
// decoders' fixed buffers, without overstating it by more than the
// frames' stored form.
func TestAnimationBytesCoversDecodingEveryFrame(t *testing.T) {
	const w, h, frames = 301, 203, 12
	const slack = 256 << 10
	for name, data := range map[string][]byte{"gif": gifOf(t, w, h, frames), "apng": apngOf(t, w, h, frames)} {
		est, animated, err := AnimationBytes(bytes.NewReader(data))
		if err != nil || !animated {
			t.Fatalf("%s: AnimationBytes = %d, %v, %v; want an animation", name, est, animated, err)
		}
		var img image.Image
		got := allocated(t, func() error {
			var err error
			img, err = render.DecodeImage(data)
			return err
		})
		if a, ok := img.(*render.Animation); !ok || len(a.Frames) != frames {
			t.Fatalf("%s decodes as %T, want an animation of %d frames", name, img, frames)
		}
		if est < got-slack || est > 3*got {
			t.Errorf("%s: estimated %d bytes, decoding every frame allocated %d", name, est, got)
		}
	}
}

// A still picture, a GIF or APNG of one frame, a file of another format
// and a GIF cut short are no animation, and decode as still pictures.
func TestAnimationBytesFindsNoAnimationInAStill(t *testing.T) {
	var still, jpg bytes.Buffer
	if err := png.Encode(&still, sample()); err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(&jpg, sample(), nil); err != nil {
		t.Fatal(err)
	}
	animated := gifOf(t, 8, 8, 3)
	for name, data := range map[string][]byte{
		"still.png":  still.Bytes(),
		"still.jpg":  jpg.Bytes(),
		"still.gif":  gifOf(t, 8, 8, 1),
		"single.png": apngOf(t, 8, 8, 1),
		"cut.gif":    animated[:len(animated)-1],
		"empty":      nil,
		"garbage":    []byte("GIF89a and then nothing a GIF holds"),
	} {
		n, ok, err := AnimationBytes(bytes.NewReader(data))
		if err != nil || ok || n != 0 {
			t.Errorf("%s: AnimationBytes = %d, %v, %v; want no animation", name, n, ok, err)
		}
	}
}
