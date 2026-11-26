package thumbnail

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/logtest"

	"github.com/stubbedev/lys/desktop/dochelper"
	"github.com/stubbedev/lys/desktop/thumbnail/thumbnailtest"
)

const epubType = "application/epub+zip"

// document is a file named name in a fresh directory, in a fresh cache.
func document(t *testing.T, name string) string {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte("%PDF-1.4"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// PDFs and EPUBs are thumbnailed by lys-docthumb, ahead of the
// thumbnailer installed for them, which here fails.
func TestDocumentsAreThumbnailedByTheHelper(t *testing.T) {
	thumbnailtest.Install(t, pdfType+";"+epubType, thumbnailtest.Argv(t, thumbnailtest.Fail, "%o"))
	for _, c := range []struct{ name, mimeType string }{{"paper.pdf", pdfType}, {"book.epub", epubType}} {
		t.Run(c.mimeType, func(t *testing.T) {
			doc := document(t, c.name)
			th := New(Config{DocumentHelper: thumbnailtest.Argv(t, thumbnailtest.Page)})
			size, err := square(t, th, doc, c.mimeType, 64)
			if err != nil || size.X != 64 {
				t.Fatalf("the %s's thumbnail = %v, %v, want lys-docthumb's", c.mimeType, size, err)
			}
		})
	}
}

// Without lys-docthumb, the thumbnailer installed for documents makes
// their thumbnails, and why is logged once.
func TestWithoutTheHelperTheInstalledThumbnailerServesDocuments(t *testing.T) {
	doc := document(t, "paper.pdf")
	thumbnailtest.Install(t, pdfType, thumbnailtest.Argv(t, thumbnailtest.Page, "%o"))
	missing := filepath.Join(t.TempDir(), dochelper.Program)
	th := New(Config{DocumentHelper: []string{missing}})
	logged := logtest.Capture(t)
	for range 2 {
		if size, err := square(t, th, doc, pdfType, 64); err != nil || size.X != 64 {
			t.Fatalf("the PDF's thumbnail = %v, %v, want the installed thumbnailer's", size, err)
		}
		t.Setenv("XDG_CACHE_HOME", t.TempDir())
	}
	want := "the installed thumbnailers make documents' thumbnails, as " + dochelper.Program + " is not run"
	if n := strings.Count(logged.String(), want); n != 1 || !strings.Contains(logged.String(), missing) {
		t.Errorf("the missing helper was logged %d times, want once naming %s:\n%s", n, missing, logged.String())
	}
}

// The lys-docthumb a Thumbnailer runs by default is the one next to the
// running program.
func TestTheHelperIsFoundNextToTheProgram(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	argv, err := helperArgv(nil)
	if err != nil || len(argv) != 1 || argv[0] != filepath.Join(filepath.Dir(exe), dochelper.Program) {
		t.Errorf("the default helper = %q, %v, want %s beside %s", argv, err, dochelper.Program, exe)
	}
	set := []string{"/opt/helper", "-x"}
	if argv, err := helperArgv(set); err != nil || strings.Join(argv, " ") != "/opt/helper -x" {
		t.Errorf("a set helper = %q, %v, want %q", argv, err, set)
	}
}

// A helper that hangs is killed at the run timeout, and the document
// is recorded as failed.
func TestAHangingHelperIsKilled(t *testing.T) {
	doc := document(t, "paper.pdf")
	th := New(Config{DocumentHelper: thumbnailtest.Argv(t, thumbnailtest.Hang), RunTimeout: 500 * time.Millisecond})
	if _, err := square(t, th, doc, pdfType, 64); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("a hanging helper = %v, want its deadline", err)
	}
	info, err := os.Stat(doc)
	if err != nil {
		t.Fatal(err)
	}
	tc, err := cacheFor(doc, info)
	if err != nil {
		t.Fatal(err)
	}
	if !tc.failed(software) {
		t.Errorf("no fail record at %s", tc.failPath(software))
	}
}

// The helper's exit status is the error the thumbnail fails with:
// each typed status its error, the runtime out of memory ErrTooLarge,
// and any other status an error that is none of them.
func TestTheHelpersExitStatusIsTheError(t *testing.T) {
	typed := []error{dochelper.ErrDamaged, dochelper.ErrEncrypted, dochelper.ErrTooLarge, dochelper.ErrUnsupported}
	for _, c := range []struct {
		name, status, message string
		want                  error
	}{
		{"damaged", "3", "document: damaged", dochelper.ErrDamaged},
		{"encrypted", "4", "document: encrypted", dochelper.ErrEncrypted},
		{"too large", "5", "document: too large", dochelper.ErrTooLarge},
		{"unsupported", "6", "document: unsupported", dochelper.ErrUnsupported},
		{"out of memory", "2", "fatal error: out of memory", dochelper.ErrTooLarge},
		{"panic", "2", "panic: boom", nil},
		{"failed", "1", "disk full", nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			doc := document(t, "paper.pdf")
			th := New(Config{DocumentHelper: thumbnailtest.Argv(t, thumbnailtest.Exit, c.status, c.message)})
			_, err := square(t, th, doc, pdfType, 64)
			if err == nil || !strings.Contains(err.Error(), c.message) {
				t.Fatalf("the helper exiting %s = %v, want a failure saying %q", c.status, err, c.message)
			}
			for _, e := range typed {
				if errors.Is(err, e) != (c.want != nil && errors.Is(c.want, e)) {
					t.Errorf("the helper exiting %s = %v; is %v: %t", c.status, err, e, errors.Is(err, e))
				}
			}
		})
	}
}

