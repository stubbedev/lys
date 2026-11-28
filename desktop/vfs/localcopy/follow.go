package localcopy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/desktop/fswatch"
	"github.com/stubbedev/lys/desktop/vfs"
)

// Outcome is what became of a change to a followed copy.
type Outcome uint8

// The outcomes: the copy went back to the server; the file there
// changed since, so it did not, until Overwrite says to replace it; it
// could not go back (the location is not connected, the server
// refused), to be tried again at the next change.
const (
	Uploaded Outcome = iota + 1
	Conflicted
	Failed
)

// Report is one outcome, and for a failure why.
type Report struct {
	Outcome Outcome
	Err     error
}

// Reach is the filesystem a followed copy goes back to and the file's
// path on it, asked for each time (vfs.Mounts.Resolve of its location),
// since the location may be disconnected and connected again meanwhile.
type Reach func() (vfs.FS, string, error)

// uploadTimeout bounds one upload of a followed copy.
const uploadTimeout = 30 * time.Minute

// saved is the event mask a copy is saved with: written and closed, or
// another file renamed over it, as editors save.
const saved = unix.IN_CLOSE_WRITE | unix.IN_MOVED_TO

// Follower follows a local copy of a file on a network location and
// sends each version saved to it back in its place.
type Follower struct {
	local     string
	reach     Reach
	report    func(Report)
	w         *fswatch.Watcher
	overwrite chan struct{}
	ctx       context.Context
	cancel    context.CancelFunc
	done      sync.WaitGroup

	// want is what the server must hold for the next upload, sent the
	// local stamp of what went back last.
	want Expect
	sent localStamp
}

// localStamp is a version of the local copy: its size and modification
// time.
type localStamp struct {
	size     int64
	modified int64
}

func stampLocal(path string) (localStamp, error) {
	st, err := os.Stat(path)
	if err != nil {
		return localStamp{}, err
	}
	return localStamp{size: st.Size(), modified: st.ModTime().UnixNano()}, nil
}

// Follow follows the local copy local, the only file in a folder of its
// own, of the file reach reaches, which was what want expects when the
// copy was taken: each time the copy is saved, it is uploaded there
// (Upload), and report is told the outcome, off the caller's goroutine.
// A version saved while the file on the server changed since is
// Conflicted, and every later one too, until Overwrite replaces it.
// The copy as it is now counts as sent.
func Follow(local string, reach Reach, want Expect, report func(Report)) (*Follower, error) {
	sent, err := stampLocal(local)
	if err != nil {
		return nil, fmt.Errorf("follow %s: %w", local, err)
	}
	w, err := fswatch.NewNamed(saved)
	if err != nil {
		return nil, fmt.Errorf("follow %s: %w", local, err)
	}
	if err := w.Add(filepath.Dir(local)); err != nil {
		return nil, errors.Join(fmt.Errorf("follow %s: %w", local, err), w.Close())
	}
	ctx, cancel := context.WithCancel(context.Background())
	f := &Follower{
		local: local, reach: reach, report: report, w: w, overwrite: make(chan struct{}, 1),
		ctx: ctx, cancel: cancel, want: want, sent: sent,
	}
	f.done.Go(f.loop)
	return f, nil
}

// Overwrite sends the copy as it is now in place of the file on the
// server, whatever that holds: the answer to a conflict.
func (f *Follower) Overwrite() {
	select {
	case f.overwrite <- struct{}{}:
	default:
	}
}

// Close stops following, waiting for an upload under way to stop.
func (f *Follower) Close() error {
	f.cancel()
	err := f.w.Close()
	f.done.Wait()
	return err
}

func (f *Follower) loop() {
	name := filepath.Base(f.local)
	for {
		select {
		case _, ok := <-f.w.Changed():
			if !ok {
				return
			}
			if b := f.w.Take(); b.Rescan || touched(b, name) {
				f.sync(false)
			}
		case <-f.overwrite:
			f.sync(true)
		case <-f.ctx.Done():
			return
		}
	}
}

func touched(b fswatch.Batch, name string) bool {
	for _, e := range b.Events {
		if e.Name == name {
			return true
		}
	}
	return false
}

// sync uploads the copy when it changed since it was last sent, or
// with force whatever the server holds.
func (f *Follower) sync(force bool) {
	now, err := stampLocal(f.local)
	if err != nil {
		f.report(Report{Outcome: Failed, Err: fmt.Errorf("state the copy %s: %w", f.local, err)})
		return
	}
	if now == f.sent && !force {
		return
	}
	want := f.want
	if force {
		want = Anything()
	}
	fsys, p, err := f.reach()
	if err != nil {
		f.report(Report{Outcome: Failed, Err: fmt.Errorf("send %s back: %w", f.local, err)})
		return
	}
	ctx, cancel := context.WithTimeout(f.ctx, uploadTimeout)
	defer cancel()
	stamp, err := Upload(ctx, fsys, p, f.local, want)
	switch {
	case f.ctx.Err() != nil:
	case errors.Is(err, ErrConflict):
		f.report(Report{Outcome: Conflicted, Err: err})
	case err != nil:
		f.report(Report{Outcome: Failed, Err: err})
	default:
		f.want, f.sent = Unchanged(stamp), now
		f.report(Report{Outcome: Uploaded})
	}
}
