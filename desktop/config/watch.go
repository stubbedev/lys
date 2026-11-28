package config

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/stubbedev/lys/lib/atomicfile"

	"github.com/stubbedev/lys/desktop/fswatch"
)

// Hot reload over inotify: the config directory is watched recursively,
// plus the real parent of a symlinked main config (never the /nix/store
// root) on its own.
// Events debounce for 100ms, capped at 2s after the first pending one
// so a continuous stream still flushes; a flush reloads what changed:
// .env files, themes/, runtime.yaml alone, or everything, which is also
// what a batch the kernel lost events from reloads.

const (
	debounceDelay    = 100 * time.Millisecond
	maxDebounceDelay = 2 * time.Second
)

type watcher struct {
	svc    *Service
	tree   *fswatch.Watcher
	parent *fswatch.Watcher
	done   chan struct{}
	wg     sync.WaitGroup
}

func (s *Service) startWatcher() error {
	tree, err := fswatch.NewRecursiveNamed(fswatch.Changes)
	if err != nil {
		return &LoadError{msg: "cannot initialize file watcher", err: err}
	}
	if err := tree.Add(s.dir); err != nil {
		_ = tree.Close()
		return &LoadError{msg: "cannot watch '" + s.dir + "'", err: err}
	}
	w := &watcher{svc: s, tree: tree, done: make(chan struct{})}
	if canonical, err := filepath.EvalSymlinks(s.MainPath()); err == nil {
		if parent := filepath.Dir(canonical); parent != s.dir && !isImmutableStore(parent) {
			w.parent = watchParent(parent)
		}
	}
	s.watcher = w
	w.wg.Add(1)
	go w.debounce()
	return nil
}

// watchParent watches the real directory of a symlinked main config on
// its own, not the tree below it; nil (logged) when it cannot.
func watchParent(dir string) *fswatch.Watcher {
	w, err := fswatch.NewNamed(fswatch.Changes)
	if err != nil {
		log.Printf("config: failed to watch canonical config folder %s: %v", dir, err)
		return nil
	}
	if err := w.Add(dir); err != nil {
		_ = w.Close()
		log.Printf("config: failed to watch canonical config folder %s: %v", dir, err)
		return nil
	}
	return w
}

// isImmutableStore reports whether dir is the Nix store root: a Home
// Manager symlink resolves there, and watching it floods the watcher
// with every build on the machine for nothing.
func isImmutableStore(dir string) bool {
	store := os.Getenv("NIX_STORE_DIR")
	if store == "" {
		store = "/nix/store"
	}
	return dir == store
}

// changes are the flushed paths of one debounce, or everything when a
// batch lost events.
type changes struct {
	paths map[string]bool
	all   bool
}

func (c *changes) add(b fswatch.Batch) {
	if b.Rescan {
		c.all = true
		return
	}
	for _, e := range b.Events {
		c.paths[filepath.Join(e.Dir, e.Name)] = true
	}
}

// debounce batches changes and flushes them after the quiet period.
func (w *watcher) debounce() {
	defer w.wg.Done()
	pending := changes{paths: map[string]bool{}}
	var timer *time.Timer
	var fire <-chan time.Time
	var ceiling time.Time
	tree, parent := w.tree.Changed(), changedOf(w.parent)
	for {
		select {
		case _, ok := <-tree:
			if !ok {
				return
			}
			pending.add(w.tree.Take())
		case _, ok := <-parent:
			if !ok {
				parent = nil
				continue
			}
			pending.add(w.parent.Take())
		case <-fire:
			paths := make([]string, 0, len(pending.paths))
			for p := range pending.paths {
				paths = append(paths, p)
			}
			all := pending.all
			pending = changes{paths: map[string]bool{}}
			ceiling = time.Time{}
			fire = nil
			if err := w.svc.reloadChanges(paths, all); err != nil {
				log.Printf("config: config reload failed:\n%v", err)
			}
			continue
		case <-w.done:
			return
		}
		now := time.Now()
		if ceiling.IsZero() {
			ceiling = now.Add(maxDebounceDelay)
		}
		deadline := nextDeadline(now, ceiling)
		if timer == nil {
			timer = time.NewTimer(time.Until(deadline))
		} else {
			timer.Reset(time.Until(deadline))
		}
		fire = timer.C
	}
}

// changedOf is w's tick channel; nil, which never ticks, for no watcher.
func changedOf(w *fswatch.Watcher) <-chan struct{} {
	if w == nil {
		return nil
	}
	return w.Changed()
}

// nextDeadline is the debounce deadline for an event at now, never
// past the batch ceiling.
func nextDeadline(now, ceiling time.Time) time.Time {
	deadline := now.Add(debounceDelay)
	if deadline.After(ceiling) {
		return ceiling
	}
	return deadline
}

func (w *watcher) close() {
	close(w.done)
	w.wg.Wait()
	_ = w.tree.Close()
	if w.parent != nil {
		_ = w.parent.Close()
	}
}

// reloadChanges applies one flushed batch (reload_and_sync): what paths
// name, or everything when all is set.
func (s *Service) reloadChanges(paths []string, all bool) error {
	themes := filepath.Join(s.dir, "themes")
	runtime := s.RuntimePath()
	env, theme, main := all, all, all
	var runtimeOnly bool
	for _, p := range paths {
		switch {
		case isEnvFile(p):
			env = true
		case p == themes || strings.HasPrefix(p, themes+string(filepath.Separator)):
			theme = true
		case p == runtime || atomicfile.IsTemporary(runtime, p):
			runtimeOnly = true
		default:
			main = true
		}
	}
	if env {
		ReloadEnvFiles(s.dir)
		s.signal(s.secretsSubs)
	}
	if theme {
		s.signal(s.themeSubs)
	}
	switch {
	case main:
		return s.reloadMain()
	case runtimeOnly:
		s.reloadRuntime()
	}
	return nil
}
