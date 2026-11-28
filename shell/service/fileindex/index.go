package fileindex

import (
	"context"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stubbedev/lys/desktop/fswatch"
	"golang.org/x/sys/unix"
)

// DefaultMax is how many entries an index holds unless told otherwise:
// past it, what lies deepest under the roots is left out.
const DefaultMax = 50_000

// RecentMax is how many recently used files an index keeps.
const RecentMax = 200

// settle is how long a burst of changes is let run before the index
// reads what changed, so a build writing thousands of files is one
// update.
const settle = 150 * time.Millisecond

// watchedChanges are the events that change a directory's names.
const watchedChanges = unix.IN_CREATE | unix.IN_DELETE | unix.IN_MOVED_FROM | unix.IN_MOVED_TO |
	unix.IN_DELETE_SELF | unix.IN_MOVE_SELF

// Config is what an index covers.
type Config struct {
	// Roots are walked breadth first, each on its own filesystem.
	Roots []string
	Rules Rules
	// RecentPath is the desktop's recently-used.xbel; empty reads none.
	RecentPath string
	// Max caps the entries; 0 is DefaultMax.
	Max int
}

// File is one indexed name.
type File struct {
	// Folder is the folder's path, shared by every file in it.
	Folder string
	Child
}

// Path is the file's full path.
func (f File) Path() string { return join(f.Folder, f.Name) }

// Snapshot is the index at one moment. It is never written again.
type Snapshot struct {
	// Files are breadth first from the roots, so a cut index keeps the
	// shallowest.
	Files []File
	// Recent are the recently used files that exist, newest first.
	Recent []string
	// Ready is false until the first walk has finished.
	Ready bool
	// Full reports that the index holds Max entries and leaves the rest
	// out until something is deleted.
	Full bool
}

// Index follows the files under its roots for as long as it is open.
// Its walk and every update run on its own goroutine; Snapshot is safe
// from any.
type Index struct {
	snap    atomic.Pointer[Snapshot]
	watcher *fswatch.Watcher
	cancel  context.CancelFunc
	done    chan struct{}
	once    sync.Once
}

