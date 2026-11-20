package fdtest

import (
	"errors"
	"os"
	"slices"
	"strconv"
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
	pid := os.Getpid()
	same, err := sameFile(pid, w.fd, pid, w.file)
	switch {
	case errors.Is(err, unix.EBADF):
		return false
	case err != nil:
		t.Fatalf("compare descriptor %d with the file it named: %v", w.fd, err)
	}
	return same
}

// Holders lists, in order, every descriptor of the process that names
// the file w follows, its own number included while that still does,
// apart from the duplicate the watch keeps.
func (w Watch) Holders(t testing.TB) []int {
	t.Helper()
	return Holders(t, w.file)
}

// Holders lists, in order, the descriptors of the process other than fd
// that name the file fd names: the copies a component was handed or
// took of it. It fails the test when fd is not open.
func Holders(t testing.TB, fd int) []int {
	t.Helper()
	pid := os.Getpid()
	if _, err := sameFile(pid, fd, pid, fd); err != nil {
		t.Fatalf("list the holders of descriptor %d: %v", fd, err)
	}
	return slices.DeleteFunc(naming(t, pid, fd, pid, descriptorsOf(t, pid)), func(holder int) bool { return holder == fd })
}

// Shared lists, in order, the pairs of a descriptor of process a and a
// descriptor of process b that name one open file, as a descriptor and
// the copies dup, fork or a socket made of it do. With a and b the
// same process, every descriptor pairs with itself too.
func Shared(t testing.TB, a, b int) [][2]int {
	t.Helper()
	ofB := descriptorsOf(t, b)
	var shared [][2]int
	for _, fdA := range descriptorsOf(t, a) {
		for _, fdB := range naming(t, a, fdA, b, ofB) {
			shared = append(shared, [2]int{fdA, fdB})
		}
	}
	return shared
}

func naming(t testing.TB, pid, fd, in int, candidates []int) []int {
	t.Helper()
	var found []int
	for _, candidate := range candidates {
		same, err := sameFile(pid, fd, in, candidate)
		if errors.Is(err, unix.EBADF) {
			continue
		}
		if err != nil {
			t.Fatalf("compare descriptor %d of process %d with descriptor %d of process %d: %v", fd, pid, candidate, in, err)
		}
		if same {
			found = append(found, candidate)
		}
	}
	return found
}

func descriptorsOf(t testing.TB, pid int) []int {
	t.Helper()
	dir := "/proc/" + strconv.Itoa(pid) + "/fd"
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("list the descriptors of process %d: %v", pid, err)
	}
	fds := make([]int, 0, len(entries))
	for _, entry := range entries {
		fd, err := strconv.Atoi(entry.Name())
		if err != nil {
			t.Fatalf("descriptor %q in %s: %v", entry.Name(), dir, err)
		}
		fds = append(fds, fd)
	}
	slices.Sort(fds)
	return fds
}

func sameFile(pidA, fdA, pidB, fdB int) (bool, error) {
	order, _, errno := unix.Syscall6(unix.SYS_KCMP, uintptr(pidA), uintptr(pidB), kcmpFile, uintptr(fdA), uintptr(fdB), 0)
	if errno != 0 {
		return false, errno
	}
	return order == 0, nil
}
