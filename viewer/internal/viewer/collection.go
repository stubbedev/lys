package viewer

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/stubbedev/lys/shell/shell/filebrowser"
)

type Collection struct {
	paths []string
	at    int
}

func Files(paths ...string) Collection { return Collection{paths: slices.Clone(paths)} }

func (c Collection) Paths() []string { return slices.Clone(c.paths) }

func (c Collection) Current() (string, bool) {
	if len(c.paths) == 0 {
		return "", false
	}
	return c.paths[c.at], true
}

func (c Collection) First() bool { return c.at == 0 }

func (c Collection) Last() bool { return c.at >= len(c.paths)-1 }

func (c Collection) Step(delta int) (Collection, bool) {
	at := c.at + delta
	if len(c.paths) == 0 || at < 0 || at >= len(c.paths) {
		return c, false
	}
	return Collection{paths: c.paths, at: at}, true
}

func (c Collection) Jump(at int) (Collection, bool) {
	if at < 0 || at >= len(c.paths) || at == c.at {
		return c, false
	}
	return Collection{paths: c.paths, at: at}, true
}

func (c Collection) End() int { return len(c.paths) - 1 }

func (c Collection) Without(path string) (Collection, int, bool) {
	i := slices.Index(c.paths, path)
	if i < 0 {
		return c, -1, false
	}
	at := c.at
	if i < at || at == len(c.paths)-1 && at > 0 {
		at--
	}
	return Collection{paths: slices.Delete(slices.Clone(c.paths), i, i+1), at: at}, i, true
}

func (c Collection) With(path string, at int) Collection {
	at = min(max(at, 0), len(c.paths))
	return Collection{paths: slices.Insert(slices.Clone(c.paths), at, path), at: at}
}

func Open(o opens, paths []string) (Collection, error) {
	if len(paths) != 1 {
		return Files(paths...), nil
	}
	path := paths[0]
	st, err := os.Stat(path)
	switch {
	case err == nil && st.IsDir():
		return listFolder(o, path, "")
	case err != nil:
		return Files(path), fmt.Errorf("open %s: %w", path, err)
	}
	return listFolder(o, filepath.Dir(path), path)
}

func listFolder(o opens, dir, opened string) (Collection, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if opened == "" {
			return Collection{}, fmt.Errorf("list the files of %s: %w", dir, err)
		}
		return Files(opened), fmt.Errorf("list the files beside %s: %w", opened, err)
	}
	files := make([]filebrowser.Entry, 0, len(entries))
	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if path == opened || !e.IsDir() && !strings.HasPrefix(e.Name(), ".") && o.name(path) {
			files = append(files, filebrowser.Entry{Path: path})
		}
	}
	if opened != "" && !slices.ContainsFunc(files, func(e filebrowser.Entry) bool { return e.Path == opened }) {
		return Files(opened), errors.New("list the files beside " + opened + ": it is not in its folder")
	}
	filebrowser.SortEntries(files, filebrowser.SortName, true)
	c := Collection{paths: make([]string, len(files))}
	for i, e := range files {
		c.paths[i] = e.Path
		if e.Path == opened {
			c.at = i
		}
	}
	return c, nil
}
