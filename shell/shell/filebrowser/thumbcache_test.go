package filebrowser

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/thumbnail"
	"github.com/stubbedev/lys/desktop/thumbnail/thumbnailtest"
	"github.com/stubbedev/lys/lib/logtest"
	"github.com/stubbedev/lys/lib/waittest"
)

func goroutineID() int64 {
	var buf [64]byte
	stack := buf[:runtime.Stack(buf[:], false)]
	stack = bytes.TrimPrefix(stack, []byte("goroutine "))
	id, err := strconv.ParseInt(string(stack[:bytes.IndexByte(stack, ' ')]), 10, 64)
	if err != nil {
		panic("filechooser: unreadable goroutine header: " + err.Error())
	}
	return id
}

// buildRows builds every row of the list, as scrolling through it does.
func (hs *harness) buildRows() {
	hs.do(func() {
		list := rows{hs.c, false}
		for i := range list.Len() {
			list.Row(i)
		}
	})
}

// countFound waits for the lookups of paths to end and reports how
// many found a thumbnail.
func (hs *harness) countFound(t *testing.T, paths []string) int {
	t.Helper()
	found := 0
	for _, p := range paths {
		hs.waitThumb(t, p, rowIconPx)
		hs.run(func() {
			if hs.c.thumbs.found[thumbKey{p, rowIconPx}] != nil {
				found++
			}
		})
	}
	return found
}

// openRows opens dir and builds every row, waiting for the lookups of
// paths to end, then cancels; it reports how many found a thumbnail.
func (hs *harness) openRows(t *testing.T, dir string, paths []string) int {
	t.Helper()
	hs.open(t, in(openFile, dir))
	hs.buildRows()
	found := hs.countFound(t, paths)
	hs.hide()
	return found
}

