package main

import (
	"bytes"
	"context"
	"debug/buildinfo"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stubbedev/lys/lib/buildtest"
	"github.com/stubbedev/lys/lib/testhome"

	"github.com/stubbedev/lys/desktop/dochelper"
	"github.com/stubbedev/lys/desktop/thumbnail"
)

// helperPath is lys-docthumb built as it ships, which TestMain builds
// before any test runs: the test binary links what the tests import
// and the helper does not, purego's libc threads among it, whose stacks
// count against the memory the helper holds itself to.
var helperPath string

func TestMain(m *testing.M) {
	os.Exit(testhome.Main(func() int {
		if err := buildHelper(); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, err)
			return 1
		}
		return m.Run()
	}))
}

// helperDir holds the built helper for as long as the tests run it.
var helperDir *buildtest.Dir

// buildHelper builds lys-docthumb in the environment its Nix package
// builds it in, at helperPath.
func buildHelper() error {
	env, err := dochelper.BuildEnv()
	if err != nil {
		return err
	}
	if helperDir, err = buildtest.Hold(dochelper.Program + "-test"); err != nil {
		return err
	}
	if err := helperDir.BuildWith(env, "./desktop/cmd/"+dochelper.Program); err != nil {
		return err
	}
	helperPath = filepath.Join(helperDir.Path(), dochelper.Program)
	return nil
}

func fixture(name string) string { return filepath.Join("..", "..", "docrender", "testdata", name) }

// helper runs lys-docthumb on req and is its exit status and output.
func helper(t *testing.T, req dochelper.Request) (int, []byte) {
	t.Helper()
	cmd := exec.Command(helperPath, req.Args()...)
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

// A damaged document and an encrypted one exit with the status that
// tells their error, and write nothing.
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
	} {
		t.Run(c.name, func(t *testing.T) { requireHelperFails(t, c.req, c.want) })
	}
}

// tightMemory is less than the pixels of a page drawn MaxSize tall, and
// more than the helper needs to open a document and draw a small page.
const tightMemory = 32 << 20

// A page past the helper's memory exits with ErrTooLarge and writes
// nothing; the same page within DocumentMemory renders.
func TestAPagePastTheHelpersMemoryIsTooLarge(t *testing.T) {
	requireHelperFails(t, request(t, fixture("cairo.pdf"), dochelper.MaxSize, tightMemory), dochelper.ErrTooLarge)
	code, out := helper(t, request(t, fixture("cairo.pdf"), dochelper.MaxSize, thumbnail.DocumentMemory))
	if code != 0 {
		t.Errorf("the page that ran out of %d bytes, within DocumentMemory: exited %d: %s", tightMemory, code, out)
	}
}

// requireHelperFails runs the helper on req and fails the test unless
// it exits with want and leaves no output.
func requireHelperFails(t *testing.T, req dochelper.Request, want error) {
	t.Helper()
	code, out := helper(t, req)
	if err := dochelper.ExitError(code, out); !errors.Is(err, want) {
		t.Errorf("the helper exited %d (%v), want %v: %s", code, err, want, out)
	}
	if _, err := os.Stat(req.Output); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the failed helper left %s: %v", req.Output, err)
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
	th := thumbnail.New(thumbnail.Config{DocumentHelper: helperPath})
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
	info, err := buildinfo.ReadFile(helperPath)
	if err != nil {
		t.Fatal(err)
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

// The helper is built without the Green Tea collector, whose span
// queue dies of a segmentation fault where the helper's memory limit
// should end it as out of memory, which ErrTooLarge is told by.
func TestTheHelperIsBuiltWithoutGreenTea(t *testing.T) {
	info, err := buildinfo.ReadFile(helperPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range info.Settings {
		if s.Key == "GOEXPERIMENT" {
			if s.Value != "nogreenteagc" {
				t.Errorf("lys-docthumb is built with GOEXPERIMENT=%s, want nogreenteagc", s.Value)
			}
			return
		}
	}
	t.Errorf("lys-docthumb is built with no GOEXPERIMENT, want nogreenteagc: %v", info.Settings)
}

// reader is a client of lys-docthumb serving the fixture as it does
// for a reader, holding memory.
func reader(t *testing.T, file string, memory dochelper.Memory) *dochelper.Client {
	t.Helper()
	doc, err := filepath.Abs(fixture(file))
	if err != nil {
		t.Fatal(err)
	}
	c := dochelper.NewClient(dochelper.Starter(helperPath, dochelper.Serving{Memory: memory}), doc, dochelper.RequestTimeout)
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
	c := reader(t, "cairo.pdf", dochelper.Memory{Hard: tightMemory, Soft: tightMemory / 3})
	info, err := c.Info()
	if err != nil {
		t.Fatal(err)
	}
	const scale = 9
	page := info.Sizes[0].At(scale)
	big := dochelper.Tile{Scale: scale, Rect: image.Rect(0, 0, min(page.X, dochelper.MaxSize), min(page.Y, dochelper.MaxSize))}
	if _, err := c.Render(big); !errors.Is(err, dochelper.ErrTooLarge) {
		t.Fatalf("a %v tile in %d bytes = %v, want ErrTooLarge", big.Rect.Size(), tightMemory, err)
	}
	if _, err := c.Render(dochelper.Tile{Scale: 1, Rect: image.Rect(0, 0, 64, 64)}); err != nil {
		t.Errorf("the tile after: %v", err)
	}
}
