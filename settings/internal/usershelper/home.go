package usershelper

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/stubbedev/lys/settings/internal/accounts"
)

// makeHome creates home for a new account of uid and gid as useradd -m
// does: a copy of skel, which may not exist, owned by the account, and
// home itself with mode. A home that exists already is ErrTaken: it is
// another account's, or a removed one's, and never handed over. Until
// the last step home is root's and closed to everyone else, so nothing
// is created where the account could have moved it.
func makeHome(skel, home string, uid, gid int, mode fs.FileMode) (err error) {
	if err := os.Mkdir(home, 0o700); err != nil {
		if errors.Is(err, fs.ErrExist) {
			return accounts.Errorf(accounts.ErrTaken, "the home folder %s exists already", home)
		}
		return fmt.Errorf("create the home folder %s: %w", home, err)
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, removeTree(home))
		}
	}()
	root, err := os.OpenRoot(home)
	if err != nil {
		return fmt.Errorf("open the home folder %s: %w", home, err)
	}
	defer func() { _ = root.Close() }()
	if err := copySkel(skel, root, uid, gid); err != nil {
		return fmt.Errorf("fill the home folder %s from %s: %w", home, skel, err)
	}
	if err := root.Chmod(".", mode); err != nil {
		return fmt.Errorf("set the mode of the home folder %s: %w", home, err)
	}
	if err := root.Lchown(".", uid, gid); err != nil {
		return fmt.Errorf("give the home folder %s to its account: %w", home, err)
	}
	return nil
}

func copySkel(skel string, root *os.Root, uid, gid int) error {
	src, err := os.OpenRoot(skel)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()
	return fs.WalkDir(src.FS(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == "." {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		perm := info.Mode().Perm()
		switch {
		case d.IsDir():
			if err = root.Mkdir(path, 0o700); err == nil {
				err = root.Chmod(path, perm)
			}
		case d.Type()&fs.ModeSymlink != 0:
			var target string
			if target, err = src.Readlink(path); err == nil {
				err = root.Symlink(target, path)
			}
		case d.Type().IsRegular():
			err = copyFile(src, root, path, perm)
		default:
			return fmt.Errorf("%s is %s, neither a file, a folder nor a link", path, d.Type())
		}
		if err != nil {
			return err
		}
		return root.Lchown(path, uid, gid)
	})
}

func copyFile(src, dst *os.Root, path string, perm fs.FileMode) (err error) {
	in, err := src.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	out, err := dst.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := out.Close(); err == nil {
			err = cerr
		}
	}()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Chmod(perm)
}

// checkHome is the home of an account of uid that may be removed with
// it: a folder that account owns, or nothing to remove where there is
// none. Anything else, a link or another's folder, is refused, as
// userdel refuses it.
func checkHome(home string, uid uint32) (exists bool, err error) {
	if !filepath.IsAbs(home) || filepath.Clean(home) != home || home == "/" {
		return false, accounts.Errorf(accounts.ErrFailed, "the home folder %q is no absolute path of its own, so it stays", home)
	}
	st, err := os.Lstat(home)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, accounts.Errorf(accounts.ErrFailed, "look at the home folder %s: %v", home, err)
	}
	owner, ok := st.Sys().(*syscall.Stat_t)
	if !st.IsDir() || !ok || owner.Uid != uid {
		return false, accounts.Errorf(accounts.ErrFailed, "%s is not a folder of uid %d, so it stays", home, uid)
	}
	return true, nil
}

// checkSpool is the mail spool of an account of uid that may be
// removed with it: a regular file that account owns, or nothing to
// remove where there is none. Anything else, a link or another's file,
// is refused.
func checkSpool(spool string, uid uint32) (exists bool, err error) {
	st, err := os.Lstat(spool)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, accounts.Errorf(accounts.ErrFailed, "look at the mail spool %s: %v", spool, err)
	}
	owner, ok := st.Sys().(*syscall.Stat_t)
	if !st.Mode().IsRegular() || !ok || owner.Uid != uid {
		return false, accounts.Errorf(accounts.ErrFailed, "%s is not a file of uid %d, so it stays", spool, uid)
	}
	return true, nil
}

func removeFile(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}

func removeTree(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("remove %s: %w", path, err)
	}
	return nil
}

// runsAs reports whether a process of /proc at proc runs as uid.
func runsAs(proc string, uid uint32) (bool, error) {
	entries, err := os.ReadDir(proc)
	if err != nil {
		return false, fmt.Errorf("list the processes in %s: %w", proc, err)
	}
	for _, e := range entries {
		if _, err := strconv.ParseUint(e.Name(), 10, 32); err != nil {
			continue
		}
		st, err := os.Lstat(filepath.Join(proc, e.Name()))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return false, fmt.Errorf("look at process %s: %w", e.Name(), err)
		}
		if owner, ok := st.Sys().(*syscall.Stat_t); ok && owner.Uid == uid {
			return true, nil
		}
	}
	return false, nil
}
