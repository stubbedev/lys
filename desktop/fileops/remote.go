package fileops

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"slices"
	"strings"
	"time"

	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/desktop/vfs"
)

// A transfer that touches a network location goes through the
// locations' filesystems (vfs): its sources are scanned on theirs, what
// they become is built beside its destination on the destination's and
// renamed into place, so the destination holds the whole copy or
// nothing of it. Links are copied as what they lead to, since a network
// filesystem keeps none. A move within one server is a rename on it.
// What such a copy made is stamped as it is built, each file by its
// size and modification time, so an undo removes it only while it is
// unchanged; a move is undone by moving it back.

// remoteDepth bounds how deep a scan over a network location goes: past
// it a folder is a loop the server did not mark as a link.
const remoteDepth = 64

// anyRemote reports whether any of locs is a network location.
func anyRemote(locs ...string) bool { return slices.ContainsFunc(locs, vfs.IsRemote) }

// errNoMounts is a network location named to an operation that was
// given no mounts to reach it through.
var errNoMounts = errors.New("no network location is connected")

// reach is the filesystem loc lies on and loc's path on it.
func reach(mounts *vfs.Mounts, loc string) (vfs.FS, string, error) {
	if mounts == nil {
		return nil, "", fmt.Errorf("reach %s: %w", loc, errNoMounts)
	}
	return mounts.Resolve(loc)
}

// volumeOf is the volume a network location lies on.
func volumeOf(loc string) (volume, error) {
	l, err := vfs.Parse(loc)
	if err != nil {
		return volume{}, err
	}
	return volume{server: l.Server}, nil
}

// scanAny scans the tree at loc: a local one with links followed, a
// remote one through its filesystem.
func (r *runner) scanAny(mounts *vfs.Mounts, loc string) (node, error) {
	if !vfs.IsRemote(loc) {
		return scan(loc, true)
	}
	fsys, p, err := reach(mounts, loc)
	if err != nil {
		return node{}, err
	}
	vol, err := volumeOf(loc)
	if err != nil {
		return node{}, err
	}
	info, err := fsys.Stat(r.ctx, p)
	if err != nil {
		return node{}, err
	}
	return r.scanRemote(fsys, p, info, vol, 0)
}

func (r *runner) scanRemote(fsys vfs.FS, p string, info vfs.Info, vol volume, depth int) (node, error) {
	n := node{name: vfs.Base(p), kind: regular, perm: 0o644, size: info.Size, mtime: unix.NsecToTimespec(info.Modified.UnixNano()), vol: vol}
	if !info.IsDir {
		return n, nil
	}
	if depth >= remoteDepth {
		return node{}, fmt.Errorf("%s lies %d folders deep, a loop", p, depth)
	}
	n.kind, n.perm, n.size = directory, 0o755, 0
	infos, err := fsys.ReadDir(r.ctx, p)
	if err != nil {
		return node{}, err
	}
	slices.SortFunc(infos, func(a, b vfs.Info) int { return strings.Compare(a.Name, b.Name) })
	n.children = make([]node, 0, len(infos))
	for _, c := range infos {
		if err := r.ctx.Err(); err != nil {
			return node{}, err
		}
		cn, err := r.scanRemote(fsys, vfs.Join(p, c.Name), c, vol, depth+1)
		if err != nil {
			return node{}, err
		}
		n.children = append(n.children, cn)
	}
	return n, nil
}

// listAny lists the folder things are put in: a local one as a local
// transfer does, a remote one through its filesystem.
func (r *runner) listAny(mounts *vfs.Mounts, dir string) (listing, error) {
	if !vfs.IsRemote(dir) {
		return list(dir)
	}
	fsys, p, err := reach(mounts, dir)
	if err != nil {
		return listing{}, err
	}
	vol, err := volumeOf(dir)
	if err != nil {
		return listing{}, err
	}
	info, err := fsys.Stat(r.ctx, p)
	if err != nil {
		return listing{}, err
	}
	if !info.IsDir {
		return listing{}, fmt.Errorf("%s is not a folder", dir)
	}
	infos, err := fsys.ReadDir(r.ctx, p)
	if err != nil {
		return listing{}, err
	}
	l := listing{vol: vol, names: make(map[string]bool, len(infos))}
	for _, i := range infos {
		l.names[i.Name] = i.IsDir
	}
	return l, nil
}

