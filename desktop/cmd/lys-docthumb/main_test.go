package main

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"testing"

	"github.com/stubbedev/lys/lib/testbuild"
	"github.com/stubbedev/lys/lib/testhome"

	"github.com/stubbedev/lys/desktop/dochelper"
	"github.com/stubbedev/lys/desktop/thumbnail"
)

// TestMain runs the package against a scratch home; started as
// lys-docthumb, the test binary is the helper instead.
func TestMain(m *testing.M) {
	if filepath.Base(os.Args[0]) == dochelper.Program {
		os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
	}
	testhome.Run(m)
}

// installed is the test binary installed as lys-docthumb in a fresh
// directory.
func installed(t *testing.T) string {
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

func fixture(name string) string { return filepath.Join("..", "..", "docrender", "testdata", name) }

// helper runs the test binary as lys-docthumb on req and is its exit
// status and output.
func helper(t *testing.T, req dochelper.Request) (int, []byte) {
	t.Helper()
	cmd := exec.Command(installed(t), req.Args()...)
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		t.Fatalf("running the helper: %v", err)
	}
	return cmd.ProcessState.ExitCode(), out
}

func request(t *testing.T, input string, size int, memory int64) dochelper.Request {
	t.Helper()
	return dochelper.Request{Input: input, Output: filepath.Join(t.TempDir(), "page.png"), Size: size, Memory: memory}
}

// The helper writes page 1 fitted into the size asked for, within the
// memory the thumbnailer gives it.
func TestTheHelperWritesPageOne(t *testing.T) {
	req := request(t, fixture("jbig2.pdf"), 200, thumbnail.DocumentMemory)
	if code, out := helper(t, req); code != 0 {
		t.Fatalf("the helper exited %d: %s", code, out)
	}
	data, err := os.ReadFile(req.Output)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("the helper's output: %v", err)
	}
	if got := img.Bounds().Size(); got.Y != 200 || got.X >= 200 {
		t.Errorf("page 1 of a portrait PDF in a 200 px box is %v", got)
	}
}

// A damaged document, an encrypted one and a page past the helper's
// memory exit with the status that tells their error, and write nothing.
func TestTheHelperExitsWithWhatWentWrong(t *testing.T) {
	data, err := os.ReadFile(fixture("cairo.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	cut := filepath.Join(t.TempDir(), "cut.pdf")
	if err := os.WriteFile(cut, data[:3000], 0o600); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name string
		req  dochelper.Request
		want error
	}{
		{"truncated", request(t, cut, 128, thumbnail.DocumentMemory), dochelper.ErrDamaged},
		{"encrypted", request(t, fixture("encrypted.pdf"), 128, thumbnail.DocumentMemory), dochelper.ErrEncrypted},
		{"past its memory", request(t, fixture("cairo.pdf"), dochelper.MaxSize, 48<<20), dochelper.ErrTooLarge},
	} {
		t.Run(c.name, func(t *testing.T) {
			code, out := helper(t, c.req)
			if err := dochelper.ExitError(code, out); !errors.Is(err, c.want) {
				t.Errorf("the helper exited %d (%v), want %v: %s", code, err, c.want, out)
			}
			if _, err := os.Stat(c.req.Output); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("the failed helper left %s: %v", c.req.Output, err)
			}
		})
	}
	code, out := helper(t, request(t, fixture("cairo.pdf"), dochelper.MaxSize, thumbnail.DocumentMemory))
	if code != 0 {
		t.Errorf("the page that ran out of 48 MiB, within DocumentMemory: exited %d: %s", code, out)
	}
}

// A request the helper cannot read fails before it opens anything.
func TestTheHelperRefusesABadRequest(t *testing.T) {
	var stderr bytes.Buffer
	if code := run([]string{"-size", "0", "--", "in", "out"}, nil, nil, &stderr); code == 0 || stderr.Len() == 0 {
		t.Errorf("a bad request exited %d saying %q", code, stderr.String())
	}
}

