// Package thumbnail makes and reads the thumbnails of files for every
// surface that shows them: the freedesktop thumbnail cache, raster
// images decoded in process within a decode budget, and installed
// .thumbnailer programs (or a surface's own command) run in a
// bubblewrap and seccomp sandbox, at most Workers at once, into rasters
// held off the Go heap.
package thumbnail

import (
	"context"
	"errors"
	"image"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/memory-go"
	"golang.org/x/image/draw"

	"github.com/stubbedev/lys/desktop/imagedecode"
	"github.com/stubbedev/lys/desktop/mime"
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
// or a fresh record that making one failed.
var ErrNone = errors.New("thumbnail: no thumbnail")

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

// Config is what a Thumbnailer shares and how long its thumbnailers
// run. A nil Budget is one of DecodeBudget bytes of its own, a zero
// RunTimeout DefaultRunTimeout.
type Config struct {
	Budget     *Budget
	RunTimeout time.Duration
}

// Thumbnailer makes thumbnails off the loop: at most Workers at once,
// their decodes within its budget, into rasters it holds. Every view
// WithCommand makes shares them.
type Thumbnailer struct {
	core *core
	// command, when set, makes every thumbnail in place of the in
	// process decode and the installed thumbnailers; failApp is the
	// directory under fail/ recording what it could not make.
	command *maker
	failApp string
}

// core is what a Thumbnailer's views share. decode opens a raster
// image, store writes a cache file whole, and the installed
// thumbnailers and the sandbox (loaded on first use) run for at most
// runTimeout each.
type core struct {
	mem        memory.Shared
	workers    chan struct{}
	budget     *Budget
	decode     func(ctx context.Context, path string) (image.Image, error)
	store      func(path string, data []byte) error
	runTimeout time.Duration
	installed  func() installedThumbnailers
	sandbox    func() (sandbox, error)
}

// installedThumbnailers are the thumbnailers per MIME type and the
// sandbox they run in.
type installedThumbnailers struct {
	sandbox sandbox
	byType  map[string]thumbnailerEntry
}

// New is a Thumbnailer as cfg says, finding the installed thumbnailers
// in the XDG data directories on first use.
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
	c.sandbox = sync.OnceValues(func() (sandbox, error) {
		sb, err := newSandbox(context.Background())
		if err != nil {
			log.Printf("thumbnail: thumbnailers are not run without a sandbox: %v", err)
		}
		return sb, err
	})
	c.installed = sync.OnceValue(func() installedThumbnailers {
		sb, err := c.sandbox()
		if err != nil {
			return installedThumbnailers{}
		}
		return installedThumbnailers{sandbox: sb, byType: loadThumbnailers(mime.DataDirs())}
	})
	return &Thumbnailer{core: c, failApp: software}
}

// WithCommand is a view of th whose thumbnails cmd makes, for every
// file, in the same sandbox, workers and budget; name is cmd's in
// errors and under fail/, so what it fails at records nothing against
// th's own thumbnailers. Thumbnails it made are cached for every app,
// and every app's are read back.
func (th *Thumbnailer) WithCommand(name string, cmd Command) *Thumbnailer {
	m := commandMaker(name, cmd)
	return &Thumbnailer{core: th.core, command: &m, failApp: software + "-" + name}
}

// Square is the file's thumbnail as an exact px square, centre-cropped
// to fill (every thumbnail is the same size, so rows align whatever the
// source's aspect ratio). It is the one the freedesktop cache holds for
// it; else, unless a fresh fail entry says making one failed before, a
// raster image decoded or the output of the thumbnailer installed for
// mimeType, written to the cache for every app and the next reader. It
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
// never scaled up, and the source's pixel size: a raster image decoded
// whole within the budget, else the thumbnail the cache holds or a
// thumbnailer makes at the box's size (whose source size is not known).
// The caller frees the result.
func (th *Thumbnailer) Fit(ctx context.Context, path, mimeType string, w, h int) (Raster, image.Point, error) {
	var out Raster
	var size image.Point
	err := th.withWorker(ctx, func() error {
		var src image.Image
		var err error
		if th.command == nil && isRaster(path) {
			var info os.FileInfo
			if info, err = os.Stat(path); err == nil {
				src, err = th.decodeRaster(ctx, path, info)
			}
			if err == nil {
				size = src.Bounds().Size()
			}
		} else {
			var made source
			made, err = th.source(ctx, path, mimeType, flavorFor(max(w, h)).px)
			src = made.img
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
// before, the command's output, a raster image decoded or the output of
// the thumbnailer installed for mimeType, written to the cache fitted to
// px's size.
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
	if tc.failed(th.failApp) {
		return source{}, ErrNone
	}
	img, err := th.make(ctx, path, info, mimeType, flavorFor(px).px)
	if err != nil {
		if ctx.Err() == nil && !errors.Is(err, ErrTooLarge) && !errors.Is(err, ErrNone) {
			th.writeFail(tc)
		}
		return source{}, err
	}
	if tc.root == "" {
		return source{img: img}, nil
	}
	flavor := flavorFor(px)
	if err := th.cache(tc, img, flavor); err != nil {
		log.Printf("thumbnail: caching the thumbnail of %s: %v", tc.uri, err)
		return source{img: img}, nil
	}
	return source{img: img, file: tc.path(flavor.dir)}, nil
}

// make makes the file's thumbnail at size: with the command when there
// is one, else decoded when it is a raster image, else with the
// thumbnailer installed for mimeType. With none of them it is ErrNone.
func (th *Thumbnailer) make(ctx context.Context, path string, info os.FileInfo, mimeType string, size int) (image.Image, error) {
	if th.command != nil {
		sb, err := th.core.sandbox()
		if err != nil {
			return nil, ErrNone
		}
		return th.core.run(ctx, sb, *th.command, path, size)
	}
	if isRaster(path) {
		return th.decodeRaster(ctx, path, info)
	}
	installed := th.core.installed()
	entry, ok := installed.byType[mimeType]
	if !ok {
		return nil, ErrNone
	}
	return th.core.run(ctx, installed.sandbox, entry.maker(), path, size)
}

// decodeRaster decodes a raster image within the byte budget.
func (th *Thumbnailer) decodeRaster(ctx context.Context, path string, info os.FileInfo) (image.Image, error) {
	if info.Size() > maxSourceBytes {
		return nil, ErrTooLarge
	}
	return th.core.decodeWithin(ctx, path)
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

// cache writes src, fitted to flavor, as the file's thumbnail there.
func (th *Thumbnailer) cache(tc thumbCache, src image.Image, flavor thumbSize) error {
	if b := src.Bounds(); b.Dx() <= flavor.px && b.Dy() <= flavor.px {
		return th.encodeAndStore(tc, src, src.Bounds(), flavor)
	}
	fitted, err := th.fitRaster(src, flavor.px, flavor.px)
	if err != nil {
		return err
	}
	err = th.encodeAndStore(tc, fitted.Image, src.Bounds(), flavor)
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
		err = th.core.store(tc.failPath(th.failApp), data)
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