// transferRemote scans, plans and carries out a transfer that touches
// a network location.
func (r *runner) transferRemote(mounts *vfs.Mounts, op transfer, reqs []request) ([]placed[vfsStamp], error) {
	dirs := make(map[string]listing)
	for i := range reqs {
		n, err := r.scanAny(mounts, reqs[i].src)
		if err != nil {
			return nil, fmt.Errorf("%s %s: %w", op.verb(), reqs[i].src, err)
		}
		reqs[i].node = n
		dir := vfs.Dir(vfs.Clean(reqs[i].target))
		if _, ok := dirs[dir]; ok {
			continue
		}
		l, err := r.listAny(mounts, dir)
		if err != nil {
			return nil, fmt.Errorf("%s %s into %s: %w", op.verb(), reqs[i].src, dir, err)
		}
		dirs[dir] = l
	}
	p, err := planTransfer(op, reqs, dirs)
	if err != nil {
		return nil, err
	}
	r.begin(p.files, p.bytes)
	var done []placed[vfsStamp]
	for _, it := range p.items {
		if err := r.ctx.Err(); err != nil {
			return done, fmt.Errorf("%s %s: %w", p.op.verb(), it.src, err)
		}
		pl, ok, err := r.transferItemRemote(mounts, p, it)
		if ok {
			done = append(done, pl)
		}
		if err != nil {
			return done, fmt.Errorf("%s %s to %s: %w", p.op.verb(), it.src, it.dst, err)
		}
	}
	return done, nil
}

// transferItemRemote carries out one item of a transfer through the
// filesystems: false when it was skipped or failed before anything was
// put in place.
func (r *runner) transferItemRemote(mounts *vfs.Mounts, p plan, it item) (placed[vfsStamp], bool, error) {
	files, bytes := it.node.count()
	if it.rename {
		bytes = 0
	}
	if it.conflict {
		a, err := r.decide(Conflict{Source: it.src, Target: it.dst, SourceIsDir: it.node.isDir(), TargetIsDir: it.targetIsDir})
		if err != nil {
			return placed[vfsStamp]{}, false, err
		}
		taken := p.taken[vfs.Dir(it.dst)]
		resolved, ok, err := resolve(it, a, taken)
		if err != nil {
			return placed[vfsStamp]{}, false, err
		}
		if !ok {
			r.advance(it.src, files, bytes)
			return placed[vfsStamp]{}, false, nil
		}
		it = resolved
		taken[vfs.Base(it.dst)] = it.node.isDir()
	}
	dstFS, dstPath, err := reach(mounts, it.dst)
	if err != nil {
		return placed[vfsStamp]{}, false, err
	}
	done := placed[vfsStamp]{src: it.src, dst: it.dst, replaced: it.replace}
	if it.rename {
		srcFS, srcPath, err := reach(mounts, it.src)
		if err != nil {
			return placed[vfsStamp]{}, false, err
		}
		if it.replace {
			if err := removeTree(r.ctx, dstFS, dstPath, nil); err != nil {
				return placed[vfsStamp]{}, false, fmt.Errorf("remove the replaced %s: %w", it.dst, err)
			}
		}
		if err := srcFS.Rename(r.ctx, srcPath, dstPath); err != nil {
			return placed[vfsStamp]{}, false, err
		}
		r.advance(it.dst, files, 0)
		return done, true, nil
	}
	m, err := r.copyIntoRemote(mounts, it, dstFS, dstPath)
	if err != nil {
		return placed[vfsStamp]{}, false, err
	}
	if p.op == copying {
		done.made = m
	}
	if p.op == moving {
		srcFS, srcPath, err := reach(mounts, it.src)
		if err != nil {
			return done, true, err
		}
		if err := removeTree(r.ctx, srcFS, srcPath, nil); err != nil {
			return done, true, fmt.Errorf("remove the source once copied: %w", err)
		}
	}
	return done, true, nil
}

