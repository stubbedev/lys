package imagedecode

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"io"
	"os"

	"github.com/stubbedev/lys/lib/exif"
	"github.com/stubbedev/memory-go"

	"github.com/stubbedev/lys/desktop/offheap"
	"github.com/stubbedev/lys/desktop/third_party/jpeg"
)

// Decoded is a decoded image whose pixels may live off the Go heap:
// Free releases them once nothing reads Image any more.
type Decoded struct {
	Image image.Image
	mem   *memory.Shared
	pix   memory.Slice[byte]
}

// Free frees the pixels a JPEG was decoded into and hands their pages
// back to the system; an image decoded onto the Go heap is left to the
// collector.
func (d Decoded) Free() error {
	if d.pix.Len() == 0 {
		return nil
	}
	return offheap.Free(d.mem, d.pix)
}

// decodeReadSize is the buffer a whole image is read through: a wallpaper
// is megabytes, which at bufio's 4 KiB default is hundreds of reads.
const decodeReadSize = 32 << 10

// OpenOffHeap decodes the image file at path as Open does, a JPEG
// straight into memory mem holds: its pixels until Free, and a
// progressive scan's coefficients only while it decodes.
func OpenOffHeap(mem *memory.Shared, path string) (Decoded, error) {
	f, err := os.Open(path) //nolint:gosec // the caller's image path
	if err != nil {
		return Decoded{}, err
	}
	defer f.Close()
	return decodeOffHeap(mem, f, path)
}

// OpenOffHeapOriented decodes the image file at path as OpenOffHeap
// does, turned as its EXIF orientation shows it: a picture stored
// turned or mirrored comes back upright as an RGBA raster in mem, the
// stored pixels freed.
func OpenOffHeapOriented(mem *memory.Shared, path string) (Decoded, error) {
	f, err := os.Open(path) //nolint:gosec // the caller's image path
	if err != nil {
		return Decoded{}, err
	}
	defer f.Close()
	o := readOrientation(f, path)
	stored, err := decodeOffHeap(mem, f, path)
	if err != nil || o == exif.TopLeft {
		return stored, err
	}
	shown, err := orient(mem, stored.Image, o)
	if err != nil {
		return Decoded{}, errors.Join(fmt.Errorf("turning %s as it shows: %w", path, err), stored.Free())
	}
	if err := stored.Free(); err != nil {
		return Decoded{}, errors.Join(fmt.Errorf("freeing the stored pixels of %s: %w", path, err), shown.Free())
	}
	return shown, nil
}

// decodeOffHeap is OpenOffHeap's decode of f, the file at path, read
// from its start.
func decodeOffHeap(mem *memory.Shared, f *os.File, path string) (Decoded, error) {
	br := bufio.NewReaderSize(f, decodeReadSize)
	if isJPEG(br) {
		img, pix, err := jpeg.Decode(br, mem)
		if err != nil {
			return Decoded{}, fmt.Errorf("decoding %s: %w", path, err)
		}
		return Decoded{Image: img, mem: mem, pix: pix}, nil
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return Decoded{}, fmt.Errorf("decoding %s: %w", path, err)
	}
	img, err := decodeFile(context.Background(), f, path)
	if err != nil {
		return Decoded{}, err
	}
	return Decoded{Image: img}, nil
}

// orientBandRows is how many rows of the shown raster orient turns at a
// time: the scratch it converts the stored pixels through is that many
// rows of the shown width, never a second whole raster.
const orientBandRows = 64

// orient is src as o shows it, an RGBA raster in mem.
func orient(mem *memory.Shared, src image.Image, o exif.Orientation) (Decoded, error) {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	sw, sh := o.Size(w, h)
	dst, pix, err := offheap.NewRGBA(mem, sw, sh)
	if err != nil {
		return Decoded{}, err
	}
	scratch, scratchPix, err := offheap.NewRGBA(mem, sw, min(orientBandRows, sh))
	if err != nil {
		return Decoded{}, errors.Join(err, offheap.Free(mem, pix))
	}
	for y := 0; y < sh; y += orientBandRows {
		end := min(y+orientBandRows, sh)
		r := o.StoredRect(image.Rect(0, y, sw, end), w, h)
		stored := &image.RGBA{Pix: scratch.Pix[:4*r.Dx()*r.Dy()], Stride: 4 * r.Dx(), Rect: image.Rect(0, 0, r.Dx(), r.Dy())}
		draw.Draw(stored, stored.Rect, src, b.Min.Add(r.Min), draw.Src)
		shown := &image.RGBA{Pix: dst.Pix[y*dst.Stride : end*dst.Stride], Stride: dst.Stride, Rect: image.Rect(0, 0, sw, end-y)}
		if err := o.Draw(shown, stored); err != nil {
			return Decoded{}, errors.Join(err, offheap.Free(mem, scratchPix, pix))
		}
	}
	if err := offheap.Free(mem, scratchPix); err != nil {
		return Decoded{}, errors.Join(err, offheap.Free(mem, pix))
	}
	return Decoded{Image: dst, mem: mem, pix: pix}, nil
}

// isJPEG reports whether br starts with the JPEG start-of-image marker.
func isJPEG(br *bufio.Reader) bool {
	magic, err := br.Peek(2)
	return err == nil && magic[0] == 0xff && magic[1] == 0xd8
}
