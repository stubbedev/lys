package thumbnail

import (
	"context"
	"errors"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stubbedev/lys/lib/logtest"
	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/desktop/thumbnail/thumbnailtest"
)

// hostileInput is a PDF the hostile thumbnailer is pointed at.
func hostileInput(t *testing.T) string {
	t.Helper()
	input := filepath.Join(t.TempDir(), "paper.pdf")
	if err := os.WriteFile(input, []byte("%PDF-1.4"), 0o600); err != nil {
		t.Fatal(err)
	}
	return input
}

// A thumbnailer runs in the sandbox seeing its input alone: it cannot
// read $HOME, reach a listener on loopback, open an IPv4 socket, make
// a call the seccomp filter denies, or write its input. The same
// thumbnailer run unsandboxed does all but fail to read its input, so
// each probe can tell.
func TestSandboxedThumbnailerSeesOnlyItsInput(t *testing.T) {
	sb, err := newSandbox(context.Background())
	if err != nil {
		t.Fatalf("no sandbox (the dev shell carries bubblewrap): %v", err)
	}
	probe := thumbnailtest.NewProbe(t)
	input := hostileInput(t)
	entry := thumbnailerEntry{argv: probe.Argv(t, "%i", "%o"), file: "hostile.thumbnailer"}
	c := New(Config{}).core
	img, err := c.run(context.Background(), sb, entry.maker(), input, 128)
	if err != nil {
		t.Fatalf("the sandboxed thumbnailer failed: %v", err)
	}
	if leaks := thumbnailtest.Leaks(t, img); len(leaks) != 0 {
		t.Errorf("the sandboxed thumbnailer %s", strings.Join(leaks, ", "))
	}
	if n := probe.Accepted(); n != 0 {
		t.Errorf("the listener on loopback accepted %d connections from the sandbox", n)
	}

	out := filepath.Join(t.TempDir(), "out.png")
	argv := entry.expand(input, "file://"+input, out, 128)
	if b, err := exec.Command(argv[0], argv[1:]...).CombinedOutput(); err != nil {
		t.Fatalf("the unsandboxed thumbnailer: %v: %s", err, b)
	}
	f, err := os.Open(out)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	free, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	if got := thumbnailtest.Leaks(t, free); strings.Join(got, ", ") != strings.Join(thumbnailtest.Unsandboxed, ", ") {
		t.Errorf("unsandboxed, the thumbnailer %v; want %v, or a probe cannot tell", got, thumbnailtest.Unsandboxed)
	}
	if probe.Accepted() == 0 {
		t.Error("the unsandboxed thumbnailer never reached the listener")
	}
}

// A thumbnailer is run by the path it was found at, not the file that
// path links to, so a multi-call binary knows which command it is;
// outside the system the sandbox shows, the file is bound at that path.
func TestAThumbnailerRunsByTheNameItWasFoundAt(t *testing.T) {
	sb, err := newSandbox(context.Background())
	if err != nil {
		t.Fatalf("no sandbox (the dev shell carries bubblewrap): %v", err)
	}
	bin, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(t.TempDir(), "thumbs-link")
	if err := os.Symlink(bin, link); err != nil {
		t.Fatal(err)
	}
	c := New(Config{}).core
	named := func(name string) maker {
		return thumbnailerEntry{argv: []string{link, thumbnailtest.Arg, thumbnailtest.Named, name, "%o"}, file: "named.thumbnailer"}.maker()
	}
	img, err := c.run(context.Background(), sb, named("thumbs-link"), hostileInput(t), 128)
	if err != nil {
		t.Fatalf("a thumbnailer run through a link: %v", err)
	}
	if b := img.Bounds(); b.Dx() != thumbnailtest.PageW {
		t.Errorf("the linked thumbnailer wrote a %v page", b)
	}
	if _, err := c.run(context.Background(), sb, named(filepath.Base(bin)), hostileInput(t), 128); err == nil {
		t.Error("the linked thumbnailer ran by the name of the file it links to")
	}
}

