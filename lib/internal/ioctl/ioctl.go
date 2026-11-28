// Package ioctl issues ioctls that the kernel may interrupt.
package ioctl

import (
	"unsafe"

	"golang.org/x/sys/unix"
)

// Retry issues the ioctl req on fd with arg, issuing it again for as
// long as the kernel answers EINTR or EAGAIN. Any other failure is
// returned as its unix.Errno.
func Retry(fd int, req uintptr, arg unsafe.Pointer) error {
	for {
		_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), req, uintptr(arg))
		switch errno {
		case 0:
			return nil
		case unix.EINTR, unix.EAGAIN:
			continue
		}
		return errno
	}
}
