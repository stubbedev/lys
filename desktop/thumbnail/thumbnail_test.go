package thumbnail

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/waittest"
	"golang.org/x/image/draw"

	"github.com/stubbedev/lys/desktop/fileuri"
	"github.com/stubbedev/lys/desktop/thumbnail/thumbnailtest"
)

const pdfType = "application/pdf"

// writeImage writes img as a PNG at path.
func writeImage(t *testing.T, path string, img image.Image) {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

// photo is a w x h PNG in a fresh directory.
func photo(t *testing.T, w, h int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "photo.png")
	writeImage(t, path, image.NewRGBA(image.Rect(0, 0, w, h)))
	return path
}

// countDecodes counts the raster decodes th makes.
func countDecodes(th *Thumbnailer) *atomic.Int32 {
	var n atomic.Int32
	decode := th.core.decode
	th.core.decode = func(ctx context.Context, path string) (image.Image, error) {
		n.Add(1)
		return decode(ctx, path)
	}
	return &n
}

// pdfWith is a PDF in a fresh cache and a Thumbnailer whose
// lys-docthumb is the fake doing mode.
func pdfWith(t *testing.T, mode string) (string, *Thumbnailer) {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	doc := filepath.Join(t.TempDir(), "paper.pdf")
	if err := os.WriteFile(doc, []byte("%PDF-1.4"), 0o600); err != nil {
		t.Fatal(err)
	}
	return doc, New(Config{DocumentHelper: thumbnailtest.Helper(t, mode)})
}

// switchHelper makes the lys-docthumb th runs the fake doing mode.
func switchHelper(t *testing.T, th *Thumbnailer, mode string) {
	t.Helper()
	h, err := newDocHelper(thumbnailtest.Helper(t, mode))
	if err != nil {
		t.Fatal(err)
	}
	th.core.documents = func() (docHelper, error) { return h, nil }
}

// square is th's px thumbnail of path, freed; it reports its size.
func square(t *testing.T, th *Thumbnailer, path, mimeType string, px int) (image.Point, error) {
	t.Helper()
	got, err := th.Square(context.Background(), path, mimeType, px)
	if err != nil {
		return image.Point{}, err
	}
	size := got.Image.Bounds().Size()
	if err := th.Free(got); err != nil {
		t.Fatal(err)
	}
	return size, nil
}

