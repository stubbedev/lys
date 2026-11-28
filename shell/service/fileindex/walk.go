// Package fileindex finds files by name under a set of roots: the rules
// that keep caches, build outputs and hidden names out, the breadth-first
// walk the launcher's indexes share, the desktop's recently used files,
// and Index, which lys-shell keeps for its lifetime and follows through
// inotify so a search reads memory instead of the disk.
package fileindex

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"syscall"
)

// MaxDepth is how far below a root a walk descends.
const MaxDepth = 8

// hiddenFileBytes bounds a directory's .hidden file.
const hiddenFileBytes = 64 << 10

// skippedDirs are trees full of files nobody searches for by name:
// dependencies, caches and build outputs.
var skippedDirs = map[string]bool{
	"node_modules": true, "vendor": true, "target": true, "__pycache__": true,
	"venv": true, "dist": true, "build": true, "result": true,
}

// Rules say what a walk leaves out besides hidden names, the names a
// directory's .hidden file lists, symlinks and skippedDirs.
type Rules struct {
	// Exclude are directory names to skip.
	Exclude []string
}

func (r Rules) skipsDir(name string) bool {
	return skippedDirs[name] || slices.Contains(r.Exclude, name)
}

// Child is one name a directory listing keeps.
type Child struct {
	Name string
	// Fold is Name lower-cased, what a search compares; it shares Name's
	// bytes when Name has no upper case.
	Fold string
	Dir  bool
}

// Listing is one directory's kept children, its modification time when
// it was read and the filesystem it is on.
type Listing struct {
	Children []Child
	Mtime    int64
	Dev      uint64
}

// List reads dir once and keeps what r does not leave out.
func (r Rules) List(dir string) (Listing, error) {
	f, err := os.Open(dir) //nolint:gosec // a directory the walk reached
	if err != nil {
		return Listing{}, err
	}
	defer func() { _ = f.Close() }()
	var st syscall.Stat_t
	if err := syscall.Fstat(int(f.Fd()), &st); err != nil {
		return Listing{}, fmt.Errorf("stat %s: %w", dir, err)
	}
	entries, err := f.ReadDir(-1)
	if err != nil {
		return Listing{}, fmt.Errorf("list %s: %w", dir, err)
	}
	var hidden map[string]bool
	for _, e := range entries {
		if e.Name() == ".hidden" && e.Type().IsRegular() {
			hidden = readHidden(dir + "/.hidden")
			break
		}
	}
	out := Listing{Children: make([]Child, 0, len(entries)), Mtime: st.Mtim.Nano(), Dev: uint64(st.Dev)} //nolint:unconvert // Dev is uint32 on some architectures
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") || hidden[name] || e.Type()&os.ModeSymlink != 0 {
			continue
		}
		isDir := e.IsDir()
		if isDir && r.skipsDir(name) {
			continue
		}
		out.Children = append(out.Children, Child{Name: name, Fold: strings.ToLower(name), Dir: isDir})
	}
	return out, nil
}

// readHidden is the names a .hidden file lists, one per line, as file
// managers read it. An unreadable one hides nothing.
func readHidden(path string) map[string]bool {
	f, err := os.Open(path) //nolint:gosec // a .hidden file in a listed directory
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()
	names := map[string]bool{}
	sc := bufio.NewScanner(io.LimitReader(f, hiddenFileBytes))
	for sc.Scan() {
		if name := strings.TrimSpace(sc.Text()); name != "" {
			names[name] = true
		}
	}
	return names
}

// End is why a walk stopped.
type End uint8

const (
	// Done listed everything under its roots.
	Done End = iota
	// Full reached its limit of entries.
	Full
	// Cut saw its context end.
	Cut
)

// Dir is one directory a walk listed: its path, depth below its root and
// the children it kept, at most the walk's limit in all.
type Dir struct {
	Path  string
	Depth int
	Listing
}

// start is where a walk begins: a root at depth 0, or a directory found
// later at its own depth, on the filesystem dev (0 for a root's own).
type start struct {
	path  string
	depth int
	dev   uint64
}

// Walk lists roots breadth first, up to limit children in all, handing
// each directory to visit. It follows no symlink and stays on each
// root's filesystem, so a network mount cannot hang it.
func Walk(ctx context.Context, roots []string, rules Rules, limit int, visit func(Dir)) End {
	starts := make([]start, len(roots))
	for i, r := range roots {
		starts[i] = start{path: r}
	}
	return walk(ctx, starts, rules.List, limit, visit)
}

func walk(ctx context.Context, queue []start, list func(string) (Listing, error), limit int, visit func(Dir)) End {
	left := limit
	for len(queue) > 0 {
		if ctx.Err() != nil {
			return Cut
		}
		s := queue[0]
		queue = queue[1:]
		l, err := list(s.path)
		if err != nil || (s.dev != 0 && l.Dev != s.dev) {
			continue
		}
		full := len(l.Children) > left
		if full {
			l.Children = l.Children[:left]
		}
		left -= len(l.Children)
		visit(Dir{Path: s.path, Depth: s.depth, Listing: l})
		if s.depth+1 < MaxDepth {
			for _, c := range l.Children {
				if c.Dir {
					queue = append(queue, start{path: join(s.path, c.Name), depth: s.depth + 1, dev: l.Dev})
				}
			}
		}
		if full || left == 0 {
			return Full
		}
	}
	return Done
}

// join is dir/name without filepath.Join's cleaning, which a listed
// directory's path never needs.
func join(dir, name string) string {
	if strings.HasSuffix(dir, "/") {
		return dir + name
	}
	return dir + "/" + name
}