// A folder of photos is decoded once, off the loop: the rows build
// while no decode may run, the thumbnails go to the freedesktop cache
// once one may, fitted to its normal size, and the second opening reads
// them back without writing any again.
func TestDecodedThumbnailsAreCached(t *testing.T) {
	hs := newHarness(t, "")
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	dir := filepath.Join(hs.root, "photos")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	src := image.NewRGBA(image.Rect(0, 0, 400, 300))
	for y := range 300 {
		for x := range 400 {
			src.SetRGBA(x, y, color.RGBA{uint8(x), uint8(y), 90, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, nil); err != nil {
		t.Fatal(err)
	}
	paths := make([]string, 20)
	for i := range paths {
		paths[i] = filepath.Join(dir, "IMG_"+strconv.Itoa(4400+i)+".jpg")
		if err := os.WriteFile(paths[i], buf.Bytes(), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	budget, release := holdBudget(t, hs)
	hs.open(t, in(openFile, dir))
	hs.buildRows()
	waittest.Until(t, func() bool { return budget.Waiting() == thumbnail.Workers }, "the workers to wait for the budget")
	if _, err := os.Stat(filepath.Join(cache, "thumbnails")); err == nil {
		t.Error("a thumbnail was written while no decode could run")
	}
	release()
	if found := hs.countFound(t, paths); found != 20 {
		t.Errorf("%d photos got a thumbnail, want 20", found)
	}
	hs.hide()
	normal := filepath.Join(cache, "thumbnails", "normal")
	first := thumbnailtest.Cached(t, cache, "normal")
	if len(first) != 20 {
		t.Fatalf("%d normal thumbnails cached, want 20", len(first))
	}
	for name := range first {
		data, err := os.ReadFile(filepath.Join(normal, name))
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("the cached thumbnail %s: %v", name, err)
		}
		if b := img.Bounds(); b.Dx() != 128 || b.Dy() != 96 {
			t.Errorf("the cached thumbnail %s is %v, want 400x300 fitted to 128x96", name, b)
		}
	}

	if found := hs.openRows(t, dir, paths); found != 20 {
		t.Errorf("%d photos got a cached thumbnail, want 20", found)
	}
	again := thumbnailtest.Cached(t, cache, "normal")
	for name, id := range first {
		if again[name] != id {
			t.Errorf("the second opening wrote %s again", name)
		}
	}
	if len(again) != len(first) {
		t.Errorf("the second opening cached %d thumbnails, the first %d", len(again), len(first))
	}
}

// installThumbnailer installs the fake thumbnailer doing mode for PDFs
// over a fresh cache and gives hs a thumbnailer that finds it.
func installThumbnailer(t *testing.T, hs *harness, mode string) {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	switchThumbnailer(t, hs, mode, thumbnail.Config{})
}

// switchThumbnailer installs the fake thumbnailer doing mode for PDFs
// in place of the one installed, over the same cache, and gives hs a
// thumbnailer as cfg says that finds it.
func switchThumbnailer(t *testing.T, hs *harness, mode string, cfg thumbnail.Config) {
	t.Helper()
	thumbnailtest.Install(t, "application/pdf", thumbnailtest.Argv(t, mode, "%s", "%u", "%o"))
	hs.c.thumbnailer = thumbnail.New(cfg)
}

func pdfFile(t *testing.T, hs *harness) string {
	t.Helper()
	doc := filepath.Join(hs.root, "docs", "paper.pdf")
	if err := os.WriteFile(doc, []byte("%PDF-1.4"), 0o600); err != nil {
		t.Fatal(err)
	}
	return doc
}

// A PDF gets the thumbnail its installed thumbnailer makes, in the
// sandbox, and the second opening reads it from the cache: a
// thumbnailer that now fails changes nothing.
func TestThumbnailersMakeThumbnailsOnce(t *testing.T) {
	hs := newHarness(t, "")
	doc := pdfFile(t, hs)
	installThumbnailer(t, hs, thumbnailtest.Page)
	if hs.openRows(t, filepath.Dir(doc), []string{doc}) != 1 {
		t.Fatal("the PDF row got no thumbnail")
	}
	switchThumbnailer(t, hs, thumbnailtest.Fail, thumbnail.Config{})
	if hs.openRows(t, filepath.Dir(doc), []string{doc}) != 1 {
		t.Error("the PDF row got no cached thumbnail")
	}
}

// A thumbnailer that hangs is killed at its timeout, and the fail
// entry it leaves stops the next opening from running one again; a
// file changed since gets another try.
func TestHangingThumbnailerFailsOnce(t *testing.T) {
	hs := newHarness(t, "")
	doc := pdfFile(t, hs)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	switchThumbnailer(t, hs, thumbnailtest.Hang, thumbnail.Config{RunTimeout: 500 * time.Millisecond})
	if hs.openRows(t, filepath.Dir(doc), []string{doc}) != 0 {
		t.Error("a hung thumbnailer made a thumbnail")
	}
	switchThumbnailer(t, hs, thumbnailtest.Page, thumbnail.Config{})
	if hs.openRows(t, filepath.Dir(doc), []string{doc}) != 0 {
		t.Error("a file that failed ran a thumbnailer again")
	}
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(doc, later, later); err != nil {
		t.Fatal(err)
	}
	if hs.openRows(t, filepath.Dir(doc), []string{doc}) != 1 {
		t.Error("a changed file got no second try")
	}
}

// Without bubblewrap no installed thumbnailer runs, a PDF row gets no
// thumbnail, and the chooser says why once.
func TestNoSandboxRunsNoThumbnailer(t *testing.T) {
	hs := newHarness(t, "")
	doc := pdfFile(t, hs)
	t.Setenv("PATH", t.TempDir())
	installThumbnailer(t, hs, thumbnailtest.Page)
	logged := logtest.Capture(t)
	for range 2 {
		if hs.openRows(t, filepath.Dir(doc), []string{doc}) != 0 {
			t.Fatal("a thumbnailer ran without a sandbox")
		}
	}
	if n := strings.Count(logged.String(), "thumbnailers are not run without a sandbox: bubblewrap (bwrap) is not installed"); n != 1 {
		t.Errorf("the missing sandbox was logged %d times, want once:\n%s", n, logged.String())
	}
}
