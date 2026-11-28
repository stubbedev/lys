package preview

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/waittest"

	"github.com/stubbedev/lys/desktop/mime"
	"github.com/stubbedev/lys/desktop/shell/treetest"
)

// testDB is a shared-mime-info fixture with the types the tests show,
// and a description for PNG.
func testDB(t *testing.T) *mime.Database {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"globs2": "50:image/png:*.png\n50:image/jpeg:*.jpg\n50:image/heif:*.heic\n50:image/jxl:*.jxl\n50:image/avif:*.avif\n" +
			"50:image/gif:*.gif\n50:image/svg+xml:*.svg\n50:text/plain:*.txt\n50:text/x-go:*.go\n" +
			"50:application/pdf:*.pdf\n50:video/x-y4m:*.y4m\n50:video/mp4:*.mp4\n50:audio/x-matroska:*.mka\n50:audio/mpeg:*.mp3\n50:application/x-thing:*.thing\n" +
			"50:text/markdown:*.md\n50:text/csv:*.csv\n50:text/tab-separated-values:*.tsv\n50:application/json:*.json\n50:application/x-desktop:*.desktop\n" +
			"50:application/zip:*.zip\n50:application/x-tar:*.tar\n50:application/x-compressed-tar:*.tar.gz\n50:application/x-bzip2-compressed-tar:*.tar.bz2\n" +
			"50:application/x-xz-compressed-tar:*.tar.xz\n50:application/x-lzma-compressed-tar:*.tar.lzma\n50:application/x-zstd-compressed-tar:*.tar.zst\n" +
			"50:application/x-7z-compressed:*.7z\n50:application/x-xz:*.xz\n50:application/epub+zip:*.epub\n50:font/ttf:*.ttf\n50:font/woff:*.woff\n50:font/woff2:*.woff2\n",
		"subclasses": "text/x-go text/plain\ntext/markdown text/plain\ntext/csv text/plain\ntext/tab-separated-values text/plain\napplication/json text/plain\n" +
			"application/x-desktop text/plain\nimage/svg+xml application/xml\napplication/xml text/plain\napplication/epub+zip application/zip\n",
		"image/png.xml": `<?xml version="1.0"?><mime-type xmlns="http://www.freedesktop.org/standards/shared-mime-info" type="image/png"><comment>PNG image</comment></mime-type>`,
	}
	for name, body := range files {
		path := filepath.Join(dir, "mime", name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return mime.Load([]string{dir})
}

// stamp is the modification time every fixture file has.
var stamp = time.Date(2026, time.March, 14, 9, 26, 0, 0, time.UTC)

func writeFile(t *testing.T, path string, data []byte) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	return path
}

// gradient is a w x h picture shading from red to blue.
func gradient(w, h int) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.NRGBA{R: uint8(255 * x / max(w-1, 1)), G: 96, B: uint8(255 * y / max(h-1, 1)), A: 255})
		}
	}
	return img
}

func pngData(t *testing.T, img image.Image) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

// y4mClip is a w x h YUV4MPEG2 clip of frames frames.
func y4mClip(w, h, frames int) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "YUV4MPEG2 W%d H%d F25:1 C420jpeg XCOLORRANGE=LIMITED\n", w, h)
	cw, ch := (w+1)/2, (h+1)/2
	for f := range frames {
		b.WriteString("FRAME\n")
		for i := range w * h {
			b.WriteByte(byte(16 + (i+f)%220))
		}
		b.Write(bytes.Repeat([]byte{128}, 2*cw*ch))
	}
	return b.Bytes()
}

// harness runs views on a loop the test pumps: what the views invoke
// queues until the test drains it, on the test's goroutine, and loads
// run on goroutines of their own, counted and watched.
type harness struct {
	t   *testing.T
	src Sources
	mu  sync.Mutex
	// queued is what the views invoked, which the next drain runs.
	queued []func()
	work   sync.WaitGroup
	// loads counts the loads started, ctxs keeps each path's last
	// load's context, and hold, while set, holds every load until it is
	// closed or its load is cancelled.
	loads int
	ctxs  map[string]context.Context
	hold  chan struct{}
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	h := &harness{t: t, src: Sources{MIME: testDB(t), Pictures: NewPictures(PictureBudget)}, ctxs: map[string]context.Context{}}
	treetest.IconTheme(t, folderIcon, "ld-file-symbolic", "ld-file-image-symbolic", "ld-file-text-symbolic", "ld-file-play-symbolic")
	t.Cleanup(func() {
		h.mu.Lock()
		if h.hold != nil {
			close(h.hold)
			h.hold = nil
		}
		h.mu.Unlock()
		h.work.Wait()
		h.drain()
	})
	return h
}

func (h *harness) invoke(fn func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.queued = append(h.queued, fn)
}

func (h *harness) spawn(fn func()) {
	h.work.Go(fn)
}

func (h *harness) load(ctx context.Context, req Request) Content {
	h.mu.Lock()
	h.loads++
	h.ctxs[req.Path] = ctx
	hold := h.hold
	h.mu.Unlock()
	if hold != nil {
		select {
		case <-hold:
		case <-ctx.Done():
		}
	}
	return Load(ctx, h.src, req)
}

// drain runs what was invoked, and what that invoked, until nothing is
// left.
func (h *harness) drain() {
	for {
		h.mu.Lock()
		queued := h.queued
		h.queued = nil
		h.mu.Unlock()
		if len(queued) == 0 {
			return
		}
		for _, fn := range queued {
			fn()
		}
	}
}

// config is the view configuration the tests start from.
func (h *harness) config() Config {
	return Config{
		Font: treetest.Font(h.t), Class: "preview-test", Image: image.Pt(320, 240), Text: image.Pt(320, 200),
		IconPx: 64, NameChars: 30, MIME: h.src.MIME, Invoke: h.invoke, Spawn: h.spawn, Load: h.load,
	}
}

func (h *harness) view(edit func(*Config)) *View {
	cfg := h.config()
	if edit != nil {
		edit(&cfg)
	}
	return New(cfg)
}

// show shows path in v and pumps the loop until its load landed.
func (h *harness) show(v *View, path string) {
	h.t.Helper()
	v.Show(path)
	h.until("the preview of "+filepath.Base(path), func() bool { return v.Renderer() != Loading })
}

// until pumps the loop until cond holds.
func (h *harness) until(what string, cond func() bool) {
	h.t.Helper()
	waittest.Until(h.t, func() bool {
		h.drain()
		return cond()
	}, what)
}

func (h *harness) started() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.loads
}

func (h *harness) ctxOf(path string) context.Context {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.ctxs[path]
}

func (h *harness) holdLoads() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.hold = make(chan struct{})
}

func (h *harness) releaseLoads() {
	h.mu.Lock()
	defer h.mu.Unlock()
	close(h.hold)
	h.hold = nil
}
