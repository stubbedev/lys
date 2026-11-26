package viewer

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/color"
	"image/color/palette"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/dochelper"
	"github.com/stubbedev/lys/desktop/docrender"
	"github.com/stubbedev/lys/desktop/fileops"
	"github.com/stubbedev/lys/desktop/mime"
	"github.com/stubbedev/lys/desktop/shell/preview"
	"github.com/stubbedev/lys/desktop/shell/treetest"
	"github.com/stubbedev/lys/lib/render"
	"github.com/stubbedev/lys/lib/testhome"
	"github.com/stubbedev/lys/lib/waittest"
	"github.com/stubbedev/lys/ui/app"
	"github.com/stubbedev/lys/ui/transfer"
	"github.com/stubbedev/lys/ui/widget"
)

func TestMain(m *testing.M) {
	if filepath.Base(os.Args[0]) == dochelper.Program {
		os.Exit(serveDocuments(os.Args[1:]))
	}
	testhome.Run(m)
}

func serveDocuments(args []string) int {
	s, ok, err := dochelper.ParseServing(args)
	if err == nil && !ok {
		err = fmt.Errorf("%s: not a serving command line: %q", dochelper.Program, args)
	}
	if err == nil {
		err = s.Memory.Limit()
	}
	if err == nil {
		err = dochelper.Serve(os.Stdin, os.Stdout, &docrender.Reader{})
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	return dochelper.ExitCode(err)
}

func installedHelper(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), dochelper.Program)
	if err := os.Symlink(exe, path); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestTheTestsReachNoHostSession(t *testing.T) { testhome.RequireNoHostSession(t) }

func testDB(t *testing.T) *mime.Database {
	t.Helper()
	dir := t.TempDir()
	globs := "50:image/png:*.png\n50:image/jpeg:*.jpg\n50:image/gif:*.gif\n50:video/mp4:*.mp4\n50:video/x-y4m:*.y4m\n" +
		"50:text/plain:*.txt\n50:application/pdf:*.pdf\n50:application/epub+zip:*.epub\n50:image/x-portable-pixmap:*.ppm\n"
	subclasses := "image/x-portable-pixmap image/x-portable-anymap\n"
	for name, body := range map[string]string{"globs2": globs, "subclasses": subclasses} {
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

func writeFile(t *testing.T, path string, data []byte) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func pngData(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.NRGBA{R: uint8(255 * x / max(w-1, 1)), G: 96, B: uint8(255 * y / max(h-1, 1)), A: 255})
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func animatedGIF(t *testing.T, frames int) []byte {
	t.Helper()
	g := &gif.GIF{}
	for i := range frames {
		img := image.NewPaletted(image.Rect(0, 0, 16, 16), palette.Plan9)
		for p := range img.Pix {
			img.Pix[p] = uint8(i * 40)
		}
		g.Image = append(g.Image, img)
		g.Delay = append(g.Delay, 10)
	}
	var b bytes.Buffer
	if err := gif.EncodeAll(&b, g); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

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

type toast struct {
	text string
	undo func()
}

type harness struct {
	t      *testing.T
	db     *mime.Database
	bin    fileops.Bin
	mu     sync.Mutex
	queued []func()
	work   sync.WaitGroup

	titles     []string
	toasts     []toast
	copied     []transfer.Content
	copyErr    error
	fullscreen []bool
	openedWith []string
	openedURIs []string
	focused    []widget.Widget
	helper     string
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	data := t.TempDir()
	h := &harness{
		t: t, db: testDB(t),
		bin: fileops.Bin{DataHome: data, UID: os.Getuid(), Volume: fileops.SystemVolume, Now: func() time.Time { return time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC) }},
	}
	treetest.IconTheme(t, "ld-file-symbolic", "ld-file-image-symbolic", "ld-file-play-symbolic", "ld-folder-symbolic", previousIcon, nextIcon, loopIcon, openWithIcon, copyIcon, trashIcon, outlineIcon, previousPageIcon, nextPageIcon, searchIcon, closeSearchIcon)
	t.Cleanup(func() {
		h.work.Wait()
		h.drain()
	})
	return h
}

func (h *harness) Invoke(fn func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.queued = append(h.queued, fn)
}

func (h *harness) Spawn(fn func()) { h.work.Go(fn) }

func (h *harness) SetTitle(title string) { h.titles = append(h.titles, title) }

func (h *harness) Toast(text string, undo func()) { h.toasts = append(h.toasts, toast{text, undo}) }

func (h *harness) Copy(content transfer.Content) error {
	if h.copyErr != nil {
		return h.copyErr
	}
	h.copied = append(h.copied, content)
	return nil
}

func (h *harness) SetFullscreen(on bool) { h.fullscreen = append(h.fullscreen, on) }

func (h *harness) OpenWith(_ widget.Boundser, path string) {
	h.openedWith = append(h.openedWith, path)
}

func (h *harness) OpenURI(uri string) { h.openedURIs = append(h.openedURIs, uri) }

func (h *harness) Focus(w widget.Widget) { h.focused = append(h.focused, w) }

func (h *harness) documents(path string) *dochelper.Client {
	if h.helper == "" {
		h.helper = installedHelper(h.t)
	}
	helper, err := dochelper.Locate(h.helper)
	if err != nil {
		return dochelper.NewClient(func() (*dochelper.Process, error) { return nil, err }, path, dochelper.RequestTimeout)
	}
	return dochelper.NewClient(dochelper.Starter(helper, dochelper.Serving{Memory: dochelper.ReadingMemory}), path, dochelper.RequestTimeout)
}

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

func (h *harness) until(what string, cond func() bool) {
	h.t.Helper()
	waittest.Until(h.t, func() bool {
		h.drain()
		return cond()
	}, what)
}

func (h *harness) viewer() *Viewer {
	db := h.db
	pictures := preview.NewPictures(preview.PictureBudget)
	v := New(Config{
		Font: treetest.Font(h.t), MIME: db, Bin: h.bin, Host: h, Documents: h.documents,
		Load: func(ctx context.Context, req preview.Request) preview.Content {
			return preview.Load(ctx, preview.Sources{MIME: db, Pictures: pictures}, req)
		},
	})
	h.t.Cleanup(v.Close)
	return v
}

func (h *harness) landed(v *Viewer) {
	h.t.Helper()
	h.until("the file shown", func() bool { return v.View().Renderer() != preview.Loading })
}

func (h *harness) open(v *Viewer, paths ...string) {
	h.t.Helper()
	c, err := Open(newOpens(h.db), paths)
	if err != nil {
		h.t.Fatal(err)
	}
	v.Open(c)
	h.landed(v)
}

func (h *harness) press(v *Viewer, keys string) bool {
	h.t.Helper()
	a, err := app.ParseAccel(keys)
	if err != nil {
		h.t.Fatal(err)
	}
	handled := v.Key(a)
	h.landed(v)
	return handled
}

func current(t *testing.T, v *Viewer) string {
	t.Helper()
	path, ok := v.Files().Current()
	if !ok {
		t.Fatal("the viewer shows no file")
	}
	return filepath.Base(path)
}

func layOut(v *Viewer) { v.Root().Arrange(render.Rect{W: 1200, H: 900}) }
