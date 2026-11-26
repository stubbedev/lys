package pwfiles

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

// LockName is the lock lckpwdf(3) takes in /etc, which useradd, passwd
// and systemd-sysusers take too.
const LockName = ".pwd.lock"

// ErrLocked is the lock staying held by another program until the
// context ended.
var ErrLocked = errors.New("the account files are locked by another program")

const lockRetry = 50 * time.Millisecond

// Lock is a held LockName.
type Lock struct{ f *os.File }

// TakeLock takes dir's LockName as lckpwdf does, a write lock on the
// whole file, waiting for another holder until ctx ends. It is an open
// file description lock, which conflicts with lckpwdf's process lock
// and with another of its own on another descriptor, so two requests
// of one process exclude each other too.
func TakeLock(ctx context.Context, dir string) (*Lock, error) {
	path := filepath.Join(dir, LockName)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0o600) //nolint:gosec // the lock's fixed name in the account files' directory
	if err != nil {
		return nil, fmt.Errorf("pwfiles: open %s: %w", path, err)
	}
	lk := unix.Flock_t{Type: unix.F_WRLCK, Whence: io.SeekStart}
	retry := time.NewTicker(lockRetry)
	defer retry.Stop()
	for {
		err := unix.FcntlFlock(f.Fd(), unix.F_OFD_SETLK, &lk)
		if err == nil {
			return &Lock{f: f}, nil
		}
		if !errors.Is(err, unix.EAGAIN) && !errors.Is(err, unix.EACCES) {
			_ = f.Close()
			return nil, fmt.Errorf("pwfiles: lock %s: %w", path, err)
		}
		select {
		case <-ctx.Done():
			_ = f.Close()
			return nil, fmt.Errorf("pwfiles: %s: %w: %w", path, ErrLocked, context.Cause(ctx))
		case <-retry.C:
		}
	}
}

// Unlock releases the lock.
func (l *Lock) Unlock() error {
	if err := l.f.Close(); err != nil {
		return fmt.Errorf("pwfiles: release %s: %w", l.f.Name(), err)
	}
	return nil
}

const selinuxLabel = "security.selinux"

type meta struct {
	mode     os.FileMode
	uid, gid int
	label    []byte
	original []byte
}

// Files is the account files of a directory, as Load read them under
// the lock, with the owner, mode and label each is written back with.
type Files struct {
	DB
	dir   string
	metas [numFiles]meta
}

// Load reads dir's account files. Hold the lock from before Load until
// after Commit.
func Load(dir string) (*Files, error) {
	f := &Files{dir: dir}
	var texts [numFiles][]byte
	for k := range numFiles {
		m, data, err := read(filepath.Join(dir, k.Name()))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		texts[k], f.metas[k] = data, m
	}
	db, err := parse(texts)
	if err != nil {
		return nil, err
	}
	f.DB = db
	return f, nil
}

func read(path string) (meta, []byte, error) {
	file, err := os.Open(path) //nolint:gosec // an account file of the directory Load was given
	if err != nil {
		return meta{}, nil, fmt.Errorf("pwfiles: read %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()
	var st unix.Stat_t
	if err := unix.Fstat(int(file.Fd()), &st); err != nil {
		return meta{}, nil, fmt.Errorf("pwfiles: stat %s: %w", path, err)
	}
	label, err := fileLabel(file)
	if err != nil {
		return meta{}, nil, fmt.Errorf("pwfiles: read the SELinux label of %s: %w", path, err)
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return meta{}, nil, fmt.Errorf("pwfiles: read %s: %w", path, err)
	}
	if data == nil {
		data = []byte{}
	}
	return meta{mode: os.FileMode(st.Mode).Perm(), uid: int(st.Uid), gid: int(st.Gid), label: label, original: data}, data, nil
}

func fileLabel(f *os.File) ([]byte, error) {
	buf := make([]byte, 256)
	for {
		n, err := unix.Fgetxattr(int(f.Fd()), selinuxLabel, buf)
		switch {
		case errors.Is(err, unix.ENODATA), errors.Is(err, unix.ENOTSUP):
			return nil, nil
		case errors.Is(err, unix.ERANGE):
			buf = make([]byte, 2*len(buf))
			continue
		case err != nil:
			return nil, err
		}
		return buf[:n], nil
	}
}

type writeFunc func(path string, data []byte, m meta) error

// Commit writes every file an edit changed, in File order, each
// replaced whole as it was owned, moded and labelled. When one fails,
// those already written get their old content back, so the files stay
// as Load found them, and the error says what failed.
func (f *Files) Commit() error { return f.commit(replace) }

func (f *Files) commit(write writeFunc) error {
	var written []File
	for k := range numFiles {
		if !f.Changed(k) {
			continue
		}
		err := write(filepath.Join(f.dir, k.Name()), []byte(f.Text(k)), f.metas[k])
		if err == nil {
			written = append(written, k)
			continue
		}
		for _, w := range written {
			if rerr := write(filepath.Join(f.dir, w.Name()), f.metas[w].original, f.metas[w]); rerr != nil {
				err = errors.Join(err, fmt.Errorf("pwfiles: restore /etc/%s: %w", w.Name(), rerr))
			}
		}
		return err
	}
	return nil
}

func replace(path string, data []byte, m meta) (err error) {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".lys-")
	if err != nil {
		return fmt.Errorf("pwfiles: write %s: %w", path, err)
	}
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmp.Name())
		}
	}()
	if err := tmp.Chown(m.uid, m.gid); err != nil {
		return fmt.Errorf("pwfiles: write %s: %w", path, err)
	}
	if err := tmp.Chmod(m.mode); err != nil {
		return fmt.Errorf("pwfiles: write %s: %w", path, err)
	}
	if m.label != nil {
		if err := unix.Fsetxattr(int(tmp.Fd()), selinuxLabel, m.label, 0); err != nil {
			return fmt.Errorf("pwfiles: label the new %s: %w", path, err)
		}
	}
	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("pwfiles: write %s: %w", path, err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("pwfiles: sync %s: %w", path, err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("pwfiles: write %s: %w", path, err)
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("pwfiles: replace %s: %w", path, err)
	}
	d, err := os.Open(dir) //nolint:gosec // the account files' directory
	if err != nil {
		return fmt.Errorf("pwfiles: sync %s: %w", dir, err)
	}
	defer func() { _ = d.Close() }()
	if err := d.Sync(); err != nil {
		return fmt.Errorf("pwfiles: sync %s: %w", dir, err)
	}
	return nil
}
