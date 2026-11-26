// Package thumbnail makes and reads the thumbnails of files for every
// surface that shows them, all with lys's own decoders: the
// freedesktop thumbnail cache, pictures decoded in process within a
// decode budget and turned as their EXIF orientation says, documents
// drawn by lys-docthumb in a child process of its own, and videos'
// frames decoded in process; at most Workers at once, into rasters held
// off the Go heap. Nothing but lys-docthumb is ever run.
package thumbnail

import (
	"context"
	"errors"
	"fmt"
	"image"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/stubbedev/lys/lib/exif"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/memory-go"
	"golang.org/x/image/draw"

	"github.com/stubbedev/lys/desktop/dochelper"
	"github.com/stubbedev/lys/desktop/imagedecode"
	"github.com/stubbedev/lys/desktop/offheap"
)

// maxSourceBytes skips raster images too large to decode for a
// thumbnail.
const maxSourceBytes = 32 << 20

// Workers is how many thumbnails a Thumbnailer makes at once, and
// DecodeBudget how many bytes of source pixels their decodes hold at
// once by default: two 24-megapixel JPEGs, or one progressive one.
const (
	Workers      = 4
	DecodeBudget = 96 << 20
)

// ErrTooLarge is a raster source past the size a thumbnail is decoded
// from.
var ErrTooLarge = errors.New("thumbnail: image too large to thumbnail")

// ErrNone is a file with no cached thumbnail, nothing to make one with,
// or a fresh record that making one failed. The reasons nothing makes
// one are errors that are ErrNone too.
var ErrNone = errors.New("thumbnail: no thumbnail")

// The reasons a file has no thumbnail to make, each ErrNone: lys
// decodes no file of its type, its video decodes only on the GPU (which
// thumbnails are not made on), it holds no video track, or the
// document helper is not installed.
var (
	ErrNoDecoder  = fmt.Errorf("%w: lys decodes no file of this type", ErrNone)
	ErrVideoOnGPU = fmt.Errorf("%w: the video decodes only on the GPU", ErrNone)
	ErrNoVideo    = fmt.Errorf("%w: the file holds no video track", ErrNone)
	ErrNoHelper   = fmt.Errorf("%w: %s is not installed", ErrNone, dochelper.Program)
)

// rasterExts are the image types decoded in process (imagedecode's
// formats).
var rasterExts = []string{"png", "jpg", "jpeg", "gif", "webp", "bmp", "tiff", "tif", "tga", "pnm", "pbm", "pgm", "ppm", "ff", "farbfeld"}

// isRaster reports an image decoded in process, by extension.
func isRaster(path string) bool {
	return slices.Contains(rasterExts, strings.ToLower(strings.TrimPrefix(filepath.Ext(path), ".")))
}

// Raster is a thumbnail whose pixels its Thumbnailer holds off the Go
// heap, drawn by its identity; the Thumbnailer's Free frees them.
type Raster struct {
	Image render.StillImage
	pix   memory.Slice[byte]
}

// Config is what a Thumbnailer shares, how long lys-docthumb runs and
// where it is installed. A nil Budget is one of DecodeBudget bytes of
// its own, a zero RunTimeout DefaultRunTimeout, and an empty
// DocumentHelper the lys-docthumb next to the running program; a
// DocumentHelper must be an absolute path to a file called
// dochelper.Program.
type Config struct {
	Budget         *Budget
	RunTimeout     time.Duration
	DocumentHelper string
}

// Thumbnailer makes thumbnails off the loop: at most Workers at once,
// their decodes within its budget, into rasters it holds.
type Thumbnailer struct {
	core *core
}

// core is what a Thumbnailer holds. decode opens a raster image, store
// writes a cache file whole, and the document helper (found on first
// use) runs for at most runTimeout each.
type core struct {
	mem        memory.Shared
	workers    chan struct{}
	budget     *Budget
	decode     func(ctx context.Context, path string) (image.Image, error)
	store      func(path string, data []byte) error
	runTimeout time.Duration
	documents  func() (docHelper, error)
}

