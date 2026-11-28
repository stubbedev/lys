package udmabuf

import (
	"testing"

	"golang.org/x/sys/unix"
)

func TestTheDmabufIsCloseOnExec(t *testing.T) {
	b, err := New(4096)
	if err != nil {
		t.Skipf("no udmabuf: %v", err)
	}
	t.Cleanup(func() { _ = b.Close() })
	flags, err := unix.FcntlInt(uintptr(b.FD), unix.F_GETFD, 0)
	if err != nil {
		t.Fatal(err)
	}
	if flags&unix.FD_CLOEXEC == 0 {
		t.Fatalf("the dmabuf's descriptor flags are %#x, without FD_CLOEXEC", flags)
	}
}
