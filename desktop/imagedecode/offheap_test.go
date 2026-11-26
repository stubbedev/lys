package imagedecode

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stubbedev/lys/lib/exif"
	"github.com/stubbedev/memory-go"

	"github.com/stubbedev/lys/desktop/imagedecode/imagedecodetest"
)

func photoJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), uint8(x ^ y), 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func wantNothingHeld(t *testing.T, mem *memory.Shared) {
	t.Helper()
	if st := mem.Stats(); st.Live != 0 {
		t.Errorf("%d allocations of %d bytes still held", st.Live, st.Bytes)
	}
}

func TestOpenOffHeapDecodesAJPEGOffTheHeap(t *testing.T) {
	const w, h = 1024, 768
	path := write(t, "photo.jpg", photoJPEG(t, w, h))
	want, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	onHeap := allocated(t, func() error { _, err := Open(path); return err })
	if onHeap < w*h*3/2 {
		t.Fatalf("Open allocated %d bytes, less than the %dx%d image: the measurement is off", onHeap, w, h)
	}
	var mem memory.Shared
	var got Decoded
	offHeap := allocated(t, func() error {
		var err error
		got, err = OpenOffHeap(&mem, path)
		return err
	})
	if offHeap > 64<<10 {
		t.Errorf("OpenOffHeap allocated %d bytes on the heap, want at most 64 KiB (Open: %d)", offHeap, onHeap)
	}
	if !reflect.DeepEqual(got.Image, want) {
		t.Error("OpenOffHeap decodes other pixels than Open")
	}
	if st := mem.Stats(); st.Live != 1 {
		t.Errorf("%d allocations held, want the image's", st.Live)
	}
	if err := got.Free(); err != nil {
		t.Fatal(err)
	}
	wantNothingHeld(t, &mem)
}

func TestOpenOffHeapDecodesOtherFormatsAsOpenDoes(t *testing.T) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, sample()); err != nil {
		t.Fatal(err)
	}
	path := write(t, "a.png", buf.Bytes())
	var mem memory.Shared
	got, err := OpenOffHeap(&mem, path)
	if err != nil {
		t.Fatal(err)
	}
	if rgba(got.Image, 1, 0) != (color.NRGBA{B: 255, A: 255}) {
		t.Errorf("pixel 1 = %v", rgba(got.Image, 1, 0))
	}
	wantNothingHeld(t, &mem)
	if err := got.Free(); err != nil {
		t.Errorf("freeing a heap image: %v", err)
	}
}

func TestOpenOffHeapRefusesWhatIsNoImageAndHoldsNothing(t *testing.T) {
	data := photoJPEG(t, 64, 64)
	dir := t.TempDir()
	for name, path := range map[string]string{
		"truncated JPEG": write(t, "cut.jpg", data[:len(data)/2]),
		"garbage":        write(t, "garbage", []byte("not an image at all")),
		"missing":        filepath.Join(dir, "missing.jpg"),
	} {
		var mem memory.Shared
		got, err := OpenOffHeap(&mem, path)
		if err == nil {
			t.Errorf("%s: decoded a %v image", name, got.Image.Bounds())
		}
		if got.Image != nil {
			t.Errorf("%s: an image came back with the error", name)
		}
		wantNothingHeld(t, &mem)
	}
}

// asShown is img turned whole as o shows it, on the heap.
func asShown(t *testing.T, img image.Image, o exif.Orientation) *image.RGBA {
	t.Helper()
	b := img.Bounds()
	src := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(src, src.Rect, img, b.Min, draw.Src)
	w, h := o.Size(b.Dx(), b.Dy())
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	if err := o.Draw(dst, src); err != nil {
		t.Fatal(err)
	}
	return dst
}

// A picture whose EXIF block records a turn or a mirror decodes upright,
// pixel for pixel as turning the whole stored picture shows it, over
// more rows than one band; only the shown raster stays held, and Free
// lets it go. A picture stored as it shows decodes as OpenOffHeap does.
func TestOpenOffHeapOrientedTurnsThePictureAsItShows(t *testing.T) {
	const w, h = 150, 97
	plain := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			plain.Set(x, y, color.RGBA{uint8(x), uint8(y), uint8(x ^ y), 255})
		}
	}
	for o := exif.TopLeft; o <= exif.LeftBottom; o++ {
		path := write(t, "photo.jpg", imagedecodetest.OrientedJPEG(t, plain, uint16(o)))
		stored, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		var mem memory.Shared
		got, err := OpenOffHeapOriented(&mem, path)
		if err != nil {
			t.Fatalf("orientation %d: %v", o, err)
		}
		if st := mem.Stats(); st.Live != 1 {
			t.Errorf("orientation %d: %d allocations held, want the shown raster's", o, st.Live)
		}
		if o == exif.TopLeft {
			if !reflect.DeepEqual(got.Image, stored) {
				t.Error("a picture stored as it shows decodes other pixels than OpenOffHeap")
			}
		} else if want := asShown(t, stored, o); !reflect.DeepEqual(got.Image, want) {
			t.Errorf("orientation %d shows a %v raster unlike the stored picture turned whole, %v", o, got.Image.Bounds(), want.Rect)
		}
		if err := got.Free(); err != nil {
			t.Fatal(err)
		}
		wantNothingHeld(t, &mem)
	}
}

// A turned picture that does not decode is an error, and holds nothing.
func TestOpenOffHeapOrientedRefusesWhatIsNoImage(t *testing.T) {
	data := imagedecodetest.OrientedJPEG(t, sample(), uint16(exif.RightTop))
	var mem memory.Shared
	got, err := OpenOffHeapOriented(&mem, write(t, "cut.jpg", data[:len(data)/2]))
	if err == nil || got.Image != nil {
		t.Errorf("a truncated turned JPEG decoded to %v, error %v", got.Image, err)
	}
	wantNothingHeld(t, &mem)
}
