package imagedecode

import (
	"bytes"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gen2brain/h265/heic"
	"github.com/gen2brain/jxl"
)

// The fixtures were written by ImageMagick 7 through libheif with x265
// (the HEIF ones), libjxl 0.12 (the JPEG XL ones) and libheif with
// libaom (halves.avif): halves is 64x48, its left half red and its
// right half blue; rotated.heic is halves stored as is with an irot box
// turning it a quarter clockwise; deep.jxl is a lossless 16-bit picture
// with alpha, which libjxl marks level 10.
func fixture(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join("testdata", name)
}

// near reports whether c is want within the error a lossy coding of
// flat colors leaves.
func near(c, want color.NRGBA) bool {
	d := func(a, b uint8) bool { return a-b < 12 || b-a < 12 }
	return d(c.R, want.R) && d(c.G, want.G) && d(c.B, want.B) && d(c.A, want.A)
}

var (
	red  = color.NRGBA{R: 255, A: 255}
	blue = color.NRGBA{B: 255, A: 255}
)

// A HEIF and a JPEG XL picture decode, from a file and from bytes,
// with their MIME types and extensions among the formats'; a HEIF
// picture turns as its container says; a JPEG XL picture of level 10
// decodes, its depth and alpha kept.
func TestHEIFAndJPEGXLPicturesDecode(t *testing.T) {
	for _, name := range []string{"halves.heic", "halves.jxl"} {
		data, err := os.ReadFile(fixture(t, name))
		if err != nil {
			t.Fatal(err)
		}
		fromFile, err := Open(fixture(t, name))
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		fromBytes, err := Decode(data)
		if err != nil {
			t.Fatalf("%s from bytes: %v", name, err)
		}
		for _, img := range []image.Image{fromFile, fromBytes} {
			if img.Bounds() != image.Rect(0, 0, 64, 48) || !near(rgba(img, 4, 24), red) || !near(rgba(img, 59, 24), blue) {
				t.Errorf("%s decodes %v as %v then %v", name, img.Bounds(), rgba(img, 4, 24), rgba(img, 59, 24))
			}
		}
	}
	turned, err := Open(fixture(t, "rotated.heic"))
	if err != nil {
		t.Fatal(err)
	}
	if turned.Bounds() != image.Rect(0, 0, 48, 64) || !near(rgba(turned, 24, 4), red) || !near(rgba(turned, 24, 59), blue) {
		t.Errorf("rotated.heic decodes %v as %v over %v", turned.Bounds(), rgba(turned, 24, 4), rgba(turned, 24, 59))
	}
	deep, err := Open(fixture(t, "deep.jxl"))
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := deep.(*image.NRGBA64); !ok {
		t.Errorf("deep.jxl decodes to a %T", deep)
	}
	if c := color.NRGBA64Model.Convert(deep.At(5, 5)).(color.NRGBA64); c.R < 58000 || c.A != 32768 {
		t.Errorf("deep.jxl's corner is %v", c)
	}
	for _, mimeType := range []string{"image/heif", "image/jxl"} {
		if !slices.Contains(Types(), mimeType) {
			t.Errorf("%s is not among the types decoded", mimeType)
		}
	}
	for _, ext := range []string{"heic", "heif", "hif", "jxl"} {
		if !slices.Contains(Extensions(), ext) {
			t.Errorf("%s is not among the extensions decoded", ext)
		}
	}
}

// A HEIF or JPEG XL picture cut short does not decode, nor does an
// AVIF picture, which lys has no decoder for.
func TestBrokenHEIFAndJPEGXLAndAVIFDoNotDecode(t *testing.T) {
	for _, name := range []string{"halves.heic", "halves.jxl", "deep.jxl"} {
		data, err := os.ReadFile(fixture(t, name))
		if err != nil {
			t.Fatal(err)
		}
		cut := data[:len(data)*2/3]
		if img, err := Open(write(t, name, cut)); err == nil {
			t.Errorf("%s cut short decodes to %v", name, img.Bounds())
		}
		if img, err := Decode(cut); err == nil {
			t.Errorf("%s's bytes cut short decode to %v", name, img.Bounds())
		}
	}
	if img, err := Open(fixture(t, "halves.avif")); err == nil {
		t.Errorf("an AVIF picture decodes to %v", img.Bounds())
	}
	if slices.Contains(Types(), "image/avif") || slices.Contains(Extensions(), "avif") {
		t.Error("AVIF is among the formats decoded")
	}
}

// DecodeBytes names a HEIF and a JPEG XL picture by their MIME types and
// covers what their decoders allocate, without overstating it past
// three times and the decoder's fixed share, for the fixtures and for
// larger pictures their own packages encode.
func TestDecodeBytesCoversHEIFAndJPEGXL(t *testing.T) {
	const w, h = 1003, 701
	src := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			src.Set(x, y, color.NRGBA{uint8(x), uint8(y), uint8(x + y), 255})
		}
	}
	encoded := map[string][]byte{}
	var b bytes.Buffer
	if err := heic.Encode(&b, src, heic.EncodeOptions{Quality: 60}); err != nil {
		t.Fatal(err)
	}
	encoded["large.heic"] = slices.Clone(b.Bytes())
	b.Reset()
	if err := jxl.Encode(&b, src, jxl.EncodeOptions{Quality: 80}); err != nil {
		t.Fatal(err)
	}
	encoded["large.jxl"] = slices.Clone(b.Bytes())
	for _, name := range []string{"halves.heic", "rotated.heic", "halves.jxl", "deep.jxl"} {
		data, err := os.ReadFile(fixture(t, name))
		if err != nil {
			t.Fatal(err)
		}
		encoded[name] = data
	}
	for name, data := range encoded {
		path := write(t, name, data)
		mimeType, est, err := MeasureData(data)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		want := map[string]string{".heic": "image/heif", ".jxl": "image/jxl"}[filepath.Ext(name)]
		if mimeType != want {
			t.Errorf("%s measures as %s, want %s", name, mimeType, want)
		}
		got := allocated(t, func() error { _, err := Open(path); return err })
		f, err := formatNamed(strings.TrimPrefix(filepath.Ext(name), "."))
		if err != nil {
			t.Fatal(err)
		}
		if est < got || est > 3*got+f.fixedBytes {
			t.Errorf("%s: estimated %d bytes, the decode allocated %d", name, est, got)
		}
	}
}
