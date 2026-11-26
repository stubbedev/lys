// Package fileops is the file operations of the file manager: copy,
// move, rename, delete, new files and folders, and the freedesktop
// trash. A long operation runs as a cancellable [Job] that reports its
// progress and asks how to resolve conflicts; each is planned over a
// scan of the files before anything is changed, and puts each thing in
// place whole or not at all.
package fileops

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

// Progress is how far a job is: files and bytes done of its totals,
// and the path it is working on.
type Progress struct {
	Files, TotalFiles int64
	Bytes, TotalBytes int64
	Current           string
}

// Action answers a conflict.
type Action uint8

const (
	// Skip leaves what is there and the source where it is.
	Skip Action = iota + 1
	// Replace puts the source in place of what is there.
	Replace
	// KeepBoth puts the source beside what is there under a free name.
	KeepBoth
)

// Conflict is a source whose target is already taken.
type Conflict struct {
	Source, Target           string
	SourceIsDir, TargetIsDir bool
}

// Resolution is the answer to a conflict; All answers every later
// conflict of the job the same way without asking.
type Resolution struct {
	Action Action
	All    bool
}

// Observer follows a job: it is told its progress and asked how to
// resolve each conflict, both on the job's goroutine.
type Observer interface {
	Progress(Progress)
	Resolve(Conflict) Resolution
}

// Op is an operation a job runs: [Copy], [Move], [Delete], [Trash],
// [EmptyTrash] or an [Undo].
type Op interface {
	run(r *runner) (*Undo, error)
}

// Job is an operation running in the background.
type Job struct {
	cancel context.CancelFunc
	done   chan struct{}
	undo   *Undo
	err    error
}

// Start runs op in the background, telling obs how it goes.
func Start(op Op, obs Observer) *Job {
	ctx, cancel := context.WithCancel(context.Background())
	j := &Job{cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(j.done)
		defer cancel()
		j.undo, j.err = op.run(&runner{ctx: ctx, obs: obs})
	}()
	return j
}

// Cancel stops the job before its next file or chunk: what it was
// writing is removed, what it finished stays.
func (j *Job) Cancel() { j.cancel() }

// Wait waits for the job to end and returns how to undo what it did
// (nil when it did nothing undoable) and why it stopped early.
func (j *Job) Wait() (*Undo, error) {
	<-j.done
	return j.undo, j.err
}

// chunk is how much of a file is copied between two progress reports
// and cancellation checks.
const chunk = 1 << 20

type runner struct {
	ctx      context.Context
	obs      Observer
	progress Progress
	all      Action
}

func (r *runner) begin(files, bytes int64) {
	r.progress.TotalFiles += files
	r.progress.TotalBytes += bytes
	r.obs.Progress(r.progress)
}

func (r *runner) advance(path string, files, bytes int64) {
	r.progress.Files += files
	r.progress.Bytes += bytes
	r.progress.Current = path
	r.obs.Progress(r.progress)
}

func (r *runner) decide(c Conflict) (Action, error) {
	if r.all != 0 {
		return r.all, nil
	}
	res := r.obs.Resolve(c)
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	if res.All {
		r.all = res.Action
	}
	return res.Action, nil
}

// execute carries out a transfer plan item by item, returning the
// moves it finished, even when it stops at an error.
func (r *runner) execute(p plan) ([]move, error) {
	r.begin(p.files, p.bytes)
	var moved []move
	for _, it := range p.items {
		if err := r.ctx.Err(); err != nil {
			return moved, fmt.Errorf("%s %s: %w", p.op.verb(), it.src, err)
		}
		placed, err := r.transfer(p, it)
		if err != nil {
			return moved, fmt.Errorf("%s %s to %s: %w", p.op.verb(), it.src, it.dst, err)
		}
		if placed != "" && p.op == moving {
			moved = append(moved, move{from: it.src, to: placed})
		}
	}
	return moved, nil
}

func (r *runner) transfer(p plan, it item) (string, error) {
	files, bytes := it.node.count()
	if it.rename {
		bytes = 0
	}
	if it.conflict {
		a, err := r.decide(Conflict{Source: it.src, Target: it.dst, SourceIsDir: it.node.isDir(), TargetIsDir: it.targetIsDir})
		if err != nil {
			return "", err
		}
		taken := p.taken[filepath.Dir(it.dst)]
		resolved, ok, err := resolve(it, a, taken)
		if err != nil {
			return "", err
		}
		if !ok {
			r.advance(it.src, files, bytes)
			return "", nil
		}
		it = resolved
		taken[filepath.Base(it.dst)] = it.node.isDir()
	}
	if it.rename {
		if err := place(it.src, it.dst, it.replace, it.node.isDir() || it.targetIsDir); err != nil {
			return "", err
		}
		r.advance(it.dst, files, 0)
		return it.dst, nil
	}
	if err := r.copyInto(it); err != nil {
		return "", err
	}
	if p.op == moving {
		if err := os.RemoveAll(it.src); err != nil {
			return it.dst, fmt.Errorf("remove the source once copied: %w", err)
		}
	}
	return it.dst, nil
}