// New is a Thumbnailer as cfg says, finding lys-docthumb on first use.
// Without it documents have no thumbnail (ErrNoHelper), and why is
// logged once.
func New(cfg Config) *Thumbnailer {
	if cfg.Budget == nil {
		cfg.Budget = NewBudget(DecodeBudget)
	}
	if cfg.RunTimeout == 0 {
		cfg.RunTimeout = DefaultRunTimeout
	}
	c := &core{
		workers: make(chan struct{}, Workers), budget: cfg.Budget,
		decode: imagedecode.OpenContext, store: storeThumbnail, runTimeout: cfg.RunTimeout,
	}
	c.documents = sync.OnceValues(func() (docHelper, error) {
		h, err := newDocHelper(cfg.DocumentHelper)
		if err != nil {
			log.Printf("thumbnail: documents have no thumbnails, as %s is not run: %v", dochelper.Program, err)
			return docHelper{}, fmt.Errorf("%w: %w", ErrNoHelper, err)
		}
		return h, nil
	})
	return &Thumbnailer{core: c}
}

// Square is the file's thumbnail as an exact px square, centre-cropped
// to fill (every thumbnail is the same size, so rows align whatever the
// source's aspect ratio). It is the one the freedesktop cache holds for
// it; else, unless a fresh fail entry says making one failed before,
// one made with lys's own decoders (make), written to the cache for
// every app and the next reader. It
// waits its turn among the workers and for its decode's bytes in the
// budget, and gives up with ctx's error once ctx is done. The caller
// frees the result.
func (th *Thumbnailer) Square(ctx context.Context, path, mimeType string, px int) (Raster, error) {
	var out Raster
	err := th.withWorker(ctx, func() error {
		src, err := th.source(ctx, path, mimeType, px)
		if err == nil {
			out, err = th.squareCrop(src.img, px)
		}
		return err
	})
	return out, err
}

// Fit is the file's image fitted into a w x h box, its aspect kept and
// never scaled up, and the source's pixel size as shown: a raster image
// decoded whole within the budget, else the thumbnail the cache holds or
// one made at the box's size (whose source size is not known). The
// caller frees the result.
func (th *Thumbnailer) Fit(ctx context.Context, path, mimeType string, w, h int) (Raster, image.Point, error) {
	var out Raster
	var size image.Point
	err := th.withWorker(ctx, func() error {
		var src image.Image
		var err error
		if isRaster(path) {
			var info os.FileInfo
			var pic made
			if info, err = os.Stat(path); err == nil {
				pic, err = th.core.decodePicture(ctx, path, info, max(w, h))
			}
			src, size = pic.img, pic.size
		} else {
			var cached source
			cached, err = th.source(ctx, path, mimeType, flavorFor(max(w, h)).px)
			src = cached.img
		}
		if err == nil {
			out, err = th.fitRaster(src, w, h)
		}
		return err
	})
	return out, size, err
}

// Image is the file's thumbnail fitted into a px square, its aspect
// kept, on the Go heap: the one the cache holds, else one made as Square
// makes it.
func (th *Thumbnailer) Image(ctx context.Context, path, mimeType string, px int) (image.Image, error) {
	var out image.Image
	err := th.withWorker(ctx, func() error {
		src, err := th.source(ctx, path, mimeType, px)
		if err != nil {
			return err
		}
		b := src.img.Bounds()
		size := FitSize(b, px, px)
		if size == b.Size() {
			out = src.img
			return nil
		}
		dst := image.NewRGBA(image.Rectangle{Max: size})
		draw.ApproxBiLinear.Scale(dst, dst.Bounds(), src.img, b, draw.Src, nil)
		out = dst
		return nil
	})
	return out, err
}

// File is the cache file holding the file's thumbnail of at least px
// where there is one, else the one made as Square makes it and written
// there. Without a cache directory to write to there is no such file:
// ErrNone.
func (th *Thumbnailer) File(ctx context.Context, path, mimeType string, px int) (string, error) {
	var out string
	err := th.withWorker(ctx, func() error {
		src, err := th.source(ctx, path, mimeType, px)
		if err != nil {
			return err
		}
		if src.file == "" {
			return ErrNone
		}
		out = src.file
		return nil
	})
	return out, err
}