// copyIntoRemote builds the item's tree in a private folder beside its
// destination, renames it into place and returns what it built, its
// top stamped where it was put.
func (r *runner) copyIntoRemote(mounts *vfs.Mounts, it item, dstFS vfs.FS, dstPath string) (m made[vfsStamp], err error) {
	srcFS, srcPath, err := reach(mounts, it.src)
	if err != nil {
		return made[vfsStamp]{}, err
	}
	stage := vfs.Join(vfs.Dir(dstPath), stageName())
	if err := dstFS.Mkdir(r.ctx, stage); err != nil {
		return made[vfsStamp]{}, err
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(r.ctx), cleanupTimeout)
		defer cancel()
		if rmErr := removeTree(ctx, dstFS, stage, nil); rmErr != nil && !errors.Is(rmErr, fs.ErrNotExist) {
			err = errors.Join(err, fmt.Errorf("remove the unfinished copy %s: %w", stage, rmErr))
		}
	}()
	built := vfs.Join(stage, it.node.name)
	if m, err = r.buildRemote(srcFS, srcPath, it.node, dstFS, built); err != nil {
		return made[vfsStamp]{}, err
	}
	if it.replace {
		if err := removeTree(r.ctx, dstFS, dstPath, nil); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return made[vfsStamp]{}, fmt.Errorf("remove the replaced %s: %w", it.dst, err)
		}
	}
	if err := dstFS.Rename(r.ctx, built, dstPath); err != nil {
		return made[vfsStamp]{}, err
	}
	info, err := dstFS.Stat(r.ctx, dstPath)
	if err != nil {
		return made[vfsStamp]{}, err
	}
	m.stamp = stampOf(info)
	return m, nil
}

// stageName is a private folder's name, unlike any other.
func stageName() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return ".fileops-" + hex.EncodeToString(b[:])
}

// buildRemote copies the tree n at src on srcFS to at on dstFS and
// returns what it built, each file stamped once written.
func (r *runner) buildRemote(srcFS vfs.FS, src string, n node, dstFS vfs.FS, at string) (made[vfsStamp], error) {
	if err := r.ctx.Err(); err != nil {
		return made[vfsStamp]{}, err
	}
	if n.kind != directory {
		if err := r.copyStream(srcFS, src, dstFS, at); err != nil {
			return made[vfsStamp]{}, err
		}
		info, err := dstFS.Stat(r.ctx, at)
		if err != nil {
			return made[vfsStamp]{}, err
		}
		return made[vfsStamp]{name: n.name, kind: regular, stamp: stampOf(info)}, nil
	}
	if err := dstFS.Mkdir(r.ctx, at); err != nil {
		return made[vfsStamp]{}, err
	}
	m := made[vfsStamp]{name: n.name, kind: directory, stamp: vfsStamp{dir: true}, children: make([]made[vfsStamp], 0, len(n.children))}
	for _, c := range n.children {
		cm, err := r.buildRemote(srcFS, vfs.Join(src, c.name), c, dstFS, vfs.Join(at, c.name))
		if err != nil {
			return made[vfsStamp]{}, err
		}
		m.children = append(m.children, cm)
	}
	r.advance(src, 1, 0)
	return m, nil
}

