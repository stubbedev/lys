package backend

import (
	"testing"

	"github.com/stubbedev/lys/lib/fdtest"
	"golang.org/x/sys/unix"
)

func TestNoFenceClosesNothing(t *testing.T) {
	f := NoFence
	if f.Valid() {
		t.Fatal("no fence counts as a fence")
	}
	if err := f.Drop(); err != nil {
		t.Fatalf("dropping no fence: %v", err)
	}
	if err := NoFence.Close(); err != nil {
		t.Fatalf("closing no fence: %v", err)
	}
}

func TestDroppingAFenceClosesItOnce(t *testing.T) {
	fd, err := unix.Eventfd(0, unix.EFD_CLOEXEC)
	if err != nil {
		t.Fatal(err)
	}
	watch := fdtest.WatchFD(t, fd)
	f := Fence(fd)
	if !f.Valid() {
		t.Fatal("an open fence counts as no fence")
	}
	if err := f.Drop(); err != nil {
		t.Fatal(err)
	}
	if watch.Open(t) || f != NoFence {
		t.Fatalf("a dropped fence left %d, open %v", f, watch.Open(t))
	}
	if err := f.Drop(); err != nil {
		t.Fatalf("a second drop closed the fence again: %v", err)
	}
}

func TestTakingAFenceLeavesNone(t *testing.T) {
	f := Fence(7)
	if taken := f.Take(); taken != 7 || f != NoFence {
		t.Fatalf("took %d and left %d", taken, f)
	}
}
