// Package recentfiles tracks the recently used files lys-ui apps open
// in the desktop's shared recently-used.xbel (lib/recentfiles), so they
// share the recents list with the desktop's other citizens. The manager
// keeps the list bounded, most-recent first, and writes atomically; a
// missing or corrupt file starts an empty list.
//
// Load, Save, Add and Remove wait on the disk. An application drives a
// Manager through Post, which runs everything in order off the loop.
package recentfiles

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/stubbedev/lys/lib/atomicfile"
	"github.com/stubbedev/lys/lib/recentfiles"
	"github.com/stubbedev/lys/lib/xdg"

	"github.com/stubbedev/lys/ui/internal/worker"
)

// Entry is one recently used file, as the shared list holds it.
type Entry = recentfiles.Entry

// maxEntries bounds the list: the spec's consumers show a handful, and
// an unbounded file grows forever.
const maxEntries = 100

// queueDepth bounds what a stuck disk can pile up behind the worker.
const queueDepth = 64

// Manager reads and writes one recents list. It is not safe for
// concurrent use: whoever posts to it touches it only through Post.
type Manager struct {
	path    string
	entries []Entry
	work    *worker.Queue
}

// DefaultPath is the desktop's shared list under env's user data
// directory (recentfiles.DefaultPath).
func DefaultPath(env xdg.Env) (string, error) { return recentfiles.DefaultPath(env) }

// New returns the manager for the given xbel path (DefaultPath for the
// desktop's shared list).
func New(path string) *Manager {
	return &Manager{path: path, work: worker.New(queueDepth)}
}

// Post queues work to run off the caller's goroutine, in order, and
// reports whether it was taken: a full or stopped queue refuses it.
func (m *Manager) Post(work func()) bool { return m.work.Post(work) }

// Stop refuses later posts, lets queued work finish within limit, and
// reports whether it did.
func (m *Manager) Stop(limit time.Duration) bool { return m.work.Stop(limit) }

// List returns the entries, most recent first.
func (m *Manager) List() []Entry {
	out := make([]Entry, len(m.entries))
	copy(out, m.entries)
	return out
}

// Add moves paths to the front of the list under app, the last one
// first, stamps them now, and persists once. Errors persisting are
// reported once here; a failed write never loses the in-memory list.
func (m *Manager) Add(app string, paths ...string) error {
	now := time.Now()
	for _, path := range paths {
		path = filepath.Clean(path)
		m.entries = slices.DeleteFunc(m.entries, func(e Entry) bool { return e.Path == path })
		m.entries = slices.Insert(m.entries, 0, Entry{Path: path, App: app, When: now})
	}
	if len(m.entries) > maxEntries {
		m.entries = m.entries[:maxEntries]
	}
	return m.Save()
}

// Remove drops path from the list and persists.
func (m *Manager) Remove(path string) error {
	path = filepath.Clean(path)
	m.entries = slices.DeleteFunc(m.entries, func(e Entry) bool {
		return e.Path == path
	})
	return m.Save()
}

// Load reads the list (recentfiles.Load), newest first and bounded; a
// missing file is an empty list, a corrupt one an empty list plus the
// error (the next Add rewrites it clean).
func (m *Manager) Load() error {
	entries, err := recentfiles.Load(m.path)
	if err != nil {
		m.entries = nil
		return err
	}
	m.entries = entries[:min(len(entries), maxEntries)]
	return nil
}

// Save writes the list atomically (temp file + rename); an empty list
// writes an empty xbel, not a deletion, so a shared desktop list does
// not flap between present and absent.
func (m *Manager) Save() error {
	body, err := recentfiles.Marshal(m.entries, "lys-ui")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(m.path), 0o700); err != nil {
		return fmt.Errorf("recentfiles: create the directory of %s: %w", m.path, err)
	}
	return atomicfile.WriteFile(m.path, body, 0o600)
}
