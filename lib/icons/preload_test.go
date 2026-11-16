package icons

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"testing"

	"github.com/stubbedev/lys/lib/assetfile"
	"github.com/stubbedev/lys/lib/waittest"
)

// heldReadFS parks every ReadFile until release hands it the contents,
// closing entered once a read is inside.
type heldReadFS struct {
	osFS
	once    sync.Once
	entered chan struct{}
	release chan string
}

func (f *heldReadFS) ReadFile(string) ([]byte, error) {
	f.once.Do(func() { close(f.entered) })
	return []byte(<-f.release), nil
}

// A held read keeps the preload inside it for as long as the test wants:
// the read must not hold the lock, and a theme switch during it must keep
// the stale raster out.
func TestPreloadReadsOutsideTheLockAndDropsAStaleDecode(t *testing.T) {
	c := newTestCache(t, "hicolor")
	fsys := &heldReadFS{entered: make(chan struct{}), release: make(chan string)}
	c.fsys = fsys
	path := filepath.Join(t.TempDir(), "held.svg")
	done := make(chan struct{})
	go func() {
		defer close(done)
		c.Preload(path, 16, 120)
	}()
	<-fsys.entered
	if !c.mu.TryLock() {
		t.Fatal("the lock is held across the read: a painting UI loop would wait")
	}
	c.mu.Unlock()
	if c.Cached(path, 16, 120) {
		t.Error("cached before its decode finished")
	}
	c.InvalidateTheme()
	fsys.release <- `<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16"/>`
	waittest.For(t, done, "the preload to finish")
	if c.Cached(path, 16, 120) {
		t.Error("a raster decoded across a theme switch must not be kept")
	}
}

func TestCachedNeverWaits(t *testing.T) {
	c := newTestCache(t, "hicolor")
	path := filepath.Join(t.TempDir(), "a.svg")
	if err := os.WriteFile(path, []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16"/>`), 0o600); err != nil {
		t.Fatal(err)
	}
	c.Preload(path, 16, 120)
	if !c.Cached(path, 16, 120) {
		t.Fatal("not cached after its preload")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.Cached(path, 16, 120) {
		t.Error("with the lock held elsewhere the answer is no")
	}
}

func TestAPipeAtAnIconPathIsRefusedWithoutBlocking(t *testing.T) {
	c := newTestCache(t, "hicolor")
	path := filepath.Join(t.TempDir(), "pipe.svg")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		c.Preload(path, 16, 120)
	}()
	waittest.For(t, done, "the preload to refuse the pipe")
	if r, ok := c.Peek(path, 16, 120); !ok || !errors.Is(r.Err, assetfile.ErrNotRegular) {
		t.Errorf("raster %+v, %v: want the pipe refused", r, ok)
	}
}
