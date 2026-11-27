// Package atomicfile replaces files whole: the content goes to a uniquely
// named temporary file beside the target, which is synced and renamed
// over it. A reader sees the old file or the new one and never a partial
// write, a crash leaves one of the two complete, and concurrent writers
// never share a temporary. WriteFileDurable also syncs the directory, so
// once it returns the new file survives a power cut.
package atomicfile

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

const (
	tempAttempts  = 10000
	selinuxLabel  = "security.selinux"
	firstAttrSize = 256
)

// Ownership is how a file is owned: its user, group, permission bits and
// SELinux label. Only OwnershipOf makes one WriteFileOwned takes.
type Ownership struct {
	perm     os.FileMode
	owned    bool
	uid, gid int
	attrs    []xattr
}

type xattr struct {
	name  string
	value []byte
}

// ErrNoOwnership is WriteFileOwned given an Ownership OwnershipOf did
// not read, which would give the file to root with no permissions.
var ErrNoOwnership = errors.New("atomicfile: an Ownership OwnershipOf did not read")

// OwnershipOf reads the ownership of the open file f. A file without an
// SELinux label, or on a file system that has none, gives one without.
func OwnershipOf(f *os.File) (Ownership, error) {
	return ownershipOf(f, selinuxLabel)
}

func ownershipOf(f *os.File, attrNames ...string) (Ownership, error) {
	fd := int(f.Fd())
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil {
		return Ownership{}, fmt.Errorf("atomicfile: read the ownership of %s: %w", f.Name(), os.NewSyscallError("fstat", err))
	}
	own := Ownership{perm: os.FileMode(st.Mode).Perm(), owned: true, uid: int(st.Uid), gid: int(st.Gid)}
	for _, name := range attrNames {
		value, err := attrOf(fd, name)
		if err != nil {
			return Ownership{}, fmt.Errorf("atomicfile: read the %s of %s: %w", name, f.Name(), err)
		}
		if value != nil {
			own.attrs = append(own.attrs, xattr{name: name, value: value})
		}
	}
	return own, nil
}

func attrOf(fd int, name string) ([]byte, error) {
	buf := make([]byte, firstAttrSize)
	for {
		n, err := unix.Fgetxattr(fd, name, buf)
		switch {
		case errors.Is(err, unix.ENODATA), errors.Is(err, unix.ENOTSUP):
			return nil, nil
		case errors.Is(err, unix.ERANGE):
			buf = make([]byte, 2*len(buf))
			continue
		case err != nil:
			return nil, os.NewSyscallError("fgetxattr", err)
		}
		return buf[:n], nil
	}
}

func (o Ownership) apply(fd int) error {
	if o.owned {
		if err := unix.Fchown(fd, o.uid, o.gid); err != nil {
			return os.NewSyscallError("fchown", err)
		}
	}
	if err := unix.Fchmod(fd, uint32(o.perm)); err != nil {
		return os.NewSyscallError("fchmod", err)
	}
	for _, a := range o.attrs {
		if err := unix.Fsetxattr(fd, a.name, a.value, 0); err != nil {
			return fmt.Errorf("set %s: %w", a.name, os.NewSyscallError("fsetxattr", err))
		}
	}
	return nil
}

func permOnly(perm os.FileMode) Ownership { return Ownership{perm: perm.Perm()} }

// Write replaces path with what write produces, with permissions perm.
// On a failure path is untouched and the temporary is removed.
func Write(path string, perm os.FileMode, write func(io.Writer) error) error {
	return replace(path, permOnly(perm), keepRename, func(f tempFile) error {
		buf := bufio.NewWriter(f)
		if err := write(buf); err != nil {
			return err
		}
		return buf.Flush()
	})
}

// WriteSeekable is Write for a producer that seeks back over what it
// wrote, as a muxer patching its header does: write is handed the
// temporary itself, unbuffered.
func WriteSeekable(path string, perm os.FileMode, write func(io.WriteSeeker) error) error {
	return replace(path, permOnly(perm), keepRename, func(f tempFile) error { return write(f) })
}

// WriteFile replaces path with data, as os.WriteFile but atomically.
func WriteFile(path string, data []byte, perm os.FileMode) error {
	return replace(path, permOnly(perm), keepRename, writeData(data))
}

// WriteFileDurable is WriteFile for data the caller reports as stored,
// which must outlive a power cut: the directory is synced after the
// rename, so the replacement is on disk when it returns. A failure of
// that sync is reported with path already replaced.
func WriteFileDurable(path string, data []byte, perm os.FileMode) error {
	return replace(path, permOnly(perm), syncDir, writeData(data))
}

