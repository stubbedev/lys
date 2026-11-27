package fileops

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/sys/unix"
)

// Copy copies Sources into the folder Dest, each under its own name.
// Links are copied as links unless FollowLinks copies what they lead
// to.
type Copy struct {
	Sources     []string
	Dest        string
	FollowLinks bool
}

// Move moves Sources into the folder Dest, each under its own name: a
// rename within one filesystem, a copy then a removal across two.
type Move struct {
	Sources []string
	Dest    string
}

// Delete removes Paths for good, a folder with all it holds; a link
// is removed, never what it leads to.
type Delete struct {
	Paths []string
}

type move struct{ from, to string }

// copied is a copy put at a name that was free and what it built
// there.
type copied struct {
	path string
	made made
}

// made is an entry a copy built: its name, its kind, its stamp, and
// for a folder what the copy put in it.
type made struct {
	name     string
	kind     kind
	stamp    stamp
	children []made
}

// count is the entries m holds, itself included.
func (m made) count() int64 {
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
// are, each an error naming it, while the rest is removed.
type Undo struct {
	moves   []move
	trashed []Item
	copies  []copied
}

func undoMoves(moves []move) *Undo {
	if len(moves) == 0 {
		return nil
	}
	return &Undo{moves: moves}
}

func undoCopies(done []placed) *Undo {
	var copies []copied
	for _, p := range done {
		if !p.replaced {
			copies = append(copies, copied{path: p.dst, made: p.made})
		}
	}
	if len(copies) == 0 {
		return nil
	}
	return &Undo{copies: copies}
}

func (c Copy) run(r *runner) (*Undo, error) {
	reqs := make([]request, 0, len(c.Sources))
	for _, src := range c.Sources {
		reqs = append(reqs, request{src: src, target: filepath.Join(c.Dest, filepath.Base(src))})
	}
	done, err := r.transferAll(copying, reqs, c.FollowLinks)
	return undoCopies(done), err
}

func (m Move) run(r *runner) (*Undo, error) {
	reqs := make([]request, 0, len(m.Sources))
	for _, src := range m.Sources {
		reqs = append(reqs, request{src: src, target: filepath.Join(m.Dest, filepath.Base(src))})
	}
	done, err := r.transferAll(moving, reqs, false)
	moves := make([]move, 0, len(done))
	for _, p := range done {
		moves = append(moves, move{from: p.src, to: p.dst})
	}
	return undoMoves(moves), err
}

func (u *Undo) run(r *runner) (*Undo, error) {
	switch {
	case len(u.moves) > 0:
		reqs := make([]request, 0, len(u.moves))
		for _, m := range slices.Backward(u.moves) {
			reqs = append(reqs, request{src: m.to, target: m.from})
		}
		_, err := r.transferAll(moving, reqs, false)
		return nil, err
	case len(u.copies) > 0:
		return nil, r.removeCopies(u.copies)
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

// removeCopies removes what each copy made, going on past what cannot
// be removed and reporting all of it.
func (r *runner) removeCopies(copies []copied) error {
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
		if err := r.removeBuilt(c.path, c.made); err != nil {
			errs = append(errs, fmt.Errorf("undo the copy %s: %w", c.path, err))
		}
	}
	return errors.Join(errs...)
}

// removeBuilt removes what a copy built at path, as [Undo] says: what
// is gone since is nothing to remove, and what is not as the copy
// built it stays, as an error naming it.
func (r *runner) removeBuilt(path string, m made) error {
	s, err := stampAt(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		r.advance(path, m.count(), 0)
		return nil
	case err != nil:
		return err
	case s != m.stamp:
		r.advance(path, m.count(), 0)
		return fmt.Errorf("%s changed since the copy made it", path)
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
func (r *runner) transferAll(op transfer, reqs []request, follow bool) ([]placed, error) {
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

func (d Delete) run(r *runner) (*Undo, error) {
	trees := make([]node, 0, len(d.Paths))
	var files int64
	for _, path := range d.Paths {
		n, err := scan(path, false)
		if err != nil {
			return nil, fmt.Errorf("delete %s: %w", path, err)
		}
		f, _ := n.count()
		files += f
		trees = append(trees, n)
	}
	r.begin(files, 0)
	for i, n := range trees {
		if err := r.remove(d.Paths[i], n); err != nil {
			return nil, fmt.Errorf("delete %s: %w", d.Paths[i], err)
		}
	}
	return nil, nil
}

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

// Rename gives path a new name in its folder, never over something
// already there; renaming to its own name does nothing.
func Rename(path, name string) (*Undo, error) {
	if err := checkName(name); err != nil {
		return nil, fmt.Errorf("rename %s: %w", path, err)
	}
	path = filepath.Clean(path)
	to := filepath.Join(filepath.Dir(path), name)
	if to == path {
		return nil, nil
	}
	if err := renameNoReplace(path, to); err != nil {
		return nil, fmt.Errorf("rename %s to %s: %w", path, name, err)
	}
	return undoMoves([]move{{from: path, to: to}}), nil
}

// NewFolder makes the folder name in dir and returns its path.
func NewFolder(dir, name string) (string, error) {
	if err := checkName(name); err != nil {
		return "", fmt.Errorf("create a folder in %s: %w", dir, err)
	}
	path := filepath.Join(dir, name)
	if err := os.Mkdir(path, 0o777); err != nil { //nolint:gosec // a folder the user asked for, as the umask allows
		return "", fmt.Errorf("create the folder %s: %w", path, err)
	}
	return path, nil
}

// NewFile makes the empty file name in dir and returns its path.
func NewFile(dir, name string) (string, error) {
	if err := checkName(name); err != nil {
		return "", fmt.Errorf("create a file in %s: %w", dir, err)
	}
	path := filepath.Join(dir, name)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666) //nolint:gosec // a file the user asked for
	if err != nil {
		return "", fmt.Errorf("create the file %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return "", errors.Join(fmt.Errorf("create the file %s: %w", path, err), os.Remove(path))
	}
	return path, nil
}