// withWorker runs fn once a worker is free, or gives up with ctx's
// error once ctx is done.
func (th *Thumbnailer) withWorker(ctx context.Context, fn func() error) error {
	select {
	case th.core.workers <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-th.core.workers }()
	return fn()
}

// source is an image a thumbnail is made from, and the cache file
// holding it ("" for none).
type source struct {
	img  image.Image
	file string
}

// source is the image a thumbnail of px is made from: the one the
// cache holds, else, unless a fresh fail entry says making one failed
// before, the one make makes, written to the cache fitted to px's size.
// A failure is recorded unless it is ctx's, ErrTooLarge or ErrNone.
func (th *Thumbnailer) source(ctx context.Context, path, mimeType string, px int) (source, error) {
	info, err := os.Stat(path)
	if err != nil {
		return source{}, err
	}
	tc, err := cacheFor(path, info)
	if err != nil {
		return source{}, err
	}
	if img, file, err := tc.lookup(px); err == nil {
		return source{img: img, file: file}, nil
	}
	if tc.failed() {
		return source{}, ErrNone
	}
	flavor := flavorFor(px)
	m, err := th.core.make(ctx, path, info, mimeType, flavor.px)
	if err != nil {
		if ctx.Err() == nil && !errors.Is(err, ErrTooLarge) && !errors.Is(err, ErrNone) {
			th.writeFail(tc)
		}
		return source{}, err
	}
	if tc.root == "" {
		return source{img: m.img}, nil
	}
	if err := th.cache(tc, m, flavor); err != nil {
		log.Printf("thumbnail: caching the thumbnail of %s: %v", tc.uri, err)
		return source{img: m.img}, nil
	}
	return source{img: m.img, file: tc.path(flavor.dir)}, nil
}

// made is a thumbnail's image and its source's pixel size as shown,
// zero where it is not known.
type made struct {
	img  image.Image
	size image.Point
}

// make makes the file's thumbnail at size with lys's own decoders: a
// raster image decoded in process, a document of a type lys-docthumb
// thumbnails drawn by it, a video's frame decoded in process. Any other
// file is ErrNoDecoder.
func (c *core) make(ctx context.Context, path string, info os.FileInfo, mimeType string, size int) (made, error) {
	switch {
	case isRaster(path):
		return c.decodePicture(ctx, path, info, size)
	case dochelper.Thumbnails(mimeType):
		h, err := c.documents()
		if err != nil {
			return made{}, err
		}
		img, err := h.thumbnail(ctx, c, path, size)
		return made{img: img}, err
	case isVideo(mimeType):
		return c.videoFrame(ctx, path)
	}
	return made{}, ErrNoDecoder
}

// decodePicture decodes a raster image within the byte budget, as its
// EXIF orientation shows it: a picture stored turned or mirrored is
// first scaled to fit a box side square, then turned upright.
func (c *core) decodePicture(ctx context.Context, path string, info os.FileInfo, box int) (made, error) {
	if info.Size() > maxSourceBytes {
		return made{}, ErrTooLarge
	}
	img, err := c.decodeWithin(ctx, path)
	if err != nil {
		return made{}, err
	}
	b := img.Bounds()
	orient := orientation(path)
	if orient == exif.TopLeft {
		return made{img: img, size: b.Size()}, nil
	}
	stored := image.NewRGBA(image.Rectangle{Max: FitSize(b, box, box)})
	draw.ApproxBiLinear.Scale(stored, stored.Rect, img, b, draw.Src, nil)
	w, h := orient.Size(stored.Rect.Dx(), stored.Rect.Dy())
	shown := image.NewRGBA(image.Rect(0, 0, w, h))
	if err := orient.Draw(shown, stored); err != nil {
		return made{}, fmt.Errorf("thumbnail: turning %s upright: %w", path, err)
	}
	sw, sh := orient.Size(b.Dx(), b.Dy())
	return made{img: shown, size: image.Pt(sw, sh)}, nil
}

// orientation is the orientation the picture at path records in its
// EXIF block, as the preview shows it: a block that does not parse is
// logged, and the picture is thumbnailed as stored.
func orientation(path string) exif.Orientation {
	f, err := os.Open(path) //nolint:gosec // the picture being thumbnailed
	if err != nil {
		log.Printf("thumbnail: reading the orientation of %s: %v", path, err)
		return exif.TopLeft
	}
	defer func() { _ = f.Close() }()
	o, err := exif.Read(f)
	if err != nil {
		log.Printf("thumbnail: the orientation of %s: %v", path, err)
		return exif.TopLeft
	}
	return o
}