// lys-docthumb sees its input and the font directories, those outside
// the sandbox's system bound apart from where they lie, and nothing of
// $HOME, the network or the denied calls.
func TestTheHelperSeesOnlyItsInputAndTheFonts(t *testing.T) {
	data := filepath.Join(os.Getenv("HOME"), ".local", "share")
	if err := os.MkdirAll(filepath.Join(data, "fonts"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_DATA_HOME", data)
	doc := document(t, "paper.pdf")
	probe := thumbnailtest.NewProbe(t)
	th := New(Config{DocumentHelper: probe.Argv(t, sandboxPath(doc), "unused")})
	img, err := th.Image(context.Background(), doc, pdfType, 128)
	if err != nil {
		t.Fatalf("the probing helper failed: %v", err)
	}
	if leaks := thumbnailtest.Leaks(t, img); len(leaks) != 0 {
		t.Errorf("the sandboxed helper %s", strings.Join(leaks, ", "))
	}
	if n := probe.Accepted(); n != 0 {
		t.Errorf("the listener on loopback accepted %d connections from the sandbox", n)
	}
	h, ok := th.core.documents()
	if !ok {
		t.Fatal("no document helper")
	}
	fonts := filepath.Join(data, "fonts")
	bound := false
	for _, b := range h.maker.binds {
		bound = bound || (b.host == fonts && strings.HasPrefix(b.at, sandboxFonts+"/"))
	}
	if !bound {
		t.Errorf("the font directory %s is not bound under %s: %+v", fonts, sandboxFonts, h.maker.binds)
	}
}

// A font directory the sandbox's system holds is read where it lies,
// and any other at a path of its own under sandboxFonts, in order.
func TestFontDirectoriesOutsideTheSystemAreBound(t *testing.T) {
	sb := sandbox{bound: []string{"/usr", "/nix/store"}}
	binds, at := fontBinds(sb, []string{"/usr/share/fonts", "/home/u/.local/share/fonts", "/nix/store/x-fonts/share", "/opt/fonts"})
	wantAt := []string{"/usr/share/fonts", sandboxFonts + "/1", "/nix/store/x-fonts/share", sandboxFonts + "/3"}
	wantBinds := []bind{{"/home/u/.local/share/fonts", sandboxFonts + "/1"}, {"/opt/fonts", sandboxFonts + "/3"}}
	if strings.Join(at, " ") != strings.Join(wantAt, " ") {
		t.Errorf("the fonts are read at %q, want %q", at, wantAt)
	}
	if len(binds) != len(wantBinds) || binds[0] != wantBinds[0] || binds[1] != wantBinds[1] {
		t.Errorf("binds = %+v, want %+v", binds, wantBinds)
	}
}

// No long-running process links the renderer through the thumbnailer:
// folio parses untrusted input only in lys-docthumb.
func TestTheThumbnailerLinksNoRenderer(t *testing.T) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		t.Fatal("the test binary carries no build information")
	}
	for _, m := range info.Deps {
		if m.Path == "github.com/gen2brain/folio" {
			t.Errorf("package thumbnail links %s %s", m.Path, m.Version)
		}
	}
}
