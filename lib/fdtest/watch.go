package fdtest

import (
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

const kcmpFile = 0

// Watch follows the file a descriptor number names, so that a test can
// tell a descriptor that was closed from a number another thread reused
// for another file.
type Watch struct {
	fd   int
	file int
}

// WatchFD starts following the file fd names, keeping a duplicate of it
// until the test ends. It fails the test when fd is not open.
func WatchFD(t testing.TB, fd int) Watch {
	t.Helper()
	file, err := unix.FcntlInt(uintptr(fd), unix.F_DUPFD_CLOEXEC, 0)
	if err != nil {
		t.Fatalf("watch descriptor %d: %v", fd, err)
	}
	t.Cleanup(func() { _ = unix.Close(file) })
	return Watch{fd: fd, file: file}
}

// FD is the descriptor number w follows.
func (w Watch) FD() int { return w.fd }

// Open reports whether w's number still names the file it named when
// the watch began: false once the descriptor was closed, whether the
// number is free or names another file.
func (w Watch) Open(t testing.TB) bool {
	t.Helper()
	pid := uintptr(os.Getpid())
	order, _, errno := unix.Syscall6(unix.SYS_KCMP, pid, pid, kcmpFile, uintptr(w.fd), uintptr(w.file), 0)
	switch errno {
	case 0:
		return order == 0
	case unix.EBADF:
		return false
	default:
		t.Fatalf("compare descriptor %d with the file it named: %v", w.fd, errno)
		return false
	}
}
