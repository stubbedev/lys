package localcopy

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/stubbedev/lys/desktop/vfs"
)

// ErrTooLarge is a file larger than a cache holds.
var ErrTooLarge = errors.New("the file is larger than the cache holds")

// Cache is a bounded set of fetched copies in one folder, the copies
// one process fetched: once they pass its bytes, the least recently
// used go. A copy is kept by its location and its stamp, so a file
// changed on the server is fetched again. It is safe for concurrent
// use.
type Cache struct {
	dir string
	max int64

	mu      sync.Mutex
	entries map[string]*entry
	held    int64
	clock   uint64
}

// entry is one copy: where it is, its size, when it was last asked
// for, and while it is fetched, ready open and err unset.
type entry struct {
	path  string
	size  int64
	used  uint64
	ready chan struct{}
	err   error
}

// NewCache is a cache of at most max bytes in dir, which it empties:
// what an earlier run left there is no copy it knows.
func NewCache(dir string, max int64) (*Cache, error) {
	if err := os.RemoveAll(dir); err != nil {
		return nil, fmt.Errorf("empty the cache %s: %w", dir, err)
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("make the cache %s: %w", dir, err)
	}
	return &Cache{dir: dir, max: max, entries: map[string]*entry{}}, nil
}

// Fetch is the local copy of the file at the location loc, p on fsys,
// as info states it: the one fetched before while it is kept, else one
// fetched now. Asked for while another call fetches it, it waits for
// that one. A file larger than the cache is ErrTooLarge.
func (c *Cache) Fetch(ctx context.Context, fsys vfs.FS, loc, p string, info vfs.Info) (string, error) {
	if info.Size > c.max {
		return "", fmt.Errorf("fetch %s of %d bytes: %w", loc, info.Size, ErrTooLarge)
	}
	key := cacheKey(loc, StampOf(info))
	c.mu.Lock()
	if e, ok := c.entries[key]; ok {
		c.clock++
		e.used = c.clock
		c.mu.Unlock()
		select {
		case <-e.ready:
		case <-ctx.Done():
			return "", ctx.Err()
		}
		if e.err != nil {
			return "", e.err
		}
		return e.path, nil
	}
	c.clock++
	e := &entry{path: filepath.Join(c.dir, key, vfs.Base(loc)), used: c.clock, ready: make(chan struct{})}
	c.entries[key] = e
	c.mu.Unlock()

	size, err := c.fill(ctx, fsys, p, e.path, StampOf(info))
	c.mu.Lock()
	defer c.mu.Unlock()
	if err != nil {
		e.err = fmt.Errorf("fetch %s: %w", loc, err)
		delete(c.entries, key)
		close(e.ready)
		return "", e.err
	}
	e.size = size
	c.held += size
	close(e.ready)
	c.evict(key)
	return e.path, nil
}

// fill fetches p into path, a folder of its own, as stamp states it.
func (c *Cache) fill(ctx context.Context, fsys vfs.FS, p, path string, stamp Stamp) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return 0, err
	}
	got, err := Fetch(ctx, fsys, p, path)
	if err == nil && !got.Equal(stamp) {
		err = fmt.Errorf("%w: listed as %d bytes, read as %d", ErrChanged, stamp.Size, got.Size)
	}
	if err != nil {
		return 0, errors.Join(err, os.RemoveAll(filepath.Dir(path)))
	}
	return got.Size, nil
}

// evict removes the least recently used copies fetched until the cache
// holds no more than its bytes, never keep's.
func (c *Cache) evict(keep string) {
	for c.held > c.max {
		victim := ""
		var oldest uint64
		for k, e := range c.entries {
			if k == keep || e.err != nil || !closed(e.ready) {
				continue
			}
			if victim == "" || e.used < oldest {
				victim, oldest = k, e.used
			}
		}
		if victim == "" {
			return
		}
		e := c.entries[victim]
		delete(c.entries, victim)
		c.held -= e.size
		if err := os.RemoveAll(filepath.Dir(e.path)); err != nil {
			log.Printf("localcopy: drop the copy %s: %v", e.path, err)
		}
	}
}

// Held is the bytes of the copies the cache keeps.
func (c *Cache) Held() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.held
}

func closed(ch chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

// cacheKey names the copy of the file at loc as s stamps it.
func cacheKey(loc string, s Stamp) string {
	sum := sha256.Sum256([]byte(loc + "\x00" + strconv.FormatInt(s.Size, 10) + "\x00" + strconv.FormatInt(s.Modified.UnixNano(), 10)))
	return hex.EncodeToString(sum[:16])
}
