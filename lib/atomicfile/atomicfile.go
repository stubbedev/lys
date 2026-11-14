// Package atomicfile replaces files whole: the content goes to a uniquely
// named temporary file beside the target, which is synced and renamed
// over it. A reader sees the old file or the new one and never a partial
// write, a crash leaves one of the two complete, and concurrent writers
// never share a temporary.
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

const tempAttempts = 10000

// Write replaces path with what write produces, with permissions perm.
// On a failure path is untouched and the temporary is removed.
func Write(path string, perm os.FileMode, write func(io.Writer) error) error {
	return replace(path, perm, func(f tempFile) error {
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
	return replace(path, perm, func(f tempFile) error { return write(f) })
}

// WriteFile replaces path with data, as os.WriteFile but atomically.
func WriteFile(path string, data []byte, perm os.FileMode) error {
	return replace(path, perm, func(f tempFile) error {
		_, err := f.Write(data)
		return err
	})
}

func replace(path string, perm os.FileMode, write func(tempFile) error) error {
	tmp, err := writeTemp(path, perm, write)
	if err != nil {
		return err
	}
	if err := unix.Rename(tmp, path); err != nil {
		_ = unix.Unlink(tmp)
		return fmt.Errorf("atomicfile: replace %s: %w", path, os.NewSyscallError("rename", err))
	}
	return nil
}

func writeTemp(path string, perm os.FileMode, write func(tempFile) error) (string, error) {
	f, tmp, err := createTemp(path)
	if err != nil {
		return "", fmt.Errorf("atomicfile: create a temporary beside %s: %w", path, err)
	}
	err = write(f)
	if err == nil {
		err = os.NewSyscallError("fchmod", unix.Fchmod(f.fd, uint32(perm.Perm())))
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
