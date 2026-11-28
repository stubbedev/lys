// Package localcopy keeps local copies of files on network locations,
// for what reads only the local disk (thumbnailers, previews, apps): a
// copy fetched whole, a bounded cache of such copies, and the way a
// copy changed on the local disk goes back in place of the file it was
// fetched from, unless that changed on the server meanwhile.
package localcopy

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/stubbedev/lys/desktop/vfs"
)

// Stamp is what a file on a network location was when it was read or
// written: its size and modification time, which a write to it moves,
// as a network filesystem keeps no inode or change time.
type Stamp struct {
	Size     int64
	Modified time.Time
}

// StampOf is the stamp of the file info states.
func StampOf(info vfs.Info) Stamp { return Stamp{Size: info.Size, Modified: info.Modified} }

// Equal reports whether s and o stamp the same file content.
func (s Stamp) Equal(o Stamp) bool { return s.Size == o.Size && s.Modified.Equal(o.Modified) }

// ErrConflict is a file on a network location that is not what an
// upload expected there: changed, removed or made since its copy was
// fetched.
var ErrConflict = errors.New("the file changed on the server since its copy was fetched")

// ErrChanged is a file that changed while it was being fetched.
var ErrChanged = errors.New("the file changed while it was fetched")

// Fetch copies the file p on fsys to the local file local, whole or
// not at all: it is written under a temporary name beside local and
// renamed over it, carrying the remote file's modification time. It
// returns the stamp of the file as it was read.
func Fetch(ctx context.Context, fsys vfs.FS, p, local string) (Stamp, error) {
	info, err := fsys.Stat(ctx, p)
	if err != nil {
		return Stamp{}, fmt.Errorf("fetch %s: %w", p, err)
	}
	if info.IsDir {
		return Stamp{}, fmt.Errorf("fetch %s: %w", p, &fs.PathError{Op: "open", Path: p, Err: fs.ErrInvalid})
	}
	stamp := StampOf(info)
	if err := fetchInto(ctx, fsys, p, local, stamp); err != nil {
		return Stamp{}, fmt.Errorf("fetch %s to %s: %w", p, local, err)
	}
	return stamp, nil
}

func fetchInto(ctx context.Context, fsys vfs.FS, p, local string, stamp Stamp) (err error) {
	in, err := fsys.Open(ctx, p)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, in.Close()) }()
	out, err := os.CreateTemp(filepath.Dir(local), "."+filepath.Base(local)+".part-*")
	if err != nil {
		return err
	}
	part := out.Name()
	defer func() {
		if err != nil {
			if rmErr := os.Remove(part); rmErr != nil && !errors.Is(rmErr, fs.ErrNotExist) {
				err = errors.Join(err, rmErr)
			}
		}
	}()
	n, err := io.Copy(out, in)
	if err = errors.Join(err, out.Close()); err != nil {
		return err
	}
	if n != stamp.Size {
		return fmt.Errorf("%w: read %d bytes of %d", ErrChanged, n, stamp.Size)
	}
	if !stamp.Modified.IsZero() {
		if err := os.Chtimes(part, time.Time{}, stamp.Modified); err != nil {
			return err
		}
	}
	return os.Rename(part, local)
}

// Expect is what an upload expects at its destination before it puts
// its copy there.
type Expect struct {
	kind  expectKind
	stamp Stamp
}

type expectKind uint8

const (
	expectUnchanged expectKind = iota
	expectAbsent
	expectAnything
)

// Unchanged expects the file as s stamps it.
func Unchanged(s Stamp) Expect { return Expect{kind: expectUnchanged, stamp: s} }

// Absent expects nothing there yet.
func Absent() Expect { return Expect{kind: expectAbsent} }

// Anything replaces whatever is there.
func Anything() Expect { return Expect{kind: expectAnything} }

// check is ErrConflict unless what is at p on fsys is what e expects.
func (e Expect) check(ctx context.Context, fsys vfs.FS, p string) (exists bool, err error) {
	info, err := fsys.Stat(ctx, p)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		if e.kind == expectUnchanged {
			return false, fmt.Errorf("%w: it is gone", ErrConflict)
		}
		return false, nil
	case err != nil:
		return false, err
	case info.IsDir:
		return true, fmt.Errorf("%w: a folder is there", ErrConflict)
	case e.kind == expectAbsent:
		return true, fmt.Errorf("%w: a file is there", ErrConflict)
	case e.kind == expectUnchanged && !StampOf(info).Equal(e.stamp):
		return true, ErrConflict
	}
	return true, nil
}

