// Package vfstest is what the vfs filesystems are tested with: an
// in-memory filesystem, and the behavior every filesystem conforms to.
package vfstest

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"maps"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/stubbedev/lys/desktop/vfs"
)

// MemFS is a filesystem in memory, a stand-in for a connected location:
// safe for concurrent use, its files readers at any offset.
type MemFS struct {
	mu     sync.Mutex
	nodes  map[string]*memNode
	closed bool
	now    func() time.Time
}

type memNode struct {
	dir      bool
	data     []byte
	modified time.Time
}

// NewMemFS is an empty filesystem: its root folder alone.
func NewMemFS() *MemFS {
	now := func() time.Time { return time.Unix(1_700_000_000, 0).UTC() }
	return &MemFS{nodes: map[string]*memNode{"/": {dir: true, modified: now()}}, now: now}
}

// Closed reports whether the filesystem was closed (disconnected).
func (m *MemFS) Closed() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.closed
}

// Close disconnects the filesystem: every later call fails.
func (m *MemFS) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

// WriteFile puts data at p, making its folders, for a test to set up.
func (m *MemFS) WriteFile(p string, data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mkdirAll(path.Dir(p))
	m.nodes[p] = &memNode{data: bytes.Clone(data), modified: m.now()}
}

// MkdirAll makes p and its parents, for a test to set up.
func (m *MemFS) MkdirAll(p string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.mkdirAll(p)
}

func (m *MemFS) mkdirAll(p string) {
	for d := p; ; d = path.Dir(d) {
		if _, ok := m.nodes[d]; !ok {
			m.nodes[d] = &memNode{dir: true, modified: m.now()}
		}
		if d == "/" {
			return
		}
	}
}

// ReadFile is the content at p, false for none.
func (m *MemFS) ReadFile(p string) ([]byte, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n, ok := m.nodes[p]
	if !ok || n.dir {
		return nil, false
	}
	return bytes.Clone(n.data), true
}

func (m *MemFS) check(op, p string) (*memNode, error) {
	if m.closed {
		return nil, &fs.PathError{Op: op, Path: p, Err: fs.ErrClosed}
	}
	n, ok := m.nodes[p]
	if !ok {
		return nil, &fs.PathError{Op: op, Path: p, Err: fs.ErrNotExist}
	}
	return n, nil
}

func (m *MemFS) info(p string, n *memNode) vfs.Info {
	return vfs.Info{Name: path.Base(p), IsDir: n.dir, Size: int64(len(n.data)), Modified: n.modified}
}

// ReadDir implements vfs.FS.
func (m *MemFS) ReadDir(_ context.Context, dir string) ([]vfs.Info, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n, err := m.check("readdir", dir)
	if err != nil {
		return nil, err
	}
	if !n.dir {
		return nil, &fs.PathError{Op: "readdir", Path: dir, Err: fs.ErrInvalid}
	}
	prefix := strings.TrimSuffix(dir, "/") + "/"
	var infos []vfs.Info
	for p, c := range m.nodes {
		if p != "/" && strings.HasPrefix(p, prefix) && !strings.Contains(p[len(prefix):], "/") {
			infos = append(infos, m.info(p, c))
		}
	}
	return infos, nil
}

// Stat implements vfs.FS.
func (m *MemFS) Stat(_ context.Context, p string) (vfs.Info, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n, err := m.check("stat", p)
	if err != nil {
		return vfs.Info{}, err
	}
	return m.info(p, n), nil
}

// Open implements vfs.FS.
func (m *MemFS) Open(_ context.Context, p string) (vfs.File, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n, err := m.check("open", p)
	if err != nil {
		return nil, err
	}
	if n.dir {
		return nil, &fs.PathError{Op: "open", Path: p, Err: fs.ErrInvalid}
	}
	return memFile{bytes.NewReader(bytes.Clone(n.data))}, nil
}

type memFile struct{ *bytes.Reader }

func (memFile) Close() error { return nil }

// Create implements vfs.FS.
func (m *MemFS) Create(_ context.Context, p string) (io.WriteCloser, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.free("create", p); err != nil {
		return nil, err
	}
	n := &memNode{modified: m.now()}
	m.nodes[p] = n
	return &memWriter{m: m, n: n}, nil
}

// free checks p may be made: its folder is there and p is not.
func (m *MemFS) free(op, p string) error {
	if m.closed {
		return &fs.PathError{Op: op, Path: p, Err: fs.ErrClosed}
	}
	if parent, ok := m.nodes[path.Dir(p)]; !ok || !parent.dir {
		return &fs.PathError{Op: op, Path: p, Err: fs.ErrNotExist}
	}
	if _, ok := m.nodes[p]; ok {
		return &fs.PathError{Op: op, Path: p, Err: fs.ErrExist}
	}
	return nil
}

type memWriter struct {
	m *MemFS
	n *memNode
}

func (w *memWriter) Write(b []byte) (int, error) {
	w.m.mu.Lock()
	defer w.m.mu.Unlock()
	w.n.data = append(w.n.data, b...)
	return len(b), nil
}

func (w *memWriter) Close() error { return nil }

// Mkdir implements vfs.FS.
func (m *MemFS) Mkdir(_ context.Context, p string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := m.free("mkdir", p); err != nil {
		return err
	}
	m.nodes[p] = &memNode{dir: true, modified: m.now()}
	return nil
}

// Rename implements vfs.FS.
func (m *MemFS) Rename(_ context.Context, from, to string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := m.check("rename", from); err != nil {
		return err
	}
	if err := m.free("rename", to); err != nil {
		return err
	}
	prefix := from + "/"
	moved := map[string]*memNode{}
	for p, n := range m.nodes {
		switch {
		case p == from:
			moved[to] = n
		case strings.HasPrefix(p, prefix):
			moved[to+"/"+p[len(prefix):]] = n
		default:
			continue
		}
		delete(m.nodes, p)
	}
	maps.Copy(m.nodes, moved)
	return nil
}

// Remove implements vfs.FS.
func (m *MemFS) Remove(_ context.Context, p string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	n, err := m.check("remove", p)
	if err != nil {
		return err
	}
	if n.dir {
		prefix := strings.TrimSuffix(p, "/") + "/"
		for q := range m.nodes {
			if strings.HasPrefix(q, prefix) {
				return &fs.PathError{Op: "remove", Path: p, Err: errors.New("directory not empty")}
			}
		}
	}
	delete(m.nodes, p)
	return nil
}
