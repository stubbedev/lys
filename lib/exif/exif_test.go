package exif

import (
	"bytes"
	"encoding/binary"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/stubbedev/lys/lib/testbuild"
)

func TestImportingCostsNothing(t *testing.T) { testbuild.RequireNoInit(t) }

func tiffBlock(order binary.ByteOrder, typ uint16, value uint16) []byte {
	var b bytes.Buffer
	if order == binary.LittleEndian {
		b.WriteString(tiffLittle)
	} else {
		b.WriteString(tiffBig)
	}
	_ = binary.Write(&b, order, uint32(8))
	_ = binary.Write(&b, order, uint16(2))
	_ = binary.Write(&b, order, []uint16{0x0100, 4})
	_ = binary.Write(&b, order, uint32(1))
	_ = binary.Write(&b, order, uint32(640))
	_ = binary.Write(&b, order, []uint16{tagOrientation, typ})
	_ = binary.Write(&b, order, uint32(1))
	_ = binary.Write(&b, order, []uint16{value, 0})
	_ = binary.Write(&b, order, uint32(0))
	return b.Bytes()
}

func encodedJPEG(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := jpeg.Encode(&b, image.NewGray(image.Rect(0, 0, 4, 2)), nil); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func withSegment(t *testing.T, marker byte, payload []byte) []byte {
	t.Helper()
	j := encodedJPEG(t)
	seg := []byte{0xff, marker, 0, 0}
	binary.BigEndian.PutUint16(seg[2:], uint16(len(payload)+2))
	return append(append(append(append([]byte{}, j[:2]...), seg...), payload...), j[2:]...)
}

func jpegWith(t *testing.T, tiff []byte) []byte {
	t.Helper()
	xmp := withSegment(t, 0xe1, []byte("http://ns.adobe.com/xap/1.0/\x00<x/>"))
	seg := []byte{0xff, 0xe1, 0, 0}
	payload := append([]byte(exifHeader), tiff...)
	binary.BigEndian.PutUint16(seg[2:], uint16(len(payload)+2))
	return append(append(append(append([]byte{}, xmp[:2]...), seg...), payload...), xmp[2:]...)
}

func webpWith(chunks ...[]byte) []byte {
	var body bytes.Buffer
	body.WriteString("WEBP")
	for _, c := range chunks {
		body.Write(c)
		if len(c)%2 == 1 {
			body.WriteByte(0)
		}
	}
	out := []byte("RIFF\x00\x00\x00\x00")
	binary.LittleEndian.PutUint32(out[4:], uint32(body.Len()))
	return append(out, body.Bytes()...)
}

func riffChunk(fourcc string, data []byte) []byte {
	c := append([]byte(fourcc), 0, 0, 0, 0)
	binary.LittleEndian.PutUint32(c[4:], uint32(len(data)))
	return append(c, data...)
}

func pngWith(t *testing.T, exif []byte) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewGray(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	p := b.Bytes()
	chunk := make([]byte, 8, 12+len(exif))
	binary.BigEndian.PutUint32(chunk, uint32(len(exif)))
	copy(chunk[4:], "eXIf")
	chunk = append(append(chunk, exif...), 0, 0, 0, 0)
	ihdrEnd := len(pngMagic) + 8 + 13 + 4
	return append(append(append([]byte{}, p[:ihdrEnd]...), chunk...), p[ihdrEnd:]...)
}

func TestReadEveryOrientationOfEveryFormat(t *testing.T) {
	for o := TopLeft; o <= LeftBottom; o++ {
		for _, order := range []binary.ByteOrder{binary.LittleEndian, binary.BigEndian} {
			tiff := tiffBlock(order, typeShort, uint16(o))
			files := map[string][]byte{
				"JPEG":                     jpegWith(t, tiff),
				"TIFF":                     tiff,
				"WebP":                     webpWith(riffChunk("VP8X", make([]byte, 10)), riffChunk("VP8L", make([]byte, 5)), riffChunk("EXIF", tiff)),
				"WebP with an Exif header": webpWith(riffChunk("VP8 ", make([]byte, 4)), riffChunk("EXIF", append([]byte(exifHeader), tiff...))),
				"PNG":                      pngWith(t, tiff),
			}
			for format, data := range files {
				got, err := Read(bytes.NewReader(data))
				if err != nil || got != o {
					t.Errorf("%s %v orientation %d: read %d, %v", format, order, o, got, err)
				}
			}
		}
	}
}

func TestReadWithoutOrientation(t *testing.T) {
	tiffNoTag := tiffBlock(binary.LittleEndian, typeShort, 1)
	binary.LittleEndian.PutUint16(tiffNoTag[8:], 1)
	var p bytes.Buffer
	if err := png.Encode(&p, image.NewGray(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		"a JPEG without EXIF":      encodedJPEG(t),
		"a JPEG with only XMP":     withSegment(t, 0xe1, []byte("http://ns.adobe.com/xap/1.0/\x00<x/>")),
		"a TIFF without the tag":   tiffNoTag,
		"a WebP without EXIF":      webpWith(riffChunk("VP8L", make([]byte, 7))),
		"a PNG without eXIf":       p.Bytes(),
		"a GIF, which has no EXIF": []byte("GIF89a"),
		"an empty file":            nil,
	}
	for name, data := range files {
		got, err := Read(bytes.NewReader(data))
		if err != nil || got != TopLeft {
			t.Errorf("%s: read %d, %v; want TopLeft", name, got, err)
		}
	}
}

func TestReadRefusesMalformedOrientation(t *testing.T) {
	bad := map[string][]byte{
		"orientation 0":              jpegWith(t, tiffBlock(binary.LittleEndian, typeShort, 0)),
		"orientation 9":              tiffBlock(binary.BigEndian, typeShort, 9),
		"orientation 257":            tiffBlock(binary.LittleEndian, typeShort, 257),
		"orientation as a LONG":      tiffBlock(binary.LittleEndian, 4, 6),
		"a truncated IFD":            tiffBlock(binary.LittleEndian, typeShort, 6)[:20],
		"a bad TIFF header in WebP":  webpWith(riffChunk("EXIF", []byte("XX*\x00\x08\x00\x00\x00"))),
		"a JPEG cut in its segments": encodedJPEG(t)[:30],
		"a PNG cut in its chunks":    pngWith(t, tiffBlock(binary.LittleEndian, typeShort, 3))[:40],
		"a JPEG without markers":     {0xff, 0xd8, 0x12, 0x34},
	}
	for name, data := range bad {
		got, err := Read(bytes.NewReader(data))
		if !errors.Is(err, ErrMalformed) {
			t.Errorf("%s: read %d, %v; want ErrMalformed", name, got, err)
		}
	}
}

func marked(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetRGBA(x, y, color.RGBA{uint8(x), uint8(y), 0, 255})
		}
	}
	return img
}