// cleanupTimeout bounds removing what a failed upload left on the
// server, which goes on after its context ended.
const cleanupTimeout = time.Minute

// Upload puts the content of the local file local at p on fsys, once
// what is there is what want expects, and returns the stamp of the file
// it put there. The content is written under a temporary name beside p
// and only then takes p's place: the file there is first renamed
// aside, so the server holds the old file or the new one whole at every
// moment, and is put back when the new one cannot take its place.
func Upload(ctx context.Context, fsys vfs.FS, p, local string, want Expect) (Stamp, error) {
	s, err := upload(ctx, fsys, p, local, want)
	if err != nil {
		return Stamp{}, fmt.Errorf("upload %s to %s: %w", local, p, err)
	}
	return s, nil
}

func upload(ctx context.Context, fsys vfs.FS, p, local string, want Expect) (Stamp, error) {
	if _, err := want.check(ctx, fsys, p); err != nil {
		return Stamp{}, err
	}
	staged := beside(p, "upload")
	if err := send(ctx, fsys, local, staged); err != nil {
		return Stamp{}, errors.Join(err, removeLeft(ctx, fsys, staged))
	}
	exists, err := want.check(ctx, fsys, p)
	if err != nil {
		return Stamp{}, errors.Join(err, removeLeft(ctx, fsys, staged))
	}
	if err := swap(ctx, fsys, staged, p, exists); err != nil {
		return Stamp{}, err
	}
	info, err := fsys.Stat(ctx, p)
	if err != nil {
		return Stamp{}, err
	}
	return StampOf(info), nil
}

// beside is a name in p's folder no other file has, for what.
func beside(p, what string) string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return path.Join(path.Dir(p), "."+path.Base(p)+".lys-"+what+"-"+hex.EncodeToString(b[:]))
}

// send writes the local file to the new file at on fsys.
func send(ctx context.Context, fsys vfs.FS, local, at string) (err error) {
	in, err := os.Open(local) //nolint:gosec // the copy the user edited
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, in.Close()) }()
	out, err := fsys.Create(ctx, at)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		return errors.Join(err, out.Close())
	}
	return out.Close()
}

// swap gives staged the name p: the file at p, when there is one, is
// renamed aside first and removed once staged took its place, or put
// back when it could not.
func swap(ctx context.Context, fsys vfs.FS, staged, p string, exists bool) error {
	if !exists {
		if err := fsys.Rename(ctx, staged, p); err != nil {
			if errors.Is(err, fs.ErrExist) {
				err = fmt.Errorf("%w: a file is there", ErrConflict)
			}
			return errors.Join(err, removeLeft(ctx, fsys, staged))
		}
		return nil
	}
	aside := beside(p, "old")
	if err := fsys.Rename(ctx, p, aside); err != nil {
		return errors.Join(err, removeLeft(ctx, fsys, staged))
	}
	if err := fsys.Rename(ctx, staged, p); err != nil {
		cctx, cancel := cleanupContext(ctx)
		defer cancel()
		if back := fsys.Rename(cctx, aside, p); back != nil {
			return errors.Join(err, fmt.Errorf("put the old file back from %s: %w", aside, back), removeLeft(ctx, fsys, staged))
		}
		return errors.Join(err, removeLeft(ctx, fsys, staged))
	}
	if err := fsys.Remove(ctx, aside); err != nil {
		return fmt.Errorf("remove the old file kept at %s: %w", aside, err)
	}
	return nil
}

func cleanupContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
}

// removeLeft removes what an upload left at p, if anything.
func removeLeft(ctx context.Context, fsys vfs.FS, p string) error {
	cctx, cancel := cleanupContext(ctx)
	defer cancel()
	if err := fsys.Remove(cctx, p); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove the unfinished upload %s: %w", p, err)
	}
	return nil
}