// A command a surface makes its thumbnails with runs in the same
// sandbox, for every file, a raster image too; what it fails at is
// recorded apart from the installed thumbnailers, which still make the
// file's thumbnail.
func TestACommandRunsInTheSandbox(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	probe := thumbnailtest.NewProbe(t)
	th := New(Config{})
	hostile := th.WithCommand("probe", func(input, output string, _ int) []string { return probe.Argv(t, input, output) })
	for _, input := range []string{hostileInput(t), photo(t, 40, 30)} {
		img, err := hostile.Image(context.Background(), input, pdfType, LargePx)
		if err != nil {
			t.Fatalf("the command on %s: %v", input, err)
		}
		if leaks := thumbnailtest.Leaks(t, img); len(leaks) != 0 {
			t.Errorf("the command on %s %s", input, strings.Join(leaks, ", "))
		}
	}
	if n := probe.Accepted(); n != 0 {
		t.Errorf("the listener on loopback accepted %d connections from the command", n)
	}

	failing := th.WithCommand("failing", func(_, output string, _ int) []string {
		return thumbnailtest.Argv(t, thumbnailtest.Fail, output)
	})
	path := photo(t, 40, 30)
	if _, err := failing.File(context.Background(), path, "", NormalPx); err == nil || errors.Is(err, ErrNone) {
		t.Fatalf("a failing command = %v, want its failure", err)
	}
	if _, err := failing.File(context.Background(), path, "", NormalPx); !errors.Is(err, ErrNone) {
		t.Errorf("a file the command failed at = %v, want ErrNone without running it again", err)
	}
	if _, err := th.File(context.Background(), path, "", NormalPx); err != nil {
		t.Errorf("the command's failure stopped the in-process decode: %v", err)
	}

	broken := th.WithCommand("broken", func(string, string, int) []string { return nil })
	if _, err := broken.File(context.Background(), photo(t, 40, 30), "", NormalPx); err == nil || !strings.Contains(err.Error(), "does not parse") {
		t.Errorf("a command that does not parse = %v", err)
	}
}

// Without bubblewrap no thumbnailer runs, a PDF gets no thumbnail and
// nothing records it as failed, and why is said once.
func TestNoSandboxRunsNoThumbnailer(t *testing.T) {
	doc, _ := pdfWith(t, thumbnailtest.Page)
	t.Setenv("PATH", t.TempDir())
	th := New(Config{})
	logged := logtest.Capture(t)
	for range 2 {
		if _, err := square(t, th, doc, pdfType, 64); !errors.Is(err, ErrNone) {
			t.Fatalf("a PDF without a sandbox = %v, want ErrNone", err)
		}
	}
	command := th.WithCommand("page", func(_, output string, _ int) []string {
		return thumbnailtest.Argv(t, thumbnailtest.Page, output)
	})
	if _, err := square(t, command, doc, pdfType, 64); !errors.Is(err, ErrNone) {
		t.Errorf("a command without a sandbox = %v, want ErrNone", err)
	}
	if n := strings.Count(logged.String(), "thumbnailers are not run without a sandbox: bubblewrap (bwrap) is not installed"); n != 1 {
		t.Errorf("the missing sandbox was logged %d times, want once:\n%s", n, logged.String())
	}
}

// The seccomp filter lays out with every jump in reach and ends in its
// verdicts.
func TestSeccompFilterEncodes(t *testing.T) {
	ops := seccompFilter()
	b, err := encodeFilter(ops)
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 8*(len(ops)+len(verdicts)) {
		t.Fatalf("%d bytes for %d instructions and %d verdicts", len(b), len(ops), len(verdicts))
	}
	far := append(make([]filterOp, 300), filterOp{})
	far[0] = test(unix.BPF_JEQ, 0, to(allow), onward)
	if _, err := encodeFilter(far); err == nil {
		t.Error("a jump past 255 instructions encoded")
	}
}