// WriteFileOwned is WriteFileDurable with the owner, group, permission
// bits and SELinux label of own. Giving the file to another user or
// label takes the privilege to; without it path is left untouched and
// the error is the kernel's.
func WriteFileOwned(path string, data []byte, own Ownership) error {
	if !own.owned {
		return fmt.Errorf("atomicfile: replace %s: %w", path, ErrNoOwnership)
	}
	return replace(path, own, syncDir, writeData(data))
}

func writeData(data []byte) func(tempFile) error {
	return func(f tempFile) error {
		_, err := f.Write(data)
		return err
	}
}

type afterRename func(dir string) error

func keepRename(string) error { return nil }

func syncDir(dir string) error {
	fd, err := unix.Open(dir, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return &os.PathError{Op: "open", Path: dir, Err: err}
	}
	err = os.NewSyscallError("fsync", unix.Fsync(fd))
	if closeErr := unix.Close(fd); err == nil && closeErr != nil {
		err = os.NewSyscallError("close", closeErr)
	}
	return err
}

func replace(path string, own Ownership, after afterRename, write func(tempFile) error) error {
	tmp, err := writeTemp(path, own, write)
	if err != nil {
		return err
	}
	if err := unix.Rename(tmp, path); err != nil {
		_ = unix.Unlink(tmp)
		return fmt.Errorf("atomicfile: replace %s: %w", path, os.NewSyscallError("rename", err))
	}
	if err := after(filepath.Dir(path)); err != nil {
		return fmt.Errorf("atomicfile: sync the directory of %s: %w", path, err)
	}
	return nil
}

func writeTemp(path string, own Ownership, write func(tempFile) error) (string, error) {
	f, tmp, err := createTemp(path)
	if err != nil {
		return "", fmt.Errorf("atomicfile: create a temporary beside %s: %w", path, err)
	}
	err = write(f)
	if err == nil {
		err = own.apply(f.fd)
	}
	if err == nil {
		err = os.NewSyscallError("fsync", unix.Fsync(f.fd))
	}
	if closeErr := unix.Close(f.fd); err == nil && closeErr != nil {
		err = os.NewSyscallError("close", closeErr)
	}
	if err != nil {
		_ = unix.Unlink(tmp)
		return "", fmt.Errorf("atomicfile: write %s: %w", path, err)
	}
	return tmp, nil
}

// IsTemporary reports whether candidate names a temporary a write of
// path uses, as a watcher of path's directory sees it come and go.
func IsTemporary(path, candidate string) bool {
	suffix, ok := strings.CutPrefix(candidate, string(appendTempPrefix(nil, path)))
	if !ok || suffix == "" {
		return false
	}
	_, err := strconv.ParseUint(suffix, 10, 32)
	return err == nil
}

func appendTempPrefix(buf []byte, path string) []byte {
	dir, base := filepath.Split(path)
	buf = append(buf, dir...)
	buf = append(buf, '.')
	buf = append(buf, base...)
	return append(buf, ".tmp"...)
}

func createTemp(path string) (tempFile, string, error) {
	buf := appendTempPrefix(make([]byte, 0, len(path)+16), path)
	prefixLen := len(buf)
	for range tempAttempts {
		buf = strconv.AppendUint(buf[:prefixLen], uint64(rand.Uint32()), 10) //nolint:gosec // a name O_EXCL retries on collision, not a secret
		name := string(buf)
		fd, err := unix.Open(name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC, 0o600)
		if errors.Is(err, unix.EEXIST) {
			continue
		}
		if err != nil {
			return tempFile{}, "", &os.PathError{Op: "open", Path: name, Err: err}
		}
		return tempFile{fd: fd}, name, nil
	}
	return tempFile{}, "", fmt.Errorf("every one of %d temporary names beside %s is taken", tempAttempts, path)
}

type tempFile struct{ fd int }

func (f tempFile) Write(p []byte) (int, error) {
	written := 0
	for written < len(p) {
		n, err := unix.Write(f.fd, p[written:])
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return written, os.NewSyscallError("write", err)
		}
		written += n
	}
	return written, nil
}

func (f tempFile) Seek(offset int64, whence int) (int64, error) {
	pos, err := unix.Seek(f.fd, offset, whence)
	if err != nil {
		return 0, os.NewSyscallError("lseek", err)
	}
	return pos, nil
}
