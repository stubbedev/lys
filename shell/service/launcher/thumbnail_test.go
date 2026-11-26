package launcher

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stubbedev/lys/desktop/config"
	"github.com/stubbedev/lys/desktop/mime"
	"github.com/stubbedev/lys/desktop/shlex"
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

// shellLine is argv as a -preview-cmd line, every word quoted.
func shellLine(argv []string) string {
	quoted := make([]string, len(argv))
	for i, a := range argv {
		quoted[i] = "'" + a + "'"
	}
	return strings.Join(quoted, " ")
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

func TestPreviewCmdPlaceholdersAreArgvNotAShellLine(t *testing.T) {
	words, _ := shlex.Split(`mkthumb "{input}" "{output}" "{size}"`)
	if argv := previewArgv(words, "/tmp/a b.raw", "/cache/x.png", 128); !reflect.DeepEqual(argv, []string{"mkthumb", "/tmp/a b.raw", "/cache/x.png", "128"}) {
		t.Errorf("argv = %q", argv)
	}
	words, _ = shlex.Split("mkthumb '{input}' '{output}'")
	if argv := previewArgv(words, "/tmp/x; rm -rf ~/.png", "/cache/x.png", 128); !reflect.DeepEqual(argv, []string{"mkthumb", "/tmp/x; rm -rf ~/.png", "/cache/x.png"}) {
		t.Errorf("a file name smuggled a command: %q", argv)
	}
}

func TestABrokenPreviewCmdProducesNothing(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	files := thumbnail.New(thumbnail.Config{})
	file := writeTemp(t, "a.png", pngOf(t, 4, 3))
	if _, ok := NewThumbnailer(files, testTypes(t), "mkthumb '{input}").Generate(context.Background(), file); ok {
		t.Error("half a command must not run, nor the system thumbnailers in its place")
	}
	if NewThumbnailer(files, testTypes(t), "   ").files != files {
		t.Error("a blank preview-cmd must not shadow the system thumbnailers")
	}
}

// -preview-cmd makes a thumbnail in the sandbox, which lands in the
// freedesktop cache; while the file is unchanged the cached one is
// reused, and once it changes the command makes it anew.
func TestGenerateRunsThePreviewCmdAndCaches(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	file := writeTemp(t, "pic.raw", pngOf(t, 40, 30))
	files := thumbnail.New(thumbnail.Config{})
	out, ok := NewThumbnailer(files, testTypes(t), "cp {input} {output}").Generate(context.Background(), file)
	if !ok || filepath.Dir(out) != filepath.Join(cache, "thumbnails", "normal") {
		t.Fatalf("generate = %q %v", out, ok)
	}
	if got := sizeOf(t, out); got != image.Pt(40, 30) {
		t.Errorf("the thumbnail is %v, want the copied 40x30", got)
	}
	if cached, ok := NewThumbnailer(files, testTypes(t), "false {input}").Generate(context.Background(), file); !ok || cached != out {
		t.Errorf("an unchanged file's cached thumbnail = %q %v, want %q reused", cached, ok, out)
	}

	if err := os.WriteFile(file, pngOf(t, 20, 10), 0o600); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(time.Hour)
	if err := os.Chtimes(file, later, later); err != nil {
		t.Fatal(err)
	}
	if again, ok := NewThumbnailer(files, testTypes(t), "cp {input} {output}").Generate(context.Background(), file); !ok || again != out {
		t.Fatalf("a changed file's thumbnail = %q %v", again, ok)
	}
	if got := sizeOf(t, out); got != image.Pt(20, 10) {
		t.Errorf("a changed file's thumbnail is %v, want the new 20x10", got)
	}
}

func TestAThumbnailerThatWritesNothingProducesNothing(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	file := writeTemp(t, "pic.raw", pngOf(t, 4, 3))
	if _, ok := NewThumbnailer(thumbnail.New(thumbnail.Config{}), testTypes(t), "true {input}").Generate(context.Background(), file); ok {
		t.Error("a successful command that wrote no file must not count")
	}
}

// A launcher file preview's thumbnail (Previews) runs the thumbnailer
// installed for its type, and -preview-cmd in its place, in the sandbox: neither reads $HOME,
// reaches the network, opens an IPv4 socket, makes a denied call or
// writes its input.
func TestAFilePreviewRunsItsThumbnailerSandboxed(t *testing.T) {
	probe := thumbnailtest.NewProbe(t)
	thumbnailtest.Install(t, "application/pdf", probe.Argv(t, "%i", "%o"))
	for name, previewCmd := range map[string]string{
		"installed":   "",
		"preview-cmd": shellLine(probe.Argv(t, "{input}", "{output}")),
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("XDG_CACHE_HOME", t.TempDir())
			doc := writeTemp(t, "paper.pdf", []byte("%PDF-1.4\x00\x01"))
			previews := NewThumbnailer(thumbnail.New(thumbnail.Config{}), testTypes(t), previewCmd).Previews()
			got, _, err := previews.Fit(ctx, doc, "application/pdf", 512, 512)
			if err != nil {
				t.Fatalf("the PDF's preview: %v", err)
			}
			if leaks := thumbnailtest.Leaks(t, got.Image); len(leaks) != 0 {
				t.Errorf("the thumbnailer %s", strings.Join(leaks, ", "))
			}
			if err := previews.Free(got); err != nil {
				t.Error(err)
			}
		})
	}
	if n := probe.Accepted(); n != 0 {
		t.Errorf("the listener on loopback accepted %d connections from the sandbox", n)
	}
}

// A file whose thumbnailer fails gets no thumbnail, and while it is
// unchanged the thumbnailer is not run for it again.
func TestAFailedThumbnailIsNotRetried(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)
	thumbnailtest.Install(t, "application/pdf", thumbnailtest.Argv(t, thumbnailtest.Fail, "%o"))
	doc := writeTemp(t, "paper.pdf", []byte("%PDF-1.4\x00\x01"))
	if _, ok := NewThumbnailer(thumbnail.New(thumbnail.Config{}), testTypes(t), "").Generate(ctx, doc); ok {
		t.Fatal("a failing thumbnailer made a thumbnail")
	}
	if failed, err := os.ReadDir(filepath.Join(cache, "thumbnails", "fail", "lys-shell")); err != nil || len(failed) != 1 {
		t.Fatalf("the failing thumbnailer left %d fail records (%v), want one", len(failed), err)
	}
	thumbnailtest.Install(t, "application/pdf", thumbnailtest.Argv(t, thumbnailtest.Page, "%o"))
	if _, ok := NewThumbnailer(thumbnail.New(thumbnail.Config{}), testTypes(t), "").Generate(ctx, doc); ok {
		t.Error("a file that failed ran a thumbnailer again")
	}
}

func TestAFileWithNoThumbnailStillHasAnIconName(t *testing.T) {
	if MimeIcon("/tmp/notes.txt") == "" {
		t.Error("a mime icon must never be empty")
	}
}

func TestThePreviewCommandRendersExactlyItsDeclaredPlaceholders(t *testing.T) {
	var declared []string
	for _, p := range config.Placeholders("launcher.preview-cmd") {
		declared = append(declared, p.Name)
	}
	slices.Sort(declared)
	if rendered := slices.Sorted(maps.Keys(previewVars("", "", 0))); len(declared) == 0 || !slices.Equal(rendered, declared) {
		t.Errorf("-preview-cmd renders %v, declares %v", rendered, declared)
	}
}
