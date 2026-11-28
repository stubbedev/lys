// Package memfd hands bytes to another process in a sealed memfd.
package memfd

import (
	"errors"
	"fmt"

	"golang.org/x/sys/unix"
)

const seals = unix.F_SEAL_SHRINK | unix.F_SEAL_GROW | unix.F_SEAL_WRITE | unix.F_SEAL_SEAL

// Sealed returns a close-on-exec memfd called name that holds exactly
// data, read from its start and sealed so that no one, its creator
// included, can change, grow or shrink it again. The caller owns the
// descriptor.
func Sealed(name string, data []byte) (int, error) {
	fd, err := unix.MemfdCreate(name, unix.MFD_CLOEXEC|unix.MFD_ALLOW_SEALING)
	if err != nil {
		return -1, fmt.Errorf("memfd: create %s: %w", name, err)
	}
	if err := fill(fd, data); err != nil {
		return -1, errors.Join(fmt.Errorf("memfd: fill %s with %d bytes: %w", name, len(data), err), unix.Close(fd))
	}
	return fd, nil
}

func fill(fd int, data []byte) error {
	for rest := data; len(rest) > 0; {
		n, err := unix.Write(fd, rest)
		switch {
		case errors.Is(err, unix.EINTR):
			continue
		case err != nil:
			return err
		}
		rest = rest[n:]
	}
	if _, err := unix.Seek(fd, 0, 0); err != nil {
		return err
	}
	_, err := unix.FcntlInt(uintptr(fd), unix.F_ADD_SEALS, seals)
	return err
}
