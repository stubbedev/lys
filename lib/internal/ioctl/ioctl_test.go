package ioctl_test

import (
	"errors"
	"testing"
	"unsafe"

	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/lib/internal/ioctl"
)

func pipe(t *testing.T) (r, w int) {
	t.Helper()
	var p [2]int
	if err := unix.Pipe2(p[:], unix.O_CLOEXEC); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = unix.Close(p[0]), unix.Close(p[1]) })
	return p[0], p[1]
}

func TestRetryFillsTheArgumentOfAnIoctlThatSucceeds(t *testing.T) {
	r, w := pipe(t)
	if _, err := unix.Write(w, []byte("lys")); err != nil {
		t.Fatal(err)
	}
	var queued int32
	if err := ioctl.Retry(r, unix.TIOCINQ, unsafe.Pointer(&queued)); err != nil {
		t.Fatal(err)
	}
	if queued != 3 {
		t.Fatalf("TIOCINQ read %d queued bytes, want 3", queued)
	}
}

func TestRetryReturnsTheErrnoOfAnIoctlThatFails(t *testing.T) {
	r, _ := pipe(t)
	var size uint64
	err := ioctl.Retry(r, unix.BLKGETSIZE64, unsafe.Pointer(&size))
	var errno unix.Errno
	if !errors.As(err, &errno) || errno != unix.ENOTTY {
		t.Fatalf("a block ioctl on a pipe gave %v, want ENOTTY", err)
	}
}
