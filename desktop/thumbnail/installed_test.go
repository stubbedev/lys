package thumbnail

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// writeThumbnailer installs a .thumbnailer named name in dir.
func writeThumbnailer(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "thumbnailers"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "thumbnailers", name+".thumbnailer"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

// An installed thumbnailer serves every MIME type it lists with its
// Exec's command line, the field codes filled per file: an earlier data
// directory wins, as does the earlier file within one.
func TestInstalledThumbnailersServeTheirTypes(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	writeThumbnailer(t, first, "b", "[Thumbnailer Entry]\nTryExec=sh\nExec=gdk-pixbuf-thumbnailer -s %s %u %o\nMimeType=image/svg+xml;image/png;\n")
	writeThumbnailer(t, first, "c", "[Thumbnailer Entry]\nExec=later %i %o\nMimeType=image/png;text/x-later;\n")
	writeThumbnailer(t, second, "a", "[Thumbnailer Entry]\nExec=other %i %o\nMimeType=image/svg+xml;application/x-other;\n")
	byType := loadThumbnailers([]string{first, second})
	svg, ok := byType["image/svg+xml"]
	if !ok {
		t.Fatal("no thumbnailer for SVG")
	}
	argv := svg.expand("/tmp/logo.svg", "file:///tmp/logo.svg", "/tmp/out.png", 128)
	if want := []string{"gdk-pixbuf-thumbnailer", "-s", "128", "file:///tmp/logo.svg", "/tmp/out.png"}; !reflect.DeepEqual(argv, want) {
		t.Errorf("argv = %q, want %q", argv, want)
	}
	if got := byType["image/png"].file; got != filepath.Join(first, "thumbnailers", "b.thumbnailer") {
		t.Errorf("PNG is served by %s, want the earlier file", got)
	}
	for mimeType, want := range map[string]string{"text/x-later": "later", "application/x-other": "other"} {
		if got := byType[mimeType].argv; len(got) == 0 || got[0] != want {
			t.Errorf("%s is served by %q, want %s", mimeType, got, want)
		}
	}
	if got := (thumbnailerEntry{argv: []string{"t", "100%%", "%i"}}).expand("/in", "", "", 1); !reflect.DeepEqual(got, []string{"t", "100%", "/in"}) {
		t.Errorf("%%%% and %%i expand to %q", got)
	}
}

// A thumbnailer missing what it needs serves no type.
func TestAThumbnailerMissingWhatItNeedsIsNotUsed(t *testing.T) {
	for name, body := range map[string]string{
		"no exec":          "[Thumbnailer Entry]\nMimeType=image/png;\n",
		"broken exec":      "[Thumbnailer Entry]\nExec=thumb '%i\nMimeType=image/png;\n",
		"outside group":    "Exec=thumb %i %o\nMimeType=image/png;\n",
		"try-exec absent":  "[Thumbnailer Entry]\nTryExec=/nonexistent/thumbnailer\nExec=thumb %i %o\nMimeType=image/png;\n",
		"another group":    "[Desktop Entry]\nExec=thumb %i %o\nMimeType=image/png;\n",
		"no mimetype list": "[Thumbnailer Entry]\nExec=thumb %i %o\n",
	} {
		dir := t.TempDir()
		writeThumbnailer(t, dir, "x", body)
		if byType := loadThumbnailers([]string{dir}); len(byType) != 0 {
			t.Errorf("%s: serves %v", name, byType)
		}
	}
}
