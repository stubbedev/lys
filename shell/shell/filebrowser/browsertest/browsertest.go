// Package browsertest holds the file browser's test fixtures, shared
// by the browser's tests and its hosts'.
package browsertest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stubbedev/lys/desktop/mime"
)

// MIME is a shared-mime-info fixture with the types the tests guess.
func MIME(t *testing.T) *mime.Database {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "mime"), 0o750); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"globs2": "50:image/png:*.png\n50:image/jpeg:*.jpg\n50:image/webp:*.webp\n50:image/gif:*.gif\n" +
			"50:text/plain:*.txt\n50:text/x-go:*.go\n50:text/markdown:*.md\n50:image/svg+xml:*.svg\n50:application/pdf:*.pdf\n50:application/epub+zip:*.epub\n",
		"subclasses": "image/svg+xml application/xml\napplication/xml text/plain\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, "mime", name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return mime.Load([]string{dir})
}

// Touch makes each of rel under root: a folder for a name ending in /,
// else an empty file, with the folders above it.
func Touch(t *testing.T, root string, rel ...string) {
	t.Helper()
	for _, r := range rel {
		p := filepath.Join(root, r)
		if strings.HasSuffix(r, "/") {
			if err := os.MkdirAll(p, 0o750); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// Tree is the browser fixture: root/{docs/, pics/a.png, b.txt, a.png,
// .dot.txt}.
func Tree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	Touch(t, root, "docs/", "pics/a.png", "b.txt", "a.png", ".dot.txt")
	return root
}

// Trashed puts a file in trash/files under name with the info text
// given ("" writes none: an orphan), and returns its path.
func Trashed(t *testing.T, trash, name, info string) string {
	t.Helper()
	for _, d := range []string{"files", "info"} {
		if err := os.MkdirAll(filepath.Join(trash, d), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	path := filepath.Join(trash, "files", name)
	if err := os.WriteFile(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if info != "" {
		if err := os.WriteFile(filepath.Join(trash, "info", name+".trashinfo"), []byte(info), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

// TrashInfo is the info of a file trashed from path.
func TrashInfo(path string) string {
	return "[Trash Info]\nPath=" + path + "\nDeletionDate=2026-10-01T12:00:00\n"
}
