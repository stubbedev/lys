package wallpaper

import (
	"log"

	"github.com/stubbedev/lys/desktop/fswatch"
	"golang.org/x/sys/unix"
)

// dirWatcher reports file-level changes in one cycling directory on
// events: a file created or deleted, any entry renamed in or out, or
// events the kernel lost. Data writes and subdirectory creation do not
// count.
type dirWatcher struct {
	w      *fswatch.Watcher
	events chan struct{}
}

const watchMask = unix.IN_CREATE | unix.IN_DELETE | unix.IN_MOVED_FROM | unix.IN_MOVED_TO

// watchDirectory starts a watch on dir; nil (logged) when the watch
// cannot be set up, which leaves cycling running without rescans.
func watchDirectory(dir string) *dirWatcher {
	w, err := fswatch.NewNamed(watchMask)
	if err != nil {
		log.Printf("wallpaper: cannot create directory watcher: %v", err)
		return nil
	}
	if err := w.Add(dir); err != nil {
		_ = w.Close()
		log.Printf("wallpaper: cannot watch directory %s: %v", dir, err)
		return nil
	}
	d := &dirWatcher{w: w, events: make(chan struct{}, 1)}
	go d.read()
	return d
}

func (d *dirWatcher) read() {
	for range d.w.Changed() {
		if !filesChanged(d.w.Take()) {
			continue
		}
		select {
		case d.events <- struct{}{}:
		default:
		}
	}
}

// filesChanged reports whether b changes a directory's files: it lost
// events, or one of them is fileEvent.
func filesChanged(b fswatch.Batch) bool {
	if b.Rescan {
		return true
	}
	for _, e := range b.Events {
		if fileEvent(e.Mask) {
			return true
		}
	}
	return false
}

// fileEvent reports whether an event's mask is a file-level change.
func fileEvent(mask uint32) bool {
	if mask&(unix.IN_MOVED_FROM|unix.IN_MOVED_TO) != 0 {
		return true
	}
	return mask&(unix.IN_CREATE|unix.IN_DELETE) != 0 && mask&unix.IN_ISDIR == 0
}

// close stops the watch; nil-safe.
func (d *dirWatcher) close() {
	if d != nil {
		_ = d.w.Close()
	}
}
