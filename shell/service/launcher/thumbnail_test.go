package launcher

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/mime"
	"github.com/stubbedev/lys/desktop/thumbnail"
	"github.com/stubbedev/lys/desktop/thumbnail/thumbnailtest"
)

// testTypes is a MIME database that knows PDFs by name.
func testTypes(t *testing.T) *mime.Database {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "mime"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "mime", "globs2"), []byte("50:application/pdf:*.pdf\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return mime.Load([]string{dir})
}

// pngOf is a w x h PNG.
func pngOf(t *testing.T, w, h int) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// sizeOf is the size of the PNG at path.
func sizeOf(t *testing.T, path string) image.Point {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	return image.Pt(cfg.Width, cfg.Height)
}

// A picture's thumbnail is decoded in process and lands in the
// freedesktop cache; while the file is unchanged the cached one is
// reused, and once it changes it is made anew.
func TestGenerateDecodesAPictureAndCaches(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	file := writeTemp(t, "pic.png", pngOf(t, 40, 30))
	thumbs := NewThumbnailer(thumbnail.New(thumbnail.Config{}), testTypes(t))
	out, ok := thumbs.Generate(context.Background(), file)
	if !ok || filepath.Dir(out) != filepath.Join(cache, "thumbnails", "normal") {
		t.Fatalf("generate = %q %v", out, ok)
	}
	if got := sizeOf(t, out); got != image.Pt(40, 30) {
		t.Errorf("the thumbnail is %v, want the decoded 40x30", got)
	}
	first, err := os.Stat(out)
	if err != nil {
		t.Fatal(err)
	}
	if cached, ok := thumbs.Generate(context.Background(), file); !ok || cached != out {
		t.Errorf("an unchanged file's cached thumbnail = %q %v, want %q reused", cached, ok, out)
	}
	if again, err := os.Stat(out); err != nil || !os.SameFile(first, again) {
		t.Errorf("an unchanged file's thumbnail was written again (%v)", err)
	}

	if err := os.WriteFile(file, pngOf(t, 20, 10), 0o600); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(file, later, later); err != nil {
		t.Fatal(err)
	}
	if again, ok := thumbs.Generate(context.Background(), file); !ok || again != out {
		t.Fatalf("a changed file's thumbnail = %q %v", again, ok)
	}
	if got := sizeOf(t, out); got != image.Pt(20, 10) {
		t.Errorf("a changed file's thumbnail is %v, want the new 20x10", got)
	}
}

// A document's thumbnail is lys-docthumb's page; without lys-docthumb
// it has none, and the row keeps its icon.
func TestGenerateRunsTheDocumentHelper(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	doc := writeTemp(t, "paper.pdf", []byte("%PDF-1.4\x00\x01"))
	files := thumbnail.New(thumbnail.Config{DocumentHelper: thumbnailtest.Helper(t, thumbnailtest.Page)})
	out, ok := NewThumbnailer(files, testTypes(t)).Generate(ctx, doc)
	if !ok {
		t.Fatal("the PDF got no thumbnail from lys-docthumb")
	}
	if got := sizeOf(t, out); got.Y != thumbnail.NormalPx {
		t.Errorf("the PDF's thumbnail is %v, want the helper's page fitted to %d", got, thumbnail.NormalPx)
	}
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	if _, ok := NewThumbnailer(thumbnail.New(thumbnail.Config{}), testTypes(t)).Generate(ctx, doc); ok {
		t.Error("a PDF got a thumbnail without lys-docthumb")
	}
}

// A file whose helper fails gets no thumbnail, and while it is
// unchanged the helper is not run for it again.
func TestAFailedThumbnailIsNotRetried(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	doc := writeTemp(t, "paper.pdf", []byte("%PDF-1.4\x00\x01"))
	failing := thumbnail.New(thumbnail.Config{DocumentHelper: thumbnailtest.Helper(t, thumbnailtest.Fail)})
	if _, ok := NewThumbnailer(failing, testTypes(t)).Generate(ctx, doc); ok {
		t.Fatal("a failing helper made a thumbnail")
	}
	if failed, err := os.ReadDir(filepath.Join(cache, "thumbnails", "fail", "lys-shell")); err != nil || len(failed) != 1 {
		t.Fatalf("the failing helper left %d fail records (%v), want one", len(failed), err)
	}
	working := thumbnail.New(thumbnail.Config{DocumentHelper: thumbnailtest.Helper(t, thumbnailtest.Page)})
	if _, ok := NewThumbnailer(working, testTypes(t)).Generate(ctx, doc); ok {
		t.Error("a file that failed ran the helper again")
	}
}

func TestAFileWithNoThumbnailStillHasAnIconName(t *testing.T) {
	if MimeIcon("/tmp/notes.txt") == "" {
		t.Error("a mime icon must never be empty")
	}
}
