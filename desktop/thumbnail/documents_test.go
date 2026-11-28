package thumbnail

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stubbedev/lys/lib/logtest"
	"github.com/stubbedev/lys/lib/waittest"

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

// requireFailRecord fails t unless doc, as it is now, has a fail record.
func requireFailRecord(t *testing.T, doc string) {
	t.Helper()
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
}

// PDFs and EPUBs are thumbnailed by lys-docthumb.
func TestDocumentsAreThumbnailedByTheHelper(t *testing.T) {
	for _, c := range []struct{ name, mimeType string }{{"paper.pdf", pdfType}, {"book.epub", epubType}} {
		t.Run(c.mimeType, func(t *testing.T) {
			doc := document(t, c.name)
			th := New(Config{DocumentHelper: thumbnailtest.Helper(t, thumbnailtest.Page)})
			size, err := square(t, th, doc, c.mimeType, 64)
			if err != nil || size.X != 64 {
				t.Fatalf("the %s's thumbnail = %v, %v, want lys-docthumb's", c.mimeType, size, err)
			}
		})
	}
}

// Without lys-docthumb a document has no thumbnail, ErrNoHelper (which
// is ErrNone, so no failure is recorded), and why is logged once.
func TestWithoutTheHelperDocumentsHaveNoThumbnail(t *testing.T) {
	doc := document(t, "paper.pdf")
	missing := filepath.Join(t.TempDir(), dochelper.Program)
	th := New(Config{DocumentHelper: missing})
	logged := logtest.Capture(t)
	for range 2 {
		if _, err := square(t, th, doc, pdfType, 64); !errors.Is(err, ErrNoHelper) || !errors.Is(err, ErrNone) {
			t.Fatalf("the PDF's thumbnail without the helper = %v, want ErrNoHelper", err)
		}
	}
	want := "documents have no thumbnails, as " + dochelper.Program + " is not run"
	if n := strings.Count(logged.String(), want); n != 1 || !strings.Contains(logged.String(), missing) {
		t.Errorf("the missing helper was logged %d times, want once naming %s:\n%s", n, missing, logged.String())
	}
}

// The lys-docthumb a Thumbnailer runs by default is the one next to the
// running program; a set one is run only when it is an absolute path to
// a file called lys-docthumb, so no other program is ever run.
func TestOnlyLysDocthumbIsRun(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	beside := filepath.Join(filepath.Dir(exe), dochelper.Program)
	if _, err := newDocHelper(""); err == nil || !strings.Contains(err.Error(), beside) {
		t.Errorf("the default helper = %v, want %s beside %s, which is not installed", err, beside, exe)
	}
	fake := thumbnailtest.Helper(t, thumbnailtest.Page)
	if h, err := newDocHelper(fake); err != nil || h.path != fake {
		t.Errorf("an installed helper = %+v, %v, want it run from %s", h, err, fake)
	}
	for _, path := range []string{"/bin/sh", exe, dochelper.Program, filepath.Join(".", dochelper.Program), filepath.Join(filepath.Dir(fake), "evince-thumbnailer")} {
		doc := document(t, "paper.pdf")
		th := New(Config{DocumentHelper: path})
		logtest.Capture(t)
		if _, err := square(t, th, doc, pdfType, 64); !errors.Is(err, ErrNoHelper) {
			t.Errorf("a document helper of %s = %v, want ErrNoHelper with nothing run", path, err)
		}
	}
}

