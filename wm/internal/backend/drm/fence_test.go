package drm

import (
	"errors"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/stubbedev/lys/wm/internal/backend"
)

func closedDescriptor(t *testing.T) backend.Fence {
	t.Helper()
	fd, err := unix.Eventfd(0, unix.EFD_CLOEXEC)
	if err != nil {
		t.Fatal(err)
	}
	high, err := unix.FcntlInt(uintptr(fd), unix.F_DUPFD_CLOEXEC, 4096)
	if err != nil {
		t.Fatal(err)
	}
	if err := errors.Join(unix.Close(fd), unix.Close(high)); err != nil {
		t.Fatal(err)
	}
	return backend.Fence(high)
}

func badDescriptors(err error) int {
	var joined interface{ Unwrap() []error }
	if errors.As(err, &joined) {
		n := 0
		for _, inner := range joined.Unwrap() {
			n += badDescriptors(inner)
		}
		return n
	}
	if errors.Is(err, unix.EBADF) {
		return 1
	}
	return 0
}

func TestAFrameWithoutAFenceClosesNoDescriptorWhenTheOldFenceFailsToClose(t *testing.T) {
	r := start(t)
	o := r.lit()
	if err := o.renderFence.Drop(); err != nil {
		t.Fatal(err)
	}
	o.renderFence = closedDescriptor(t)
	for _, s := range o.buffers {
		s.fence = func() (int, error) { return int(backend.NoFence), nil }
	}
	if err := o.ScheduleFrame(); err != nil {
		t.Fatal(err)
	}
	err := r.runUntil("the frame", func() bool { return len(r.reports) > 0 })
	err = errors.Join(append([]error{err}, r.reports...)...)
	if n := badDescriptors(err); n != 1 {
		t.Fatalf("the frame failed with %d bad descriptors, want only the old fence's: %v", n, err)
	}
}
