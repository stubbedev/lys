package main

import (
	"bytes"
	"context"
	"errors"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"testing"

	"github.com/stubbedev/lys/lib/testhome"

	"github.com/stubbedev/lys/desktop/dochelper"
	"github.com/stubbedev/lys/desktop/thumbnail"
)

// helperArg, first on the command line, makes the test binary
// lys-docthumb.
const helperArg = "lys-docthumb-under-test"

// TestMain runs the package against a scratch home; started with
// helperArg, the test binary is the helper instead.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == helperArg {
		os.Exit(run(os.Args[2:], os.Stderr))
	}
	testhome.Run(m)
}

func fixture(name string) string { return filepath.Join("..", "..", "docrender", "testdata", name) }

// helper runs the test binary as lys-docthumb on req, outside any
// sandbox, and is its exit status and output.
func helper(t *testing.T, req dochelper.Request) (int, []byte) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, append([]string{helperArg}, req.Args()...)...)
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
	if code := run([]string{"-size", "0", "--", "in", "out"}, &stderr); code == 0 || stderr.Len() == 0 {
		t.Errorf("a bad request exited %d saying %q", code, stderr.String())
	}
}

// The thumbnailer runs the helper in its sandbox for PDFs and EPUBs,
// and its failures reach the caller as their errors.
func TestTheThumbnailerRunsTheHelper(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	th := thumbnail.New(thumbnail.Config{DocumentHelper: []string{exe, helperArg}})
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