// The package starts no process but in docHelper.run, and there only
// lys-docthumb at the path newDocHelper checked, as dochelper starts it.
func TestNothingButTheHelperIsExecuted(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	starts := map[string]bool{
		"exec.Command": true, "exec.CommandContext": true, "os.StartProcess": true,
		"syscall.Exec": true, "syscall.ForkExec": true, "syscall.StartProcess": true,
		"unix.Exec": true, "unix.ForkExec": true, "dochelper.Command": true,
	}
	var calls []string
	fset := token.NewFileSet()
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			ast.Inspect(fn, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				pkg, ok := sel.X.(*ast.Ident)
				if !ok || !starts[pkg.Name+"."+sel.Sel.Name] {
					return true
				}
				recv := ""
				if fn.Recv != nil && len(fn.Recv.List) == 1 {
					if id, ok := fn.Recv.List[0].Type.(*ast.Ident); ok {
						recv = id.Name
					}
				}
				program := ""
				if arg := 1; len(call.Args) > arg {
					if p, ok := call.Args[arg].(*ast.SelectorExpr); ok {
						if x, ok := p.X.(*ast.Ident); ok {
							program = x.Name + "." + p.Sel.Name
						}
					}
				}
				calls = append(calls, recv+"."+fn.Name.Name+": "+pkg.Name+"."+sel.Sel.Name+"("+program+")")
				return true
			})
		}
	}
	if want := "docHelper.run: dochelper.Command(h.path)"; len(calls) != 1 || calls[0] != want {
		t.Errorf("the package starts processes at %q, want only %q", calls, want)
	}
}

// A helper that hangs fails at the run timeout, and the document is
// recorded as failed.
func TestAHangingHelperFailsAtTheRunTimeout(t *testing.T) {
	doc := document(t, "paper.pdf")
	th := New(Config{DocumentHelper: thumbnailtest.Helper(t, thumbnailtest.Hang), RunTimeout: 100 * time.Millisecond})
	if _, err := square(t, th, doc, pdfType, 64); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("a hanging helper = %v, want its deadline", err)
	}
	requireFailRecord(t, doc)
}

// A helper ended while it hangs is killed with its whole process
// group, here the process it started, and the document is not
// recorded as failed.
func TestAnEndedHelperIsKilledWithItsProcessGroup(t *testing.T) {
	doc := document(t, "paper.pdf")
	helper := thumbnailtest.Helper(t, thumbnailtest.Hang)
	th := New(Config{DocumentHelper: helper, RunTimeout: time.Minute})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ended := make(chan error, 1)
	go func() {
		_, err := th.Square(ctx, doc, pdfType, 64)
		ended <- err
	}()
	pid := 0
	waittest.UntilSlow(t, func() bool {
		data, err := os.ReadFile(filepath.Join(filepath.Dir(helper), thumbnailtest.HangPID))
		if err != nil {
			return false
		}
		pid, err = strconv.Atoi(string(data))
		return err == nil
	}, "the hanging helper to start its process")
	cancel()
	if err := waittest.For(t, ended, "the ended helper's thumbnail"); !errors.Is(err, context.Canceled) {
		t.Fatalf("an ended hanging helper = %v, want context.Canceled", err)
	}
	waittest.Until(t, func() bool { return gone(pid) }, "the process the hanging helper started to be killed with it")
	info, err := os.Stat(doc)
	if err != nil {
		t.Fatal(err)
	}
	tc, err := cacheFor(doc, info)
	if err != nil {
		t.Fatal(err)
	}
	if tc.failed() {
		t.Errorf("an ended helper left a fail record at %s", tc.failPath())
	}
}

// gone reports a process that no longer runs: reaped, or a zombie.
func gone(pid int) bool {
	if err := syscall.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
		return true
	}
	stat, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return true
	}
	end := strings.LastIndexByte(string(stat), ')')
	return end >= 0 && end+2 < len(stat) && stat[end+2] == 'Z'
}

// A helper that allocates past the memory it is given dies of it, which
// is ErrTooLarge, and the document is recorded as failed.
func TestAHelperPastItsMemoryFails(t *testing.T) {
	doc := document(t, "paper.pdf")
	th := New(Config{DocumentHelper: thumbnailtest.Helper(t, thumbnailtest.OverMemory)})
	_, err := square(t, th, doc, pdfType, 64)
	if !errors.Is(err, dochelper.ErrTooLarge) || !strings.Contains(err.Error(), "out of memory") && !strings.Contains(err.Error(), "cannot allocate memory") {
		t.Fatalf("a helper past its memory = %v, want ErrTooLarge saying it ran out of memory", err)
	}
	requireFailRecord(t, doc)
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
			th := New(Config{DocumentHelper: thumbnailtest.Helper(t, thumbnailtest.Exit, c.status, c.message)})
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
