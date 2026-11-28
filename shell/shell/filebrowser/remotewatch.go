package filebrowser

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/stubbedev/lys/desktop/fswatch"
	"github.com/stubbedev/lys/desktop/vfs"
)

// watchSource is a folder followed: through inotify on the local disk,
// by listing it again on a network location.
type watchSource interface {
	changeSource
	Close() error
}

// pollMaxNames is how many changed names one listing hands over; past
// it the folder is read again whole.
const pollMaxNames = 256

// pollWatch follows a folder on a network location, which tells of no
// change itself, by listing it every interval: it ticks with the names
// added, removed or changed since the listing before. A listing that
// fails (the location was disconnected, the server did not answer)
// changes nothing.
type pollWatch struct {
	ticks chan struct{}
	stop  chan struct{}
	done  sync.WaitGroup
	once  sync.Once

	mu      sync.Mutex
	pending []string
	rescan  bool
}

// listed is what a listing says of one entry, to tell a change by.
type listed struct {
	dir      bool
	size     int64
	modified int64
}

// pollFolder starts following the folder dir on mounts every interval.
func pollFolder(mounts *vfs.Mounts, dir string, every time.Duration) *pollWatch {
	w := &pollWatch{ticks: make(chan struct{}, 1), stop: make(chan struct{})}
	w.done.Go(func() {
		defer close(w.ticks)
		last, ok := listFolder(mounts, dir)
		t := time.NewTicker(every)
		defer t.Stop()
		for {
			select {
			case <-w.stop:
				return
			case <-t.C:
			}
			now, read := listFolder(mounts, dir)
			if !read {
				continue
			}
			if ok {
				w.note(changedNames(last, now))
			}
			last, ok = now, true
		}
	})
	return w
}

// listFolder is the entries of the folder dir by name, false when it
// cannot be listed.
func listFolder(mounts *vfs.Mounts, dir string) (map[string]listed, bool) {
	fsys, p, err := mounts.Resolve(dir)
	if err != nil {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), remoteTimeout)
	defer cancel()
	infos, err := fsys.ReadDir(ctx, p)
	if err != nil {
		return nil, false
	}
	out := make(map[string]listed, len(infos))
	for _, i := range infos {
		out[i.Name] = listed{dir: i.IsDir, size: i.Size, modified: i.Modified.UnixNano()}
	}
	return out, true
}

// changedNames are the names added, removed or changed from before to
// now, in name order.
func changedNames(before, now map[string]listed) []string {
	var names []string
	for name, l := range now {
		if b, ok := before[name]; !ok || b != l {
			names = append(names, name)
		}
	}
	for name := range before {
		if _, ok := now[name]; !ok {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}

// note keeps names for Take and ticks, unless there are none.
func (w *pollWatch) note(names []string) {
	if len(names) == 0 {
		return
	}
	w.mu.Lock()
	for _, n := range names {
		if !slices.Contains(w.pending, n) {
			w.pending = append(w.pending, n)
		}
	}
	if len(w.pending) > pollMaxNames {
		w.pending, w.rescan = nil, true
	}
	w.mu.Unlock()
	select {
	case w.ticks <- struct{}{}:
	default:
	}
}

// Changed ticks once per listing that found changes; it closes after
// Close.
func (w *pollWatch) Changed() <-chan struct{} { return w.ticks }

// Take hands over the names changed since the last Take.
func (w *pollWatch) Take() fswatch.Batch {
	w.mu.Lock()
	defer w.mu.Unlock()
	b := fswatch.Batch{Rescan: w.rescan}
	for _, n := range w.pending {
		b.Events = append(b.Events, fswatch.Event{Name: n})
	}
	w.pending, w.rescan = nil, false
	if b.Rescan {
		b.Events = nil
	}
	return b
}

// Close stops listing, waiting for a listing under way.
func (w *pollWatch) Close() error {
	w.once.Do(func() { close(w.stop) })
	w.done.Wait()
	return nil
}