// The thumbnailer runs the helper as a child process for PDFs and
// EPUBs, and its failures reach the caller as their errors.
func TestTheThumbnailerRunsTheHelper(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	th := thumbnail.New(thumbnail.Config{DocumentHelper: installed(t)})
	for _, c := range []struct{ file, mimeType string }{{"cairo.pdf", "application/pdf"}, {"blocks.epub", "application/epub+zip"}} {
		img, err := th.Image(context.Background(), fixture(c.file), c.mimeType, 128)
		if err != nil {
			t.Fatalf("the thumbnail of %s: %v", c.file, err)
		}
		if got := img.Bounds().Size(); got.Y != 128 || got.X >= 128 {
			t.Errorf("the thumbnail of %s is %v, want a portrait page 128 px tall", c.file, got)
		}
	}
	if _, err := th.Image(context.Background(), fixture("encrypted.pdf"), "application/pdf", 128); !errors.Is(err, dochelper.ErrEncrypted) {
		t.Errorf("the thumbnail of an encrypted PDF = %v, want ErrEncrypted", err)
	}
}

// The helper is the binary that links folio, which the thumbnailer's
// own test proves it does not.
func TestTheHelperLinksTheRenderer(t *testing.T) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		t.Fatal("the test binary carries no build information")
	}
	for _, m := range info.Deps {
		if m.Path == "github.com/gen2brain/folio" {
			if m.Version != "v0.3.1" {
				t.Errorf("folio is %s, want the pinned v0.3.1", m.Version)
			}
			return
		}
	}
	t.Error("lys-docthumb does not link folio")
}

// reader is a client of the test binary serving the fixture as
// lys-docthumb does for a reader, holding memory.
func reader(t *testing.T, file string, memory dochelper.Memory) *dochelper.Client {
	t.Helper()
	doc, err := filepath.Abs(fixture(file))
	if err != nil {
		t.Fatal(err)
	}
	c := dochelper.NewClient(dochelper.Starter(installed(t), dochelper.Serving{Memory: memory}), doc, dochelper.RequestTimeout)
	t.Cleanup(func() { _ = c.Close() })
	return c
}

// Served, the helper says what the document holds, draws its tiles and
// reads its pages' text and links, as docrender does.
func TestTheHelperServesADocument(t *testing.T) {
	c := reader(t, "reading.pdf", dochelper.ReadingMemory)
	info, err := c.Info()
	if err != nil {
		t.Fatal(err)
	}
	if info.Pages() != 3 || info.Label(0) != "i" || len(info.Outline) != 2 {
		t.Errorf("reading.pdf holds %+v", info)
	}
	tile := dochelper.Tile{Page: 0, Scale: 1, Rect: image.Rect(60, 40, 320, 140)}
	img, err := c.Render(tile)
	if err != nil {
		t.Fatal(err)
	}
	if img.Rect.Size() != tile.Rect.Size() || img.Pix[0] != 255 {
		t.Errorf("the tile is %v starting %v, want %v on white", img.Rect, img.Pix[:4], tile.Rect.Size())
	}
	text, err := c.Text(1)
	if err != nil || len(text.Lines) != 2 || len(text.Lines[0].Chars) != len("Page two holds a needle") {
		t.Errorf("page 2 says %+v, %v", text, err)
	}
	links, err := c.Links(0)
	if err != nil || len(links) != 2 || links[0].Page != 2 || links[1].URI != "https://example.org/" {
		t.Errorf("page 1 leads %+v, %v", links, err)
	}
	if err := c.Close(); err != nil {
		t.Errorf("closing: %v", err)
	}
}

// A tile past the served helper's memory fails with ErrTooLarge, and
// the helper started again draws the next.
func TestAServedTilePastTheHelpersMemoryIsTooLarge(t *testing.T) {
	if testbuild.Race {
		t.Skip("the race detector's allocator does not fit the 160 MiB the helper is held to")
	}
	c := reader(t, "cairo.pdf", dochelper.Memory{Hard: 160 << 20, Soft: 48 << 20})
	info, err := c.Info()
	if err != nil {
		t.Fatal(err)
	}
	const scale = 9
	page := info.Sizes[0].At(scale)
	big := dochelper.Tile{Scale: scale, Rect: image.Rect(0, 0, min(page.X, dochelper.MaxSize), min(page.Y, dochelper.MaxSize))}
	if _, err := c.Render(big); !errors.Is(err, dochelper.ErrTooLarge) {
		t.Fatalf("a %v tile in 160 MiB = %v, want ErrTooLarge", big.Rect.Size(), err)
	}
	if _, err := c.Render(dochelper.Tile{Scale: 1, Rect: image.Rect(0, 0, 64, 64)}); err != nil {
		t.Errorf("the tile after: %v", err)
	}
}
