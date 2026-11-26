package filebrowser

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/stubbedev/lys/desktop/fileuri"
	"github.com/stubbedev/lys/desktop/i18n"
	"github.com/stubbedev/lys/lib/atomicfile"
	"github.com/stubbedev/lys/lib/xdg"
)

// BookmarksFile is GTK 3's bookmarks file, which GTK 4 reads too; ""
// without a config directory.
func BookmarksFile(env xdg.Env) string {
	dir, ok := env.ConfigDir("gtk-3.0")
	if !ok {
		return ""
	}
	return filepath.Join(dir, "bookmarks")
}

// bookmarkLines are the bookmarks file's lines as they are, each with
// its line end, and its mode; none for a file that is not there.
func bookmarkLines(file string) ([]string, os.FileMode, error) {
	data, err := os.ReadFile(file) //nolint:gosec // the user's own GTK bookmarks
	if errors.Is(err, fs.ErrNotExist) {
		return nil, 0o644, nil
	}
	if err != nil {
		return nil, 0, err
	}
	mode := os.FileMode(0o644)
	if info, err := os.Stat(file); err == nil {
		mode = info.Mode().Perm()
	}
	return strings.SplitAfter(string(data), "\n"), mode, nil
}

// bookmarked reports a bookmark line naming dir.
func bookmarked(line, dir string) bool {
	uri, _, _ := strings.Cut(strings.TrimSpace(line), " ")
	path, ok := fileuri.Path(uri)
	return ok && filepath.Clean(path) == dir
}

// addBookmark appends dir to the bookmarks file unless a line names it
// already, which reports false. Every other line stays byte for byte.
func addBookmark(file, dir string) (bool, error) {
	lines, mode, err := bookmarkLines(file)
	if err != nil {
		return false, fmt.Errorf("bookmarking %s: %w", dir, err)
	}
	for _, l := range lines {
		if bookmarked(l, dir) {
			return false, nil
		}
	}
	if n := len(lines); n > 0 && lines[n-1] != "" && !strings.HasSuffix(lines[n-1], "\n") {
		lines[n-1] += "\n"
	}
	lines = append(lines, fileuri.FromPath(dir)+"\n")
	if err := writeBookmarks(file, lines, mode); err != nil {
		return false, fmt.Errorf("bookmarking %s: %w", dir, err)
	}
	return true, nil
}

// removeBookmark drops the lines naming dir from the bookmarks file;
// every other line stays byte for byte.
func removeBookmark(file, dir string) error {
	lines, mode, err := bookmarkLines(file)
	if err != nil {
		return fmt.Errorf("removing the bookmark of %s: %w", dir, err)
	}
	kept := lines[:0]
	for _, l := range lines {
		if !bookmarked(l, dir) {
			kept = append(kept, l)
		}
	}
	if err := writeBookmarks(file, kept, mode); err != nil {
		return fmt.Errorf("removing the bookmark of %s: %w", dir, err)
	}
	return nil
}

func writeBookmarks(file string, lines []string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(file), 0o700); err != nil {
		return err
	}
	return atomicfile.WriteFile(file, []byte(strings.Join(lines, "")), mode)
}

// bookmark adds dir to the GTK bookmarks off the loop, once, and lists
// it under Favorites.
func (c *Browser) bookmark(dir string) {
	file := c.d.BookmarksFile
	if file == "" || virtualLocation(dir) {
		return
	}
	OffLoop(c, func() error {
		_, err := addBookmark(file, dir)
		return err
	}, c.bookmarksChanged)
}

// unbookmark removes dir from the GTK bookmarks off the loop.
func (c *Browser) unbookmark(dir string) {
	file := c.d.BookmarksFile
	if file == "" {
		return
	}
	OffLoop(c, func() error { return removeBookmark(file, dir) }, c.bookmarksChanged)
}

// bookmarksChanged lists the bookmarks again, or warns that they could
// not be written.
func (c *Browser) bookmarksChanged(err error) {
	if err != nil {
		c.failed(i18n.T("file-chooser-bookmarks-failed"), err)
		return
	}
	OffLoop(c, c.d.Places, func(places []Place) { c.fillPlaces(c.ui.favorites, c.favorites(places)) })
}

// droppedOnSidebar bookmarks a folder dropped on the sidebar; a file
// dropped there opens its folder, as a drop on the files does.
func (c *Browser) droppedOnSidebar(path string) {
	OffLoop(c, statPath(path), func(st pathStat) {
		if st.err == nil && st.info.IsDir() {
			c.bookmark(path)
			return
		}
		c.dropped(path)
	})
}