// decodeWithin decodes the image file at path once the bytes its pixels
// take are free in the budget.
func (c *core) decodeWithin(ctx context.Context, path string) (image.Image, error) {
	need, err := imagedecode.DecodeBytes(path)
	if err != nil {
		return nil, err
	}
	release, err := c.budget.Acquire(ctx, need)
	if err != nil {
		return nil, err
	}
	defer release()
	return c.decode(ctx, path)
}

// cache writes m, fitted to flavor, as the file's thumbnail there.
func (th *Thumbnailer) cache(tc thumbCache, m made, flavor thumbSize) error {
	sourceRect := image.Rectangle{Max: m.size}
	if b := m.img.Bounds(); b.Dx() <= flavor.px && b.Dy() <= flavor.px {
		return th.encodeAndStore(tc, m.img, sourceRect, flavor)
	}
	fitted, err := th.fitRaster(m.img, flavor.px, flavor.px)
	if err != nil {
		return err
	}
	err = th.encodeAndStore(tc, fitted.Image, sourceRect, flavor)
	return errors.Join(err, th.Free(fitted))
}

func (th *Thumbnailer) encodeAndStore(tc thumbCache, img image.Image, source image.Rectangle, flavor thumbSize) error {
	data, err := tc.encode(img, source)
	if err != nil {
		return err
	}
	return th.core.store(tc.path(flavor.dir), data)
}

// FitSize is the size b scales to inside a w x h box, its aspect kept
// and never larger than b.
func FitSize(b image.Rectangle, w, h int) image.Point {
	if b.Dx() <= w && b.Dy() <= h {
		return b.Size()
	}
	if b.Dx()*h >= b.Dy()*w {
		return image.Pt(w, max(1, b.Dy()*w/b.Dx()))
	}
	return image.Pt(max(1, b.Dx()*h/b.Dy()), h)
}

// fitRaster is src scaled into a w x h box as FitSize says.
func (th *Thumbnailer) fitRaster(src image.Image, w, h int) (Raster, error) {
	size := FitSize(src.Bounds(), w, h)
	dst, pix, err := offheap.NewRGBA(&th.core.mem, size.X, size.Y)
	if err != nil {
		return Raster{}, err
	}
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
	return Raster{Image: render.NewStillImage(dst), pix: pix}, nil
}

// writeFail records that no thumbnail could be made for the file as
// it is now, so no reader tries again until it changes.
func (th *Thumbnailer) writeFail(tc thumbCache) {
	if tc.root == "" {
		return
	}
	data, err := tc.encode(image.NewRGBA(image.Rect(0, 0, 1, 1)), image.Rectangle{})
	if err == nil {
		err = th.core.store(tc.failPath(), data)
	}
	if err != nil {
		log.Printf("thumbnail: recording the failed thumbnail of %s: %v", tc.uri, err)
	}
}

// squareCrop scales the largest centred square of src to px.
func (th *Thumbnailer) squareCrop(src image.Image, px int) (Raster, error) {
	dst, pix, err := offheap.NewRGBA(&th.core.mem, px, px)
	if err != nil {
		return Raster{}, err
	}
	b := src.Bounds()
	side := min(b.Dx(), b.Dy())
	crop := image.Rect(0, 0, side, side).Add(b.Min).Add(image.Pt((b.Dx()-side)/2, (b.Dy()-side)/2))
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, crop, draw.Src, nil)
	return Raster{Image: render.NewStillImage(dst), pix: pix}, nil
}

// Free frees the rasters' pixels; nothing may draw them any more.
func (th *Thumbnailer) Free(rasters ...Raster) error {
	pix := make([]memory.Slice[byte], len(rasters))
	for i, r := range rasters {
		pix[i] = r.pix
	}
	return offheap.Free(&th.core.mem, pix...)
}

// Live is how many rasters th and its views made that are not freed.
func (th *Thumbnailer) Live() int { return th.core.mem.Stats().Live }