// Open starts an index: the walk runs behind it, and Snapshot is empty
// and not Ready until it has.
func Open(cfg Config) (*Index, error) {
	if len(cfg.Roots) == 0 {
		return nil, errors.New("fileindex: no roots to index")
	}
	if cfg.Max <= 0 {
		cfg.Max = DefaultMax
	}
	w, err := fswatch.New(watchedChanges, false)
	if err != nil {
		return nil, fmt.Errorf("fileindex: watch %v: %w", cfg.Roots, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	ix := &Index{watcher: w, cancel: cancel, done: make(chan struct{})}
	ix.snap.Store(&Snapshot{})
	t := &tree{cfg: cfg, dirs: map[string]*node{}, watch: w.Add}
	go ix.run(ctx, t)
	return ix, nil
}

// Snapshot is the index as it stands.
func (ix *Index) Snapshot() *Snapshot { return ix.snap.Load() }

// Close stops following the files and waits for the goroutine to end.
func (ix *Index) Close() {
	ix.once.Do(func() {
		ix.cancel()
		_ = ix.watcher.Close()
		<-ix.done
	})
}

func (ix *Index) run(ctx context.Context, t *tree) {
	defer close(ix.done)
	t.refresh(ctx)
	if ctx.Err() != nil {
		return
	}
	ix.snap.Store(t.snapshot())
	timer := time.NewTimer(0)
	<-timer.C
	pending := false
	for {
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case _, ok := <-ix.watcher.Changed():
			if !ok {
				return
			}
			if !pending {
				pending = true
				timer.Reset(settle)
			}
		case <-timer.C:
			pending = false
			if t.refresh(ctx) {
				ix.snap.Store(t.snapshot())
			}
		}
	}
}

// node is one indexed directory.
type node struct {
	depth    int
	dev      uint64
	mtime    int64
	children []Child
}

// tree is the index's own state, touched only by its goroutine.
type tree struct {
	cfg   Config
	dirs  map[string]*node
	count int
	watch func(string) error
	// watchFailed has logged that a directory could not be watched.
	watchFailed bool
	loggedFull  bool
	recentMtime int64
	recent      []string
}

// refresh brings the tree up to the disk: roots not indexed yet are
// walked, directories whose modification time moved are listed again
// and what they gained or lost applied, and the recent files read again
// when their list or any directory changed, since a recent file deleted
// leaves them. It reports whether anything did.
func (t *tree) refresh(ctx context.Context) bool {
	changed := false
	for _, r := range t.cfg.Roots {
		if _, ok := t.dirs[r]; !ok {
			changed = t.walkFrom(ctx, []start{{path: r}}) || changed
		}
	}
	paths := make([]string, 0, len(t.dirs))
	for p := range t.dirs {
		paths = append(paths, p)
	}
	slices.Sort(paths)
	for _, p := range paths {
		if ctx.Err() != nil {
			return changed
		}
		n, ok := t.dirs[p]
		if !ok {
			continue
		}
		var st unix.Stat_t
		if err := unix.Stat(p, &st); err != nil || st.Mode&unix.S_IFMT != unix.S_IFDIR {
			t.drop(p)
			changed = true
			continue
		}
		if st.Mtim.Nano() == n.mtime {
			continue
		}
		changed = t.relist(ctx, p, n) || changed
	}
	return t.refreshRecent(changed) || changed
}

func (t *tree) refreshRecent(force bool) bool {
	if t.cfg.RecentPath == "" {
		return false
	}
	var mtime int64
	var st unix.Stat_t
	if err := unix.Stat(t.cfg.RecentPath, &st); err == nil {
		mtime = st.Mtim.Nano()
	}
	if mtime == t.recentMtime && t.recent != nil && !force {
		return false
	}
	if t.recent == nil {
		t.watchDir(filepath.Dir(t.cfg.RecentPath))
	}
	t.recentMtime = mtime
	recent := ReadRecent(t.cfg.RecentPath, RecentMax)
	if recent == nil {
		recent = []string{}
	}
	if slices.Equal(recent, t.recent) && t.recent != nil {
		return false
	}
	t.recent = recent
	return true
}

// walkFrom indexes the trees at starts within what the cap leaves.
func (t *tree) walkFrom(ctx context.Context, starts []start) bool {
	left := t.cfg.Max - t.count
	if left <= 0 {
		t.noteFull()
		return false
	}
	added := false
	end := walk(ctx, starts, t.cfg.Rules, left, func(d Dir) {
		t.dirs[d.Path] = &node{depth: d.Depth, dev: d.Dev, mtime: d.Mtime, children: d.Children}
		t.count += len(d.Children)
		t.watchDir(d.Path)
		added = true
	})
	if end == Full {
		t.noteFull()
	}
	return added
}

// relist reads p again and applies the difference to the tree.
func (t *tree) relist(ctx context.Context, p string, n *node) bool {
	l, err := t.cfg.Rules.List(p)
	if err != nil {
		t.drop(p)
		return true
	}
	n.mtime = l.Mtime
	now := make(map[Child]bool, len(l.Children))
	for _, c := range l.Children {
		now[c] = true
	}
	had := make(map[Child]bool, len(n.children))
	kept := n.children[:0:0]
	changed := false
	for _, c := range n.children {
		had[c] = true
		if now[c] {
			kept = append(kept, c)
			continue
		}
		changed = true
		t.count--
		if c.Dir {
			t.drop(join(p, c.Name))
		}
	}
	var fresh []start
	for _, c := range l.Children {
		if had[c] {
			continue
		}
		if t.count >= t.cfg.Max {
			t.noteFull()
			break
		}
		kept = append(kept, c)
		t.count++
		changed = true
		if c.Dir && n.depth+1 < MaxDepth {
			fresh = append(fresh, start{path: join(p, c.Name), depth: n.depth + 1, dev: n.dev})
		}
	}
	n.children = kept
	if len(fresh) > 0 {
		t.walkFrom(ctx, fresh)
	}
	return changed
}

// drop forgets p and everything indexed below it.
func (t *tree) drop(p string) {
	n, ok := t.dirs[p]
	if !ok {
		return
	}
	delete(t.dirs, p)
	t.count -= len(n.children)
	for _, c := range n.children {
		if c.Dir {
			t.drop(join(p, c.Name))
		}
	}
}

func (t *tree) watchDir(p string) {
	if err := t.watch(p); err != nil && !t.watchFailed {
		t.watchFailed = true
		log.Printf("fileindex: %v; changes there and in other directories that fail the same way stay unseen", err)
	}
}

func (t *tree) noteFull() {
	if !t.loggedFull {
		t.loggedFull = true
		log.Printf("fileindex: holding its cap of %d entries; deeper files are left out", t.cfg.Max)
	}
}

// snapshot lists the tree breadth first from the roots.
func (t *tree) snapshot() *Snapshot {
	files := make([]File, 0, t.count)
	queue := make([]string, 0, len(t.dirs))
	for _, r := range t.cfg.Roots {
		if _, ok := t.dirs[r]; ok {
			queue = append(queue, r)
		}
	}
	for i := 0; i < len(queue); i++ {
		p := queue[i]
		n := t.dirs[p]
		for _, c := range n.children {
			files = append(files, File{Folder: p, Child: c})
			if c.Dir {
				if _, ok := t.dirs[join(p, c.Name)]; ok {
					queue = append(queue, join(p, c.Name))
				}
			}
		}
	}
	return &Snapshot{Files: files, Recent: t.recent, Ready: true, Full: t.count >= t.cfg.Max}
}
