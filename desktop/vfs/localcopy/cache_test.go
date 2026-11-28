package localcopy

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stubbedev/lys/desktop/vfs"
	"github.com/stubbedev/lys/desktop/vfs/vfstest"
)

// counting counts the files opened on it.
type counting struct {
	*vfstest.MemFS
	opens atomic.Int64
}

func (c *counting) Open(ctx context.Context, p string) (vfs.File, error) {
	c.opens.Add(1)
	return c.MemFS.Open(ctx, p)
}

func stat(t *testing.T, fsys vfs.FS, p string) vfs.Info {
	t.Helper()
	info, err := fsys.Stat(context.Background(), p)
	if err != nil {
		t.Fatal(err)
	}
	return info
}

// The cache fetches a file once for any number of callers at once, again
// once the file changed, drops the least recently used copy past its
// bytes, and refuses a file larger than it holds; a fetch that fails
// keeps nothing.
func TestTheCacheFetchesOnceAndStaysBounded(t *testing.T) {
	mem := &counting{MemFS: vfstest.NewMemFS()}
	for _, n := range []string{"a", "b", "c"} {
		mem.WriteFile("/"+n, []byte("1234"))
	}
	mem.WriteFile("/big", []byte("12345678901"))
	c, err := NewCache(filepath.Join(t.TempDir(), "cache"), 10)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	paths := make([]string, 8)
	for i := range paths {
		wg.Go(func() {
			p, err := c.Fetch(t.Context(), mem, "sftp://h/a", "/a", stat(t, mem, "/a"))
			if err != nil {
				t.Error(err)
			}
			paths[i] = p
		})
	}
	wg.Wait()
	if mem.opens.Load() != 1 || read(t, paths[0]) != "1234" || filepath.Base(paths[0]) != "a" {
		t.Fatalf("eight fetches at once opened the file %d times, the copy %s", mem.opens.Load(), paths[0])
	}
	if slices.ContainsFunc(paths, func(p string) bool { return p != paths[0] }) {
		t.Errorf("the fetches got %v, want one copy", paths)
	}
	b, err := c.Fetch(t.Context(), mem, "sftp://h/b", "/b", stat(t, mem, "/b"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Fetch(t.Context(), mem, "sftp://h/a", "/a", stat(t, mem, "/a")); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Fetch(t.Context(), mem, "sftp://h/c", "/c", stat(t, mem, "/c")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(b); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("the least recently used copy is still there: %v", err)
	}
	if _, err := os.Stat(paths[0]); err != nil {
		t.Errorf("the copy used since was dropped: %v", err)
	}
	if c.Held() > 10 {
		t.Errorf("the cache holds %d bytes, past its 10", c.Held())
	}
	if _, err := c.Fetch(t.Context(), mem, "sftp://h/big", "/big", stat(t, mem, "/big")); !errors.Is(err, ErrTooLarge) {
		t.Errorf("a file larger than the cache: %v, want %v", err, ErrTooLarge)
	}
	opens := mem.opens.Load()
	mem.WriteFile("/a", []byte("12"))
	again, err := c.Fetch(t.Context(), mem, "sftp://h/a", "/a", stat(t, mem, "/a"))
	if err != nil || read(t, again) != "12" || mem.opens.Load() != opens+1 {
		t.Errorf("the changed file fetched as %q, %v, opening it %d times more", read(t, again), err, mem.opens.Load()-opens)
	}
	listed := stat(t, mem, "/c")
	mem.WriteFile("/c", []byte("123"))
	if _, err := c.Fetch(t.Context(), mem, "sftp://h/c2", "/c", listed); !errors.Is(err, ErrChanged) {
		t.Errorf("a file changed since it was listed: %v, want %v", err, ErrChanged)
	}
	if _, err := c.Fetch(t.Context(), mem, "sftp://h/gone", "/gone", vfs.Info{Name: "gone", Size: 1}); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a file not there: %v", err)
	}
}