func TestSquareIsACentreCroppedSquare(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	src := image.NewRGBA(image.Rect(0, 0, 16, 8))
	for y := range 8 {
		for x := range 16 {
			c := color.RGBA{200, 30, 40, 255}
			if x >= 8 {
				c = color.RGBA{20, 60, 220, 255}
			}
			src.Set(x, y, c)
		}
	}
	path := filepath.Join(t.TempDir(), "t.png")
	writeImage(t, path, src)
	th := New(Config{})
	ctx := context.Background()
	got, err := th.Square(ctx, path, "", 24)
	if err != nil {
		t.Fatal(err)
	}
	if got.Image.Bounds() != image.Rect(0, 0, 24, 24) {
		t.Fatalf("bounds = %v, want a 24px square", got.Image.Bounds())
	}
	if l, r := got.Image.RGBAAt(0, 12), got.Image.RGBAAt(23, 12); l.R <= l.B || r.B <= r.R {
		t.Errorf("left %v / right %v, want red then blue", l, r)
	}
	if err := th.Free(got); err != nil {
		t.Fatal(err)
	}
	if _, err := th.Square(ctx, filepath.Join(t.TempDir(), "missing.png"), "", 24); err == nil {
		t.Error("a missing file thumbnailed")
	}
	garbage := filepath.Join(t.TempDir(), "g.png")
	if err := os.WriteFile(garbage, []byte("not an image"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := th.Square(ctx, garbage, "", 24); err == nil {
		t.Error("garbage thumbnailed")
	}
	if live := th.Live(); live != 0 {
		t.Errorf("%d thumbnails left allocated", live)
	}
}

// A raster source past the size thumbnails are decoded from is refused
// before anything is decoded, and leaves no fail record (the next look
// costs a stat); one at the size is decoded.
func TestAnOversizedSourceIsRefused(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	th := New(Config{})
	decodes := countDecodes(th)
	big := filepath.Join(t.TempDir(), "big.png")
	if err := os.WriteFile(big, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Truncate(big, maxSourceBytes+1); err != nil {
		t.Fatal(err)
	}
	if _, err := square(t, th, big, "", 24); !errors.Is(err, ErrTooLarge) {
		t.Errorf("an oversized file = %v, want ErrTooLarge", err)
	}
	if n := decodes.Load(); n != 0 {
		t.Errorf("an oversized file was decoded %d times", n)
	}
	if _, err := os.Stat(filepath.Join(cache, "thumbnails", "fail")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("an oversized file left a fail record (%v)", err)
	}

	padded := photo(t, 40, 30)
	if err := os.Truncate(padded, maxSourceBytes); err != nil {
		t.Fatal(err)
	}
	if _, err := square(t, th, padded, "", 24); err != nil {
		t.Errorf("an image at the size = %v, want its thumbnail", err)
	}
	if n := decodes.Load(); n != 1 {
		t.Errorf("an image at the size was decoded %d times, want once", n)
	}
}

// A photo's thumbnail is the centred square of the full decode scaled
// down, pixel for pixel as the chooser drew it before the thumbnails
// moved off the heap.
func TestSquareMatchesTheFullDecode(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	src := image.NewRGBA(image.Rect(0, 0, 600, 400))
	for y := range 400 {
		for x := range 600 {
			src.SetRGBA(x, y, color.RGBA{uint8(x * 255 / 600), uint8(y * 255 / 400), uint8((x + y) % 256), 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "photo.jpg")
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	decoded, err := jpeg.Decode(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	want := image.NewRGBA(image.Rect(0, 0, 96, 96))
	draw.ApproxBiLinear.Scale(want, want.Bounds(), decoded, image.Rect(100, 0, 500, 400), draw.Src, nil)

	th := New(Config{})
	got, err := th.Square(context.Background(), path, "", 96)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := th.Free(got); err != nil {
			t.Error(err)
		}
	}()
	if got.Image.ID == 0 {
		t.Error("the thumbnail has no identity to forget it by")
	}
	if !bytes.Equal(got.Image.Pix, want.Pix) || got.Image.Rect != want.Rect {
		t.Error("the thumbnail differs from the full decode's centred square")
	}
}

func TestDecodesRasterImagesByExtension(t *testing.T) {
	for path, want := range map[string]bool{
		"/x/a.png": true, "/x/a.JPG": true, "/x/photo.jpeg": true, "/x/b.webp": true,
		"/x/c.tga": true, "/x/d.ppm": true, "/x/e.ff": true, "/x/IMG_1.HEIC": true, "/x/f.heif": true, "/x/g.jxl": true,
		"/x/a.avif": false, "/x/a.txt": false, "/x/a.svg": false, "/x/noext": false, "/x/a.pdf": false,
	} {
		if got := isRaster(path); got != want {
			t.Errorf("isRaster(%s) = %v", path, got)
		}
	}
}

// A decoded photo goes to the freedesktop cache, fitted to the size
// asked for and carrying the spec's URI, modification time and size, in
// a directory only the user may enter; while its Thumb::MTime matches
// it is read back instead of decoded again, and once the file changes
// it is decoded and written anew.
func TestACachedThumbnailIsReusedAndAnOutdatedOneRemade(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	path := photo(t, 400, 300)
	th := New(Config{})
	decodes := countDecodes(th)
	file, err := th.File(context.Background(), path, "", NormalPx)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	tc, err := cacheFor(path, info)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(cache, "thumbnails", "normal", tc.name); file != want {
		t.Errorf("the thumbnail is at %s, want %s", file, want)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if b := img.Bounds(); b.Dx() != 128 || b.Dy() != 96 {
		t.Errorf("the cached thumbnail is %v, want 400x300 fitted to 128x96", b)
	}
	if got := pngText(data, "Thumb::URI"); got != fileuri.FromPath(path) {
		t.Errorf("Thumb::URI %q, want %q", got, fileuri.FromPath(path))
	}
	if got := pngText(data, "Thumb::MTime"); got != strconv.FormatInt(info.ModTime().Unix(), 10) {
		t.Errorf("Thumb::MTime %q for %v", got, info.ModTime())
	}
	if got := pngText(data, "Thumb::Size"); got != strconv.FormatInt(info.Size(), 10) {
		t.Errorf("Thumb::Size %q, want %d", got, info.Size())
	}
	if dir, err := os.Stat(filepath.Dir(file)); err != nil || dir.Mode().Perm() != 0o700 {
		t.Errorf("the size directory is %v (%v), want 0700", dir.Mode(), err)
	}

	for range 3 {
		if _, err := square(t, th, path, "", 48); err != nil {
			t.Fatal(err)
		}
	}
	if n := decodes.Load(); n != 1 {
		t.Errorf("a cached photo was decoded %d times, want once", n)
	}

	writeImage(t, path, image.NewRGBA(image.Rect(0, 0, 60, 80)))
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatal(err)
	}
	got, err := th.Image(context.Background(), path, "", NormalPx)
	if err != nil {
		t.Fatal(err)
	}
	if n := decodes.Load(); n != 2 {
		t.Errorf("a changed photo was decoded %d times in all, want twice", n)
	}
	if b := got.Bounds(); b.Dx() != 60 || b.Dy() != 80 {
		t.Errorf("the changed photo's thumbnail is %v, want its new 60x80", b)
	}
	data, err = os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if got := pngText(data, "Thumb::MTime"); got != strconv.FormatInt(later.Unix(), 10) {
		t.Errorf("the rewritten thumbnail's Thumb::MTime is %q, want %d", got, later.Unix())
	}
}

// A thumbnail the cache holds is read while its Thumb::MTime matches,
// whatever made it; a stale one is no thumbnail.
func TestThumbnailFromTheCache(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	doc := filepath.Join(t.TempDir(), "report.pdf")
	if err := os.WriteFile(doc, []byte("%PDF"), 0o600); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(doc)
	if err != nil {
		t.Fatal(err)
	}
	tc, err := cacheFor(doc, info)
	if err != nil {
		t.Fatal(err)
	}
	put := func(mtime int64) {
		stale := tc
		stale.mtime = strconv.FormatInt(mtime, 10)
		data, err := stale.encode(image.NewRGBA(image.Rect(0, 0, 8, 8)), image.Rectangle{})
		if err != nil {
			t.Fatal(err)
		}
		if err := storeThumbnail(tc.path("large"), data); err != nil {
			t.Fatal(err)
		}
	}
	put(info.ModTime().Unix())
	th := New(Config{})
	if size, err := square(t, th, doc, "", 48); err != nil || size.X != 48 {
		t.Errorf("a cached thumbnail = %v, %v", size, err)
	}
	put(info.ModTime().Unix() - 60)
	if _, err := square(t, th, doc, "", 48); !errors.Is(err, ErrNone) {
		t.Errorf("a stale cached thumbnail = %v, want none", err)
	}
	if got := pngText([]byte("not a png"), "Thumb::MTime"); got != "" {
		t.Errorf("garbage carries %q", got)
	}
}

// A PDF gets the thumbnail lys-docthumb makes, and the next look reads
// it from the cache: a helper that now fails changes nothing.
func TestDocumentThumbnailsAreMadeOnce(t *testing.T) {
	doc, th := pdfWith(t, thumbnailtest.Page)
	size, err := square(t, th, doc, pdfType, 64)
	if err != nil || size.X != 64 {
		t.Fatalf("the PDF's thumbnail = %v, %v", size, err)
	}
	switchHelper(t, th, thumbnailtest.Fail)
	if _, err := square(t, th, doc, pdfType, 64); err != nil {
		t.Errorf("the PDF's cached thumbnail = %v", err)
	}
}

// A helper that fails leaves a fail record, and while the file is
// unchanged the helper does not run for it again; a file changed since
// gets another try. One that hangs is killed at its timeout and fails
// the same way.
func TestAFailingHelperIsNotRetried(t *testing.T) {
	for _, mode := range []string{thumbnailtest.Fail, thumbnailtest.Hang} {
		t.Run(mode, func(t *testing.T) {
			doc, th := pdfWith(t, mode)
			th.core.runTimeout = 500 * time.Millisecond
			if _, err := square(t, th, doc, pdfType, 64); err == nil || errors.Is(err, ErrNone) {
				t.Fatalf("a helper that does not finish = %v, want its failure", err)
			}
			info, err := os.Stat(doc)
			if err != nil {
				t.Fatal(err)
			}
			tc, err := cacheFor(doc, info)
			if err != nil {
				t.Fatal(err)
			}
			if !tc.failed() {
				t.Errorf("no fail record at %s", tc.failPath())
			}
			switchHelper(t, th, thumbnailtest.Page)
			th.core.runTimeout = DefaultRunTimeout
			if _, err := square(t, th, doc, pdfType, 64); !errors.Is(err, ErrNone) {
				t.Errorf("a file that failed = %v, want ErrNone without running the helper", err)
			}
			later := time.Now().Add(time.Hour)
			if err := os.Chtimes(doc, later, later); err != nil {
				t.Fatal(err)
			}
			if _, err := square(t, th, doc, pdfType, 64); err != nil {
				t.Errorf("a changed file got no second try: %v", err)
			}
		})
	}
}

// A file of a type lys decodes nothing of is ErrNoDecoder, which is
// ErrNone, and records no failure, so its next look as a type lys
// decodes makes one.
func TestNothingToThumbnailWithRecordsNoFailure(t *testing.T) {
	doc, th := pdfWith(t, thumbnailtest.Page)
	if _, err := square(t, th, doc, "application/x-nothing", 64); !errors.Is(err, ErrNoDecoder) || !errors.Is(err, ErrNone) {
		t.Fatalf("a type lys decodes nothing of = %v, want ErrNoDecoder", err)
	}
	if _, err := square(t, th, doc, pdfType, 64); err != nil {
		t.Errorf("the helper did not run after a look without one: %v", err)
	}
}

// The helper's output is decoded within the budget: while the budget
// is held whole it waits, and once it is free it is decoded.
func TestHelperOutputWaitsForTheBudget(t *testing.T) {
	doc, _ := pdfWith(t, thumbnailtest.Page)
	budget := NewBudget(DecodeBudget)
	th := New(Config{Budget: budget, DocumentHelper: thumbnailtest.Helper(t, thumbnailtest.Page)})
	release, err := budget.Acquire(context.Background(), budget.Total())
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() {
		_, err := square(t, th, doc, pdfType, 64)
		done <- err
	}()
	waittest.Until(t, func() bool { return budget.Waiting() == 1 }, "the helper's output to wait for the budget")
	select {
	case err := <-done:
		t.Fatalf("the output was decoded while the budget was held: %v", err)
	case <-time.After(quietWindow):
	}
	release()
	if err := waittest.For(t, done, "the output to be decoded"); err != nil {
		t.Fatal(err)
	}
}

// Fit is a raster image decoded whole into the box, its aspect kept and
// never scaled up, with its source size; a document is lys-docthumb's
// page, of no known source size.
func TestFitKeepsTheAspect(t *testing.T) {
	path := photo(t, 400, 300)
	doc, th := pdfWith(t, thumbnailtest.Page)
	got, size, err := th.Fit(context.Background(), path, "", 200, 200)
	if err != nil {
		t.Fatal(err)
	}
	if b := got.Image.Bounds(); b.Dx() != 200 || b.Dy() != 150 || size != image.Pt(400, 300) {
		t.Errorf("the fitted photo is %v of %v, want 200x150 of 400x300", b, size)
	}
	small, size, err := th.Fit(context.Background(), photo(t, 40, 30), "", 200, 200)
	if err != nil {
		t.Fatal(err)
	}
	if b := small.Image.Bounds(); b.Dx() != 40 || size != image.Pt(40, 30) {
		t.Errorf("a small photo fitted to %v, want it kept at 40x30", b)
	}
	page, size, err := th.Fit(context.Background(), doc, pdfType, 400, 400)
	if err != nil {
		t.Fatal(err)
	}
	if b := page.Image.Bounds(); b.Dx() != thumbnailtest.PageW || b.Dy() != thumbnailtest.PageH || size != (image.Point{}) {
		t.Errorf("the PDF's page is %v of %v, want the helper's %dx%d of no known size", b, size, thumbnailtest.PageW, thumbnailtest.PageH)
	}
	if err := th.Free(got, small, page); err != nil {
		t.Fatal(err)
	}
	if live := th.Live(); live != 0 {
		t.Errorf("%d rasters live after freeing every one", live)
	}
}

// Without a cache directory a thumbnail is still made, but there is no
// file holding it.
func TestNoCacheDirectoryHoldsNoFile(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "")
	path := photo(t, 40, 30)
	th := New(Config{})
	if _, err := th.File(context.Background(), path, "", NormalPx); !errors.Is(err, ErrNone) {
		t.Errorf("a thumbnail file without a cache = %v, want ErrNone", err)
	}
	img, err := th.Image(context.Background(), path, "", NormalPx)
	if err != nil || img.Bounds().Dx() != 40 {
		t.Errorf("a thumbnail without a cache = %v, %v", img, err)
	}
}

// Readers of a thumbnail being written see no file or a whole one,
// never part of it.
func TestThumbnailWritesAreWhole(t *testing.T) {
	path := filepath.Join(t.TempDir(), "thumbnails", "normal", "x.png")
	var big bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 256, 256))
	for i := range img.Pix {
		img.Pix[i] = uint8(i * 7)
	}
	if err := png.Encode(&big, img); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	var partial atomic.Int32
	go func() {
		defer close(done)
		for range 200 {
			data, err := os.ReadFile(path)
			if err == nil && !bytes.Equal(data, big.Bytes()) {
				partial.Add(1)
			}
		}
	}()
	for range 50 {
		if err := storeThumbnail(path, big.Bytes()); err != nil {
			t.Fatal(err)
		}
	}
	<-done
	if partial.Load() != 0 {
		t.Errorf("%d reads saw a partial thumbnail", partial.Load())
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 1 {
		t.Errorf("the size directory holds %d entries (%v), want the thumbnail alone", len(entries), err)
	}
}
