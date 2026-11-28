package vfs

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
)

// Mount is a connected location: its server, what it shows as, the
// location it opens at, and its filesystem.
type Mount struct {
	Server Server
	Label  string
	Home   string
	Conn   Conn
}

// Mounts are the filesystems a process browses: the local disk and
// every location connected, each mounted at its server's root. It is
// safe for concurrent use.
type Mounts struct {
	local    *LocalFS
	mu       sync.Mutex
	mounts   map[Server]Mount
	watchers map[uint64]func()
	next     uint64
}

// NewMounts is the local disk with nothing connected.
func NewMounts() *Mounts {
	return &Mounts{local: NewLocal(), mounts: map[Server]Mount{}, watchers: map[uint64]func(){}}
}

// Local is the local disk's filesystem.
func (m *Mounts) Local() *LocalFS { return m.local }

// Add mounts mt's connection at its server's root; a server already
// mounted is an error.
func (m *Mounts) Add(mt Mount) error {
	m.mu.Lock()
	if _, ok := m.mounts[mt.Server]; ok {
		m.mu.Unlock()
		return fmt.Errorf("mount %s: already connected", mt.Server)
	}
	m.mounts[mt.Server] = mt
	m.mu.Unlock()
	m.changed()
	return nil
}

// Remove disconnects s, closing its connection.
func (m *Mounts) Remove(s Server) error {
	m.mu.Lock()
	mt, ok := m.mounts[s]
	delete(m.mounts, s)
	m.mu.Unlock()
	if !ok {
		return &NotMountedError{Server: s}
	}
	m.changed()
	if err := mt.Conn.Close(); err != nil {
		return fmt.Errorf("disconnect %s: %w", s, err)
	}
	return nil
}

// Mounted reports whether s is connected.
func (m *Mounts) Mounted(s Server) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	_, ok := m.mounts[s]
	return ok
}

// List is every location connected, ordered by its root.
func (m *Mounts) List() []Mount {
	m.mu.Lock()
	list := make([]Mount, 0, len(m.mounts))
	for _, mt := range m.mounts {
		list = append(list, mt)
	}
	m.mu.Unlock()
	slices.SortFunc(list, func(a, b Mount) int { return strings.Compare(a.Server.String(), b.Server.String()) })
	return list
}

// Resolve is the filesystem loc lies on and loc's path on it: the local
// disk for a local path, the server's mount for a remote one, an
// *NotMountedError while it is not connected.
func (m *Mounts) Resolve(loc string) (FS, string, error) {
	if !IsRemote(loc) {
		return m.local, loc, nil
	}
	l, err := Parse(loc)
	if err != nil {
		return nil, "", err
	}
	m.mu.Lock()
	mt, ok := m.mounts[l.Server]
	m.mu.Unlock()
	if !ok {
		return nil, "", &NotMountedError{Server: l.Server}
	}
	return mt.Conn, l.Path, nil
}

// Watch calls changed (on the goroutine that connected or disconnected)
// each time a location is connected or disconnected, until stop.
func (m *Mounts) Watch(changed func()) (stop func()) {
	m.mu.Lock()
	id := m.next
	m.next++
	m.watchers[id] = changed
	m.mu.Unlock()
	return func() {
		m.mu.Lock()
		delete(m.watchers, id)
		m.mu.Unlock()
	}
}

func (m *Mounts) changed() {
	m.mu.Lock()
	ids := make([]uint64, 0, len(m.watchers))
	for id := range m.watchers {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	fns := make([]func(), len(ids))
	for i, id := range ids {
		fns[i] = m.watchers[id]
	}
	m.mu.Unlock()
	for _, fn := range fns {
		fn()
	}
}

// Close disconnects every location.
func (m *Mounts) Close() error {
	var errs []error
	for _, mt := range m.List() {
		errs = append(errs, m.Remove(mt.Server))
	}
	return errors.Join(errs...)
}
