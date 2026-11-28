package fileops

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/desktop/vfs"
)

// Copy copies Sources into the folder Dest, each under its own name.
// Links are copied as links unless FollowLinks copies what they lead
// to, or the copy touches a network location. Mounts reach the network
// locations named, nil for none.
type Copy struct {
	Sources     []string
	Dest        string
	FollowLinks bool
	Mounts      *vfs.Mounts
}

// Move moves Sources into the folder Dest, each under its own name: a
// rename within one filesystem, a copy then a removal across two.
// Mounts reach the network locations named, nil for none.
type Move struct {
	Sources []string
	Dest    string
	Mounts  *vfs.Mounts
}

// Delete removes Paths for good, a folder with all it holds; a link
// is removed, never what it leads to. Mounts reach the network
// locations named, nil for none.
type Delete struct {
	Paths  []string
	Mounts *vfs.Mounts
}

type move struct{ from, to string }

// copied is a copy put at a name that was free and what it built
// there, each entry stamped as S: a [stamp] on the local disk, a
// [vfsStamp] through the filesystems.
type copied[S comparable] struct {
	path string
	made made[S]
}

// made is an entry a copy built: its name, its kind, its stamp, and
// for a folder what the copy put in it.
type made[S comparable] struct {
	name     string
	kind     kind
	stamp    S
	children []made[S]
}

// count is the entries m holds, itself included.
func (m made[S]) count() int64 {
	n := int64(1)
	for _, c := range m.children {
		n += c.count()
	}
	return n
}

// stamp is what an entry is: its inode, and for a file or a link the
// time it last changed, which any change to either moves. A folder is
// its inode alone: it is only ever removed once empty.
type stamp struct {
	inode
	ctime unix.Timespec
}

// stampAt is the stamp of what is at path, its link if it is one.
func stampAt(path string) (stamp, error) {
	var st unix.Stat_t
	if err := unix.Lstat(path, &st); err != nil {
		return stamp{}, &os.PathError{Op: "stat", Path: path, Err: err}
	}
	s := stamp{dev: st.Dev, ino: st.Ino}
	if st.Mode&unix.S_IFMT != unix.S_IFDIR {
		s.ctime = st.Ctim
	}
	return s, nil
}

// vfsStamp is what an entry a copy built through the filesystems is: a
// file its size and modification time, which a write to it moves, a
// folder only that it is one, since a network filesystem keeps no
// inode or change time. A folder is only ever removed once empty.
type vfsStamp struct {
	dir      bool
	size     int64
	modified int64
}

// stampOf is the stamp of the entry info states.
func stampOf(info vfs.Info) vfsStamp {
	if info.IsDir {
		return vfsStamp{dir: true}
	}
	return vfsStamp{size: info.Size, modified: info.Modified.UnixNano()}
}

// Undo reverses the last move, rename, trash or copy: run as an [Op],
// it moves each thing back, restores it from the trash, or removes
// what the copy made.
//
// A copy's undo removes exactly what the copy created: every copy it
// put at a free name (a plain copy, Keep Both, or a copy into its own
// folder), entry by entry as the copy built it. Each entry is removed
// only while what is at its path is still exactly what the copy built:
// a file or a link the same inode, unchanged since the copy finished
// it, and a folder the same inode, removed only once empty. A copy
// that replaced what was there is not undone, since what it replaced
// is gone; nothing that was there before the copy is touched. An entry
// already removed is nothing to undo. A file or a link changed,
// replaced or recreated since, which is the user's work now, and a
// folder that holds anything the copy did not put there, stay as they
// are, each an error naming it, while the rest is removed. A copy made
// through the filesystems, to or from a network location, is undone
// the same way through them, a file there unchanged while its size and
// modification time are.
type Undo struct {
	moves        []move
	trashed      []Item
	copies       []copied[stamp]
	remoteCopies []copied[vfsStamp]
	mounts       *vfs.Mounts
}

func undoMoves(moves []move, mounts *vfs.Mounts) *Undo {
	if len(moves) == 0 {
		return nil
	}
	return &Undo{moves: moves, mounts: mounts}
}

// copiesOf is what the copies done put at a free name built.
func copiesOf[S comparable](done []placed[S]) []copied[S] {
	var copies []copied[S]
	for _, p := range done {
		if !p.replaced {
			copies = append(copies, copied[S]{path: p.dst, made: p.made})
		}
	}
	return copies
}

func undoCopies(local []copied[stamp], remote []copied[vfsStamp], mounts *vfs.Mounts) *Undo {
	if len(local) == 0 && len(remote) == 0 {
		return nil
	}
	return &Undo{copies: local, remoteCopies: remote, mounts: mounts}
}

