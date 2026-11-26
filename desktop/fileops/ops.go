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

// copied is a copy put at a name that was free: the tree built there,
// and what its top was when it was put there.
type copied struct {
	path string
	id   inode
	tree node
}

// Undo reverses the last move, rename, trash or copy: run as an [Op],
// it moves each thing back, restores it from the trash, or removes
// what the copy made.
//
// A copy's undo removes exactly what the copy created: every copy it
// put at a free name (a plain copy, Keep Both, or a copy into its own
// folder), file by file and folder by folder as the copy built it, and
// a copy's folder only once it is empty. A copy that replaced what was
// there is not undone, since what it replaced is gone; nothing that
// was there before the copy is touched. A copy already removed is
// nothing to undo; a name that now holds something else than the copy
// made, or a copied folder that holds files put there since, is left
// as it is and is an error.
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
			copies = append(copies, copied{path: p.dst, id: p.id, tree: p.node})
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

// removeCopies removes what each copy made, going on past one that
// cannot be removed and reporting every one that was not.
func (r *runner) removeCopies(copies []copied) error {
	var files int64
	for _, c := range copies {
		f, _ := c.tree.count()
		files += f
	}
	r.begin(files, 0)
	var errs []error
	for _, c := range slices.Backward(copies) {
		if err := r.ctx.Err(); err != nil {
			return errors.Join(append(errs, fmt.Errorf("undo the copy %s: %w", c.path, err))...)
		}
		id, err := identify(c.path)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			f, _ := c.tree.count()
			r.advance(c.path, f, 0)
			continue
		case err != nil:
			errs = append(errs, fmt.Errorf("undo the copy %s: %w", c.path, err))
			continue
		case id != c.id:
			errs = append(errs, fmt.Errorf("undo the copy %s: it holds something else than the copy now", c.path))
			continue
		}
		if err := r.removeBuilt(c.path, c.tree); err != nil {
			errs = append(errs, fmt.Errorf("undo the copy %s: %w", c.path, err))
		}
	}
	return errors.Join(errs...)
}

// removeBuilt removes the tree a copy built at path: what is gone
// since is nothing to remove, and a folder that holds more than the
// copy put in it stays, as an error.
func (r *runner) removeBuilt(path string, n node) error {
	if err := r.ctx.Err(); err != nil {
		return err
	}
	var err error
	if n.isDir() {
		for _, c := range n.children {
			if err := r.removeBuilt(filepath.Join(path, c.name), c); err != nil {
				return err
			}
		}
		err = unix.Rmdir(path)
	} else {
		err = unix.Unlink(path)
	}
	if err != nil && !errors.Is(err, unix.ENOENT) {
		return &os.PathError{Op: "remove", Path: path, Err: err}
	}
	r.advance(path, 1, 0)
	return nil
}

// identify is what is at path, its link if it is one.
func identify(path string) (inode, error) {
	var st unix.Stat_t
	if err := unix.Lstat(path, &st); err != nil {
		return inode{}, &os.PathError{Op: "stat", Path: path, Err: err}
	}
	return inode{dev: st.Dev, ino: st.Ino}, nil
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