// removeBuiltRemote removes what a copy built at the location loc, p
// on fsys, as [Undo] says: what is gone since is nothing to remove, and
// what is not as the copy built it stays, as an error naming it.
func (r *runner) removeBuiltRemote(fsys vfs.FS, loc, p string, m made[vfsStamp]) error {
	info, err := fsys.Stat(r.ctx, p)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		r.advance(loc, m.count(), 0)
		return nil
	case err != nil:
		return err
	case stampOf(info) != m.stamp:
		r.advance(loc, m.count(), 0)
		return changedSince(loc)
	}
	var errs []error
	for _, c := range m.children {
		if err := r.ctx.Err(); err != nil {
			return errors.Join(append(errs, err)...)
		}
		if err := r.removeBuiltRemote(fsys, vfs.Join(loc, c.name), vfs.Join(p, c.name), c); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		r.advance(loc, 1, 0)
		return errors.Join(errs...)
	}
	if err := fsys.Remove(r.ctx, p); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove %s: %w", loc, err)
	}
	r.advance(loc, 1, 0)
	return nil
}

// copyStream copies the file src to the new file at, a chunk between
// progress reports and cancellation checks.
func (r *runner) copyStream(srcFS vfs.FS, src string, dstFS vfs.FS, at string) (err error) {
	in, err := srcFS.Open(r.ctx, src)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, in.Close()) }()
	out, err := dstFS.Create(r.ctx, at)
	if err != nil {
		return err
	}
	closed := false
	defer func() {
		if !closed {
			err = errors.Join(err, out.Close())
		}
	}()
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
	closed = true
	if err := out.Close(); err != nil {
		return err
	}
	r.advance(src, 1, 0)
	return nil
}

// removeTree removes p on fsys, a folder with all it holds, telling
// removed (when set) of each entry gone.
func removeTree(ctx context.Context, fsys vfs.FS, p string, removed func(string)) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	info, err := fsys.Stat(ctx, p)
	if err != nil {
		return err
	}
	if info.IsDir && !info.Link {
		infos, err := fsys.ReadDir(ctx, p)
		if err != nil {
			return err
		}
		for _, c := range infos {
			if err := removeTree(ctx, fsys, vfs.Join(p, c.Name), removed); err != nil {
				return err
			}
		}
	}
	if err := fsys.Remove(ctx, p); err != nil {
		return err
	}
	if removed != nil {
		removed(p)
	}
	return nil
}

// cleanupTimeout bounds removing what a stopped copy left, which goes
// on after the job is cancelled.
const cleanupTimeout = time.Minute

// deleteRemote removes the network location loc for good, counting
// each entry it removes.
func (r *runner) deleteRemote(mounts *vfs.Mounts, loc string) error {
	fsys, p, err := reach(mounts, loc)
	if err != nil {
		return err
	}
	return removeTree(r.ctx, fsys, p, func(at string) { r.advance(vfs.Join(vfs.Root(loc), at), 1, 0) })
}

// renameRemote gives the network location loc the location to in its
// folder, never over something there.
func renameRemote(ctx context.Context, mounts *vfs.Mounts, loc, to string) error {
	fsys, p, err := reach(mounts, loc)
	if err != nil {
		return err
	}
	return fsys.Rename(ctx, p, vfs.Join(vfs.Dir(p), vfs.Base(to)))
}

// deleteAll removes paths for good, every one scanned before anything
// is removed: a local one directly, a network location through its
// filesystem.
func (r *runner) deleteAll(mounts *vfs.Mounts, paths []string) error {
	trees := make([]node, 0, len(paths))
	var files int64
	for _, path := range paths {
		var n node
		var err error
		if vfs.IsRemote(path) {
			n, err = r.scanAny(mounts, path)
		} else {
			n, err = scan(path, false)
		}
		if err != nil {
			return fmt.Errorf("delete %s: %w", path, err)
		}
		f, _ := n.count()
		files += f
		trees = append(trees, n)
	}
	r.begin(files, 0)
	for i, path := range paths {
		var err error
		if vfs.IsRemote(path) {
			err = r.deleteRemote(mounts, path)
		} else {
			err = r.remove(path, trees[i])
		}
		if err != nil {
			return fmt.Errorf("delete %s: %w", path, err)
		}
	}
	return nil
}