func (c Copy) run(r *runner) (*Undo, error) {
	reqs := make([]request, 0, len(c.Sources))
	for _, src := range c.Sources {
		reqs = append(reqs, request{src: src, target: vfs.Join(c.Dest, vfs.Base(src))})
	}
	if anyRemote(append(slices.Clone(c.Sources), c.Dest)...) {
		done, err := r.transferRemote(c.Mounts, copying, reqs)
		return undoCopies(nil, copiesOf(done), c.Mounts), err
	}
	done, err := r.transferAll(copying, reqs, c.FollowLinks)
	return undoCopies(copiesOf(done), nil, nil), err
}

func (m Move) run(r *runner) (*Undo, error) {
	reqs := make([]request, 0, len(m.Sources))
	for _, src := range m.Sources {
		reqs = append(reqs, request{src: src, target: vfs.Join(m.Dest, vfs.Base(src))})
	}
	moves, err := r.transferMoves(m.Mounts, reqs)
	return undoMoves(moves, m.Mounts), err
}

// transferMoves moves reqs, through the filesystems when any touches a
// network location, and returns what it moved.
func (r *runner) transferMoves(mounts *vfs.Mounts, reqs []request) ([]move, error) {
	for _, req := range reqs {
		if anyRemote(req.src, req.target) {
			done, err := r.transferRemote(mounts, moving, reqs)
			return movesOf(done), err
		}
	}
	done, err := r.transferAll(moving, reqs, false)
	return movesOf(done), err
}

// movesOf is what the transfers done moved.
func movesOf[S comparable](done []placed[S]) []move {
	moves := make([]move, 0, len(done))
	for _, p := range done {
		moves = append(moves, move{from: p.src, to: p.dst})
	}
	return moves
}

func (u *Undo) run(r *runner) (*Undo, error) {
	switch {
	case len(u.moves) > 0:
		reqs := make([]request, 0, len(u.moves))
		for _, m := range slices.Backward(u.moves) {
			reqs = append(reqs, request{src: m.to, target: m.from})
		}
		_, err := r.transferMoves(u.mounts, reqs)
		return nil, err
	case len(u.copies) > 0:
		return nil, removeCopies(r, u.copies, r.removeBuilt)
	case len(u.remoteCopies) > 0:
		return nil, removeCopies(r, u.remoteCopies, func(loc string, m made[vfsStamp]) error {
			fsys, p, err := reach(u.mounts, loc)
			if err != nil {
				return err
			}
			return r.removeBuiltRemote(fsys, loc, p, m)
		})
	}
	r.begin(int64(len(u.trashed)), 0)
	for _, it := range slices.Backward(u.trashed) {
		if err := r.ctx.Err(); err != nil {
			return nil, fmt.Errorf("restore %s: %w", it.Original, err)
		}
		if err := Restore(it); err != nil {
			return nil, err
		}
		r.advance(it.Original, 1, 0)
	}
	return nil, nil
}

// removeCopies removes what each copy made through remove, going on
// past what cannot be removed and reporting all of it.
func removeCopies[S comparable](r *runner, copies []copied[S], remove func(string, made[S]) error) error {
	var files int64
	for _, c := range copies {
		files += c.made.count()
	}
	r.begin(files, 0)
	var errs []error
	for _, c := range slices.Backward(copies) {
		if err := r.ctx.Err(); err != nil {
			return errors.Join(append(errs, fmt.Errorf("undo the copy %s: %w", c.path, err))...)
		}
		if err := remove(c.path, c.made); err != nil {
			errs = append(errs, fmt.Errorf("undo the copy %s: %w", c.path, err))
		}
	}
	return errors.Join(errs...)
}

// removeBuilt removes what a copy built at path, as [Undo] says: what
// is gone since is nothing to remove, and what is not as the copy
// built it stays, as an error naming it.
func (r *runner) removeBuilt(path string, m made[stamp]) error {
	s, err := stampAt(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		r.advance(path, m.count(), 0)
		return nil
	case err != nil:
		return err
	case s != m.stamp:
		r.advance(path, m.count(), 0)
		return changedSince(path)
	}
	if m.kind != directory {
		return r.removed(path, unix.Unlink(path))
	}
	var errs []error
	for _, c := range m.children {
		if err := r.ctx.Err(); err != nil {
			return errors.Join(append(errs, err)...)
		}
		if err := r.removeBuilt(filepath.Join(path, c.name), c); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		r.advance(path, 1, 0)
		return errors.Join(errs...)
	}
	return r.removed(path, unix.Rmdir(path))
}

// changedSince is the error of an entry at path an undo leaves, as it
// is not what the copy made there.
func changedSince(path string) error { return fmt.Errorf("%s changed since the copy made it", path) }

// removed counts path as undone once err, its removal's outcome, says
// it is gone.
func (r *runner) removed(path string, err error) error {
	if err != nil && !errors.Is(err, unix.ENOENT) {
		return &os.PathError{Op: "remove", Path: path, Err: err}
	}
	r.advance(path, 1, 0)
	return nil
}

