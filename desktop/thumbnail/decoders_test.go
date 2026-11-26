package thumbnail

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stubbedev/lys/lib/exif"
	"github.com/stubbedev/lys/lib/gpu"
)

// mediaFixture is a file of ui/media's test data.
func mediaFixture(name string) string {
	return filepath.Join("..", "..", "ui", "media", "testdata", name)
}

// orientedPNG is a w x h PNG, its left half red and its right half
// blue, recording orient in an eXIf chunk.
func orientedPNG(t *testing.T, w, h int, orient exif.Orientation) string {
	t.Helper()
	src := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			c := color.RGBA{220, 20, 20, 255}
			if x >= w/2 {
				c = color.RGBA{20, 20, 220, 255}
			}
			src.SetRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, src); err != nil {
		t.Fatal(err)
	}
	var tiff bytes.Buffer
	tiff.WriteString("II*\x00")
	le := binary.LittleEndian
	_ = binary.Write(&tiff, le, uint32(8))
	_ = binary.Write(&tiff, le, uint16(1))
	_ = binary.Write(&tiff, le, []uint16{0x0112, 3})
	_ = binary.Write(&tiff, le, uint32(1))
	_ = binary.Write(&tiff, le, []uint16{uint16(orient), 0})
	_ = binary.Write(&tiff, le, uint32(0))
	chunk := binary.BigEndian.AppendUint32(nil, uint32(tiff.Len()))
	chunk = append(chunk, "eXIf"...)
	chunk = append(chunk, tiff.Bytes()...)
	chunk = binary.BigEndian.AppendUint32(chunk, crc32.ChecksumIEEE(chunk[4:]))
	data := buf.Bytes()
	ihdrEnd := 8 + 4 + 4 + 13 + 4
	out := append(append(append([]byte{}, data[:ihdrEnd]...), chunk...), data[ihdrEnd:]...)
	path := filepath.Join(t.TempDir(), "turned.png")
	if err := os.WriteFile(path, out, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// isRed and isBlue tell the two halves of orientedPNG apart.
func isRed(c color.RGBA) bool  { return c.R > 150 && c.B < 100 }
func isBlue(c color.RGBA) bool { return c.B > 150 && c.R < 100 }

// A picture is thumbnailed in process as its EXIF orientation shows it:
// a 40x20 picture stored turned a quarter (RightTop) shows 20x40, the
// stored left half on top, and its shown size is the source's.
func TestAPictureIsThumbnailedUpright(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	path := orientedPNG(t, 40, 20, exif.RightTop)
	th := New(Config{})
	decodes := countDecodes(th)
	got, size, err := th.Fit(context.Background(), path, "image/png", 128, 128)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := th.Free(got); err != nil {
			t.Error(err)
		}
	}()
	if b := got.Image.Bounds(); b.Dx() != 20 || b.Dy() != 40 || size != image.Pt(20, 40) {
		t.Fatalf("the turned picture is %v of %v, want 20x40 of 20x40", b, size)
	}
	if top, bottom := got.Image.RGBAAt(10, 2), got.Image.RGBAAt(10, 37); !isRed(top) || !isBlue(bottom) {
		t.Errorf("top %v / bottom %v, want the stored left half (red) on top", top, bottom)
	}
	img, err := th.Image(context.Background(), path, "image/png", NormalPx)
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 20 || b.Dy() != 40 {
		t.Errorf("the turned picture's cached thumbnail is %v, want it upright at 20x40", b)
	}
	if n := decodes.Load(); n != 2 {
		t.Errorf("the picture was decoded %d times, want once for Fit and once for the cache", n)
	}
	stored, _, err := th.Fit(context.Background(), orientedPNG(t, 40, 20, exif.TopLeft), "image/png", 128, 128)
	if err != nil {
		t.Fatal(err)
	}
	if b := stored.Image.Bounds(); b.Dx() != 40 || b.Dy() != 20 {
		t.Errorf("a picture shown as stored is %v, want 40x20", b)
	}
	if err := th.Free(stored); err != nil {
		t.Error(err)
	}
}

// y4mVideo is a YUV4MPEG2 stream of frames w x h frames at 25 fps, the
// first black and the rest red.
func y4mVideo(t *testing.T, w, h, frames int) string {
	t.Helper()
	var b bytes.Buffer
	b.WriteString("YUV4MPEG2 W" + strconv.Itoa(w) + " H" + strconv.Itoa(h) + " F25:1 C420jpeg XCOLORRANGE=FULL\n")
	for i := range frames {
		y, u, v := byte(0), byte(128), byte(128)
		if i > 0 {
			y, u, v = 76, 85, 255
		}
		b.WriteString("FRAME\n")
		b.Write(bytes.Repeat([]byte{y}, w*h))
		b.Write(bytes.Repeat([]byte{u}, w*h/4))
		b.Write(bytes.Repeat([]byte{v}, w*h/4))
	}
	path := filepath.Join(t.TempDir(), "clip.y4m")
	if err := os.WriteFile(path, b.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// A video's thumbnail is a frame a tenth into it, decoded in process:
// past the black first frame of a ten-frame clip, the red second.
func TestAVideoIsThumbnailedByItsFrame(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	clip := y4mVideo(t, 64, 32, 10)
	th := New(Config{})
	img, err := th.Image(context.Background(), clip, "video/x-yuv4mpeg", NormalPx)
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 64 || b.Dy() != 32 {
		t.Fatalf("the clip's thumbnail is %v, want its 64x32 frame", b)
	}
	if r, g, b, _ := img.At(32, 16).RGBA(); r>>8 < 200 || g>>8 > 60 || b>>8 > 60 {
		t.Errorf("the clip's thumbnail is %d,%d,%d at its centre, want the red of its second frame", r>>8, g>>8, b>>8)
	}
	if live := th.core.budget.Held(); live != 0 {
		t.Errorf("the frame's decode holds %d bytes of the budget after it", live)
	}
}

// A file lys has no decoder for keeps its icon, each for a defined
// reason that is ErrNone and records no failure: a video no GPU here
// decodes, a video file holding sound alone, and a type lys decodes
// nothing of.
func TestNoDecoderIsADefinedReason(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	th := New(Config{})
	th.core.frames = failingFrames(fmt.Errorf("media: H.264 in clip.mp4: %w", gpu.ErrNoVideoDecoder))
	for _, c := range []struct {
		name, path, mimeType string
		want                 error
	}{
		{"a video no GPU here decodes", mediaFixture("h264-aac.mp4"), "video/mp4", ErrVideoOnGPU},
		{"a video file of sound alone", mediaFixture("tone.mka"), "video/x-matroska", ErrNoVideo},
		{"a type lys decodes nothing of", mediaFixture("tone.flac"), "audio/flac", ErrNoDecoder},
	} {
		if _, err := square(t, th, c.path, c.mimeType, 64); !errors.Is(err, c.want) || !errors.Is(err, ErrNone) {
			t.Errorf("%s = %v, want %v", c.name, err, c.want)
		}
	}
	if _, err := os.Stat(filepath.Join(cache, "thumbnails", "fail")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a file without a decoder left a fail record (%v)", err)
	}
}