// copyInto builds the item's tree in a private folder beside its
// destination and renames it into place, so the destination holds the
// whole copy or nothing of it, whatever stops the copy.
func (r *runner) copyInto(it item) (err error) {
	stage, err := os.MkdirTemp(filepath.Dir(it.dst), ".fileops-")
	if err != nil {
		return err
	}
	defer func() {
		if rmErr := removeStage(stage); rmErr != nil {
			err = errors.Join(err, fmt.Errorf("remove the unfinished copy %s: %w", stage, rmErr))
		}
	}()
	built := filepath.Join(stage, it.node.name)
	if err := r.build(it.src, it.node, built); err != nil {
		return err
	}
	return place(built, it.dst, it.replace, it.node.isDir() || it.targetIsDir)
}

func (r *runner) build(src string, n node, at string) error {
	if err := r.ctx.Err(); err != nil {
		return err
	}
	switch n.kind {
	case regular:
		if err := r.copyFile(src, n, at); err != nil {
			return err
		}
	case symlink:
		if err := os.Symlink(n.target, at); err != nil {
			return err
		}
	case directory:
		if err := os.Mkdir(at, 0o700); err != nil {
			return err
		}
		for _, c := range n.children {
			if err := r.build(filepath.Join(src, c.name), c, filepath.Join(at, c.name)); err != nil {
				return err
			}
		}
		if err := unix.Chmod(at, n.perm); err != nil {
			return &os.PathError{Op: "chmod", Path: at, Err: err}
		}
	}
	if err := unix.UtimesNanoAt(unix.AT_FDCWD, at, []unix.Timespec{{Nsec: unix.UTIME_OMIT}, n.mtime}, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return &os.PathError{Op: "set the modification time of", Path: at, Err: err}
	}
	if n.kind != regular {
		r.advance(src, 1, 0)
	}
	return nil
}

func (r *runner) copyFile(src string, n node, at string) (err error) {
	in, err := os.Open(src) //nolint:gosec // a file the user asked to copy
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, in.Close()) }()
	out, err := os.OpenFile(at, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600) //nolint:gosec // inside the job's private folder
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, out.Close()) }()
	for {
		if err := r.ctx.Err(); err != nil {
			return err
		}
		copied, err := io.CopyN(out, in, chunk)
		r.progress.Bytes += copied
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		r.advance(src, 0, 0)
	}
	if err := unix.Fchmod(int(out.Fd()), n.perm); err != nil {
		return &os.PathError{Op: "chmod", Path: at, Err: err}
	}
	if err := out.Sync(); err != nil {
		return err
	}
	r.advance(src, 1, 0)
	return nil
}

// place renames from onto to: never over something there unless
// replace is set, and then atomically, exchanging the two when either
// is a folder and removing what was there.
func place(from, to string, replace, dirs bool) error {
	switch {
	case !replace:
		return renameNoReplace(from, to)
	case !dirs:
		return os.Rename(from, to)
	}
	if err := unix.Renameat2(unix.AT_FDCWD, from, unix.AT_FDCWD, to, unix.RENAME_EXCHANGE); err != nil {
		return &os.LinkError{Op: "exchange", Old: from, New: to, Err: err}
	}
	if err := os.RemoveAll(from); err != nil {
		return fmt.Errorf("remove the replaced %s: %w", to, err)
	}
	return nil
}

func renameNoReplace(from, to string) error {
	if err := unix.Renameat2(unix.AT_FDCWD, from, unix.AT_FDCWD, to, unix.RENAME_NOREPLACE); err != nil {
		return &os.LinkError{Op: "rename", Old: from, New: to, Err: err}
	}
	return nil
}

// removeStage removes a job's private folder whatever the folders
// copied into it forbid: they are the job's own, made writable first.
func removeStage(stage string) error {
	err := filepath.WalkDir(stage, func(path string, de os.DirEntry, err error) error {
		if err != nil || !de.IsDir() {
			return err
		}
		return os.Chmod(path, 0o700) //nolint:gosec // the job's own folder, searchable so it can be emptied
	})
	return errors.Join(err, os.RemoveAll(stage))
}
