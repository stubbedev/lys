package filebrowser

import (
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"time"
	"unsafe"

	"github.com/stubbedev/lys/desktop/vfs"
)

// pathChunk is how much of a listing's paths is written at a time.
const pathChunk = 256 << 10

// remoteTimeout bounds one call to a network location the browser
// makes: a listing, a stat, a folder made.
const remoteTimeout = 30 * time.Second

// listDir lists the folder loc on whatever filesystem it lies on, in
// full, sorted by path. Only the kinds rule applies here, because it
// cannot change while the listing is cached; the hidden toggle, the
// type filter and the search text partition the cached listing. A
// folder that cannot be read is an error; an entry that cannot be
// stated (a dangling link) is listed as a file with nothing known
// about it.
func listDir(mounts *vfs.Mounts, loc string, kinds Kinds) ([]Entry, error) {
	fsys, p, err := mounts.Resolve(loc)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), remoteTimeout)
	defer cancel()
	infos, err := fsys.ReadDir(ctx, p)
	if err != nil {
		return nil, err
	}
	paths := pathArena{prefix: dirPrefix(loc)}
	entries := make([]Entry, 0, len(infos))
	for _, info := range infos {
		if !info.IsDir && kinds == FoldersOnly {
			continue
		}
		entries = append(entries, Entry{Path: paths.join(info.Name), IsDir: info.IsDir, Size: info.Size, Modified: info.Modified})
	}
	slices.SortFunc(entries, func(a, b Entry) int { return strings.Compare(a.Path, b.Path) })
	return entries, nil
}

// dirPrefix is what a name in dir is appended to for its path.
func dirPrefix(dir string) string {
	if strings.HasSuffix(dir, "/") {
		return dir
	}
	return dir + "/"
}

// pathArena writes a listing's paths, prefix and name, into shared
// chunks on the Go heap and hands out strings over them: a chunk is
// only ever appended to, and the strings keep it alive for as long as
// any row or label holds one.
type pathArena struct {
	prefix string
	chunk  []byte
}

func (a *pathArena) join(name string) string {
	n := len(a.prefix) + len(name)
	if cap(a.chunk)-len(a.chunk) < n {
		a.chunk = make([]byte, 0, max(pathChunk, n))
	}
	start := len(a.chunk)
	a.chunk = append(a.chunk, a.prefix...)
	a.chunk = append(a.chunk, name...)
	return unsafe.String(&a.chunk[start], n) //nolint:gosec // the chunk is never written below its length again
}

// statEntry states loc, following a link, into an Entry: false when
// nothing is there or kinds does not list it. A dangling link is a
// file with nothing known about it.
func statEntry(mounts *vfs.Mounts, loc string, kinds Kinds) (Entry, bool) {
	fsys, p, err := mounts.Resolve(loc)
	if err != nil {
		return Entry{}, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), remoteTimeout)
	defer cancel()
	info, err := fsys.Stat(ctx, p)
	if err != nil || !info.IsDir && kinds == FoldersOnly {
		return Entry{}, false
	}
	return Entry{Path: loc, IsDir: info.IsDir, Size: info.Size, Modified: info.Modified}, true
}

// readHidden is what the folder loc's .hidden file holds, nil without
// one.
func readHidden(mounts *vfs.Mounts, loc string) []byte {
	fsys, p, err := mounts.Resolve(vfs.Join(loc, ".hidden"))
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), remoteTimeout)
	defer cancel()
	f, err := fsys.Open(ctx, p)
	if err != nil {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(f, hiddenMax))
	if err = errors.Join(err, f.Close()); err != nil {
		return nil
	}
	return data
}

// hiddenMax bounds the .hidden file read.
const hiddenMax = 1 << 20