// transferAll scans each request's source and its target's folder,
// plans the transfer and carries it out.
func (r *runner) transferAll(op transfer, reqs []request, follow bool) ([]placed[stamp], error) {
	dirs := make(map[string]listing)
	for i := range reqs {
		n, err := scan(reqs[i].src, follow)
		if err != nil {
			return nil, fmt.Errorf("%s %s: %w", op.verb(), reqs[i].src, err)
		}
		reqs[i].node = n
		dir := filepath.Dir(filepath.Clean(reqs[i].target))
		if _, ok := dirs[dir]; ok {
			continue
		}
		l, err := list(dir)
		if err != nil {
			return nil, fmt.Errorf("%s %s into %s: %w", op.verb(), reqs[i].src, dir, err)
		}
		dirs[dir] = l
	}
	p, err := planTransfer(op, reqs, dirs)
	if err != nil {
		return nil, err
	}
	return r.execute(p)
}

func (d Delete) run(r *runner) (*Undo, error) { return nil, r.deleteAll(d.Mounts, d.Paths) }

func (r *runner) remove(path string, n node) error {
	if err := r.ctx.Err(); err != nil {
		return err
	}
	if n.isDir() {
		for _, c := range n.children {
			if err := r.remove(filepath.Join(path, c.name), c); err != nil {
				return err
			}
		}
		if err := unix.Rmdir(path); err != nil {
			return &os.PathError{Op: "remove", Path: path, Err: err}
		}
	} else if err := unix.Unlink(path); err != nil {
		return &os.PathError{Op: "remove", Path: path, Err: err}
	}
	r.advance(path, 1, 0)
	return nil
}

// checkName refuses what cannot name a file within a folder.
func checkName(name string) error {
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, "/\x00") {
		return fmt.Errorf("%q is not a file name", name)
	}
	return nil
}

// remoteTimeout bounds a rename or a new file or folder on a network
// location.
const remoteTimeout = 30 * time.Second

// Rename gives path a new name in its folder, never over something
// already there; renaming to its own name does nothing. Mounts reach
// a network location, nil for none.
func Rename(mounts *vfs.Mounts, path, name string) (*Undo, error) {
	if err := checkName(name); err != nil {
		return nil, fmt.Errorf("rename %s: %w", path, err)
	}
	path = vfs.Clean(path)
	to := vfs.Join(vfs.Dir(path), name)
	if to == path {
		return nil, nil
	}
	var err error
	if vfs.IsRemote(path) {
		ctx, cancel := context.WithTimeout(context.Background(), remoteTimeout)
		defer cancel()
		err = renameRemote(ctx, mounts, path, to)
	} else {
		err = renameNoReplace(path, to)
	}
	if err != nil {
		return nil, fmt.Errorf("rename %s to %s: %w", path, name, err)
	}
	return undoMoves([]move{{from: path, to: to}}, mounts), nil
}

// NewFolder makes the folder name in dir and returns its path. Mounts
// reach a network location, nil for none.
func NewFolder(mounts *vfs.Mounts, dir, name string) (string, error) {
	if err := checkName(name); err != nil {
		return "", fmt.Errorf("create a folder in %s: %w", dir, err)
	}
	path := vfs.Join(dir, name)
	if err := makeNew(mounts, path, func(ctx context.Context, fsys vfs.FS, p string) error { return fsys.Mkdir(ctx, p) }); err != nil {
		return "", fmt.Errorf("create the folder %s: %w", path, err)
	}
	return path, nil
}

// NewFile makes the empty file name in dir and returns its path.
// Mounts reach a network location, nil for none.
func NewFile(mounts *vfs.Mounts, dir, name string) (string, error) {
	if err := checkName(name); err != nil {
		return "", fmt.Errorf("create a file in %s: %w", dir, err)
	}
	path := vfs.Join(dir, name)
	err := makeNew(mounts, path, func(ctx context.Context, fsys vfs.FS, p string) error {
		w, err := fsys.Create(ctx, p)
		if err != nil {
			return err
		}
		if err := w.Close(); err != nil {
			return errors.Join(err, fsys.Remove(ctx, p))
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("create the file %s: %w", path, err)
	}
	return path, nil
}

// makeNew makes what create makes at the location path, on the local
// disk or the network location it names.
func makeNew(mounts *vfs.Mounts, path string, create func(context.Context, vfs.FS, string) error) error {
	ctx, cancel := context.WithTimeout(context.Background(), remoteTimeout)
	defer cancel()
	if !vfs.IsRemote(path) {
		return create(ctx, &vfs.LocalFS{}, path)
	}
	fsys, p, err := reach(mounts, path)
	if err != nil {
		return err
	}
	return create(ctx, fsys, p)
}