func TestDrawPlacesTheStoredCornersAsEXIFDefines(t *testing.T) {
	const w, h = 3, 2
	type corner struct{ x, y int }
	want := map[Orientation][2]corner{
		TopLeft:     {{0, 0}, {2, 0}},
		TopRight:    {{2, 0}, {0, 0}},
		BottomRight: {{2, 1}, {0, 1}},
		BottomLeft:  {{0, 1}, {2, 1}},
		LeftTop:     {{0, 0}, {0, 2}},
		RightTop:    {{1, 0}, {1, 2}},
		RightBottom: {{1, 2}, {1, 0}},
		LeftBottom:  {{0, 2}, {0, 0}},
	}
	src := marked(w, h)
	for o := TopLeft; o <= LeftBottom; o++ {
		sw, sh := o.Size(w, h)
		dst := image.NewRGBA(image.Rect(10, 20, 10+sw, 20+sh))
		if err := o.Draw(dst, src); err != nil {
			t.Fatal(err)
		}
		first, last := want[o][0], want[o][1]
		if got := dst.RGBAAt(10+first.x, 20+first.y); got != (color.RGBA{0, 0, 0, 255}) {
			t.Errorf("orientation %d shows the stored origin elsewhere: %v at %v", o, got, first)
		}
		if got := dst.RGBAAt(10+last.x, 20+last.y); got != (color.RGBA{w - 1, 0, 0, 255}) {
			t.Errorf("orientation %d shows the first row's end elsewhere: %v at %v", o, got, last)
		}
		for y := range sh {
			for x := range sw {
				p := dst.RGBAAt(10+x, 20+y)
				back := o.StoredRect(image.Rect(x, y, x+1, y+1), w, h)
				if back.Dx() != 1 || back.Dy() != 1 || p != src.RGBAAt(back.Min.X, back.Min.Y) {
					t.Fatalf("orientation %d: shown (%d,%d) holds %v, StoredRect says %v", o, x, y, p, back)
				}
			}
		}
	}
}

func TestDrawRefusesAMisSizedTarget(t *testing.T) {
	err := RightTop.Draw(image.NewRGBA(image.Rect(0, 0, 3, 2)), marked(3, 2))
	if err == nil {
		t.Fatal("a 3x2 target took a quarter turn of a 3x2 raster")
	}
}

func TestThenComposesAsDrawingTwice(t *testing.T) {
	src := marked(3, 2)
	draw := func(o Orientation, img *image.RGBA) *image.RGBA {
		w, h := o.Size(img.Rect.Dx(), img.Rect.Dy())
		dst := image.NewRGBA(image.Rect(0, 0, w, h))
		if err := o.Draw(dst, img); err != nil {
			t.Fatal(err)
		}
		return dst
	}
	for a := TopLeft; a <= LeftBottom; a++ {
		for b := TopLeft; b <= LeftBottom; b++ {
			if got, want := draw(a.Then(b), src), draw(b, draw(a, src)); !bytes.Equal(got.Pix, want.Pix) || got.Rect != want.Rect {
				t.Errorf("%d then %d = %d draws differently from drawing both", a, b, a.Then(b))
			}
		}
	}
	turn := TopLeft
	for range 4 {
		turn = turn.Then(RightTop)
	}
	if turn != TopLeft || TopRight.Then(TopRight) != TopLeft || RightTop.Then(LeftBottom) != TopLeft {
		t.Error("four quarter turns, two mirrors, or a turn and its inverse are not the identity")
	}
}

func TestAnInvalidOrientationPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("orientation 0 was taken as one")
		}
	}()
	Orientation(0).Size(1, 1)
}

func TestShownAgreesWithStoredRect(t *testing.T) {
	const w, h = 5, 3
	for o := TopLeft; o <= LeftBottom; o++ {
		sw, sh := o.Size(w, h)
		for y := range sh {
			for x := range sw {
				s := o.StoredRect(image.Rect(x, y, x+1, y+1), w, h)
				gx, gy := o.Shown(float64(s.Min.X)+0.5, float64(s.Min.Y)+0.5, w, h)
				if gx != float64(x)+0.5 || gy != float64(y)+0.5 {
					t.Fatalf("orientation %d shows stored %v's centre at (%v,%v), not shown (%d,%d)'s", o, s.Min, gx, gy, x, y)
				}
			}
		}
	}
}
